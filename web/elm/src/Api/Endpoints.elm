module Api.Endpoints exposing (Endpoint(..), toPath)

import Concourse
import Network.Pagination
import Url.Builder


type Endpoint
    = Job Concourse.JobIdentifier
    | Jobs Concourse.PipelineIdentifier
    | AllJobs
    | JobBuild Concourse.JobBuildIdentifier
    | JobBuilds Concourse.JobIdentifier
    | Build Concourse.BuildId
    | BuildPlan Concourse.BuildId
    | BuildPrep Concourse.BuildId
    | Resource Concourse.ResourceIdentifier
    | ResourceVersions Concourse.ResourceIdentifier
    | ResourceVersionInputTo Concourse.VersionedResourceIdentifier
    | ResourceVersionOutputOf Concourse.VersionedResourceIdentifier
    | Resources Concourse.PipelineIdentifier
    | BuildResources Concourse.BuildId
    | AllResources
    | Check Int
    | AllPipelines
    | Pipeline Concourse.PipelineIdentifier
    | AllTeams
    | TeamPipelines Concourse.TeamName
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

        Resource id ->
            pipelinePath id ++ [ "resources", id.resourceName ]

        ResourceVersions id ->
            pipelinePath id ++ [ "resources", id.resourceName, "versions" ]

        ResourceVersionInputTo id ->
            pipelinePath id
                ++ [ "resources"
                   , id.resourceName
                   , "versions"
                   , String.fromInt id.versionID
                   , "input_to"
                   ]

        ResourceVersionOutputOf id ->
            pipelinePath id
                ++ [ "resources"
                   , id.resourceName
                   , "versions"
                   , String.fromInt id.versionID
                   , "output_of"
                   ]

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

        AllTeams ->
            basePath ++ [ "teams" ]

        TeamPipelines teamName ->
            basePath ++ [ "teams", teamName, "pipelines" ]

        ClusterInfo ->
            basePath ++ [ "info" ]

        UserInfo ->
            baseSkyPath ++ [ "userinfo" ]

        Logout ->
            baseSkyPath ++ [ "logout" ]
