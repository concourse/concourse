module Api.Endpoints exposing (Endpoint(..), toUrl)

import Concourse
import Concourse.Pagination exposing (Page)
import Network.Pagination
import Url.Builder


type Endpoint
    = Job Concourse.JobIdentifier
    | Jobs Concourse.PipelineIdentifier
    | AllJobs
    | JobBuild Concourse.JobBuildIdentifier
    | JobBuilds Concourse.JobIdentifier (Maybe Page)
    | Build Concourse.BuildId
    | BuildPlan Concourse.BuildId
    | BuildPrep Concourse.BuildId
    | Resource Concourse.ResourceIdentifier
    | ResourceVersions Concourse.ResourceIdentifier (Maybe Page)
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


toUrl : Endpoint -> String
toUrl endpoint =
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
            Url.Builder.absolute
                (pipelinePath id ++ [ "jobs", id.jobName ])
                []

        Jobs id ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "jobs" ])
                []

        AllJobs ->
            Url.Builder.absolute
                (basePath ++ [ "jobs" ])
                []

        JobBuild id ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "jobs", id.jobName, "builds", id.buildName ])
                []

        JobBuilds id page ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "jobs", id.jobName, "builds" ])
                (Network.Pagination.params page)

        Build id ->
            Url.Builder.absolute
                (basePath ++ [ "builds", String.fromInt id ])
                []

        BuildPlan id ->
            Url.Builder.absolute
                (basePath ++ [ "builds", String.fromInt id, "plan" ])
                []

        BuildPrep id ->
            Url.Builder.absolute
                (basePath ++ [ "builds", String.fromInt id, "preparation" ])
                []

        Resource id ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "resources", id.resourceName ])
                []

        ResourceVersions id page ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "resources", id.resourceName, "versions" ])
                (Network.Pagination.params page)

        ResourceVersionInputTo id ->
            Url.Builder.absolute
                (pipelinePath id
                    ++ [ "resources"
                       , id.resourceName
                       , "versions"
                       , String.fromInt id.versionID
                       , "input_to"
                       ]
                )
                []

        ResourceVersionOutputOf id ->
            Url.Builder.absolute
                (pipelinePath id
                    ++ [ "resources"
                       , id.resourceName
                       , "versions"
                       , String.fromInt id.versionID
                       , "output_of"
                       ]
                )
                []

        Resources id ->
            Url.Builder.absolute
                (pipelinePath id ++ [ "resources" ])
                []

        BuildResources id ->
            Url.Builder.absolute
                (basePath ++ [ "builds", String.fromInt id, "resources" ])
                []

        AllResources ->
            Url.Builder.absolute
                (basePath ++ [ "resources" ])
                []

        Check id ->
            Url.Builder.absolute
                (basePath ++ [ "checks", String.fromInt id ])
                []

        AllPipelines ->
            Url.Builder.absolute
                (basePath ++ [ "pipelines" ])
                []

        Pipeline id ->
            Url.Builder.absolute
                (pipelinePath id)
                []

        AllTeams ->
            Url.Builder.absolute
                (basePath ++ [ "teams" ])
                []

        TeamPipelines teamName ->
            Url.Builder.absolute
                (basePath ++ [ "teams", teamName, "pipelines" ])
                []

        ClusterInfo ->
            Url.Builder.absolute
                (basePath ++ [ "info" ])
                []

        UserInfo ->
            Url.Builder.absolute
                (baseSkyPath ++ [ "userinfo" ])
                []

        Logout ->
            Url.Builder.absolute
                (baseSkyPath ++ [ "logout" ])
                []
