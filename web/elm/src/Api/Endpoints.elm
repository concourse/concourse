module Api.Endpoints exposing
    ( BuildEndpoint(..)
    , Endpoint(..)
    , JobEndpoint(..)
    , PipelineEndpoint(..)
    , ResourceEndpoint(..)
    , ResourceVersionEndpoint(..)
    , TeamEndpoint(..)
    , toString
    )

import Concourse exposing (DatabaseID)
import Url.Builder


type Endpoint
    = PipelinesList
    | Pipeline Concourse.PipelineIdentifier PipelineEndpoint
    | JobsList
    | Job Concourse.JobIdentifier JobEndpoint
    | JobBuild Concourse.JobBuildIdentifier
    | Build Concourse.BuildId BuildEndpoint
    | ResourcesList
    | Resource Concourse.ResourceIdentifier ResourceEndpoint
    | ResourceVersion Concourse.VersionedResourceIdentifier ResourceVersionEndpoint
    | Check Int
    | TeamsList
    | Team Concourse.TeamName TeamEndpoint
    | ClusterInfo
    | Cli
    | UserInfo
    | Logout


type PipelineEndpoint
    = BasePipeline
    | PausePipeline
    | UnpausePipeline
    | ExposePipeline
    | HidePipeline
    | PipelineJobsList
    | PipelineResourcesList


type JobEndpoint
    = BaseJob
    | PauseJob
    | UnpauseJob
    | JobBuildsList


type BuildEndpoint
    = BaseBuild
    | BuildPlan
    | BuildPrep
    | AbortBuild
    | BuildResourcesList
    | BuildEventStream


type ResourceEndpoint
    = BaseResource
    | ResourceVersionsList
    | UnpinResource
    | CheckResource
    | PinResourceComment


type ResourceVersionEndpoint
    = ResourceVersionInputTo
    | ResourceVersionOutputOf
    | PinResourceVersion
    | EnableResourceVersion
    | DisableResourceVersion


type TeamEndpoint
    = TeamPipelinesList
    | OrderTeamPipelines


basePath : List String
basePath =
    [ "api", "v1" ]


baseSkyPath : List String
baseSkyPath =
    [ "sky" ]


pipelinePath : DatabaseID -> List String
pipelinePath pipelineId =
    basePath ++ [ "pipelines", String.fromInt pipelineId ]


resourcePath : { r | pipelineId : DatabaseID, resourceName : String } -> List String
resourcePath { pipelineId, resourceName } =
    pipelinePath pipelineId
        ++ [ "resources", resourceName ]


toString : List Url.Builder.QueryParameter -> Endpoint -> String
toString query endpoint =
    Url.Builder.absolute (toPath endpoint) query


toPath : Endpoint -> List String
toPath endpoint =
    case endpoint of
        PipelinesList ->
            basePath ++ [ "pipelines" ]

        Pipeline id subEndpoint ->
            pipelinePath id ++ pipelineEndpointToPath subEndpoint

        JobsList ->
            basePath ++ [ "jobs" ]

        Job id subEndpoint ->
            pipelinePath id.pipelineId ++ [ "jobs", id.jobName ] ++ jobEndpointToPath subEndpoint

        JobBuild id ->
            pipelinePath id.pipelineId ++ [ "jobs", id.jobName, "builds", id.buildName ]

        Build id subEndpoint ->
            basePath ++ [ "builds", String.fromInt id ] ++ buildEndpointToPath subEndpoint

        ResourcesList ->
            basePath ++ [ "resources" ]

        Resource id subEndpoint ->
            resourcePath id ++ resourceEndpointToPath subEndpoint

        ResourceVersion id subEndpoint ->
            resourcePath id ++ [ "versions", String.fromInt id.versionID ] ++ resourceVersionEndpointToPath subEndpoint

        Check id ->
            basePath ++ [ "checks", String.fromInt id ]

        TeamsList ->
            basePath ++ [ "teams" ]

        Team teamName subEndpoint ->
            basePath ++ [ "teams", teamName ] ++ teamEndpointToPath subEndpoint

        ClusterInfo ->
            basePath ++ [ "info" ]

        Cli ->
            basePath ++ [ "cli" ]

        UserInfo ->
            basePath ++ [ "user" ]

        Logout ->
            baseSkyPath ++ [ "logout" ]


pipelineEndpointToPath : PipelineEndpoint -> List String
pipelineEndpointToPath endpoint =
    case endpoint of
        BasePipeline ->
            []

        PausePipeline ->
            [ "pause" ]

        UnpausePipeline ->
            [ "unpause" ]

        ExposePipeline ->
            [ "expose" ]

        HidePipeline ->
            [ "hide" ]

        PipelineJobsList ->
            [ "jobs" ]

        PipelineResourcesList ->
            [ "resources" ]


jobEndpointToPath : JobEndpoint -> List String
jobEndpointToPath endpoint =
    case endpoint of
        BaseJob ->
            []

        PauseJob ->
            [ "pause" ]

        UnpauseJob ->
            [ "unpause" ]

        JobBuildsList ->
            [ "builds" ]


buildEndpointToPath : BuildEndpoint -> List String
buildEndpointToPath endpoint =
    case endpoint of
        BaseBuild ->
            []

        BuildPlan ->
            [ "plan" ]

        BuildPrep ->
            [ "preparation" ]

        AbortBuild ->
            [ "abort" ]

        BuildResourcesList ->
            [ "resources" ]

        BuildEventStream ->
            [ "events" ]


resourceEndpointToPath : ResourceEndpoint -> List String
resourceEndpointToPath endpoint =
    case endpoint of
        BaseResource ->
            []

        ResourceVersionsList ->
            [ "versions" ]

        UnpinResource ->
            [ "unpin" ]

        CheckResource ->
            [ "check" ]

        PinResourceComment ->
            [ "pin_comment" ]


resourceVersionEndpointToPath : ResourceVersionEndpoint -> List String
resourceVersionEndpointToPath endpoint =
    case endpoint of
        ResourceVersionInputTo ->
            [ "input_to" ]

        ResourceVersionOutputOf ->
            [ "output_of" ]

        PinResourceVersion ->
            [ "pin" ]

        EnableResourceVersion ->
            [ "enable" ]

        DisableResourceVersion ->
            [ "disable" ]


teamEndpointToPath : TeamEndpoint -> List String
teamEndpointToPath endpoint =
    case endpoint of
        TeamPipelinesList ->
            [ "pipelines" ]

        OrderTeamPipelines ->
            [ "pipelines", "ordering" ]
