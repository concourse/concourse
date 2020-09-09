module DashboardPreviewTests exposing (all)

import Application.Application as Application
import Colors
import Common exposing (defineHoverBehaviour, isColorWithStripes, queryView)
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Dashboard.DashboardPreview as DP
import DashboardTests exposing (whenOnDashboard)
import Data
import Expect
import Message.Callback as Callback
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Message.Subscription as Subscription
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Set
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, containing, id, style, text)
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
            , hoverable = JobPreview AllPipelinesSection jobId
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
                    |> withStatus BuildStatusSucceeded
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.success ]
        , test "succeeding paused job has blue background" <|
            \_ ->
                job
                    |> withStatus BuildStatusSucceeded
                    |> isPaused
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.paused ]
        , test "succeeding running job has striped green background" <|
            \_ ->
                job
                    |> withStatus BuildStatusSucceeded
                    |> withNextBuild
                    |> viewJob
                    |> isColorWithStripes
                        { thick = Colors.successFaded
                        , thin = Colors.success
                        }
        , test "failing job has red background" <|
            \_ ->
                job
                    |> withStatus BuildStatusFailed
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.failure ]
        , test "failing paused job has blue background" <|
            \_ ->
                job
                    |> withStatus BuildStatusFailed
                    |> isPaused
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.paused ]
        , test "failing running job has striped red background" <|
            \_ ->
                job
                    |> withStatus BuildStatusFailed
                    |> withNextBuild
                    |> viewJob
                    |> isColorWithStripes
                        { thick = Colors.failureFaded
                        , thin = Colors.failure
                        }
        , test "erroring job has amber background" <|
            \_ ->
                job
                    |> withStatus BuildStatusErrored
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.error ]
        , test "erroring paused job has blue background" <|
            \_ ->
                job
                    |> withStatus BuildStatusErrored
                    |> isPaused
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.paused ]
        , test "erroring running job has striped amber background" <|
            \_ ->
                job
                    |> withStatus BuildStatusErrored
                    |> withNextBuild
                    |> viewJob
                    |> isColorWithStripes
                        { thick = Colors.errorFaded
                        , thin = Colors.error
                        }
        , test "aborted job has amber background" <|
            \_ ->
                job
                    |> withStatus BuildStatusAborted
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.aborted ]
        , test "aborted paused job has blue background" <|
            \_ ->
                job
                    |> withStatus BuildStatusAborted
                    |> isPaused
                    |> viewJob
                    |> Query.has [ style "background-color" Colors.paused ]
        , test "aborted running job has striped amber background" <|
            \_ ->
                job
                    |> withStatus BuildStatusAborted
                    |> withNextBuild
                    |> viewJob
                    |> isColorWithStripes
                        { thick = Colors.abortedFaded
                        , thin = Colors.aborted
                        }
        , describe "preview in favorites section" <|
            let
                dashboardWithJobInFavoritesSection =
                    dashboardWithJob
                        >> Application.handleDelivery
                            (Subscription.FavoritedPipelinesReceived <| Ok <| Set.singleton 1)
                        >> Tuple.first

                findJobPreviewInFavoritesSection =
                    queryView
                        >> Query.find [ id "dashboard-favorite-pipelines" ]
                        >> Query.find [ class "card", containing [ text "pipeline" ] ]
                        >> Query.find [ class "parallel-grid" ]
                        >> Query.children []
                        >> Query.first
            in
            [ defineHoverBehaviour
                { name = "pending job"
                , setup = dashboardWithJobInFavoritesSection job
                , query = findJobPreviewInFavoritesSection
                , unhoveredSelector =
                    { description = "light grey background"
                    , selector = [ style "background-color" Colors.pending ]
                    }
                , hoverable = JobPreview FavoritesSection jobId
                , hoveredSelector =
                    { description = "dark grey background"
                    , selector = [ style "background-color" Colors.pendingFaded ]
                    }
                }
            , test "hovering over job preview in favorites section does not highlight in all pipelines section" <|
                \_ ->
                    dashboardWithJob job
                        |> Application.update
                            (Update <| Hover <| Just (JobPreview FavoritesSection jobId))
                        |> Tuple.first
                        |> findJobPreview
                        |> Query.has [ style "background-color" Colors.pending ]
            ]
        ]


viewJob : Concourse.Job -> Query.Single TopLevelMessage
viewJob =
    dashboardWithJob >> findJobPreview


dashboardWithJob : Concourse.Job -> Application.Model
dashboardWithJob j =
    whenOnDashboard { highDensity = False }
        |> Application.handleCallback
            (Callback.AllJobsFetched <|
                Ok
                    [ j
                    , { j | pipelineName = "other" }
                    ]
            )
        |> Tuple.first
        |> Application.handleCallback
            (Callback.AllTeamsFetched <|
                Ok
                    [ { id = 0, name = "team" }
                    ]
            )
        |> Tuple.first
        |> Application.handleCallback
            (Callback.AllPipelinesFetched <|
                Ok
                    [ Data.pipeline "team" 1 |> Data.withName "pipeline"
                    , Data.pipeline "team" 2 |> Data.withName "other"
                    ]
            )
        |> Tuple.first


findJobPreview : Application.Model -> Query.Single TopLevelMessage
findJobPreview =
    queryView
        >> Query.find [ class "dashboard-team-group", containing [ text "team" ] ]
        >> Query.find [ class "card", containing [ text "pipeline" ] ]
        >> Query.find [ class "parallel-grid" ]
        >> Query.children []
        >> Query.first


job : Concourse.Job
job =
    Data.job 1 |> Data.withPipelineName "pipeline"


withNextBuild : Concourse.Job -> Concourse.Job
withNextBuild j =
    Data.withNextBuild
        (Data.jobBuild BuildStatusStarted
            |> Data.withId 2
            |> Data.withName "2"
            |> Data.withJob (Just jobId)
            |> Just
        )
        j


withStatus : BuildStatus -> Concourse.Job -> Concourse.Job
withStatus status j =
    Data.withFinishedBuild
        (Data.jobBuild status
            |> Data.withId 1
            |> Data.withName "1"
            |> Data.withJob (Just jobId)
            |> Just
        )
        j


isPaused : Concourse.Job -> Concourse.Job
isPaused j =
    { j | paused = True }


jobId : Concourse.JobIdentifier
jobId =
    Data.jobId
