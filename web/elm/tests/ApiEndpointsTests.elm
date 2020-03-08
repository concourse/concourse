module ApiEndpointsTests exposing (all)

import Api.Endpoints exposing (Endpoint(..), toPath)
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
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "jobs", "job" ]
        , test "Jobs" <|
            \_ ->
                Jobs
                    { pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "jobs" ]
        , test "AllJobs" <|
            \_ ->
                AllJobs
                    |> toPath
                    |> Expect.equal [ "api", "v1", "jobs" ]
        , test "JobBuild" <|
            \_ ->
                JobBuild
                    { buildName = "build"
                    , jobName = "job"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "jobs", "job", "builds", "build" ]
        , test "PauseJob" <|
            \_ ->
                PauseJob
                    { jobName = "job"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "jobs", "job", "pause" ]
        , test "UnpauseJob" <|
            \_ ->
                UnpauseJob
                    { jobName = "job"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "jobs", "job", "unpause" ]
        , test "JobBuilds" <|
            \_ ->
                JobBuilds
                    { jobName = "job"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "jobs", "job", "builds" ]
        , test "Build" <|
            \_ ->
                Build 1
                    |> toPath
                    |> Expect.equal [ "api", "v1", "builds", "1" ]
        , test "BuildPlan" <|
            \_ ->
                BuildPlan 1
                    |> toPath
                    |> Expect.equal [ "api", "v1", "builds", "1", "plan" ]
        , test "BuildPrep" <|
            \_ ->
                BuildPrep 1
                    |> toPath
                    |> Expect.equal [ "api", "v1", "builds", "1", "preparation" ]
        , test "AbortBuild" <|
            \_ ->
                AbortBuild 1
                    |> toPath
                    |> Expect.equal [ "api", "v1", "builds", "1", "abort" ]
        , test "Resource" <|
            \_ ->
                Resource
                    { resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource" ]
        , test "ResourceVersions" <|
            \_ ->
                ResourceVersions
                    { resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource", "versions" ]
        , test "ResourceVersionInputTo" <|
            \_ ->
                ResourceVersionInputTo
                    { versionID = 1
                    , resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource", "versions", "1", "input_to" ]
        , test "ResourceVersionOutputOf" <|
            \_ ->
                ResourceVersionOutputOf
                    { versionID = 1
                    , resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource", "versions", "1", "output_of" ]
        , test "PinResourceVersion" <|
            \_ ->
                PinResourceVersion
                    { versionID = 1
                    , resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource", "versions", "1", "pin" ]
        , test "UnpinResource" <|
            \_ ->
                UnpinResource
                    { resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource", "unpin" ]
        , test "EnableResourceVersion" <|
            \_ ->
                EnableResourceVersion
                    { versionID = 1
                    , resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource", "versions", "1", "enable" ]
        , test "DisableResourceVersion" <|
            \_ ->
                DisableResourceVersion
                    { versionID = 1
                    , resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource", "versions", "1", "disable" ]
        , test "CheckResource" <|
            \_ ->
                CheckResource
                    { resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource", "check" ]
        , test "PinResourceComment" <|
            \_ ->
                PinResourceComment
                    { resourceName = "resource"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources", "resource", "pin_comment" ]
        , test "Resources" <|
            \_ ->
                Resources
                    { pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "resources" ]
        , test "BuildResources" <|
            \_ ->
                BuildResources 1
                    |> toPath
                    |> Expect.equal [ "api", "v1", "builds", "1", "resources" ]
        , test "AllResources" <|
            \_ ->
                AllResources
                    |> toPath
                    |> Expect.equal [ "api", "v1", "resources" ]
        , test "Check" <|
            \_ ->
                Check 1
                    |> toPath
                    |> Expect.equal [ "api", "v1", "checks", "1" ]
        , test "AllPipelines" <|
            \_ ->
                AllPipelines
                    |> toPath
                    |> Expect.equal [ "api", "v1", "pipelines" ]
        , test "Pipeline" <|
            \_ ->
                Pipeline
                    { pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline" ]
        , test "PausePipeline" <|
            \_ ->
                PausePipeline
                    { pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "pause" ]
        , test "UnpausePipeline" <|
            \_ ->
                UnpausePipeline
                    { pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "unpause" ]
        , test "ExposePipeline" <|
            \_ ->
                ExposePipeline
                    { pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "expose" ]
        , test "HidePipeline" <|
            \_ ->
                HidePipeline
                    { pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "pipeline", "hide" ]
        , test "AllTeams" <|
            \_ ->
                AllTeams
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams" ]
        , test "TeamPipelines" <|
            \_ ->
                TeamPipelines "team"
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines" ]
        , test "OrderTeamPipelines" <|
            \_ ->
                OrderTeamPipelines "team"
                    |> toPath
                    |> Expect.equal [ "api", "v1", "teams", "team", "pipelines", "ordering" ]
        , test "ClusterInfo" <|
            \_ ->
                ClusterInfo
                    |> toPath
                    |> Expect.equal [ "api", "v1", "info" ]
        , test "UserInfo" <|
            \_ ->
                UserInfo
                    |> toPath
                    |> Expect.equal [ "sky", "userinfo" ]
        , test "Logout" <|
            \_ ->
                Logout
                    |> toPath
                    |> Expect.equal [ "sky", "logout" ]
        ]
