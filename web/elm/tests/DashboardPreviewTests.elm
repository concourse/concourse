module DashboardPreviewTests exposing (all)

import Application.Application as Application
import Colors
import Common exposing (defineHoverBehaviour, isColorWithStripes, queryView)
import Concourse
import Dashboard.DashboardPreview as DP
import Expect
import Message.Callback as Callback
import Message.Message exposing (DomID(..))
import Message.TopLevelMessage exposing (TopLevelMessage)
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, style)
import Time
import Url


all : Test
all =
    describe "job boxes in dashboard pipeline preview"
        [ test "fills available space" <|
            \_ ->
                job
                    |> viewJob
                    |> Query.has [ style "flex-grow" "1" ]
        , test "has small separation between adjacent jobs" <|
            \_ ->
                job
                    |> viewJob
                    |> Query.has [ style "margin" "2px" ]
        , test "link fills available space" <|
            \_ ->
                job
                    |> viewJob
                    |> Expect.all
                        [ Query.has [ style "display" "flex" ]
                        , Query.children []
                            >> Query.count (Expect.equal 1)
                        , Query.children []
                            >> Query.first
                            >> Query.has [ style "flex-grow" "1" ]
                        ]
        , defineHoverBehaviour
            { name = "pending job"
            , setup = dashboardWithJob job
            , query = findJobPreview
            , unhoveredSelector =
                { description = "light grey background"
                , selector = [ style "background-color" Colors.pending ]
                }
            , hoverable = JobPreview jobId
            , hoveredSelector =
                { description = "dark grey background"
                , selector = [ style "background-color" Colors.pendingFaded ]
                }
            }
        , test "pending paused job has blue background" <|
            \_ ->
                job
                    |> isPaused
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.paused ]
        , test "pending running job has grey striped background" <|
            \_ ->
                job
                    |> withNextBuild
                    |> viewJob
                    |> isColorWithStripes
                        { thick = Colors.pendingFaded
                        , thin = Colors.pending
                        }
        , test "succeeding job has green background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusSucceeded
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.success ]
        , test "succeeding paused job has blue background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusSucceeded
                    |> isPaused
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.paused ]
        , test "succeeding running job has striped green background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusSucceeded
                    |> withNextBuild
                    |> viewJob
                    |> isColorWithStripes
                        { thick = Colors.successFaded
                        , thin = Colors.success
                        }
        , test "failing job has red background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusFailed
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.failure ]
        , test "failing paused job has blue background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusFailed
                    |> isPaused
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.paused ]
        , test "failing running job has striped red background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusFailed
                    |> withNextBuild
                    |> viewJob
                    |> isColorWithStripes
                        { thick = Colors.failureFaded
                        , thin = Colors.failure
                        }
        , test "erroring job has amber background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusErrored
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.error ]
        , test "erroring paused job has blue background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusErrored
                    |> isPaused
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.paused ]
        , test "erroring running job has striped amber background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusErrored
                    |> withNextBuild
                    |> viewJob
                    |> isColorWithStripes
                        { thick = Colors.errorFaded
                        , thin = Colors.error
                        }
        , test "aborted job has amber background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusAborted
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.aborted ]
        , test "aborted paused job has blue background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusAborted
                    |> isPaused
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.paused ]
        , test "aborted running job has striped amber background" <|
            \_ ->
                job
                    |> withStatus Concourse.BuildStatusAborted
                    |> withNextBuild
                    |> viewJob
                    |> isColorWithStripes
                        { thick = Colors.abortedFaded
                        , thin = Colors.aborted
                        }
        ]


viewJob : Concourse.Job -> Query.Single TopLevelMessage
viewJob =
    dashboardWithJob >> findJobPreview


dashboardWithJob : Concourse.Job -> Application.Model
dashboardWithJob j =
    Common.init "/"
        |> Application.handleCallback
            (Callback.APIDataFetched <|
                Ok
                    ( Time.millisToPosix 0
                    , { teams = [ { id = 0, name = "team" } ]
                      , pipelines =
                            [ { id = 0
                              , name = "pipeline"
                              , paused = False
                              , public = True
                              , teamName = "team"
                              , groups = []
                              }
                            ]
                      , jobs = [ j ]
                      , resources = []
                      , user = Nothing
                      , version = "0.0.0-dev"
                      }
                    )
            )
        |> Tuple.first


findJobPreview : Application.Model -> Query.Single TopLevelMessage
findJobPreview =
    queryView
        >> Query.find [ class "parallel-grid" ]
        >> Query.children []
        >> Query.first


job : Concourse.Job
job =
    { pipeline =
        { teamName = "team"
        , pipelineName = "pipeline"
        }
    , name = "job"
    , pipelineName = "pipeline"
    , teamName = "team"
    , nextBuild = Nothing
    , finishedBuild = Nothing
    , transitionBuild = Nothing
    , paused = False
    , disableManualTrigger = False
    , inputs = []
    , outputs = []
    , groups = []
    }


withNextBuild : Concourse.Job -> Concourse.Job
withNextBuild j =
    { j
        | nextBuild =
            Just
                { id = 2
                , name = "2"
                , job = Just jobId
                , status = Concourse.BuildStatusStarted
                , duration = { startedAt = Nothing, finishedAt = Nothing }
                , reapTime = Nothing
                }
    }


withStatus : Concourse.BuildStatus -> Concourse.Job -> Concourse.Job
withStatus status j =
    { j
        | finishedBuild =
            Just
                { id = 1
                , name = "1"
                , job = Just jobId
                , status = status
                , duration = { startedAt = Nothing, finishedAt = Nothing }
                , reapTime = Nothing
                }
    }


isPaused : Concourse.Job -> Concourse.Job
isPaused j =
    { j | paused = True }


jobId : Concourse.JobIdentifier
jobId =
    { teamName = "team"
    , pipelineName = "pipeline"
    , jobName = "job"
    }
