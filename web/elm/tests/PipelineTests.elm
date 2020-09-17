module PipelineTests exposing (all)

import Application.Application as Application
import Assets
import Char
import Common exposing (defineHoverBehaviour, queryView)
import Concourse
import Concourse.Cli exposing (Cli(..))
import DashboardTests exposing (iconSelector)
import Data
import Expect exposing (..)
import Html.Attributes as Attr
import Json.Encode
import Keyboard
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message exposing (Message(..))
import Message.Subscription as Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        )
import Message.TopLevelMessage as Msgs
import Pipeline.Pipeline as Pipeline exposing (update)
import Routes
import Set
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
    , pipelineRunningKeyframes = ""
    }


pipelineFetched pipeline =
    Application.handleCallback (Callback.PipelineFetched (Ok <| pipeline))
        >> Tuple.first
        >> Application.handleCallback (Callback.AllPipelinesFetched (Ok <| [ pipeline ]))
        >> Tuple.first


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
                        , path = "/pipelines/1"
                        , query = Just "group=other-group"
                        , fragment = Nothing
                        }
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.PipelineFetched
                                (Ok <|
                                    (Data.pipeline "team" 1
                                        |> Data.withName "pipeline"
                                        |> Data.withGroups groups
                                    )
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
                                , unhoveredSelector =
                                    { description = "dark outline"
                                    , selector =
                                        [ style "border" "1px solid #2b2a2a" ]
                                    }
                                , hoverable = Message.Message.JobGroup 0
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
                                                        "/pipelines/1?group=group"
                                                , tag "a"
                                                ]
                                        , Query.index 1
                                            >> Query.has
                                                [ text "other-group"
                                                , attribute <|
                                                    Attr.href
                                                        "/pipelines/1?group=other-group"
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
            , test "groups bar and pipeline view lay out vertically" <|
                \_ ->
                    setupGroupsBar sampleGroups
                        |> Common.queryView
                        |> Query.find [ id "pipeline-container" ]
                        |> Query.has
                            [ style "display" "flex"
                            , style "flex-direction" "column"
                            ]
            ]
        , test "pipeline view fills available space" <|
            \_ ->
                Common.init "/pipelines/1"
                    |> Common.queryView
                    |> Query.find [ id "pipeline-container" ]
                    |> Query.has [ style "flex-grow" "1" ]
        , test "gets screen size on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = ""
                    , csrfToken = csrfToken
                    , authToken = ""
                    , pipelineRunningKeyframes = ""
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "pipelines/1"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains Effects.GetScreenSize
        , test "subscribes to screen resizes" <|
            \_ ->
                Common.init "/pipelines/1"
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnWindowResize
        , test "title should include the pipeline name" <|
            \_ ->
                Common.init "/pipelines/1"
                    |> pipelineFetched (Data.pipeline "team" 1 |> Data.withName "pipelineName")
                    |> Application.view
                    |> .title
                    |> Expect.equal "pipelineName - Concourse"
        , test "pipeline background should be set from display config" <|
            \_ ->
                Common.init "/pipelines/1"
                    |> Application.handleCallback
                        (Callback.PipelineFetched
                            (Ok <|
                                (Data.pipeline "team" 0
                                    |> Data.withName "pipeline"
                                    |> Data.withBackgroundImage "some-background.jpg"
                                )
                            )
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "pipeline-background" ]
                    |> Query.has
                        [ style "background-image" "url(\"some-background.jpg\")"
                        , style "background-repeat" "no-repeat"
                        , style "background-size" "cover"
                        , style "background-position" "center"
                        , style "opacity" "30%"
                        , style "filter" "grayscale(1)"
                        ]
        , describe "update" <|
            let
                defaultModel : Pipeline.Model
                defaultModel =
                    Pipeline.init
                        { pipelineLocator = Data.pipelineId
                        , turbulenceImgSrc = "some-turbulence-img-src"
                        , selectedGroups = []
                        }
                        |> Tuple.first
            in
            [ test "CLI icons at bottom right" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> Common.queryView
                        |> Query.find [ class "cli-downloads" ]
                        |> Query.children []
                        |> Expect.all
                            [ Query.index 0
                                >> Query.has
                                    [ style "background-image" <|
                                        Assets.backgroundImage <|
                                            Just (Assets.CliIcon OSX)
                                    , style "background-position" "50% 50%"
                                    , style "background-repeat" "no-repeat"
                                    , style "width" "12px"
                                    , style "height" "12px"
                                    , style "display" "inline-block"
                                    , attribute <| Attr.download ""
                                    ]
                            , Query.index 1
                                >> Query.has
                                    [ style "background-image" <|
                                        Assets.backgroundImage <|
                                            Just (Assets.CliIcon Windows)
                                    , style "background-position" "50% 50%"
                                    , style "background-repeat" "no-repeat"
                                    , style "width" "12px"
                                    , style "height" "12px"
                                    , style "display" "inline-block"
                                    , attribute <| Attr.download ""
                                    ]
                            , Query.index 2
                                >> Query.has
                                    [ style "background-image" <|
                                        Assets.backgroundImage <|
                                            Just (Assets.CliIcon Linux)
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
                    Common.init "/pipelines/1"
                        |> Application.subscriptions
                        |> Expect.all
                            [ Common.contains (Subscription.OnClockTick OneSecond)
                            , Common.contains (Subscription.OnClockTick FiveSeconds)
                            , Common.contains (Subscription.OnClockTick OneMinute)
                            ]
            , test "on five second timer, refreshes pipeline" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> Application.update
                            (Msgs.DeliveryReceived
                                (ClockTicked FiveSeconds <|
                                    Time.millisToPosix 0
                                )
                            )
                        |> Tuple.second
                        |> Common.contains (Effects.FetchPipeline Data.pipelineId)
            , test "on one minute timer, refreshes version" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> Application.update
                            (Msgs.DeliveryReceived
                                (ClockTicked OneMinute <|
                                    Time.millisToPosix 0
                                )
                            )
                        |> Tuple.second
                        |> Expect.equal [ Effects.FetchClusterInfo ]
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
                        Common.init "/pipelines/1"
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
                        Common.init "/pipelines/1"
                            |> clockTick
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.count (Expect.equal 20)
                , test "HideLegendTimeTicked reaches timeout" <|
                    \_ ->
                        Common.init "/pipelines/1"
                            |> clockTickALot 11
                            |> Common.queryView
                            |> Query.hasNot [ id "legend" ]
                , test "Mouse action after legend hidden reshows legend" <|
                    \_ ->
                        Common.init "/pipelines/1"
                            |> clockTickALot 11
                            |> Application.update (Msgs.DeliveryReceived <| Moused { x = 0, y = 0 })
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.has [ id "legend" ]
                ]
            , rspecStyleDescribe "when on pipeline page"
                (Common.init "/pipelines/1")
                [ it "shows a pin icon on top bar" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ id "pin-icon" ]
                , it "shows a star icon on top bar" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ id "top-bar-favorited-icon" ]
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
                        >> Query.index 1
                        >> Query.has
                            [ style "background-image" <|
                                Assets.backgroundImage <|
                                    Just Assets.ConcourseLogoWhite
                            , style "background-position" "50% 50%"
                            , style "background-repeat" "no-repeat"
                            , style "background-size" "42px 42px"
                            , style "width" "54px"
                            , style "height" "54px"
                            ]
                , it "concourse logo on the left is a link to homepage" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find
                            [ style "background-image" <|
                                Assets.backgroundImage <|
                                    Just Assets.ConcourseLogoWhite
                            ]
                        >> Query.has [ tag "a", attribute <| Attr.href "/" ]
                ]
            , test "top bar lays out contents horizontally" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style "display" "inline-block" ]
            , test "top bar maximizes spacing between the left and right navs" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style "justify-content" "space-between"
                            , style "width" "100%"
                            ]
            , test "top bar is sticky" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style "z-index" "999"
                            , style "position" "fixed"
                            ]
            , test "breadcrumb items are laid out horizontally" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumbs" ]
                        |> Query.children []
                        |> Query.each
                            (Query.has [ style "display" "inline-block" ])
            , describe "top bar positioning"
                [ testTopBarPositioning "Dashboard" "/"
                , testTopBarPositioning "Pipeline" "/pipelines/1"
                , testTopBarPositioning "Job" "/pipelines/1/jobs/job"
                , testTopBarPositioning "Build" "/pipelines/1/jobs/job/builds/build"
                , testTopBarPositioning "Resource" "/pipelines/1/resources/resource"
                , testTopBarPositioning "FlySuccess" "/fly_success"
                ]
            , rspecStyleDescribe "when on job page"
                (Common.init "/pipelines/1/jobs/job/builds/1")
                [ it "shows no pin icon on top bar when viewing build page" <|
                    Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.hasNot [ id "pin-icon" ]
                ]
            , test "top nav bar is blue when pipeline is paused" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> Application.handleCallback
                            (Callback.PipelineFetched
                                (Ok <|
                                    (Data.pipeline "team" 1
                                        |> Data.withName "pipeline"
                                        |> Data.withPaused True
                                    )
                                )
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style "background-color" "#3498db" ]
            , test "top nav bar isn't blue when pipeline is archived" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> Application.handleCallback
                            (Callback.PipelineFetched
                                (Ok <|
                                    (Data.pipeline "team" 1
                                        |> Data.withName "pipeline"
                                        |> Data.withPaused True
                                        |> Data.withArchived True
                                    )
                                )
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.hasNot [ style "background-color" "#3498db" ]
            , test "breadcrumb list is laid out horizontally" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> pipelineFetched (Data.pipeline "team" 1 |> Data.withName "pipeline")
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumbs" ]
                        |> Query.has
                            [ style "display" "inline-block"
                            , style "padding" "0 10px"
                            ]
            , test "pipeline breadcrumb is laid out horizontally" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> pipelineFetched (Data.pipeline "team" 1 |> Data.withName "pipeline")
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.has [ style "display" "inline-block" ]
            , test "top bar has pipeline breadcrumb with icon rendered first" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> pipelineFetched (Data.pipeline "team" 1 |> Data.withName "pipeline")
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has pipelineBreadcrumbSelector
            , test "top bar has pipeline name after pipeline icon" <|
                \_ ->
                    Common.init "/pipelines/1"
                        |> pipelineFetched (Data.pipeline "team" 1 |> Data.withName "pipeline")
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ id "breadcrumb-pipeline" ]
                        |> Query.has [ text "pipeline" ]
            , describe "top bar star icon" <|
                let
                    givenFavoritedPipelinesFetched =
                        Common.init "/pipelines/1"
                            |> pipelineFetched (Data.pipeline "team" 1 |> Data.withName "pipeline")
                            |> Application.handleDelivery
                                (Subscription.FavoritedPipelinesReceived <|
                                    Ok <|
                                        Set.singleton 1
                                )

                    favMsg =
                        Msgs.Update <|
                            Message.Message.Click <|
                                Message.Message.TopBarFavoritedIcon 1

                    iSeeStarUnfilled =
                        Query.has
                            (iconSelector
                                { size = "20px"
                                , image =
                                    Assets.FavoritedToggleIcon
                                        False
                                }
                            )
                in
                [ defineHoverBehaviour
                    { name = "star icon"
                    , setup = Common.init "/pipelines/1"
                    , query =
                        queryView
                            >> Query.find [ id "top-bar-favorited-icon" ]
                            >> Query.children []
                            >> Query.first
                    , unhoveredSelector =
                        { description = "faded star icon"
                        , selector =
                            [ style "opacity" "0.5"
                            , style "cursor" "pointer"
                            , style "margin" "17px"
                            ]
                                ++ iconSelector
                                    { size = "20px"
                                    , image = Assets.FavoritedToggleIcon False
                                    }
                        }
                    , hoveredSelector =
                        { description = "bright star icon"
                        , selector =
                            [ style "opacity" "1"
                            , style "cursor" "pointer"
                            , style "margin" "17px"
                            ]
                                ++ iconSelector
                                    { size = "20px"
                                    , image = Assets.FavoritedToggleIcon False
                                    }
                        }
                    , hoverable = Message.Message.TopBarFavoritedIcon -1
                    }
                , test "favoriting icon has click handler" <|
                    \_ ->
                        givenFavoritedPipelinesFetched
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "top-bar-favorited-icon" ]
                            |> Query.children []
                            |> Query.first
                            |> Event.simulate Event.click
                            |> Event.expect favMsg
                , test "click has FavoritedPipeline effect" <|
                    \_ ->
                        givenFavoritedPipelinesFetched
                            |> Tuple.first
                            |> Application.update
                                (Msgs.Update <|
                                    Message.Message.Click <|
                                        Message.Message.TopBarFavoritedIcon
                                            1
                                )
                            |> Tuple.second
                            |> Expect.equal
                                [ Effects.SaveFavoritedPipelines <|
                                    Set.empty
                                ]
                , test "clicking favorited icon unfills the filled star" <|
                    \_ ->
                        givenFavoritedPipelinesFetched
                            |> Tuple.first
                            |> Application.update
                                (Msgs.Update <|
                                    Message.Message.Click <|
                                        Message.Message.TopBarFavoritedIcon
                                            1
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> iSeeStarUnfilled
                ]
            ]
        ]


pinBadgeSelector : List Selector.Selector
pinBadgeSelector =
    [ id "pin-badge" ]


pipelineBreadcrumbSelector : List Selector.Selector
pipelineBreadcrumbSelector =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.BreadcrumbIcon Assets.PipelineComponent)
    , style "background-repeat" "no-repeat"
    ]


jobBreadcrumbSelector : List Selector.Selector
jobBreadcrumbSelector =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.BreadcrumbIcon Assets.JobComponent)
    , style "background-repeat" "no-repeat"
    ]


resourceBreadcrumbSelector : List Selector.Selector
resourceBreadcrumbSelector =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.BreadcrumbIcon Assets.ResourceComponent)
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
