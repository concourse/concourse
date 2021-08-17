module ApplicationTests exposing (all)

import Application.Application as Application
import Browser
import Common exposing (queryView)
import Concourse
import Data
import Dict
import Expect
import HoverState
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription as Subscription exposing (Delivery(..))
import Message.TopLevelMessage as Msgs
import Routes
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (id, style)
import Url


all : Test
all =
    describe "top-level application"
        [ test "should subscribe to clicks from the not-automatically-linked boxes in the pipeline" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/"
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnNonHrefLinkClicked
        , test "subscribes to local storage" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/"
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnLocalStorageReceived
        , test "subscribes to browser cache" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/"
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnCacheReceived
        , test "loads favorited pipelines/instance groups on init" <|
            \_ ->
                Application.init Data.flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/teams/t/pipelines/p/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Expect.all
                        [ Common.contains Effects.LoadFavoritedPipelines
                        , Common.contains Effects.LoadFavoritedInstanceGroups
                        ]
        , test "clicking a not-automatically-linked box in the pipeline redirects" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/"
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            NonHrefLinkClicked "/foo/bar"
                        )
                    |> Tuple.second
                    |> Expect.equal [ Effects.LoadExternal "/foo/bar" ]
        , test "received token is passed to all subsequent requests" <|
            \_ ->
                let
                    pipelineIdentifier =
                        { pipelineName = "p", teamName = "t" }
                in
                Common.init "/"
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            TokenReceived <|
                                Ok "real-token"
                        )
                    |> Tuple.first
                    |> .session
                    |> .csrfToken
                    |> Expect.equal "real-token"
        , test "subscribes to mouse events when dragging the side bar handle" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/jobs/j"
                    |> Application.update
                        (Msgs.Update <|
                            Click SideBarResizeHandle
                        )
                    |> Tuple.first
                    |> Application.subscriptions
                    |> Expect.all
                        [ Common.contains Subscription.OnMouse
                        , Common.contains Subscription.OnMouseUp
                        ]
        , test "cannot select text when dragging sidebar" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/jobs/j"
                    |> Application.update
                        (Msgs.Update <|
                            Click SideBarResizeHandle
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has
                        [ style "user-select" "none"
                        , style "-ms-user-select" "none"
                        , style "-moz-user-select" "none"
                        , style "-khtml-user-select" "none"
                        , style "-webkit-user-select" "none"
                        , style "-webkit-touch-callout" "none"
                        ]
        , test "can select text when not dragging sidebar" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/jobs/j"
                    |> Common.queryView
                    |> Query.hasNot [ style "user-select" "none" ]
        , test "page-wrapper fills height" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/jobs/j"
                    |> Application.update
                        (Msgs.Update <|
                            Click SideBarResizeHandle
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "page-wrapper" ]
                    |> Query.has [ style "height" "100%" ]
        , test "changing route clears hovered element" <|
            \_ ->
                Common.init "/teams/t/pipelines/p/jobs/j"
                    |> Application.update (Msgs.Update <| Hover <| Just PinIcon)
                    |> Tuple.first
                    |> Application.handleDelivery
                        (RouteChanged <|
                            Routes.Dashboard
                                { searchType = Routes.Normal ""
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                        )
                    |> Tuple.first
                    |> .session
                    |> .hovered
                    |> Expect.equal HoverState.NoHover
        , describe "pipeline groups propagation"
            [ test "navigating through sub routes of a pipeline persists the groups" <|
                \_ ->
                    Common.initRoute
                        (Routes.Pipeline
                            { id =
                                { teamName = "t"
                                , pipelineName = "p"
                                , pipelineInstanceVars = Dict.empty
                                }
                            , groups = [ "test-group" ]
                            }
                        )
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <| Ok [ Data.pipeline "t" 1 |> Data.withName "p" ])
                        |> Tuple.first
                        |> Application.handleDelivery
                            (RouteChanged <|
                                Routes.Job
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        , pipelineInstanceVars = Dict.empty
                                        , jobName = "j"
                                        }
                                    , page = Nothing
                                    , groups = []
                                    }
                            )
                        |> Tuple.first
                        |> Application.handleDelivery
                            (RouteChanged <|
                                Routes.Build
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        , pipelineInstanceVars = Dict.empty
                                        , jobName = "j"
                                        , buildName = "b"
                                        }
                                    , highlight = Routes.HighlightNothing
                                    , groups = []
                                    }
                            )
                        |> Tuple.first
                        |> Application.handleDelivery
                            (RouteChanged <|
                                Routes.Resource
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        , pipelineInstanceVars = Dict.empty
                                        , resourceName = "r"
                                        }
                                    , page = Nothing
                                    , version = Nothing
                                    , groups = []
                                    }
                            )
                        |> Tuple.first
                        |> Application.handleDelivery
                            (RouteChanged <|
                                Routes.Causality
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        , pipelineInstanceVars = Dict.empty
                                        , resourceName = "r"
                                        , versionID = 1
                                        }
                                    , direction = Concourse.Downstream
                                    , version = Nothing
                                    , groups = []
                                    }
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ Common.routeHref <|
                                Routes.Pipeline
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        , pipelineInstanceVars = Dict.empty
                                        }
                                    , groups = [ "test-group" ]
                                    }
                            ]
            , test "navigating to no groups pipeline page does not propagate the groups" <|
                \_ ->
                    Common.initRoute
                        (Routes.Pipeline
                            { id =
                                { teamName = "t"
                                , pipelineName = "p"
                                , pipelineInstanceVars = Dict.empty
                                }
                            , groups = [ "test-group" ]
                            }
                        )
                        |> Application.handleDelivery
                            (RouteChanged <|
                                Routes.Pipeline
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        , pipelineInstanceVars = Dict.empty
                                        }
                                    , groups = []
                                    }
                            )
                        |> Tuple.first
                        |> .session
                        |> .route
                        |> Routes.getGroups
                        |> Expect.equal []
            ]
        ]
