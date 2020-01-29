module DashboardRenderingTests exposing (all)

import Application.Application as Application
import Common
import Data
import Expect
import Json.Encode as Encode
import Message.Callback as Callback
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription as Subscription
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

        hasPipelineCard name { x, y, width, height } =
            Query.find [ class "pipeline-wrapper", containing [ text name ] ]
                >> Query.has
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
        , test "sets the container height to the height of the cards" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> containerHasHeight 268
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
        , test "requests the dashboard viewport when the window is resized" <|
            \_ ->
                Common.init "/"
                    |> Application.handleDelivery
                        (Subscription.WindowResized 800 600)
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
                        (Callback.GotViewport Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 300 600
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ hasPipelineCard "pipeline-0"
                            { x = 25
                            , y = 0
                            , width = 272
                            , height = 268
                            }
                        , hasPipelineCard "pipeline-1"
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
                        (Callback.GotViewport Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 650 200
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ hasPipelineCard "pipeline-0"
                            { x = 25
                            , y = 0
                            , width = 272
                            , height = 268
                            }
                        , hasPipelineCard "pipeline-1"
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
                        (Callback.GotViewport Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 600 300
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ hasPipelineCard "pipeline-0"
                            { x = 25
                            , y = 0
                            , width = 272
                            , height = 268 * 2 + 25
                            }
                        , hasPipelineCard "pipeline-1"
                            { x = 25 * 2 + 272
                            , y = 0
                            , width = 272
                            , height = 268
                            }
                        , containerHasHeight <| 268 * 2 + 25
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
                        (Callback.GotViewport Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 600 500
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-pipelines" ]
                    |> Expect.all
                        [ hasPipelineCard "pipeline-0"
                            { x = 25
                            , y = 0
                            , width = 272
                            , height = 268
                            }
                        , hasPipelineCard "pipeline-1"
                            { x = 25 * 2 + 272
                            , y = 0
                            , width = 272
                            , height = 268
                            }
                        , hasPipelineCard "pipeline-2"
                            { x = 25
                            , y = 268 + 25
                            , width = 272
                            , height = 268
                            }
                        , containerHasHeight <| 268 * 2 + 25
                        ]
        , test "doesn't render rows below the viewport" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 600 200
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
                        (Callback.GotViewport Callback.AlwaysShow <|
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
                        (Callback.GotViewport Callback.AlwaysShow <|
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
        , test "considers a group's y-offset when determining visibility" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok [ Data.pipeline "team" 0, Data.pipeline "team" 1, Data.pipeline "team-2" 2 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotViewport Callback.AlwaysShow <|
                            Ok <|
                                viewportWithSize 300 300
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "team-2" ]
                    |> Query.hasNot [ class "pipeline-wrapper" ]
        ]
