port module Dashboard exposing (Model, Msg, init, update, subscriptions, view)

import Concourse
import Concourse.BuildStatus
import Concourse.Job
import Concourse.Pipeline
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class, classList, href, src)
import RemoteData
import Time exposing (Time)


type alias Model =
    { pipelines : RemoteData.WebData (List Concourse.Pipeline)
    , jobs : Dict Concourse.PipelineName (RemoteData.WebData (List Concourse.Job))
    , turbulenceImgSrc : String
    }


type Msg
    = PipelinesResponse (RemoteData.WebData (List Concourse.Pipeline))
    | JobsResponse Concourse.PipelineName (RemoteData.WebData (List Concourse.Job))
    | AutoRefresh Time


init : String -> ( Model, Cmd Msg )
init turbulencePath =
    ( { pipelines = RemoteData.NotAsked
      , jobs = Dict.empty
      , turbulenceImgSrc = turbulencePath
      }
    , fetchPipelines
    )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        PipelinesResponse response ->
            ( { model | pipelines = response }
            , case response of
                RemoteData.Success pipelines ->
                    Cmd.batch (List.map fetchJobs pipelines)

                _ ->
                    Cmd.none
            )

        JobsResponse pipelineName response ->
            ( { model | jobs = Dict.insert pipelineName response model.jobs }, Cmd.none )

        AutoRefresh _ ->
            ( model, fetchPipelines )


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ Time.every (5 * Time.second) AutoRefresh ]


view : Model -> Html msg
view model =
    case model.pipelines of
        RemoteData.Success pipelines ->
            let
                pipelineStates =
                    List.sortWith pipelineStatusComparison <|
                        List.filter ((/=) RemoteData.NotAsked << .jobs) <|
                            List.map
                                (\pipeline ->
                                    { pipeline = pipeline
                                    , jobs =
                                        Maybe.withDefault RemoteData.NotAsked <|
                                            Dict.get pipeline.name model.jobs
                                    }
                                )
                                pipelines

                pipelinesByTeam =
                    List.foldl
                        (\pipelineState byTeam ->
                            Dict.update pipelineState.pipeline.teamName
                                (\mps ->
                                    Just (pipelineState :: Maybe.withDefault [] mps)
                                )
                                byTeam
                        )
                        Dict.empty
                        (List.reverse pipelineStates)
            in
                Html.div [ class "dashboard" ]
                    (Dict.values (Dict.map viewGroup pipelinesByTeam))

        RemoteData.Failure _ ->
            Html.div
                [ class "error-message" ]
                [ Html.div [ class "message" ]
                    [ Html.img [ src model.turbulenceImgSrc, class "seatbelt" ] []
                    , Html.p [] [ Html.text "experiencing turbulence" ]
                    , Html.p [ class "explanation" ] []
                    ]
                ]

        _ ->
            Html.text ""


pipelineStatusComparison : PipelineState -> PipelineState -> Order
pipelineStatusComparison pipeline otherPipeline =
    let
        status =
            pipelineStatus pipeline

        otherStatus =
            pipelineStatus otherPipeline

        failedString =
            Concourse.BuildStatus.show Concourse.BuildStatusFailed
    in
        if status == otherStatus then
            EQ
        else if status == failedString then
            LT
        else if otherStatus == failedString then
            GT
        else
            EQ


type alias PipelineState =
    { pipeline : Concourse.Pipeline
    , jobs : RemoteData.WebData (List Concourse.Job)
    }


viewGroup : String -> List PipelineState -> Html msg
viewGroup teamName pipelines =
    Html.div [ class "dashboard-team-group" ]
        [ Html.div [ class "dashboard-team-name" ]
            [ Html.text teamName
            ]
        , Html.div [ class "dashboard-team-pipelines" ]
            (List.map viewPipeline pipelines)
        ]


viewPipeline : PipelineState -> Html msg
viewPipeline state =
    Html.div
        [ classList
            [ ( "dashboard-pipeline", True )
            , ( "dashboard-paused", state.pipeline.paused )
            , ( "dashboard-running", isPipelineRunning state )
            , ( "dashboard-status-" ++ pipelineStatus state, not state.pipeline.paused )
            ]
        ]
        [ Html.div [ class "dashboard-pipeline-banner" ] []
        , Html.a [ class "dashboard-pipeline-content", href state.pipeline.url ]
            [ Html.div [ class "dashboard-pipeline-icon" ]
                []
            , Html.div [ class "dashboard-pipeline-name" ]
                [ Html.text state.pipeline.name ]
            ]
        ]


isPipelineRunning : PipelineState -> Bool
isPipelineRunning { jobs } =
    case jobs of
        RemoteData.Success js ->
            List.any (\job -> job.nextBuild /= Nothing) js

        _ ->
            False


pipelineStatus : PipelineState -> String
pipelineStatus { jobs } =
    case jobs of
        RemoteData.Success js ->
            Concourse.BuildStatus.show (jobsStatus js)

        _ ->
            "unknown"


jobsStatus : List Concourse.Job -> Concourse.BuildStatus
jobsStatus jobs =
    let
        statuses =
            List.map (\job -> Maybe.withDefault Concourse.BuildStatusPending <| Maybe.map .status job.finishedBuild) jobs
    in
        if List.member Concourse.BuildStatusFailed statuses then
            Concourse.BuildStatusFailed
        else if List.member Concourse.BuildStatusErrored statuses then
            Concourse.BuildStatusErrored
        else if List.member Concourse.BuildStatusAborted statuses then
            Concourse.BuildStatusAborted
        else if List.member Concourse.BuildStatusSucceeded statuses then
            Concourse.BuildStatusSucceeded
        else
            Concourse.BuildStatusPending


fetchPipelines : Cmd Msg
fetchPipelines =
    Cmd.map PipelinesResponse <|
        RemoteData.asCmd Concourse.Pipeline.fetchPipelines


fetchJobs : Concourse.Pipeline -> Cmd Msg
fetchJobs pipeline =
    Cmd.map (JobsResponse pipeline.name) <|
        RemoteData.asCmd <|
            Concourse.Job.fetchJobs
                { teamName = pipeline.teamName
                , pipelineName = pipeline.name
                }
