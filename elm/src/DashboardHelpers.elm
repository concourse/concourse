module DashboardHelpers
    exposing
        ( PipelineId
        , PipelineWithJobs
        , classifyJob
        , containsStatus
        , groupPipelines
        , jobStatuses
        , jobsByPipelineId
        , pipelineStatusFromJobs
        , pipelinesWithJobs
        , resourceErrorsByPipelineIdentifier
        )

import Concourse
import Dict exposing (Dict)


type alias PipelineId =
    Int


type alias PipelineWithJobs =
    { pipeline : Concourse.Pipeline
    , jobs : List Concourse.Job
    , resourceError : Bool
    }


pipelinesWithJobs : Dict PipelineId (List Concourse.Job) -> Dict ( String, String ) Bool -> List Concourse.Pipeline -> List PipelineWithJobs
pipelinesWithJobs pipelineJobs pipelineResourceErrors pipelines =
    List.map
        (\pipeline ->
            { pipeline = pipeline
            , jobs =
                Maybe.withDefault [] <| Dict.get pipeline.id pipelineJobs
            , resourceError = Maybe.withDefault False <| Dict.get ( pipeline.teamName, pipeline.name ) pipelineResourceErrors
            }
        )
        pipelines


groupPipelines : List ( String, List PipelineWithJobs ) -> ( String, PipelineWithJobs ) -> List ( String, List PipelineWithJobs )
groupPipelines pipelines ( teamName, pipeline ) =
    case pipelines of
        [] ->
            [ ( teamName, [ pipeline ] ) ]

        s :: ss ->
            if Tuple.first s == teamName then
                ( teamName, pipeline :: (Tuple.second s) ) :: ss
            else
                s :: (groupPipelines ss ( teamName, pipeline ))


jobStatuses : List Concourse.Job -> List (Maybe Concourse.BuildStatus)
jobStatuses jobs =
    List.concatMap
        (\job ->
            [ Maybe.map .status job.finishedBuild
            , Maybe.map .status job.nextBuild
            ]
        )
        jobs


containsStatus : Concourse.BuildStatus -> List (Maybe Concourse.BuildStatus) -> Bool
containsStatus status statuses =
    List.any
        (\s ->
            case s of
                Just s ->
                    status == s

                Nothing ->
                    False
        )
        statuses


jobsByPipelineId : List Concourse.Pipeline -> List Concourse.Job -> Dict PipelineId (List Concourse.Job)
jobsByPipelineId pipelines jobs =
    let
        pipelinesByIdentifier =
            List.foldl
                (\pipeline byIdentifier -> Dict.insert ( pipeline.teamName, pipeline.name ) pipeline byIdentifier)
                Dict.empty
                pipelines
    in
        List.foldl
            (\job byPipelineId -> classifyJob job pipelinesByIdentifier byPipelineId)
            Dict.empty
            jobs


classifyJob : Concourse.Job -> Dict ( String, String ) Concourse.Pipeline -> Dict PipelineId (List Concourse.Job) -> Dict PipelineId (List Concourse.Job)
classifyJob job pipelines pipelineJobs =
    let
        pipelineIdentifier =
            ( job.teamName, job.pipelineName )

        mPipeline =
            Dict.get pipelineIdentifier pipelines
    in
        case mPipeline of
            Nothing ->
                pipelineJobs

            Just pipeline ->
                let
                    jobs =
                        Maybe.withDefault [] <| Dict.get pipeline.id pipelineJobs
                in
                    Dict.insert pipeline.id (job :: jobs) pipelineJobs


resourceErrorsByPipelineIdentifier : List Concourse.Resource -> Dict ( String, String ) Bool
resourceErrorsByPipelineIdentifier resources =
    List.foldl
        (\resource byPipelineId ->
            let
                pipelineIdentifier =
                    ( resource.teamName, resource.pipelineName )

                resourceCheckError =
                    not (String.isEmpty resource.checkError)

                resourceError =
                    case Dict.get pipelineIdentifier byPipelineId of
                        Nothing ->
                            resourceCheckError

                        Just checkError ->
                            checkError || resourceCheckError
            in
                Dict.insert pipelineIdentifier resourceError byPipelineId
        )
        Dict.empty
        resources


pipelineStatusFromJobs : List Concourse.Job -> Bool -> Concourse.PipelineStatus
pipelineStatusFromJobs jobs includeNextBuilds =
    let
        statuses =
            jobStatuses jobs
    in
        if containsStatus Concourse.BuildStatusPending statuses then
            Concourse.PipelineStatusPending
        else if includeNextBuilds && List.any (\job -> job.nextBuild /= Nothing) jobs then
            Concourse.PipelineStatusRunning
        else if containsStatus Concourse.BuildStatusFailed statuses then
            Concourse.PipelineStatusFailed
        else if containsStatus Concourse.BuildStatusErrored statuses then
            Concourse.PipelineStatusErrored
        else if containsStatus Concourse.BuildStatusAborted statuses then
            Concourse.PipelineStatusAborted
        else if containsStatus Concourse.BuildStatusSucceeded statuses then
            Concourse.PipelineStatusSucceeded
        else
            Concourse.PipelineStatusPending
