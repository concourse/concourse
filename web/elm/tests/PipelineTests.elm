module PipelineTests exposing (all)

import Application.Application as Application
import Char
import Common
import DashboardTests exposing (defineHoverBehaviour)
import Expect exposing (..)
import Html.Attributes as Attr
import Json.Encode
import Keyboard
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message exposing (Message(..))
import Message.Subscription as Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as Msgs
import Pipeline.Pipeline as Pipeline exposing (update)
import Routes
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector as Selector
    exposing
        ( attribute
        , class
        , containing
        , id
        , style
        , tag
        , text
        )
import Time
import Url


rspecStyleDescribe : String -> model -> List (model -> Test) -> Test
rspecStyleDescribe description beforeEach subTests =
    Test.describe description
        (subTests |> List.map (\f -> f beforeEach))


it : String -> (model -> Expectation) -> model -> Test
it desc expectationFunc model =
    Test.test desc <|
        \_ -> expectationFunc model


flags : Application.Flags
flags =
    { turbulenceImgSrc = ""
    , notFoundImgSrc = ""
    , csrfToken = csrfToken
    , authToken = ""
    , instanceName = ""
    , pipelineRunningKeyframes = ""
    }


all : Test
all =
    describe "Pipeline"
        [ describe "groups" <|
            let
                sampleGroups =
                    [ { name = "group"
                      , jobs = []
                      , resources = []
                      }
                    , { name = "other-group"
                      , jobs = []
                      , resources = []
                      }
                    ]

                setupGroupsBar groups =
                    Application.init
                        flags
                        { protocol = Url.Http
                        , host = ""
                        , port_ = Nothing
                        , path = "/teams/team/pipelines/pipeline"
                        , query = Just "group=other-group"
                        , fragment = Nothing
                        }
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.PipelineFetched
                                (Ok
                                    { id = 0
                                    , name = "pipeline"
                                    , paused = False
                                    , public = True
                                    , teamName = "team"
                                    , groups = groups
                                    }
                                )
                            )
                        |> Tuple.first
            in
            [ describe "groups bar styling"
                [ describe "with groups"
                    [ test "has light text on a dark background" <|
                        \_ ->
                            setupGroupsBar sampleGroups
                                |> Common.queryView
                                |> Query.find [ id "groups-bar" ]
                                |> Query.has
                                    [ style "background-color" "#2b2a2a"
                                    , style "color" "#ffffff"
                                    ]
                    , test "lays out groups in a horizontal list" <|
                        \_ ->
                            setupGroupsBar sampleGroups
                                |> Common.queryView
                                |> Query.find [ id "groups-bar" ]
                                |> Query.has
                                    [ style "display" "flex"
                                    , style "flex-flow" "row wrap"
                                    , style "padding" "5px"
                                    ]
                    , describe "each group" <|
                        let
                            findGroups =
                                Common.queryView
                                    >> Query.find [ id "groups-bar" ]
                                    >> Query.children []
                        in
                        [ test "the individual groups are nicely spaced" <|
                            \_ ->
                                setupGroupsBar sampleGroups
                                    |> findGroups
                                    |> Query.each
                                        (Query.has
                                            [ style "margin" "5px"
                                            , style "padding" "10px"
                                            ]
                                        )
                        , test "the individual groups have large text" <|
                            \_ ->
                                setupGroupsBar sampleGroups
                                    |> findGroups
                                    |> Query.each
                                        (Query.has [ style "font-size" "14px" ])
                        , describe "the individual groups should each have a box around them"
                            [ test "the unselected ones faded" <|
                                \_ ->
                                    setupGroupsBar sampleGroups
                                        |> findGroups
                                        |> Query.index 0
                                        |> Query.has
                                            [ style "opacity" "0.6"
                                            , style "background"
                                                "rgba(151, 151, 151, 0.1)"
                                            , style "border" "1px solid #2b2a2a"
                                            ]
                            , defineHoverBehaviour
                                { name = "group"
                                , setup = setupGroupsBar sampleGroups
                                , query = findGroups >> Query.index 0
                                , updateFunc =
                                    \msg ->
                                        Application.update msg
                                            >> Tuple.first
                                , unhoveredSelector =
                                    { description = "dark outline"
                                    , selector =
                                        [ style "border" "1px solid #2b2a2a" ]
                                    }
                                , mouseEnterMsg =
                                    Msgs.Update <|
                                        Hover <|
                                            Just <|
                                                Message.Message.JobGroup 0
                                , mouseLeaveMsg = Msgs.Update <| Hover Nothing
                                , hoveredSelector =
                                    { description = "light grey outline"
                                    , selector =
                                        [ style "border" "1px solid #fff2" ]
                                    }
                                }
                            , test "the selected ones brighter" <|
                                \_ ->
                                    setupGroupsBar sampleGroups
                                        |> findGroups
                                        |> Query.index 1
                                        |> Query.has
                                            [ style "opacity" "1"
                                            , style "background" "rgba(151, 151, 151, 0.1)"
                                            , style "border" "1px solid #979797"
                                            ]
                            ]
                        , test "each group should have a name and link" <|
                            \_ ->
                                setupGroupsBar sampleGroups
                                    |> findGroups
                                    |> Expect.all
                                        [ Query.index 0
                                            >> Query.has
                                                [ text "group"
                                                , attribute <|
                                                    Attr.href
                                                        "/teams/team/pipelines/pipeline?group=group"
                                                , tag "a"
                                                ]
                                        , Query.index 1
                                            >> Query.has
                                                [ text "other-group"
                                                , attribute <|
                                                    Attr.href
                                                        "/teams/team/pipelines/pipeline?group=other-group"
                                                , tag "a"
                                                ]
                                        ]
                        ]
                    ]
                , test "with no groups does not display groups list" <|
                    \_ ->
                        setupGroupsBar []
                            |> Common.queryView
                            |> Query.findAll [ id "groups-bar" ]
                            |> Query.count (Expect.equal 0)
                , test "KeyPressed" <|
                    \_ ->
                        setupGroupsBar []
                            |> Application.update
                                (Msgs.DeliveryReceived <|
                                    KeyDown <|
                                        { ctrlKey = False
                                        , shiftKey = False
                                        , metaKey = False
                                        , code = Keyboard.A
                                        }
                                )
                            |> Tuple.second
                            |> Expect.equal []
                , test "KeyPressed f" <|
                    \_ ->
                        setupGroupsBar []
                            |> Application.update
                                (Msgs.DeliveryReceived <|
                                    KeyDown <|
                                        { ctrlKey = False
                                        , shiftKey = False
                                        , metaKey = False
                                        , code = Keyboard.F
                                        }
                                )
                            |> Tuple.second
                            |> Expect.equal [ Effects.ResetPipelineFocus ]
                , test "KeyPressed F" <|
                    \_ ->
                        setupGroupsBar []
                            |> Application.update
                                (Msgs.DeliveryReceived <|
                                    KeyDown <|
                                        { ctrlKey = False
                                        , shiftKey = True
                                        , metaKey = False
                                        , code = Keyboard.F
                                        }
                                )
                            |> Tuple.second
                            |> Expect.equal [ Effects.ResetPipelineFocus ]
                ]
            ]
        , test "title should include the pipline name" <|
            \_ ->
                Common.init "/teams/team/pipelines/pipelineName"
                    |> Application.view
                    |> .title
                    |> Expect.equal "pipelineName - Concourse"
        , describe "update" <|
            let
                defaultModel : Pipeline.Model
                defaultModel =
                    Pipeline.init
                        { pipelineLocator =
                            { teamName = "some-team"
                            , pipelineName = "some-pipeline"
                            }
                        , turbulenceImgSrc = "some-turbulence-img-src"
                        , selectedGroups = []
                        }
                        |> Tuple.first
            in
            [ test "CLI icons at bottom right" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Common.queryView
                        |> Query.find [ class "cli-downloads" ]
                        |> Query.children []
                        |> Expect.all
                            [ Query.index 0
                                >> Query.has
                                    [ style "background-image" "url(/public/images/apple-logo.svg)"
                                    , style "background-position" "50% 50%"
                                    , style "background-repeat" "no-repeat"
                                    , style "width" "12px"
                                    , style "height" "12px"
                                    , style "display" "inline-block"
                                    , attribute <| Attr.download ""
                                    ]
                            , Query.index 1
                                >> Query.has
                                    [ style "background-image" "url(/public/images/windows-logo.svg)"
                                    , style "background-position" "50% 50%"
                                    , style "background-repeat" "no-repeat"
                                    , style "width" "12px"
                                    , style "height" "12px"
                                    , style "display" "inline-block"
                                    , attribute <| Attr.download ""
                                    ]
                            , Query.index 2
                                >> Query.has
                                    [ style "background-image" "url(/public/images/linux-logo.svg)"
                                    , style "background-position" "50% 50%"
                                    , style "background-repeat" "no-repeat"
                                    , style "width" "12px"
                                    , style "height" "12px"
                                    , style "display" "inline-block"
                                    , attribute <| Attr.download ""
                                    ]
                            ]
            , test "pipeline subscribes to 1s, 5s, and 1m timers" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Application.subscriptions
                        |> Expect.all
                            [ List.member (Subscription.OnClockTick OneSecond) >> Expect.true "not on one second?"
                            , List.member (Subscription.OnClockTick FiveSeconds) >> Expect.true "not on five seconds?"
                            , List.member (Subscription.OnClockTick OneMinute) >> Expect.true "not on one minute?"
                            ]
            , test "on five second timer, refreshes pipeline" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Application.update
                            (Msgs.DeliveryReceived
                                (ClockTicked FiveSeconds <|
                                    Time.millisToPosix 0
                                )
                            )
                        |> Tuple.second
                        |> Expect.equal
                            [ Effects.FetchPipeline
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                }
                            ]
            , test "on one minute timer, refreshes version" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Application.update
                            (Msgs.DeliveryReceived
                                (ClockTicked OneMinute <|
                                    Time.millisToPosix 0
                                )
                            )
                        |> Tuple.second
                        |> Expect.equal [ Effects.FetchVersion ]
            , describe "Legend" <|
                let
                    clockTick =
                        Application.update
                            (Msgs.DeliveryReceived
                                (ClockTicked OneSecond <|
                                    Time.millisToPosix 0
                                )
                            )
                            >> Tuple.first

                    clockTickALot n =
                        List.foldr (>>) identity (List.repeat n clockTick)
                in
                [ test "Legend has definition for pinned resource color" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Expect.all
                                [ Query.count (Expect.equal 20)
                                , Query.index 1 >> Query.has [ text "succeeded" ]
                                , Query.index 3 >> Query.has [ text "errored" ]
                                , Query.index 5 >> Query.has [ text "aborted" ]
                                , Query.index 7 >> Query.has [ text "paused" ]
                                , Query.index 8 >> Query.has [ style "background-color" "#5c3bd1" ]
                                , Query.index 9 >> Query.has [ text "pinned" ]
                                , Query.index 11 >> Query.has [ text "failed" ]
                                , Query.index 13 >> Query.has [ text "pending" ]
                                , Query.index 15 >> Query.has [ text "started" ]
                                , Query.index 17 >> Query.has [ text "dependency" ]
                                , Query.index 19 >> Query.has [ text "dependency (trigger)" ]
                                ]
                , test "HideLegendTimerTicked" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> clockTick
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.count (Expect.equal 20)
                , test "HideLegendTimeTicked reaches timeout" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> clockTickALot 11
                            |> Common.queryView
                            |> Query.hasNot [ id "legend" ]
                , test "Mouse action after legend hidden reshows legend" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> clockTickALot 11
                            |> Application.update (Msgs.DeliveryReceived Moused)
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.has [ id "legend" ]
                ]
            , rspecStyleDescribe "when on pipeline page"
                (Common.init "/teams/team/pipelines/pipeline")
                [ it "shows a pin icon on top bar" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ id "pin-icon" ]
                , it "top bar has a dark grey background" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style "background-color" "#1e1d1d" ]
                , it "top bar lays out contents horizontally" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style "display" "flex" ]
                , it "top bar maximizes spacing between the left and right navs" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style "justify-content" "space-between" ]
                , it "top bar has a square concourse logo on the left" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.children []
                        >> Query.index 0
                        >> Query.has
                            [ style "background-image"
                                "url(/public/images/concourse-logo-white.svg)"
                            , style "background-position" "50% 50%"
                            , style "background-repeat" "no-repeat"
                            , style "background-size" "42px 42px"
                            , style "width" "54px"
                            , style "height" "54px"
                            ]
                , it "concourse logo on the left is a link to homepage" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.children []
                        >> Query.index 0
                        >> Query.has [ tag "a", attribute <| Attr.href "/" ]
                , it "pin icon has a pin background" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style "background-image" "url(/public/images/pin-ic-white.svg)" ]
                , it "mousing over pin icon does nothing if there are no pinned resources" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children []
                        >> Query.first
                        >> Event.simulate Event.mouseEnter
                        >> Event.toResult
                        >> Expect.err
                , it "there is some space between the pin icon and the user menu" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style "margin-right" "15px" ]
                , it "pin icon has relative positioning" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style "position" "relative" ]
                , it "pin icon does not have circular background" <|
                    Common.queryView
                        >> Query.findAll
                            [ id "pin-icon"
                            , style "border-radius" "50%"
                            ]
                        >> Query.count (Expect.equal 0)
                , it "pin icon has white color when pipeline has pinned resources" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style "background-image" "url(/public/images/pin-ic-white.svg)" ]
                , it "pin icon has pin badge when pipeline has pinned resources" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has pinBadgeSelector
                , it "pin badge is purple" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style "background-color" "#5c3bd1" ]
                , it "pin badge is circular" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style "border-radius" "50%"
                            , style "width" "15px"
                            , style "height" "15px"
                            ]
                , it "pin badge is near the top right of the pin icon" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style "position" "absolute"
                            , style "top" "3px"
                            , style "right" "3px"
                            ]
                , it "content inside pin badge is centered horizontally and vertically" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style "display" "flex"
                            , style "align-items" "center"
                            , style "justify-content" "center"
                            ]
                , it "pin badge shows count of pinned resources, centered" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.findAll [ tag "div", containing [ text "1" ] ]
                        >> Query.count (Expect.equal 1)
                , it "pin badge has no other children" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.children []
                        >> Query.count (Expect.equal 1)
                , it "pin counter works with multiple pinned resources" <|
                    givenMultiplePinnedResources
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.findAll [ tag "div", containing [ text "2" ] ]
                        >> Query.count (Expect.equal 1)
                , it "before Hover msg no list of pinned resources is visible" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.hasNot [ tag "ul" ]
                , it "mousing over pin icon sends Hover msg" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children []
                        >> Query.first
                        >> Event.simulate Event.mouseEnter
                        >> Event.expect (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                , it "Hover msg causes pin icon to have light grey circular background" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has
                            [ style "background-color" "rgba(255, 255, 255, 0.3)"
                            , style "border-radius" "50%"
                            ]
                , it "Hover msg causes dropdown list of pinned resources to appear" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children [ tag "ul" ]
                        >> Query.count (Expect.equal 1)
                , it "on Hover, pin badge has no other children" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.children []
                        >> Query.count (Expect.equal 1)
                , it "dropdown list of pinned resources contains resource name" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ tag "li", containing [ text "resource" ] ]
                , it "dropdown list of pinned resources shows resource names in bold" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.find [ tag "li", containing [ text "resource" ] ]
                        >> Query.findAll [ tag "div", containing [ text "resource" ], style "font-weight" "700" ]
                        >> Query.count (Expect.equal 1)
                , it "dropdown list of pinned resources shows pinned version of each resource" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.find [ tag "li", containing [ text "resource" ] ]
                        >> Query.has [ tag "table", containing [ text "v1" ] ]
                , it "dropdown list of pinned resources has white background" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style "background-color" "#ffffff" ]
                , it "dropdown list of pinned resources is drawn over other elements on the page" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style "z-index" "1" ]
                , it "dropdown list of pinned resources has dark grey text" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style "color" "#1e1d1d" ]
                , it "dropdown list has upward-pointing arrow" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children
                            [ style "border-width" "5px"
                            , style "border-style" "solid"
                            , style "border-color" "transparent transparent #ffffff transparent"
                            ]
                        >> Query.count (Expect.equal 1)
                , it "dropdown list of pinned resources is offset below and left of the pin icon" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has
                            [ style "position" "absolute"
                            , style "top" "100%"
                            , style "right" "0"
                            , style "margin-top" "0"
                            ]
                , it "dropdown list of pinned resources stretches horizontally to fit content" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style "white-space" "nowrap" ]
                , it "dropdown list of pinned resources has no bullet points" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style "list-style-type" "none" ]
                , it "dropdown list has comfortable padding" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style "padding" "10px" ]
                , it "dropdown list arrow is centered below the pin icon above the list" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children
                            [ style "border-width" "5px"
                            , style "border-style" "solid"
                            , style "border-color"
                                "transparent transparent #ffffff transparent"
                            ]
                        >> Query.first
                        >> Query.has
                            [ style "top" "100%"
                            , style "right" "50%"
                            , style "margin-right" "-5px"
                            , style "margin-top" "-10px"
                            , style "position" "absolute"
                            ]
                , it "mousing off the pin icon sends Hover Nothing msg" <|
                    givenPinnedResource
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children []
                        >> Query.first
                        >> Event.simulate Event.mouseLeave
                        >> Event.expect (Msgs.Update <| Message.Message.Hover Nothing)
                , it "clicking a pinned resource sends a Navigation TopLevelMessage" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "li" ]
                        >> Event.simulate Event.click
                        >> Event.expect
                            (Msgs.Update <|
                                Message.Message.GoToRoute <|
                                    Routes.Resource
                                        { id =
                                            { teamName = "team"
                                            , pipelineName = "pipeline"
                                            , resourceName = "resource"
                                            }
                                        , page = Nothing
                                        }
                            )
                , it "Hover msg causes dropdown list of pinned resources to disappear" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Application.update (Msgs.Update <| Message.Message.Hover Nothing)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.hasNot [ tag "ul" ]
                , it "pinned resources in the dropdown should have a pointer cursor" <|
                    givenPinnedResource
                        >> Application.update (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Expect.all
                            [ Query.findAll [ tag "li" ]
                                >> Query.each (Query.has [ style "cursor" "pointer" ])
                            , Query.findAll [ style "cursor" "pointer" ]
                                >> Query.each (Query.has [ tag "li" ])
                            ]
                ]
            , test "top bar lays out contents horizontally" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style "display" "inline-block" ]
            , test "top bar maximizes spacing between the left and right navs" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style "justify-content" "space-between"
                            , style "width" "100%"
                            ]
            , test "top bar is sticky" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style "z-index" "999"
                            , style "position" "fixed"
                            ]
            , test "breadcrumb items are laid out horizontally" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumbs" ]
                        |> Query.children []
                        |> Query.each
                            (Query.has [ style "display" "inline-block" ])
            , describe "top bar positioning"
                [ testTopBarPositioning "Dashboard" "/"
                , testTopBarPositioning "Pipeline" "/teams/team/pipelines/pipeline"
                , testTopBarPositioning "Job" "/teams/team/pipelines/pipeline/jobs/job"
                , testTopBarPositioning "Build" "/teams/team/pipelines/pipeline/jobs/job/builds/build"
                , testTopBarPositioning "Resource" "/teams/team/pipelines/pipeline/resources/resource"
                , testTopBarPositioning "FlySuccess" "/fly_success"
                ]
            , rspecStyleDescribe "when on job page"
                (Common.init "/teams/team/pipeline/pipeline/jobs/job/builds/1")
                [ it "shows no pin icon on top bar when viewing build page" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.hasNot [ id "pin-icon" ]
                ]
            , test "top nav bar is blue when pipeline is paused" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Application.handleCallback
                            (Callback.PipelineFetched
                                (Ok
                                    { id = 0
                                    , name = "pipeline"
                                    , paused = True
                                    , public = True
                                    , teamName = "team"
                                    , groups = []
                                    }
                                )
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style "background-color" "#3498db" ]
            , test "breadcrumb list is laid out horizontally" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumbs" ]
                        |> Query.has
                            [ style "display" "inline-block"
                            , style "padding" "0 10px"
                            ]
            , test "pipeline breadcrumb is laid out horizontally" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.has [ style "display" "inline-block" ]
            , test "top bar has pipeline breadcrumb with icon rendered first" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has pipelineBreadcrumbSelector
            , test "top bar has pipeline name after pipeline icon" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.has [ text "pipeline" ]
            ]
        ]


pinBadgeSelector : List Selector.Selector
pinBadgeSelector =
    [ id "pin-badge" ]


pipelineBreadcrumbSelector : List Selector.Selector
pipelineBreadcrumbSelector =
    [ style "background-image" "url(/public/images/ic-breadcrumb-pipeline.svg)"
    , style "background-repeat" "no-repeat"
    ]


jobBreadcrumbSelector : List Selector.Selector
jobBreadcrumbSelector =
    [ style "background-image" "url(/public/images/ic-breadcrumb-job.svg)"
    , style "background-repeat" "no-repeat"
    ]


resourceBreadcrumbSelector : List Selector.Selector
resourceBreadcrumbSelector =
    [ style "background-image" "url(/public/images/ic-breadcrumb-resource.svg)"
    , style "background-repeat" "no-repeat"
    ]


csrfToken : String
csrfToken =
    "csrf_token"


givenPinnedResource : Application.Model -> Application.Model
givenPinnedResource =
    Application.handleCallback
        (Callback.ResourcesFetched <|
            Ok <|
                Json.Encode.list identity
                    [ Json.Encode.object
                        [ ( "team_name", Json.Encode.string "team" )
                        , ( "pipeline_name", Json.Encode.string "pipeline" )
                        , ( "name", Json.Encode.string "resource" )
                        , ( "pinned_version", Json.Encode.object [ ( "version", Json.Encode.string "v1" ) ] )
                        ]
                    ]
        )
        >> Tuple.first


givenMultiplePinnedResources : Application.Model -> Application.Model
givenMultiplePinnedResources =
    Application.handleCallback
        (Callback.ResourcesFetched <|
            Ok <|
                Json.Encode.list identity
                    [ Json.Encode.object
                        [ ( "team_name", Json.Encode.string "team" )
                        , ( "pipeline_name", Json.Encode.string "pipeline" )
                        , ( "name", Json.Encode.string "resource" )
                        , ( "pinned_version", Json.Encode.object [ ( "version", Json.Encode.string "v1" ) ] )
                        ]
                    , Json.Encode.object
                        [ ( "team_name", Json.Encode.string "team" )
                        , ( "pipeline_name", Json.Encode.string "pipeline" )
                        , ( "name", Json.Encode.string "other-resource" )
                        , ( "pinned_version", Json.Encode.object [ ( "version", Json.Encode.string "v2" ) ] )
                        ]
                    ]
        )
        >> Tuple.first


testTopBarPositioning : String -> String -> Test
testTopBarPositioning pageName url =
    describe pageName
        [ test "whole page fills the whole screen" <|
            \_ ->
                Common.init url
                    |> Common.queryView
                    |> Query.has
                        [ id "page-including-top-bar"
                        , style "height" "100%"
                        ]
        , test "lower section fills the whole screen as well" <|
            \_ ->
                Common.init url
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has
                        [ style "padding-top" "54px"
                        , style "height" "100%"
                        ]
        ]
