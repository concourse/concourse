module DashboardWatchTests exposing (all)

import Api.EventSource exposing (Event(..), EventEnvelope)
import Application.Application as Application
import Common
import Concourse.ListAllJobsEvent exposing (JobUpdate(..), ListAllJobsEvent(..))
import DashboardTests exposing (whenOnDashboard)
import Data
import Expect
import Html.Attributes as Attr
import Message.Callback as Callback exposing (Callback(..))
import Message.Effects as Effects exposing (Effect(..))
import Message.Message as Msgs
import Message.Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as ApplicationMsgs
import SubPage.SubPage exposing (..)
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector
    exposing
        ( attribute
        , class
        , containing
        , text
        )
import Time
import Url


all =
    describe "watch endpoints on the Dashboard"
        [ describe "ListAllJobs"
            [ test "opens ListAllJobs event stream on dashboard load" <|
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
                        |> Common.contains OpenListAllJobsEventStream
            , test "doesn't poll ListAllJobs while stream is open" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> Application.handleDelivery
                            (ClockTicked FiveSeconds <|
                                Time.millisToPosix 0
                            )
                        |> Tuple.second
                        |> Common.notContains FetchAllJobs
            , test "receiving a FetchAllJobs event doesn't trigger polling while stream is open" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> Application.handleCallback
                            (AllJobsFetched <| Ok [])
                        |> Tuple.first
                        |> Application.handleDelivery
                            (ClockTicked FiveSeconds <|
                                Time.millisToPosix 0
                            )
                        |> Tuple.second
                        |> Common.notContains FetchAllJobs
            , test "reopens event stream on logged out" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> Application.handleCallback
                            (LoggedOut <| Ok ())
                        |> Tuple.second
                        |> Common.contains OpenListAllJobsEventStream
            , test "initial event populates all jobs" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> Application.handleDelivery
                            (ListAllJobsEventsReceived <|
                                Ok
                                    [ envelope <|
                                        Event <|
                                            Initial
                                                [ Data.job 0 0
                                                    |> Data.withName "job"
                                                    |> Data.withPipelineName "pipeline"
                                                ]
                                    ]
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.has [ attribute <| Attr.attribute "data-tooltip" "job" ]
            , describe "patch event" <|
                let
                    setup initialJobs jobUpdates =
                        whenOnDashboard { highDensity = False }
                            |> Application.handleDelivery
                                (ListAllJobsEventsReceived <| Ok [ envelope <| Event <| Initial initialJobs ])
                            |> Tuple.first
                            |> Application.handleDelivery
                                (ListAllJobsEventsReceived <| Ok [ envelope <| Event <| Patch jobUpdates ])
                in
                [ test "PUT with new job adds it to jobs list" <|
                    \_ ->
                        setup []
                            [ Put 0 (Data.job 0 0 |> Data.withName "job") ]
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.has [ attribute <| Attr.attribute "data-tooltip" "job" ]
                , test "PUT with existing job updates it" <|
                    \_ ->
                        setup
                            [ Data.job 0 0 |> Data.withName "A"
                            , Data.job 1 0 |> Data.withName "B"
                            ]
                            [ Put 1
                                (Data.job 1 0
                                    |> Data.withName "B"
                                    |> Data.withInputs [ Data.input [ "A" ] ]
                                )
                            ]
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ class "card", containing [ text "pipeline-0" ] ]
                            |> Query.findAll [ class "parallel-grid" ]
                            |> Query.count (Expect.equal 2)
                , test "PUT replaces existing job when pipeline is renamed" <|
                    \_ ->
                        setup
                            [ Data.job 0 0 |> Data.withPipelineName "p1" ]
                            [ Put 0
                                (Data.job 0 0 |> Data.withPipelineName "p2")
                            ]
                            |> Tuple.first
                            |> Expect.all
                                [ Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "p1" ]
                                    )
                                    >> Tuple.first
                                    >> Common.queryView
                                    >> Query.find [ class "card", containing [ text "p1" ] ]
                                    >> Query.hasNot [ attribute <| Attr.attribute "data-tooltip" "job" ]
                                , Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "p2" ]
                                    )
                                    >> Tuple.first
                                    >> Common.queryView
                                    >> Query.find [ class "card", containing [ text "p2" ] ]
                                    >> Query.has [ attribute <| Attr.attribute "data-tooltip" "job" ]
                                ]
                , test "DELETE deletes job with id" <|
                    \_ ->
                        setup [ Data.job 0 0 ] [ Delete 0 ]
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.hasNot [ attribute <| Attr.attribute "data-tooltip" "job" ]
                ]
            , describe "falls back to polling if event streaming is disabled for ListAllJobs" <|
                let
                    setup =
                        whenOnDashboard { highDensity = False }
                            |> Application.handleDelivery
                                (ListAllJobsEventsReceived <| Ok [ envelope NetworkError ])
                in
                [ test "auto refreshes jobs on five-second tick after previous request finishes" <|
                    \_ ->
                        setup
                            |> Tuple.first
                            |> Application.handleCallback
                                (AllJobsFetched <| Ok [])
                            |> Tuple.first
                            |> Application.handleDelivery
                                (ClockTicked FiveSeconds <|
                                    Time.millisToPosix 0
                                )
                            |> Tuple.second
                            |> Common.contains FetchAllJobs
                , test "stops polling jobs if the endpoint is disabled" <|
                    \_ ->
                        setup
                            |> Tuple.first
                            |> Application.handleCallback
                                (AllJobsFetched <| Data.httpNotImplemented)
                            |> Tuple.first
                            |> Application.handleDelivery
                                (ClockTicked FiveSeconds <|
                                    Time.millisToPosix 0
                                )
                            |> Tuple.second
                            |> Common.notContains FetchAllJobs
                , test "auto refreshes jobs on next five-second tick after dropping" <|
                    \_ ->
                        setup
                            |> Tuple.first
                            |> Application.handleCallback
                                (AllJobsFetched <| Ok [])
                            |> Tuple.first
                            |> Application.update
                                (ApplicationMsgs.Update <| Msgs.DragStart "team" "pipeline")
                            |> Tuple.first
                            |> Application.handleDelivery
                                (ClockTicked FiveSeconds <|
                                    Time.millisToPosix 0
                                )
                            |> Tuple.first
                            |> Application.update
                                (ApplicationMsgs.Update <| Msgs.DragEnd)
                            |> Tuple.first
                            |> Application.handleDelivery
                                (ClockTicked FiveSeconds <|
                                    Time.millisToPosix 0
                                )
                            |> Tuple.second
                            |> Common.contains FetchAllJobs
                , test "don't poll all jobs until the last request finishes" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> Application.handleCallback
                                (Callback.AllJobsFetched <| Ok [])
                            |> Tuple.first
                            |> Application.handleDelivery
                                (ClockTicked FiveSeconds <| Time.millisToPosix 0)
                            |> Tuple.first
                            |> Application.handleDelivery
                                (ClockTicked FiveSeconds <| Time.millisToPosix 0)
                            |> Tuple.second
                            |> Common.notContains Effects.FetchAllJobs
                ]
            ]
        ]


envelope : Event a -> EventEnvelope a
envelope e =
    { data = e, url = "/api/v1/jobs" }
