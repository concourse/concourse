module PipelineGridTests exposing (all)

import Application.Application as Application
import Common
import Concourse
import Data
import Expect
import Json.Encode as Encode
import Message.Callback as Callback
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription as Subscription exposing (Delivery(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
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
            Query.find [ class "pipeline-wrapper", containing [ text name ] ]

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
    in
    describe "dashboard rendering"
        [ test "renders the pipelines container as position relative" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
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
                    |> Common.contains (GetViewportOf Dashboard Callback.AlwaysShow)
        , test "fetches the viewport of the scrollable area when the window is resized" <|
            \_ ->
                Common.init "/"
                    |> Application.handleDelivery
                        (Subscription.WindowResized 800 600)
                    |> Tuple.second
                    |> Common.contains (GetViewportOf Dashboard Callback.AlwaysShow)
        , test "fetches the viewport of the scrollable area when the sidebar is opened" <|
            \_ ->
                Common.init "/"
                    |> Application.update (Update <| Click HamburgerMenu)
                    |> Tuple.second
                    |> Common.contains (GetViewportOf Dashboard Callback.AlwaysShow)
        , test "fetches the viewport of the scrollable area when the sidebar state is loaded" <|
            \_ ->
                Common.init "/"
                    |> Application.handleDelivery (SideBarStateReceived Nothing)
                    |> Tuple.second
                    |> Common.contains (GetViewportOf Dashboard Callback.AlwaysShow)
        , test "renders pipeline cards in a single column grid when the viewport is narrow" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 300 600
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-0"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25
                                , y = 268 + 25
                                , width = 272
                                , height = 268
                                }
                        ]
        , test "renders pipeline cards in a multi-column grid when the viewport is wide" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 650 200
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-0"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25 * 2 + 272
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        ]
        , test "ignores viewport updates for DOM elements other than the dashboard" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 650 200
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport LoginButton Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 100 50
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-0"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25 * 2 + 272
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        ]
        , test "pipelines with many jobs are rendered as cards spanning several rows" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Ok <|
                                List.repeat 15 (Data.job 0)
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 600 300
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-0"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268 * 2 + 25
                                }
                        , findPipelineCard "pipeline-1"
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
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Ok <|
                                jobsWithDepth 0 15
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 950 300
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-0"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272 * 2 + 25
                                , height = 268
                                }
                        , findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25 + 2 * (272 + 25)
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        ]
        , test "wraps cards to the next row" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 600 500
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ findPipelineCard "pipeline-0"
                            >> hasBounds
                                { x = 25
                                , y = 0
                                , width = 272
                                , height = 268
                                }
                        , findPipelineCard "pipeline-1"
                            >> hasBounds
                                { x = 25 * 2 + 272
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
                        , containerHasHeight <| (268 + 25) * 2
                        ]
        , test "sets the container height to the height of the cards" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 300 600
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> containerHasHeight ((268 + 25) * 2)
        , test "doesn't render rows below the viewport" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 300 200
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Query.hasNot [ class "pipeline-wrapper", containing [ text "pipeline-2" ] ]
        , test "body has a scroll handler" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team" 2 ]
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
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 600 250
                        )
                    |> Tuple.first
                    |> Application.update
                        (Update <|
                            Scrolled
                                { scrollTop = 600
                                , scrollHeight = 3240
                                , clientHeight = 250
                                }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Query.hasNot [ class "pipeline-wrapper", containing [ text "pipeline-0" ] ]
        , test "tall cards are not hidden when only its top row is scrolled out of view" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Ok <|
                                List.repeat 30 (Data.job 0)
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 600 250
                        )
                    |> Tuple.first
                    |> Application.update
                        (Update <|
                            Scrolled
                                { scrollTop = 600
                                , scrollHeight = 3240
                                , clientHeight = 250
                                }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Query.has [ class "pipeline-wrapper", containing [ text "pipeline-0" ] ]
        , test "groups that are outside the viewport have no visible pipelines" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team-2" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 300 200
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "team-2" ]
                    |> Query.hasNot [ class "pipeline-wrapper" ]
        , test "groups that are scrolled into view have visible pipelines" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team-2" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 300 200
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
                    |> Query.has [ class "pipeline-wrapper", containing [ text "pipeline-2" ] ]
        , test "pipeline wrapper has a z-index of 1 when hovering over it" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.first
                    |> Application.update
                        (Update <|
                            Hover <|
                                Just <|
                                    JobPreview
                                        { teamName = "team"
                                        , pipelineName = "pipeline-0"
                                        , jobName = "job"
                                        }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Query.has [ class "pipeline-wrapper", style "z-index" "1" ]
        , describe "drop areas" <|
            [ test "renders a drop area over each pipeline card" <|
                \_ ->
                    Common.init "/"
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                                Ok <|
                                    viewportWithSize 600 500
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
                    Common.init "/"
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                                Ok <|
                                    viewportWithSize 300 200
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "dashboard-team-pipelines" ]
                        |> Query.findAll [ class "drop-area" ]
                        |> Query.count (Expect.equal 2)
            , test "renders the drop area up one row when the card breaks the row, but there is space for a smaller card" <|
                \_ ->
                    Common.init "/"
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllJobsFetched <|
                                Ok <|
                                    jobsWithDepth 1 15
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.GotViewport Dashboard Callback.AlwaysShow <|
                                Ok <|
                                    viewportWithSize 600 500
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
        ]


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
