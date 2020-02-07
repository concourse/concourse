module Message.Callback exposing
    ( ApiEntity(..)
    , Callback(..)
    , HttpMethod(..)
    , Route(..)
    , TooltipPolicy(..)
    )

import Browser.Dom
import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Http
import Json.Encode
import Message.Message
    exposing
        ( DomID
        , VersionId
        , VersionToggleAction
        , VisibilityAction
        )
import Time


type alias Fetched a =
    Result Http.Error a


type Route
    = RouteJob Concourse.JobIdentifier
    | RouteJobs Concourse.PipelineIdentifier
    | RouteJobBuild Concourse.JobBuildIdentifier
    | RouteJobBuilds Concourse.JobIdentifier (Maybe Page)


type HttpMethod
    = GET
    | POST


type ApiEntity
    = Job Concourse.Job
    | Jobs Json.Encode.Value
    | Builds (Paginated Concourse.Build)
    | Build Concourse.Build


type Callback
    = EmptyCallback
    | ApiResponse Route HttpMethod (Fetched ApiEntity)
    | GotCurrentTime Time.Posix
    | GotCurrentTimeZone Time.Zone
    | PipelineFetched (Fetched Concourse.Pipeline)
    | PipelinesFetched (Fetched (List Concourse.Pipeline))
    | PipelineToggled Concourse.PipelineIdentifier (Fetched ())
    | PipelinesOrdered String (Fetched ())
    | UserFetched (Fetched Concourse.User)
    | ResourcesFetched (Fetched Json.Encode.Value)
    | BuildResourcesFetched (Fetched ( Int, Concourse.BuildResources ))
    | ResourceFetched (Fetched Concourse.Resource)
    | VersionedResourcesFetched (Fetched ( Maybe Page, Paginated Concourse.VersionedResource ))
    | ClusterInfoFetched (Fetched Concourse.ClusterInfo)
    | PausedToggled (Fetched ())
    | InputToFetched (Fetched ( VersionId, List Concourse.Build ))
    | OutputOfFetched (Fetched ( VersionId, List Concourse.Build ))
    | VersionPinned (Fetched ())
    | VersionUnpinned (Fetched ())
    | VersionToggled VersionToggleAction VersionId (Fetched ())
    | Checked (Fetched Concourse.Check)
    | CommentSet (Fetched ())
    | AllTeamsFetched (Fetched (List Concourse.Team))
    | AllJobsFetched (Fetched (List Concourse.Job))
    | AllResourcesFetched (Fetched (List Concourse.Resource))
    | LoggedOut (Fetched ())
    | ScreenResized Browser.Dom.Viewport
    | BuildFetched (Fetched Concourse.Build)
    | BuildPrepFetched Concourse.BuildId (Fetched Concourse.BuildPrep)
    | BuildHistoryFetched (Fetched (Paginated Concourse.Build))
    | PlanAndResourcesFetched Int (Fetched ( Concourse.BuildPlan, Concourse.BuildResources ))
    | BuildAborted (Fetched ())
    | VisibilityChanged VisibilityAction Concourse.PipelineIdentifier (Fetched ())
    | AllPipelinesFetched (Fetched (List Concourse.Pipeline))
    | GotViewport DomID TooltipPolicy (Result Browser.Dom.Error Browser.Dom.Viewport)
    | GotElement (Result Browser.Dom.Error Browser.Dom.Element)


type TooltipPolicy
    = AlwaysShow
    | OnlyShowWhenOverflowing
