module Api.Endpoints exposing (Endpoint(..), toUrl)

import Concourse
import Concourse.Pagination exposing (Page)
import Network.Pagination
import Url.Builder


type Endpoint
    = Job Concourse.JobIdentifier
    | Jobs Concourse.PipelineIdentifier
    | Builds Concourse.JobIdentifier (Maybe Page)
    | Resource Concourse.ResourceIdentifier


toUrl : Endpoint -> String
toUrl endpoint =
    let
        basePath =
            [ "api", "v1" ]

        pipelinePath { pipelineName, teamName } =
            basePath ++ [ "teams", teamName, "pipelines", pipelineName ]
    in
    case endpoint of
        Job id ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "jobs", id.jobName ])
                []

        Jobs id ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "jobs" ])
                []

        Builds id page ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "jobs", id.jobName, "builds" ])
                (Network.Pagination.params page)

        Resource id ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "resources", id.resourceName ])
                []
