module ApiEndpointsTests exposing (testEndpoints, testToString)

import Api.Endpoints as E exposing (Endpoint(..), toString)
import Expect
import Test exposing (Test, describe, test)
import Url.Builder


testEndpoints : Test
testEndpoints =
    describe "ApiEndpoints" <|
        let
            toPath =
                toString []
        in
        [ test "PipelinesList" <|
            \_ ->
                PipelinesList
                    |> toPath
                    |> Expect.equal "/api/v1/pipelines"
        , describe "Pipeline" <|
            let
                basePipelineEndpoint =
                    Pipeline
                        { pipelineName = "pipeline"
                        , teamName = "team"
                        }
            in
            [ test "Base" <|
                \_ ->
                    E.BasePipeline
                        |> basePipelineEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline"
            , test "Pause" <|
                \_ ->
                    E.PausePipeline
                        |> basePipelineEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/pause"
            , test "Unpause" <|
                \_ ->
                    E.UnpausePipeline
                        |> basePipelineEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/unpause"
            , test "Expose" <|
                \_ ->
                    E.ExposePipeline
                        |> basePipelineEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/expose"
            , test "Hide" <|
                \_ ->
                    E.HidePipeline
                        |> basePipelineEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/hide"
            , test "JobsList" <|
                \_ ->
                    E.PipelineJobsList
                        |> basePipelineEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs"
            , test "ResourcesList" <|
                \_ ->
                    E.PipelineResourcesList
                        |> basePipelineEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources"
            ]
        , test "JobsList" <|
            \_ ->
                JobsList
                    |> toPath
                    |> Expect.equal "/api/v1/jobs"
        , describe "Job" <|
            let
                baseJobEndpoint =
                    Job
                        { jobName = "job"
                        , pipelineName = "pipeline"
                        , teamName = "team"
                        }
            in
            [ test "Base" <|
                \_ ->
                    E.BaseJob
                        |> baseJobEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs/job"
            , test "Pause" <|
                \_ ->
                    E.PauseJob
                        |> baseJobEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs/job/pause"
            , test "Unpause" <|
                \_ ->
                    E.UnpauseJob
                        |> baseJobEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs/job/unpause"
            , test "BuildsList" <|
                \_ ->
                    E.JobBuildsList
                        |> baseJobEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs/job/builds"
            ]
        , test "JobBuild" <|
            \_ ->
                JobBuild
                    { buildName = "build"
                    , jobName = "job"
                    , pipelineName = "pipeline"
                    , teamName = "team"
                    }
                    |> toPath
                    |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/jobs/job/builds/build"
        , describe "Build" <|
            let
                baseBuildEndpoint =
                    Build 1
            in
            [ test "Base" <|
                \_ ->
                    E.BaseBuild
                        |> baseBuildEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/builds/1"
            , test "Plan" <|
                \_ ->
                    E.BuildPlan
                        |> baseBuildEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/builds/1/plan"
            , test "Prep" <|
                \_ ->
                    E.BuildPrep
                        |> baseBuildEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/builds/1/preparation"
            , test "Abort" <|
                \_ ->
                    E.AbortBuild
                        |> baseBuildEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/builds/1/abort"
            , test "ResourcesList" <|
                \_ ->
                    E.BuildResourcesList
                        |> baseBuildEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/builds/1/resources"
            , test "EventStream" <|
                \_ ->
                    E.BuildEventStream
                        |> baseBuildEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/builds/1/events"
            ]
        , test "ResourcesList" <|
            \_ ->
                ResourcesList
                    |> toPath
                    |> Expect.equal "/api/v1/resources"
        , describe "Resource" <|
            let
                baseResourceEndpoint =
                    Resource
                        { resourceName = "resource"
                        , pipelineName = "pipeline"
                        , teamName = "team"
                        }
            in
            [ test "Base" <|
                \_ ->
                    E.BaseResource
                        |> baseResourceEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource"
            , test "VersionsList" <|
                \_ ->
                    E.ResourceVersionsList
                        |> baseResourceEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource/versions"
            , test "Unpin" <|
                \_ ->
                    E.UnpinResource
                        |> baseResourceEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource/unpin"
            , test "Check" <|
                \_ ->
                    E.CheckResource
                        |> baseResourceEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource/check"
            , test "PinComment" <|
                \_ ->
                    E.PinResourceComment
                        |> baseResourceEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource/pin_comment"
            ]
        , describe "ResourceVersion" <|
            let
                baseVersionEndpoint =
                    ResourceVersion
                        { versionID = 1
                        , resourceName = "resource"
                        , pipelineName = "pipeline"
                        , teamName = "team"
                        }
            in
            [ test "InputTo" <|
                \_ ->
                    E.ResourceVersionInputTo
                        |> baseVersionEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource/versions/1/input_to"
            , test "OutputOf" <|
                \_ ->
                    E.ResourceVersionOutputOf
                        |> baseVersionEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource/versions/1/output_of"
            , test "Pin" <|
                \_ ->
                    E.PinResourceVersion
                        |> baseVersionEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource/versions/1/pin"
            , test "Enable" <|
                \_ ->
                    E.EnableResourceVersion
                        |> baseVersionEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource/versions/1/enable"
            , test "Disable" <|
                \_ ->
                    E.DisableResourceVersion
                        |> baseVersionEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/pipeline/resources/resource/versions/1/disable"
            ]
        , test "Check" <|
            \_ ->
                Check 1
                    |> toPath
                    |> Expect.equal "/api/v1/checks/1"
        , test "TeamsList" <|
            \_ ->
                TeamsList
                    |> toPath
                    |> Expect.equal "/api/v1/teams"
        , describe "Team" <|
            let
                baseTeamEndpoint =
                    Team "team"
            in
            [ test "PipelinesList" <|
                \_ ->
                    E.TeamPipelinesList
                        |> baseTeamEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines"
            , test "OrderPipelines" <|
                \_ ->
                    E.OrderTeamPipelines
                        |> baseTeamEndpoint
                        |> toPath
                        |> Expect.equal "/api/v1/teams/team/pipelines/ordering"
            ]
        , test "ClusterInfo" <|
            \_ ->
                ClusterInfo
                    |> toPath
                    |> Expect.equal "/api/v1/info"
        , test "Cli" <|
            \_ ->
                Cli
                    |> toPath
                    |> Expect.equal "/api/v1/cli"
        , test "UserInfo" <|
            \_ ->
                UserInfo
                    |> toPath
                    |> Expect.equal "/api/v1/user"
        , test "Logout" <|
            \_ ->
                Logout
                    |> toPath
                    |> Expect.equal "/sky/logout"
        ]


testToString : Test
testToString =
    describe "toString"
        [ test "adds query params" <|
            \_ ->
                Logout
                    |> toString [ Url.Builder.string "hello" "world" ]
                    |> Expect.equal "/sky/logout?hello=world"
        ]
