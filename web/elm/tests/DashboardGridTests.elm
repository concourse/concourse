module DashboardGridTests exposing (all)

import Application.Application as Application
import Common
import Concourse
import Data
import Expect
import Json.Encode as Encode
import Message.Callback as Callback
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Message.Subscription as Subscription exposing (Delivery(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Set
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, containing, id, style, text)
import Url


all : Test
all =
    let
        viewportWithSize width height =
            { scene =
                { width = width
                , height = height
                }
            , viewport =
                { width = width
                , height = height
                , x = 0
                , y = 0
                }
            }

        findPipelineCard name =
            Query.find [ class "card-wrapper", containing [ text name ] ]

        hasBounds { x, y, width, height } =
            Query.has
                [ style "position" "absolute"
                , style "transform"
                    ("translate("
                        ++ String.fromInt x
                        ++ "px,"
                        ++ String.fromInt y
                        ++ "px)"
                    )
                , style "width" (String.fromInt width ++ "px")
                , style "height" (String.fromInt height ++ "px")
                ]

        containerHasHeight height =
            Query.has
                [ class "dashboard-team-pipelines"
                , style "height" <| String.fromInt height ++ "px"
                ]

        loadDashboardWithSize width height =
            Common.init "/"
                |> Application.handleCallback
                    (Callback.GotViewport Dashboard <|
                        Ok <|
                            viewportWithSize width height
                    )
                |> Tuple.first
    in
    describe "dashboard rendering"
        [ test "renders the pipelines container as position relative" <|
            \_ ->
                loadDashboardWithSize 600 600
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has
                        [ class "dashboard-team-pipelines"
                        , style "position" "relative"
                        ]
        , test "fetches the viewport of the scrollable area on load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , pipelineRunningKeyframes = "pipeline-running"
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains (GetViewportOf Dashboard)
        , test "fetches the viewport of the scrollable area when the window is resized" <|
            \_ ->
                loadDashboardWithSize 600 600
                    |> Application.handleDelivery
                        (Subscription.WindowResized 800 600)
                    |> Tuple.second
                    |> Common.contains (GetViewportOf Dashboard)
        , test "fetches the viewport of the scrollable area when the sidebar is opened" <|
            \_ ->
                loadDashboardWithSize 600 600
                    |> Application.update (Update <| Click HamburgerMenu)
                    |> Tuple.second
                    |> Common.contains (GetViewportOf Dashboard)
        , test "fetches the viewport of the scrollable area when the sidebar state is loaded" <|
            \_ ->
                loadDashboardWithSize 600 600
                    |> Application.handleDelivery
                        (SideBarStateReceived
                            (Ok
                                { isOpen = True
                                , width = 275
                                }
                            )
                        )
                    |> Tuple.second
                    |> Common.contains (GetViewportOf Dashboard)
        , test "renders pipeline cards in a single column grid when the viewport is narrow" <|
            \_ ->
                loadDashboardWithSize 300 600
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , findPipelineCard "pipeline-2"
                            >> hasBounds
                                { x = 25
                                , y = 268 + 25
                                , width = 272
                                , height = 268
                                }
                        ]
        , test "renders pipeline cards in a multi-column grid when the viewport is wide" <|
            \_ ->
                loadDashboardWithSize 650 200
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , findPipelineCard "pipeline-2"
                            >> hasBounds
                                { x = 25 * 2 + 272
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        ]
        , test "ignores viewport updates for DOM elements other than the dashboard" <|
            \_ ->
                loadDashboardWithSize 650 200
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport LoginButton <|
                            Ok <|
                                viewportWithSize 100 50
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , findPipelineCard "pipeline-2"
                            >> hasBounds
                                { x = 25 * 2 + 272
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        ]
        , test "pipelines with many jobs are rendered as cards spanning several rows" <|
            \_ ->
                loadDashboardWithSize 600 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Ok <|
                                jobsWithHeight 1 15
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268 * 2 + 25
                                }
                        , findPipelineCard "pipeline-2"
                            >> hasBounds
                                { x = 25 * 2 + 272
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , containerHasHeight <| (268 + 25) * 2
                        ]
        , test "pipelines with many dependant jobs are rendered as spanning multiple columns" <|
            \_ ->
                loadDashboardWithSize 950 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Ok <|
                                jobsWithDepth 1 15
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272 * 2 + 25
                                , height = 268
                                }
                        , findPipelineCard "pipeline-2"
                            >> hasBounds
                                { x = 25 + 2 * (272 + 25)
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        ]
        , test "wraps cards to the next row" <|
            \_ ->
                loadDashboardWithSize 600 500
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2, Data.pipeline "team" 3 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , findPipelineCard "pipeline-2"
                            >> hasBounds
                                { x = 25 * 2 + 272
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , findPipelineCard "pipeline-3"
                            >> hasBounds
                                { x = 25
                                , y = 268 + 25
                                , width = 272
                                , height = 268
                                }
                        , containerHasHeight <| (268 + 25) * 2
                        ]
        , test "sets the container height to the height of the cards" <|
            \_ ->
                loadDashboardWithSize 300 600
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> containerHasHeight ((268 + 25) * 2)
        , test "doesn't render rows below the viewport" <|
            \_ ->
                loadDashboardWithSize 300 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2, Data.pipeline "team" 3 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Query.hasNot [ class "card-wrapper", containing [ text "pipeline-3" ] ]
        , test "body has a scroll handler" <|
            \_ ->
                loadDashboardWithSize 300 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2, Data.pipeline "team" 3 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "dashboard" ]
                    |> Event.simulate
                        (Event.custom "scroll" <|
                            Encode.object
                                [ ( "target"
                                  , Encode.object
                                        [ ( "clientHeight", Encode.int 0 )
                                        , ( "scrollTop", Encode.int 0 )
                                        , ( "scrollHeight", Encode.int 0 )
                                        ]
                                  )
                                ]
                        )
                    |> Event.expect
                        (Update <|
                            Scrolled
                                { scrollHeight = 0
                                , scrollTop = 0
                                , clientHeight = 0
                                }
                        )
        , test "rows are hidden as they are scrolled out of view" <|
            \_ ->
                loadDashboardWithSize 600 200
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2, Data.pipeline "team" 3 ]
                        )
                    |> Tuple.first
                    |> Application.update
                        (Update <|
                            Scrolled
                                { scrollTop = 700
                                , scrollHeight = 3240
                                , clientHeight = 200
                                }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Query.hasNot [ class "card-wrapper", containing [ text "pipeline-1" ] ]
        , test "tall cards are not hidden when only its top row is scrolled out of view" <|
            \_ ->
                loadDashboardWithSize 600 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Ok <|
                                jobsWithHeight 1 30
                        )
                    |> Tuple.first
                    |> Application.update
                        (Update <|
                            Scrolled
                                { scrollTop = 600
                                , scrollHeight = 3240
                                , clientHeight = 300
                                }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Query.has [ class "card-wrapper", containing [ text "pipeline-1" ] ]
        , test "groups that are outside the viewport have no visible pipelines" <|
            \_ ->
                loadDashboardWithSize 300 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2, Data.pipeline "team-2" 3 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "team-2" ]
                    |> Query.hasNot [ class "card-wrapper" ]
        , test "groups that are scrolled into view have visible pipelines" <|
            \_ ->
                loadDashboardWithSize 300 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2, Data.pipeline "team-2" 3 ]
                        )
                    |> Tuple.first
                    |> Application.update
                        (Update <|
                            Scrolled
                                { scrollTop = 600
                                , scrollHeight = 3240
                                , clientHeight = 200
                                }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "team-2" ]
                    |> Query.has [ class "card-wrapper", containing [ text "pipeline-3" ] ]
        , test "pipeline wrapper has a z-index of 1 when hovering over a job" <|
            \_ ->
                loadDashboardWithSize 300 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.update
                        (Update <|
                            Hover <|
                                Just <|
                                    JobPreview AllPipelinesSection Data.jobId
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Query.has [ class "card-wrapper", style "z-index" "1" ]
        , test "pipeline wrapper has a z-index of 1 when hovering over the wrapper" <|
            \_ ->
                loadDashboardWithSize 300 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.update
                        (Update <|
                            Hover <|
                                Just <|
                                    PipelineWrapper Data.pipelineId
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Query.has [ class "card-wrapper", style "z-index" "1" ]
        , test "pipeline wrapper responds to mouse over" <|
            \_ ->
                loadDashboardWithSize 300 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "card-wrapper" ]
                    |> Event.simulate Event.mouseOver
                    |> Event.expect
                        (Update <|
                            Hover <|
                                Just <|
                                    PipelineWrapper Data.pipelineId
                        )
        , test "pipeline wrapper responds to mouse out" <|
            \_ ->
                loadDashboardWithSize 300 300
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "card-wrapper" ]
                    |> Event.simulate Event.mouseOut
                    |> Event.expect (Update <| Hover Nothing)
        , describe "drop areas" <|
            [ test "renders a drop area over each pipeline card" <|
                \_ ->
                    loadDashboardWithSize 600 500
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2, Data.pipeline "team" 3 ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "dashboard-team-pipelines" ]
                        |> Query.findAll [ class "drop-area" ]
                        |> Expect.all
                            [ Query.index 0
                                >> hasBounds
                                    { x = 0
                                    , y = 0
                                    , width = 272 + 25
                                    , height = 268
                                    }
                            , Query.index 1
                                >> hasBounds
                                    { x = 272 + 25
                                    , y = 0
                                    , width = 272 + 25
                                    , height = 268
                                    }
                            , Query.index 2
                                >> hasBounds
                                    { x = 0
                                    , y = 268 + 25
                                    , width = 272 + 25
                                    , height = 268
                                    }
                            ]
            , test "does not render drop areas for rows that are not visible" <|
                \_ ->
                    loadDashboardWithSize 300 300
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2, Data.pipeline "team" 3 ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "dashboard-team-pipelines" ]
                        |> Query.findAll [ class "drop-area" ]
                        |> Query.count (Expect.equal 2)
            , test "renders the final drop area to the right of the last card" <|
                \_ ->
                    loadDashboardWithSize 600 300
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 1 ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "dashboard-team-pipelines" ]
                        |> Query.findAll [ class "drop-area" ]
                        |> Query.index -1
                        |> hasBounds
                            { x = 272 + 25
                            , y = 0
                            , width = 272 + 25
                            , height = 268
                            }
            , test "renders the drop area up one row when the card breaks the row, but there is space for a smaller card" <|
                \_ ->
                    loadDashboardWithSize 600 500
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllJobsFetched <|
                                Ok <|
                                    jobsWithDepth 2 15
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "dashboard-team-pipelines" ]
                        |> Query.findAll [ class "drop-area" ]
                        |> Expect.all
                            [ Query.index 0
                                >> hasBounds
                                    { x = 0
                                    , y = 0
                                    , width = 272 + 25
                                    , height = 268
                                    }
                            , Query.index 1
                                >> hasBounds
                                    { x = 272 + 25
                                    , y = 0
                                    , width = 272 + 25
                                    , height = 268
                                    }
                            ]
            ]
        , describe "when there are favorited pipelines" <|
            let
                gotFavoritedPipelines ids =
                    Application.handleDelivery
                        (Subscription.FavoritedPipelinesReceived <| Ok <| Set.fromList ids)

                findHeader t =
                    Query.find [ class "headers" ]
                        >> Query.find [ class "header", containing [ text t ] ]
            in
            [ test "renders a favorites pipeline section" <|
                \_ ->
                    loadDashboardWithSize 600 500
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                            )
                        |> Tuple.first
                        |> gotFavoritedPipelines [ 1 ]
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.has [ id "dashboard-favorite-pipelines" ]
            , test "renders pipeline cards in the favorites section" <|
                \_ ->
                    loadDashboardWithSize 600 500
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 1 ]
                            )
                        |> Tuple.first
                        |> gotFavoritedPipelines [ 1 ]
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-favorite-pipelines" ]
                        |> findPipelineCard "pipeline-1"
                        |> hasBounds
                            { x = 25
                            , y = 60
                            , width = 272
                            , height = 268
                            }
            , test "favorite section has the height of the cards" <|
                \_ ->
                    loadDashboardWithSize 300 500
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                            )
                        |> Tuple.first
                        |> gotFavoritedPipelines [ 1, 2 ]
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-favorite-pipelines" ]
                        |> Query.has
                            [ style "height" <|
                                String.fromFloat (2 * 268 + 2 * 60)
                                    ++ "px"
                            ]
            , test "offsets all pipelines section by height of the favorites section" <|
                \_ ->
                    loadDashboardWithSize 300 200
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                            )
                        |> Tuple.first
                        |> gotFavoritedPipelines [ 1, 2 ]
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "dashboard-team-pipelines" ]
                        |> Query.hasNot [ class "card-wrapper", containing [ text "pipeline-2" ] ]
            , test "renders team header above the first pipeline card" <|
                \_ ->
                    loadDashboardWithSize 300 200
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 1 ]
                            )
                        |> Tuple.first
                        |> gotFavoritedPipelines [ 1 ]
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-favorite-pipelines" ]
                        |> findHeader "team"
                        |> hasBounds { x = 25, y = 0, width = 272, height = 60 }
            , test "renders multiple teams' headers" <|
                \_ ->
                    loadDashboardWithSize 600 200
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team1" 1, Data.pipeline "team2" 2 ]
                            )
                        |> Tuple.first
                        |> gotFavoritedPipelines [ 1, 2 ]
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-favorite-pipelines" ]
                        |> Expect.all
                            [ findHeader "team1"
                                >> hasBounds { x = 25, y = 0, width = 272, height = 60 }
                            , findHeader "team2"
                                >> hasBounds { x = 272 + 25 * 2, y = 0, width = 272, height = 60 }
                            ]
            , test "renders one header per team" <|
                \_ ->
                    loadDashboardWithSize 600 200
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team1" 1, Data.pipeline "team1" 1 ]
                            )
                        |> Tuple.first
                        |> gotFavoritedPipelines [ 1, 2 ]
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-favorite-pipelines" ]
                        |> findHeader "team1"
                        |> hasBounds { x = 25, y = 0, width = 272 * 2 + 25, height = 60 }
            , test "renders a 'continued' header when a team spans multiple rows" <|
                \_ ->
                    loadDashboardWithSize 300 600
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team1" 1, Data.pipeline "team1" 1 ]
                            )
                        |> Tuple.first
                        |> gotFavoritedPipelines [ 1, 2 ]
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-favorite-pipelines" ]
                        |> findHeader "team1 (continued)"
                        |> hasBounds { x = 25, y = 268 + 60, width = 272, height = 60 }
            ]
        ]


jobsWithHeight : Int -> Int -> List Concourse.Job
jobsWithHeight pipelineID height =
    List.range 1 height
        |> List.map
            (\i ->
                let
                    job =
                        Data.job pipelineID
                in
                { job | name = String.fromInt i }
            )


jobsWithDepth : Int -> Int -> List Concourse.Job
jobsWithDepth pipelineID depth =
    let
        job =
            Data.job pipelineID
    in
    if depth < 1 then
        []

    else if depth == 1 then
        [ { job
            | name = "1"
            , inputs =
                [ { name = ""
                  , resource = ""
                  , passed = []
                  , trigger = False
                  }
                ]
          }
        ]

    else
        { job
            | name = String.fromInt depth
            , inputs =
                [ { name = ""
                  , resource = ""
                  , passed = [ String.fromInt <| depth - 1 ]
                  , trigger = False
                  }
                ]
        }
            :: (jobsWithDepth pipelineID <| depth - 1)
