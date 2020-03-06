module ApiEndpointsTests exposing (all)

import Api.Endpoints exposing (Endpoint(..), toUrl)
import Concourse.Pagination as Pagination
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
        , test "Jobs" <|
            \_ ->
                Jobs
                    { pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toUrl
                    |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs"
        , test "Builds, no page" <|
            \_ ->
                Builds
                    { jobName = "job"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    Nothing
                    |> toUrl
                    |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs/job/builds"
        , test "Builds, has page" <|
            \_ ->
                Builds
                    { jobName = "job"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    (Just { limit = 1, direction = Pagination.Since 1 })
                    |> toUrl
                    |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs/job/builds?limit=1&since=1"
        , test "Resource" <|
            \_ ->
                Resource
                    { resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toUrl
                    |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource"
        ]
