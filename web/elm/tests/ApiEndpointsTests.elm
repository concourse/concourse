module ApiEndpointsTests exposing (all)

import Api.Endpoints exposing (Endpoint(..), toUrl)
import Expect
import Test exposing (Test, describe, test)


all : Test
all =
    describe "ApiEndpoints"
        [ test "Job" <|
            \_ ->
                Job
                    { jobName = "job"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toUrl
                    |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs/job"
        ]
