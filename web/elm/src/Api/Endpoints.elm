module Api.Endpoints exposing (Endpoint(..), toUrl)

import Concourse
import Url.Builder


type Endpoint
    = Job Concourse.JobIdentifier


toUrl : Endpoint -> String
toUrl endpoint =
    case endpoint of
        Job { teamName, pipelineName, jobName } ->
            Url.Builder.absolute
                [ "api"
                , "v1"
                , "teams"
                , teamName
                , "pipelines"
                , pipelineName
                , "jobs"
                , jobName
                ]
                []
