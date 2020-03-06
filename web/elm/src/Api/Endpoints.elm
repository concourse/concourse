module Api.Endpoints exposing (Endpoint(..), toUrl)

import Concourse
import Concourse.Pagination exposing (Page)
import Network.Pagination
import Url.Builder


type Endpoint
    = Job Concourse.JobIdentifier
    | Jobs Concourse.PipelineIdentifier
    | Builds Concourse.JobIdentifier (Maybe Page)


toUrl : Endpoint -> String
toUrl endpoint =
    let
        basePath =
            [ "api", "v1" ]
    in
    case endpoint of
        Job { teamName, pipelineName, jobName } ->
            Url.Builder.absolute
                (basePath
                    ++ [ "teams"
                       , teamName
                       , "pipelines"
                       , pipelineName
                       , "jobs"
                       , jobName
                       ]
                )
                []

        Jobs { teamName, pipelineName } ->
            Url.Builder.absolute
                (basePath
                    ++ [ "teams"
                       , teamName
                       , "pipelines"
                       , pipelineName
                       , "jobs"
                       ]
                )
                []

        Builds { teamName, pipelineName, jobName } page ->
            Url.Builder.absolute
                (basePath
                    ++ [ "teams"
                       , teamName
                       , "pipelines"
                       , pipelineName
                       , "jobs"
                       , jobName
                       , "builds"
                       ]
                )
                (Network.Pagination.params page)
