module Api.Endpoints exposing (Endpoint(..), toUrl)

import Concourse
import Url.Builder


type Endpoint
    = Job Concourse.JobIdentifier
    | Jobs Concourse.PipelineIdentifier


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
