module DashboardHelpers
    exposing
        ( PipelineId
        , classifyJob
        , groupPipelines
        , jobsByPipelineId
        , pipelinesWithJobs
        , resourceErrorsByPipelineIdentifier
        )

import Concourse
import Dashboard.Pipeline as Pipeline
import Dict exposing (Dict)


type alias PipelineId =
    Int


pipelinesWithJobs : Dict PipelineId (List Concourse.Job) -> Dict ( String, String ) Bool -> List Concourse.Pipeline -> List Pipeline.PipelineWithJobs
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


groupPipelines : List ( String, List Pipeline.PipelineWithJobs ) -> ( String, Pipeline.PipelineWithJobs ) -> List ( String, List Pipeline.PipelineWithJobs )
groupPipelines pipelines ( teamName, pipeline ) =
    case pipelines of
        [] ->
            [ ( teamName, [ pipeline ] ) ]

        s :: ss ->
            if Tuple.first s == teamName then
                ( teamName, pipeline :: (Tuple.second s) ) :: ss
            else
                s :: (groupPipelines ss ( teamName, pipeline ))


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
                    resource.failingToCheck

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
