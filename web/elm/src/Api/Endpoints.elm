module Api.Endpoints exposing (Endpoint(..), toPath)

import Concourse


type Endpoint
    = Job Concourse.JobIdentifier
    | Jobs Concourse.PipelineIdentifier
    | AllJobs
    | PauseJob Concourse.JobIdentifier
    | UnpauseJob Concourse.JobIdentifier
    | JobBuild Concourse.JobBuildIdentifier
    | JobBuilds Concourse.JobIdentifier
    | Build Concourse.BuildId
    | BuildPlan Concourse.BuildId
    | BuildPrep Concourse.BuildId
    | AbortBuild Concourse.BuildId
    | Resource Concourse.ResourceIdentifier
    | ResourceVersions Concourse.ResourceIdentifier
    | ResourceVersionInputTo Concourse.VersionedResourceIdentifier
    | ResourceVersionOutputOf Concourse.VersionedResourceIdentifier
    | PinResourceVersion Concourse.VersionedResourceIdentifier
    | UnpinResource Concourse.ResourceIdentifier
    | EnableResourceVersion Concourse.VersionedResourceIdentifier
    | DisableResourceVersion Concourse.VersionedResourceIdentifier
    | CheckResource Concourse.ResourceIdentifier
    | PinResourceComment Concourse.ResourceIdentifier
    | Resources Concourse.PipelineIdentifier
    | BuildResources Concourse.BuildId
    | AllResources
    | Check Int
    | AllPipelines
    | Pipeline Concourse.PipelineIdentifier
    | PausePipeline Concourse.PipelineIdentifier
    | UnpausePipeline Concourse.PipelineIdentifier
    | ExposePipeline Concourse.PipelineIdentifier
    | HidePipeline Concourse.PipelineIdentifier
    | AllTeams
    | TeamPipelines Concourse.TeamName
    | OrderTeamPipelines Concourse.TeamName
    | ClusterInfo
    | UserInfo
    | Logout


toPath : Endpoint -> List String
toPath endpoint =
    let
        basePath =
            [ "api", "v1" ]

        pipelinePath { pipelineName, teamName } =
            basePath ++ [ "teams", teamName, "pipelines", pipelineName ]

        resourcePath { pipelineName, teamName, resourceName } =
            pipelinePath { pipelineName = pipelineName, teamName = teamName }
                ++ [ "resources", resourceName ]

        baseSkyPath =
            [ "sky" ]
    in
    case endpoint of
        Job id ->
            pipelinePath id ++ [ "jobs", id.jobName ]

        Jobs id ->
            pipelinePath id ++ [ "jobs" ]

        AllJobs ->
            basePath ++ [ "jobs" ]

        PauseJob id ->
            pipelinePath id ++ [ "jobs", id.jobName, "pause" ]

        UnpauseJob id ->
            pipelinePath id ++ [ "jobs", id.jobName, "unpause" ]

        JobBuild id ->
            pipelinePath id ++ [ "jobs", id.jobName, "builds", id.buildName ]

        JobBuilds id ->
            pipelinePath id ++ [ "jobs", id.jobName, "builds" ]

        Build id ->
            basePath ++ [ "builds", String.fromInt id ]

        BuildPlan id ->
            basePath ++ [ "builds", String.fromInt id, "plan" ]

        BuildPrep id ->
            basePath ++ [ "builds", String.fromInt id, "preparation" ]

        AbortBuild id ->
            basePath ++ [ "builds", String.fromInt id, "abort" ]

        Resource id ->
            resourcePath id

        ResourceVersions id ->
            resourcePath id ++ [ "versions" ]

        ResourceVersionInputTo id ->
            resourcePath id ++ [ "versions", String.fromInt id.versionID, "input_to" ]

        ResourceVersionOutputOf id ->
            resourcePath id ++ [ "versions", String.fromInt id.versionID, "output_of" ]

        PinResourceVersion id ->
            resourcePath id ++ [ "versions", String.fromInt id.versionID, "pin" ]

        UnpinResource id ->
            resourcePath id ++ [ "unpin" ]

        EnableResourceVersion id ->
            resourcePath id ++ [ "versions", String.fromInt id.versionID, "enable" ]

        DisableResourceVersion id ->
            resourcePath id ++ [ "versions", String.fromInt id.versionID, "disable" ]

        CheckResource id ->
            resourcePath id ++ [ "check" ]

        PinResourceComment id ->
            resourcePath id ++ [ "pin_comment" ]

        Resources id ->
            pipelinePath id ++ [ "resources" ]

        BuildResources id ->
            basePath ++ [ "builds", String.fromInt id, "resources" ]

        AllResources ->
            basePath ++ [ "resources" ]

        Check id ->
            basePath ++ [ "checks", String.fromInt id ]

        AllPipelines ->
            basePath ++ [ "pipelines" ]

        Pipeline id ->
            pipelinePath id

        PausePipeline id ->
            pipelinePath id ++ [ "pause" ]

        UnpausePipeline id ->
            pipelinePath id ++ [ "unpause" ]

        ExposePipeline id ->
            pipelinePath id ++ [ "expose" ]

        HidePipeline id ->
            pipelinePath id ++ [ "hide" ]

        AllTeams ->
            basePath ++ [ "teams" ]

        TeamPipelines teamName ->
            basePath ++ [ "teams", teamName, "pipelines" ]

        OrderTeamPipelines teamName ->
            basePath ++ [ "teams", teamName, "pipelines", "ordering" ]

        ClusterInfo ->
            basePath ++ [ "info" ]

        UserInfo ->
            baseSkyPath ++ [ "userinfo" ]

        Logout ->
            baseSkyPath ++ [ "logout" ]
