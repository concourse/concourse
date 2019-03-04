module PipelineTests exposing (all)

import Application.Application as Application
import Application.Msgs as Msgs
import Build.Msgs
import Callback
import Char
import Effects
import Expect exposing (..)
import Html.Attributes as Attr
import Json.Encode
import Pipeline.Msgs exposing (Msg(..))
import Pipeline.Pipeline as Pipeline exposing (update)
import Resource.Msgs
import Routes
import SubPage.Msgs
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
import Time exposing (Time)
import TopBar.Msgs


rspecStyleDescribe : String -> model -> List (model -> Test) -> Test
rspecStyleDescribe description beforeEach subTests =
    Test.describe description
        (subTests |> List.map (\f -> f beforeEach))


it : String -> (model -> Expectation) -> model -> Test
it desc expectationFunc model =
    Test.test desc <|
        \_ -> expectationFunc model


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
                        { turbulenceImgSrc = ""
                        , notFoundImgSrc = ""
                        , csrfToken = ""
                        , authToken = ""
                        , pipelineRunningKeyframes = ""
                        }
                        { href = ""
                        , host = ""
                        , hostname = ""
                        , protocol = ""
                        , origin = ""
                        , port_ = ""
                        , pathname = "/teams/team/pipelines/pipeline"
                        , search = "?groups=other-group"
                        , hash = ""
                        , username = ""
                        , password = ""
                        }
                        |> Tuple.first
                        |> Application.handleCallback
                            (Effects.SubPage 1)
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
                        |> Application.view
                        |> Query.fromHtml
            in
            [ describe "groups bar styling"
                [ describe "with groups"
                    [ test "is flush with the bottom of the top bar" <|
                        \_ ->
                            setupGroupsBar sampleGroups
                                |> Query.find [ id "groups-bar" ]
                                |> Query.has [ style [ ( "margin-top", "54px" ) ] ]
                    , test "has light text on a dark background" <|
                        \_ ->
                            setupGroupsBar sampleGroups
                                |> Query.find [ id "groups-bar" ]
                                |> Query.has
                                    [ style
                                        [ ( "background-color", "#2b2a2a" )
                                        , ( "color", "#fff" )
                                        ]
                                    ]
                    , test "lays out groups in a horizontal list" <|
                        \_ ->
                            setupGroupsBar sampleGroups
                                |> Query.find [ id "groups-bar" ]
                                |> Query.has
                                    [ style
                                        [ ( "flex-grow", "1" )
                                        , ( "display", "flex" )
                                        , ( "flex-flow", "row wrap" )
                                        , ( "padding", "5px" )
                                        ]
                                    ]
                    , test "the individual groups are nicely spaced" <|
                        \_ ->
                            setupGroupsBar sampleGroups
                                |> Query.find [ id "groups-bar" ]
                                |> Query.findAll [ tag "li" ]
                                |> Query.each
                                    (Query.has
                                        [ style
                                            [ ( "margin", "5px" )
                                            , ( "padding", "10px" )
                                            ]
                                        ]
                                    )
                    , test "the individual groups have no list style" <|
                        \_ ->
                            setupGroupsBar sampleGroups
                                |> Query.find [ id "groups-bar" ]
                                |> Query.has [ style [ ( "list-style", "none" ) ] ]
                    , test "the individual groups have large text" <|
                        \_ ->
                            setupGroupsBar sampleGroups
                                |> Query.find [ id "groups-bar" ]
                                |> Query.findAll [ tag "li" ]
                                |> Query.each
                                    (Query.has [ style [ ( "font-size", "14px" ) ] ])
                    , describe "the individual groups should each have a box around them"
                        [ test "the unselected ones faded" <|
                            \_ ->
                                setupGroupsBar sampleGroups
                                    |> Query.find [ id "groups-bar" ]
                                    |> Query.findAll [ tag "li" ]
                                    |> Query.index 0
                                    |> Query.has
                                        [ style
                                            [ ( "opacity", "0.6" )
                                            , ( "background", "rgba(151, 151, 151, 0.1)" )
                                            , ( "border", "1px solid #2b2a2a" )
                                            ]
                                        ]
                        , test "the selected ones brighter" <|
                            \_ ->
                                setupGroupsBar sampleGroups
                                    |> Query.find [ id "groups-bar" ]
                                    |> Query.findAll [ tag "li" ]
                                    |> Query.index 1
                                    |> Query.has
                                        [ style
                                            [ ( "opacity", "1" )
                                            , ( "background", "rgba(151, 151, 151, 0.1)" )
                                            , ( "border", "1px solid #979797" )
                                            ]
                                        ]
                        ]
                    , test "the individual groups should each have a group name and link" <|
                        \_ ->
                            setupGroupsBar sampleGroups
                                |> Query.find [ id "groups-bar" ]
                                |> Query.findAll [ tag "li" ]
                                |> Expect.all
                                    [ Query.index 0
                                        >> Query.find [ tag "a" ]
                                        >> Query.has
                                            [ text "group"
                                            , attribute <| Attr.href "/teams/team/pipelines/pipeline?groups=group"
                                            ]
                                    , Query.index 1
                                        >> Query.find [ tag "a" ]
                                        >> Query.has
                                            [ text "other-group"
                                            , attribute <| Attr.href "/teams/team/pipelines/pipeline?groups=other-group"
                                            ]
                                    ]
                    ]
                , test "with no groups doesn not display groups list" <|
                    \_ ->
                        setupGroupsBar []
                            |> Query.findAll [ id "groups-bar" ]
                            |> Query.count (Expect.equal 0)
                ]
            ]
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
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ class "cli-downloads" ]
                        |> Query.children []
                        |> Expect.all
                            [ Query.index 0
                                >> Query.has
                                    [ style
                                        [ ( "background-image", "url(/public/images/apple-logo.svg)" )
                                        , ( "background-position", "50% 50%" )
                                        , ( "background-repeat", "no-repeat" )
                                        , ( "width", "12px" )
                                        , ( "height", "12px" )
                                        , ( "display", "inline-block" )
                                        ]
                                    ]
                            , Query.index 1
                                >> Query.has
                                    [ style
                                        [ ( "background-image", "url(/public/images/windows-logo.svg)" )
                                        , ( "background-position", "50% 50%" )
                                        , ( "background-repeat", "no-repeat" )
                                        , ( "width", "12px" )
                                        , ( "height", "12px" )
                                        , ( "display", "inline-block" )
                                        ]
                                    ]
                            , Query.index 2
                                >> Query.has
                                    [ style
                                        [ ( "background-image", "url(/public/images/linux-logo.svg)" )
                                        , ( "background-position", "50% 50%" )
                                        , ( "background-repeat", "no-repeat" )
                                        , ( "width", "12px" )
                                        , ( "height", "12px" )
                                        , ( "display", "inline-block" )
                                        ]
                                    ]
                            ]
            , test "HideLegendTimerTicked" <|
                \_ ->
                    defaultModel
                        |> update (HideLegendTimerTicked 0)
                        |> Tuple.mapFirst .hideLegendCounter
                        |> Expect.equal ( 1 * Time.second, [] )
            , test "HideLegendTimeTicked reaches timeout" <|
                \_ ->
                    { defaultModel | hideLegendCounter = 10 * Time.second }
                        |> update (HideLegendTimerTicked 0)
                        |> Tuple.mapFirst .hideLegend
                        |> Expect.equal ( True, [] )
            , test "ShowLegend" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ class "legend" ]
                        |> Query.children []
                        |> Expect.all
                            [ Query.count (Expect.equal 20)
                            , Query.index 1 >> Query.has [ text "succeeded" ]
                            , Query.index 3 >> Query.has [ text "errored" ]
                            , Query.index 5 >> Query.has [ text "aborted" ]
                            , Query.index 7 >> Query.has [ text "paused" ]
                            , Query.index 8 >> Query.has [ style [ ( "background-color", "#5C3BD1" ) ] ]
                            , Query.index 9 >> Query.has [ text "pinned" ]
                            , Query.index 11 >> Query.has [ text "failed" ]
                            , Query.index 13 >> Query.has [ text "pending" ]
                            , Query.index 15 >> Query.has [ text "started" ]
                            , Query.index 17 >> Query.has [ text "dependency" ]
                            , Query.index 19 >> Query.has [ text "dependency (trigger)" ]
                            ]
            , test "Legend has definition for pinned resource color" <|
                \_ ->
                    { defaultModel | hideLegend = True, hideLegendCounter = 3 * Time.second }
                        |> update ShowLegend
                        |> Expect.all
                            [ \( m, _ ) -> m.hideLegend |> Expect.equal False
                            , \( m, _ ) -> m.hideLegendCounter |> Expect.equal 0
                            , \( _, e ) -> Expect.equal [] e
                            ]
            , test "KeyPressed" <|
                \_ ->
                    defaultModel
                        |> update (KeyPressed (Char.toCode 'a'))
                        |> Expect.equal ( defaultModel, [] )
            , test "KeyPressed f" <|
                \_ ->
                    defaultModel
                        |> update (KeyPressed (Char.toCode 'f'))
                        |> Expect.notEqual ( defaultModel, [] )
            , rspecStyleDescribe "when on pipeline page"
                (init "/teams/team/pipelines/pipeline")
                [ it "shows a pin icon on top bar" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ id "pin-icon" ]
                , it "top bar has a dark grey background" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style [ ( "background-color", "#1e1d1d" ) ] ]
                , it "top bar lays out contents horizontally" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style [ ( "display", "flex" ) ] ]
                , it "top bar maximizes spacing between the left and right navs" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style [ ( "justify-content", "space-between" ) ] ]
                , it "top bar has a square concourse logo on the left" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.children []
                        >> Query.index 1
                        >> Query.has
                            [ style
                                [ ( "background-image", "url(/public/images/concourse-logo-white.svg)" )
                                , ( "background-position", "50% 50%" )
                                , ( "background-repeat", "no-repeat" )
                                , ( "background-size", "42px 42px" )
                                , ( "width", "54px" )
                                , ( "height", "54px" )
                                ]
                            ]
                , it "concourse logo on the left is a link to homepage" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.children []
                        >> Query.index 1
                        >> Query.has [ tag "a", attribute <| Attr.href "/" ]
                , it "pin icon has a pin background" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style [ ( "background-image", "url(/public/images/pin-ic-white.svg)" ) ] ]
                , it "mousing over pin icon does nothing if there are no pinned resources" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children []
                        >> Query.first
                        >> Event.simulate Event.mouseEnter
                        >> Event.toResult
                        >> Expect.err
                , it "there is some space between the pin icon and the user menu" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style [ ( "margin-right", "15px" ) ] ]
                , it "pin icon has relative positioning" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style [ ( "position", "relative" ) ] ]
                , it "pin icon does not have circular background" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.findAll
                            [ id "pin-icon"
                            , style
                                [ ( "border-radius", "50%" )
                                ]
                            ]
                        >> Query.count (Expect.equal 0)
                , it "pin icon has white color when pipeline has pinned resources" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style [ ( "background-image", "url(/public/images/pin-ic-white.svg)" ) ] ]
                , it "pin icon has pin badge when pipeline has pinned resources" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has pinBadgeSelector
                , it "pin badge is purple" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style [ ( "background-color", "#5C3BD1" ) ] ]
                , it "pin badge is circular" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style
                                [ ( "border-radius", "50%" )
                                , ( "width", "15px" )
                                , ( "height", "15px" )
                                ]
                            ]
                , it "pin badge is near the top right of the pin icon" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style
                                [ ( "position", "absolute" )
                                , ( "top", "3px" )
                                , ( "right", "3px" )
                                ]
                            ]
                , it "content inside pin badge is centered horizontally and vertically" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "align-items", "center" )
                                , ( "justify-content", "center" )
                                ]
                            ]
                , it "pin badge shows count of pinned resources, centered" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.findAll [ tag "div", containing [ text "1" ] ]
                        >> Query.count (Expect.equal 1)
                , it "pin badge has no other children" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.children []
                        >> Query.count (Expect.equal 1)
                , it "pin counter works with multiple pinned resources" <|
                    givenMultiplePinnedResources
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.findAll [ tag "div", containing [ text "2" ] ]
                        >> Query.count (Expect.equal 1)
                , it "before TogglePinIconDropdown msg no list of pinned resources is visible" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.hasNot [ tag "ul" ]
                , it "mousing over pin icon sends TogglePinIconDropdown msg" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children []
                        >> Query.first
                        >> Event.simulate Event.mouseEnter
                        >> Event.expect (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                , it "TogglePinIconDropdown msg causes pin icon to have light grey circular background" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has
                            [ style
                                [ ( "background-color", "rgba(255, 255, 255, 0.3)" )
                                , ( "border-radius", "50%" )
                                ]
                            ]
                , it "TogglePinIconDropdown msg causes dropdown list of pinned resources to appear" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children [ tag "ul" ]
                        >> Query.count (Expect.equal 1)
                , it "on TogglePinIconDropdown, pin badge has no other children" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.children []
                        >> Query.count (Expect.equal 1)
                , it "dropdown list of pinned resources contains resource name" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ tag "li", containing [ text "resource" ] ]
                , it "dropdown list of pinned resources shows resource names in bold" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.find [ tag "li", containing [ text "resource" ] ]
                        >> Query.findAll [ tag "div", containing [ text "resource" ], style [ ( "font-weight", "700" ) ] ]
                        >> Query.count (Expect.equal 1)
                , it "dropdown list of pinned resources shows pinned version of each resource" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.find [ tag "li", containing [ text "resource" ] ]
                        >> Query.has [ tag "table", containing [ text "v1" ] ]
                , it "dropdown list of pinned resources has white background" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style [ ( "background-color", "#fff" ) ] ]
                , it "dropdown list of pinned resources is drawn over other elements on the page" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style [ ( "z-index", "1" ) ] ]
                , it "dropdown list of pinned resources has dark grey text" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style [ ( "color", "#1e1d1d" ) ] ]
                , it "dropdown list has upward-pointing arrow" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children
                            [ style
                                [ ( "border-width", "5px" )
                                , ( "border-style", "solid" )
                                , ( "border-color", "transparent transparent #fff transparent" )
                                ]
                            ]
                        >> Query.count (Expect.equal 1)
                , it "dropdown list of pinned resources is offset below and left of the pin icon" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has
                            [ style
                                [ ( "position", "absolute" )
                                , ( "top", "100%" )
                                , ( "right", "0" )
                                , ( "margin-top", "0" )
                                ]
                            ]
                , it "dropdown list of pinned resources stretches horizontally to fit content" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has
                            [ style
                                [ ( "white-space", "nowrap" ) ]
                            ]
                , it "dropdown list of pinned resources has no bullet points" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has
                            [ style
                                [ ( "list-style-type", "none" ) ]
                            ]
                , it "dropdown list has comfortable padding" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has
                            [ style
                                [ ( "padding", "10px" ) ]
                            ]
                , it "dropdown list arrow is centered below the pin icon above the list" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children
                            [ style
                                [ ( "border-width", "5px" )
                                , ( "border-style", "solid" )
                                , ( "border-color", "transparent transparent #fff transparent" )
                                ]
                            ]
                        >> Query.first
                        >> Query.has
                            [ style
                                [ ( "top", "100%" )
                                , ( "right", "50%" )
                                , ( "margin-right", "-5px" )
                                , ( "margin-top", "-10px" )
                                , ( "position", "absolute" )
                                ]
                            ]
                , it "mousing off the pin icon sends TogglePinIconDropdown msg" <|
                    givenPinnedResource
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children []
                        >> Query.first
                        >> Event.simulate Event.mouseLeave
                        >> Event.expect (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                , it "clicking a pinned resource sends a Navigation Msg" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "li" ]
                        >> Event.simulate Event.click
                        >> Event.expect
                            (wrapTopBarMessage <|
                                TopBar.Msgs.GoToRoute <|
                                    Routes.Resource
                                        { id =
                                            { teamName = "team"
                                            , pipelineName = "pipeline"
                                            , resourceName = "resource"
                                            }
                                        , page = Nothing
                                        }
                            )
                , it "TogglePinIconDropdown msg causes dropdown list of pinned resources to disappear" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.hasNot [ tag "ul" ]
                , it "pinned resources in the dropdown should have a pointer cursor" <|
                    givenPinnedResource
                        >> Application.update (wrapTopBarMessage TopBar.Msgs.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Expect.all
                            [ Query.findAll [ tag "li" ]
                                >> Query.each (Query.has [ style [ ( "cursor", "pointer" ) ] ])
                            , Query.findAll [ style [ ( "cursor", "pointer" ) ] ]
                                >> Query.each (Query.has [ tag "li" ])
                            ]
                ]
            , test "top bar lays out contents horizontally" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style [ ( "display", "inline-block" ) ] ]
            , test "top bar maximizes spacing between the left and right navs" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style [ ( "justify-content", "space-between" ), ( "width", "100%" ) ] ]
            , test "top bar is sticky" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style [ ( "z-index", "999" ), ( "position", "fixed" ) ] ]
            , test "breadcrumb items are laid out horizontally" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumbs" ]
                        |> Query.children []
                        |> Query.each
                            (Query.has [ style [ ( "display", "inline-block" ) ] ])
            , describe "top bar positioning"
                [ testTopBarPositioning "Dashboard" "/"
                , testTopBarPositioning "Pipeline" "/teams/team/pipelines/pipeline"
                , testTopBarPositioning "Job" "/teams/team/pipelines/pipeline/jobs/job"
                , testTopBarPositioning "Build" "/teams/team/pipelines/pipeline/jobs/job/builds/build"
                , testTopBarPositioning "Resource" "/teams/team/pipelines/pipeline/resources/resource"
                , testTopBarPositioning "FlySuccess" "/fly_success"
                ]
            , rspecStyleDescribe "when on job page"
                (init "/teams/team/pipeline/pipeline/jobs/job/builds/1")
                [ it "shows no pin icon on top bar when viewing build page" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.hasNot [ id "pin-icon" ]
                ]
            , test "top nav bar is blue when pipeline is paused" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.handleCallback
                            (Effects.SubPage 1)
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
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style
                                [ ( "background-color", "#3498db" ) ]
                            ]
            , test "breadcrumb list is laid out horizontally" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumbs" ]
                        |> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , test "pipeline breadcrumb is laid out horizontally" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.has [ style [ ( "display", "inline-block" ) ] ]
            , test "top bar has pipeline breadcrumb with icon rendered first" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has pipelineBreadcrumbSelector
            , test "top bar has pipeline name after pipeline icon" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.has [ text "pipeline" ]
            , test "pipeline breadcrumb should have a link to the pipeline page" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumbs" ]
                        |> Query.children []
                        |> Query.first
                        |> Event.simulate Event.click
                        |> Event.expect
                            (wrapTopBarMessage <|
                                TopBar.Msgs.GoToRoute <|
                                    Routes.Pipeline { id = { teamName = "team", pipelineName = "pipeline" }, groups = [] }
                            )
            , describe "build page"
                [ test "pipeline breadcrumb should have a link to the pipeline page when viewing build details" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline/jobs/build/builds/1"
                            |> Application.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "breadcrumbs" ]
                            |> Query.children []
                            |> Query.index 0
                            |> Event.simulate Event.click
                            |> Event.expect
                                (Msgs.SubMsg 1 <|
                                    SubPage.Msgs.BuildMsg <|
                                        Build.Msgs.FromTopBar <|
                                            TopBar.Msgs.GoToRoute <|
                                                Routes.Pipeline { id = { teamName = "team", pipelineName = "pipeline" }, groups = [] }
                                )
                , test "there should be a / between pipeline and job in breadcrumb" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline/jobs/build/builds/1"
                            |> Application.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "breadcrumbs" ]
                            |> Query.children []
                            |> Query.index 1
                            |> Query.has [ text "/" ]
                , test "top bar has job breadcrumb with job icon rendered first" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                            |> Application.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "breadcrumbs" ]
                            |> Query.children []
                            |> Query.index 2
                            |> Query.has jobBreadcrumbSelector
                , test "top bar has build name after job icon" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                            |> Application.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "breadcrumbs" ]
                            |> Query.children []
                            |> Query.index 2
                            |> Query.has [ text "job" ]
                ]
            , describe "resource page"
                [ test "pipeline breadcrumb should have a link to the pipeline page when viewing resource details" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline/resources/resource"
                            |> Application.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "breadcrumbs" ]
                            |> Query.children []
                            |> Query.index 0
                            |> Event.simulate Event.click
                            |> Event.expect
                                (Msgs.SubMsg 1 <|
                                    SubPage.Msgs.ResourceMsg <|
                                        Resource.Msgs.TopBarMsg <|
                                            TopBar.Msgs.GoToRoute <|
                                                Routes.Pipeline { id = { teamName = "team", pipelineName = "pipeline" }, groups = [] }
                                )
                , test "there should be a / between pipeline and resource in breadcrumb" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline/resources/resource"
                            |> Application.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "breadcrumbs" ]
                            |> Query.children []
                            |> Query.index 1
                            |> Query.has [ text "/" ]
                , test "top bar has resource breadcrumb with resource icon rendered first" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline/resources/resource"
                            |> Application.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "breadcrumbs" ]
                            |> Query.children []
                            |> Query.index 2
                            |> Query.has resourceBreadcrumbSelector
                , test "top bar has resource name after resource icon" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline/resources/resource"
                            |> Application.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "breadcrumbs" ]
                            |> Query.children []
                            |> Query.index 2
                            |> Query.has [ text "resource" ]
                ]
            ]
        ]


pinBadgeSelector : List Selector.Selector
pinBadgeSelector =
    [ id "pin-badge" ]


pipelineBreadcrumbSelector : List Selector.Selector
pipelineBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic-breadcrumb-pipeline.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


jobBreadcrumbSelector : List Selector.Selector
jobBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic-breadcrumb-job.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


resourceBreadcrumbSelector : List Selector.Selector
resourceBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic-breadcrumb-resource.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


init : String -> Application.Model
init path =
    Application.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = ""
        , csrfToken = ""
        , authToken = ""
        , pipelineRunningKeyframes = ""
        }
        { href = ""
        , host = ""
        , hostname = ""
        , protocol = ""
        , origin = ""
        , port_ = ""
        , pathname = path
        , search = ""
        , hash = ""
        , username = ""
        , password = ""
        }
        |> Tuple.first


givenPinnedResource : Application.Model -> Application.Model
givenPinnedResource =
    Application.handleCallback
        (Effects.SubPage -1)
        (Callback.ResourcesFetched <|
            Ok <|
                Json.Encode.list
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
        (Effects.SubPage -1)
        (Callback.ResourcesFetched <|
            Ok <|
                Json.Encode.list
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


wrapTopBarMessage : TopBar.Msgs.Msg -> Msgs.Msg
wrapTopBarMessage =
    Pipeline.Msgs.FromTopBar >> SubPage.Msgs.PipelineMsg >> Msgs.SubMsg 1


testTopBarPositioning : String -> String -> Test
testTopBarPositioning pageName url =
    describe pageName
        [ test "whole page fills the whole screen" <|
            \_ ->
                init url
                    |> Application.view
                    |> Query.fromHtml
                    |> Query.find [ id "page-including-top-bar" ]
                    |> Query.has [ style [ ( "height", "100%" ) ] ]
        , test "lower section fills the whole screen as well" <|
            \_ ->
                init url
                    |> Application.view
                    |> Query.fromHtml
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has
                        [ style
                            -- this padding ugliness is necessary because pipeline's page is weird and not offset
                            [ ( "padding-top"
                              , if pageName == "Pipeline" || pageName == "Build" then
                                    "0"

                                else
                                    "54px"
                              )
                            , ( "height", "100%" )
                            ]
                        ]
        ]
