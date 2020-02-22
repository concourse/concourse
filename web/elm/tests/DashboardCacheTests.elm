module DashboardCacheTests exposing (all)

import Application.Application as Application
import Common
import Data
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Subscription as Subscription exposing (Delivery(..))
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, containing, id, style, text)
import Url


all : Test
all =
    describe "dashboard cache tests"
        [ test "requests the cached jobs on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , pipelineRunningKeyframes = ""
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains LoadCachedJobs
        , test "requests the cached pipelines on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , pipelineRunningKeyframes = ""
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains LoadCachedPipelines
        , test "subscribes to receive cached jobs" <|
            \_ ->
                Common.init "/"
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnCachedJobsReceived
        , test "subscribes to receive cached pipelines" <|
            \_ ->
                Common.init "/"
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnCachedPipelinesReceived
        , test "renders pipelines when receive cached pipelines delivery" <|
            \_ ->
                Common.init "/"
                    |> Application.handleDelivery
                        (CachedPipelinesReceived <|
                            Ok <|
                                [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ class "pipeline-wrapper", containing [ text "pipeline-0" ] ]
        , test "renders jobs in pipelines when receive cached jobs delivery" <|
            \_ ->
                Common.init "/"
                    |> Application.handleDelivery
                        (CachedPipelinesReceived <|
                            Ok <|
                                [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (CachedJobsReceived <|
                            Ok <|
                                [ Data.job 0 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "pipeline-wrapper" ]
                    |> Query.has [ class "parallel-grid" ]
        , test "ignores the job cache after fetching successfully" <|
            \_ ->
                Common.init "/"
                    |> Application.handleDelivery
                        (CachedPipelinesReceived <|
                            Ok <|
                                [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (AllJobsFetched <|
                            Ok <|
                                [ Data.job 0 ]
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (CachedJobsReceived <|
                            Ok <|
                                []
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "pipeline-wrapper" ]
                    |> Query.has [ class "parallel-grid" ]
        , test "saves jobs to cache when fetched" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (AllJobsFetched <|
                            Ok <|
                                [ Data.job 0 ]
                        )
                    |> Tuple.second
                    |> Common.contains (SaveCachedJobs [ Data.job 0 ])
        , test "does not save jobs to cache when fetched with no change" <|
            \_ ->
                Common.init "/"
                    |> Application.handleDelivery
                        (CachedJobsReceived <|
                            Ok <|
                                [ Data.job 0 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (AllJobsFetched <|
                            Ok <|
                                [ Data.job 0 ]
                        )
                    |> Tuple.second
                    |> Common.notContains (SaveCachedJobs [ Data.job 0 ])
        , test "saves pipelines to cache when fetched" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (AllPipelinesFetched <|
                            Ok <|
                                [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.second
                    |> Common.contains (SaveCachedPipelines [ Data.pipeline "team" 0 ])
        , test "does not save pipelines to cache when fetched with no change" <|
            \_ ->
                Common.init "/"
                    |> Application.handleDelivery
                        (CachedPipelinesReceived <|
                            Ok <|
                                [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (AllPipelinesFetched <|
                            Ok <|
                                [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.second
                    |> Common.notContains (SaveCachedPipelines [ Data.pipeline "team" 0 ])
        ]
