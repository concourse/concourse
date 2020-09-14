module DashboardCacheTests exposing (all)

import Application.Application as Application
import Common
import Concourse.BuildStatus exposing (BuildStatus(..))
import DashboardTests exposing (whenOnDashboard)
import Data
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message as Message exposing (DropTarget(..))
import Message.Subscription as Subscription exposing (Delivery(..))
import Message.TopLevelMessage as TopLevelMessage
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, containing, text)
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
        , test "requests the cached teams on page load" <|
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
                    |> Common.contains LoadCachedTeams
        , test "subscribes to receive cached jobs" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnCachedJobsReceived
        , test "subscribes to receive cached pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnCachedPipelinesReceived
        , test "subscribes to receive cached teams" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnCachedTeamsReceived
        , test "renders pipelines when receive cached pipelines delivery" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleDelivery
                        (CachedPipelinesReceived <|
                            Ok <|
                                [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ class "card-wrapper", containing [ text "pipeline-0" ] ]
        , test "renders jobs in pipelines when receive cached jobs delivery" <|
            \_ ->
                whenOnDashboard { highDensity = False }
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
                    |> Query.find [ class "card-wrapper" ]
                    |> Query.has [ class "parallel-grid" ]
        , test "ignores the job cache after fetching successfully" <|
            \_ ->
                whenOnDashboard { highDensity = False }
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
                    |> Query.find [ class "card-wrapper" ]
                    |> Query.has [ class "parallel-grid" ]
        , test "saves jobs to cache when fetched" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (AllJobsFetched <|
                            Ok <|
                                [ Data.job 0 ]
                        )
                    |> Tuple.second
                    |> Common.contains (SaveCachedJobs [ Data.job 0 ])
        , test "removes build information from jobs when saving to cache" <|
            \_ ->
                let
                    jobWithoutBuild =
                        Data.job 0

                    jobWithBuild =
                        { jobWithoutBuild
                            | finishedBuild = Just <| Data.jobBuild BuildStatusSucceeded
                            , transitionBuild = Just <| Data.jobBuild BuildStatusSucceeded
                            , nextBuild = Just <| Data.jobBuild BuildStatusSucceeded
                        }
                in
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (AllJobsFetched <|
                            Ok <|
                                [ jobWithBuild ]
                        )
                    |> Tuple.second
                    |> Common.contains (SaveCachedJobs [ jobWithoutBuild ])
        , test "does not save jobs to cache when fetched with no change" <|
            \_ ->
                whenOnDashboard { highDensity = False }
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
        , test "bounds the number of cached jobs to 1000" <|
            \_ ->
                let
                    firstNJobs n =
                        List.range 0 (n - 1) |> List.map Data.job
                in
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (AllJobsFetched <|
                            Ok <|
                                firstNJobs 2000
                        )
                    |> Tuple.second
                    |> Common.contains (SaveCachedJobs <| firstNJobs 1000)
        , test "saves pipelines to cache when fetched" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (AllPipelinesFetched <|
                            Ok <|
                                [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.second
                    |> Common.contains (SaveCachedPipelines [ Data.pipeline "team" 0 ])
        , test "ignores cached pipelines if we've already fetched from network" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (AllPipelinesFetched <|
                            Ok <|
                                [ Data.pipeline "team" 0 ]
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (CachedPipelinesReceived <|
                            Ok <|
                                []
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ class "card-wrapper", containing [ text "pipeline-0" ] ]
        , test "does not save pipelines to cache when fetched with no change" <|
            \_ ->
                whenOnDashboard { highDensity = False }
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
        , test "saves pipelines to cache when re-ordered" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (AllPipelinesFetched <|
                            Ok <|
                                [ Data.pipeline "team" 0, Data.pipeline "team" 1 ]
                        )
                    |> Tuple.first
                    |> Application.update
                        (TopLevelMessage.Update <| Message.DragStart "team" "pipeline-0")
                    |> Tuple.first
                    |> Application.update
                        (TopLevelMessage.Update <| Message.DragOver <| After "pipeline-1")
                    |> Tuple.first
                    |> Application.update
                        (TopLevelMessage.Update <| Message.DragEnd)
                    |> Tuple.second
                    |> Common.contains (SaveCachedPipelines [ Data.pipeline "team" 1, Data.pipeline "team" 0 ])
        , test "saves teams to cache when fetched" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (AllTeamsFetched <|
                            Ok <|
                                [ { id = 0, name = "team-0" } ]
                        )
                    |> Tuple.second
                    |> Common.contains (SaveCachedTeams [ { id = 0, name = "team-0" } ])
        , test "does not save teams to cache when fetched with no change" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleDelivery
                        (CachedTeamsReceived <|
                            Ok <|
                                [ { id = 0, name = "team-0" } ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (AllTeamsFetched <|
                            Ok <|
                                [ { id = 0, name = "team-0" } ]
                        )
                    |> Tuple.second
                    |> Common.notContains (SaveCachedPipelines [ Data.pipeline "team" 0 ])
        , test "deletes cached pipelines on logged out" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (LoggedOut <| Ok ())
                    |> Tuple.second
                    |> Common.contains DeleteCachedPipelines
        , test "deletes cached jobs on logged out" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (LoggedOut <| Ok ())
                    |> Tuple.second
                    |> Common.contains DeleteCachedJobs
        , test "deletes cached teams on logged out" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (LoggedOut <| Ok ())
                    |> Tuple.second
                    |> Common.contains DeleteCachedTeams
        ]
