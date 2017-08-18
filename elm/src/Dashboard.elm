port module Dashboard exposing (Model, Msg, init, update, view)

import Concourse
import Concourse.BuildStatus
import Concourse.Job
import Concourse.Pipeline
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class, classList, href)
import RemoteData


type alias Model =
    { pipelines : RemoteData.WebData (List PipelineState)
    }


type alias PipelineState =
    { pipeline : Concourse.Pipeline
    , jobs : RemoteData.WebData (List Concourse.Job)
    }


type Msg
    = PipelinesResponse (RemoteData.WebData (List Concourse.Pipeline))
    | JobsResponse String (RemoteData.WebData (List Concourse.Job))


init : ( Model, Cmd Msg )
init =
    ( { pipelines = RemoteData.NotAsked
      }
    , fetchPipelines
    )


initPipelineState : Concourse.Pipeline -> PipelineState
initPipelineState pipeline =
    { pipeline = pipeline
    , jobs = RemoteData.NotAsked
    }


updatePipelineState : String -> RemoteData.WebData (List Concourse.Job) -> PipelineState -> PipelineState
updatePipelineState pipelineName response state =
    if state.pipeline.name == pipelineName then
        { state | jobs = response }
    else
        state


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        PipelinesResponse response ->
            ( { model
                | pipelines =
                    RemoteData.map (List.map initPipelineState) response
              }
            , case response of
                RemoteData.Success pipelines ->
                    Cmd.batch (List.map fetchJobs pipelines)

                _ ->
                    Cmd.none
            )

        JobsResponse pipelineName response ->
            ( { model | pipelines = RemoteData.map (List.map (updatePipelineState pipelineName response)) model.pipelines }, Cmd.none )


view : Model -> Html msg
view model =
    case model.pipelines of
        RemoteData.Success pipelines ->
            let
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
                        pipelines
            in
                Html.div [ class "dashboard" ]
                    (Dict.values (Dict.map viewGroup pipelinesByTeam))

        _ ->
            Html.text ""


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
            , ( "dashboard-status-" ++ pipelineStatus state, True )
            ]
        ]
        [ Html.div [ class "dashboard-pipeline-icon" ]
            []
        , Html.div [ class "dashboard-pipeline-name" ]
            [ Html.a [ href state.pipeline.url ] [ Html.text state.pipeline.name ] ]
        ]


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
        statuses default =
            List.map (\job -> Maybe.withDefault default <| Maybe.map .status job.finishedBuild) jobs
    in
        if List.member Concourse.BuildStatusFailed (statuses Concourse.BuildStatusPending) then
            Concourse.BuildStatusFailed
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
