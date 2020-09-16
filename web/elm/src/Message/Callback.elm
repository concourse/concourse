module Message.Callback exposing (Callback(..))

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


type Callback
    = EmptyCallback
    | GotCurrentTime Time.Posix
    | GotCurrentTimeZone Time.Zone
    | BuildTriggered (Fetched Concourse.Build)
    | JobBuildsFetched (Fetched ( Page, Paginated Concourse.Build ))
    | JobFetched (Fetched Concourse.Job)
    | JobsFetched (Fetched Json.Encode.Value)
    | PipelineFetched (Fetched Concourse.Pipeline)
    | PipelinesFetched (Fetched (List Concourse.Pipeline))
    | PipelineToggled Concourse.PipelineIdentifier (Fetched ())
    | PipelinesOrdered String (Fetched ())
    | UserFetched (Fetched Concourse.User)
    | ResourcesFetched (Fetched Json.Encode.Value)
    | BuildResourcesFetched (Fetched ( Int, Concourse.BuildResources ))
    | ResourceFetched (Fetched Concourse.Resource)
    | VersionedResourcesFetched (Fetched ( Page, Paginated Concourse.VersionedResource ))
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
    | BuildJobDetailsFetched (Fetched Concourse.Job)
    | BuildFetched (Fetched Concourse.Build)
    | BuildPrepFetched Concourse.BuildId (Fetched Concourse.BuildPrep)
    | BuildHistoryFetched (Fetched (Paginated Concourse.Build))
    | PlanAndResourcesFetched Int (Fetched ( Concourse.BuildPlan, Concourse.BuildResources ))
    | BuildAborted (Fetched ())
    | VisibilityChanged VisibilityAction Concourse.PipelineIdentifier (Fetched ())
    | AllPipelinesFetched (Fetched (List Concourse.Pipeline))
    | GotViewport DomID (Result Browser.Dom.Error Browser.Dom.Viewport)
    | GotElement (Result Browser.Dom.Error Browser.Dom.Element)
