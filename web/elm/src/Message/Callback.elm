module Message.Callback exposing (Callback(..))

import Browser.Dom
import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Http
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
    | BuildCommentSet Int String (Fetched ())
    | JobBuildsFetched (Fetched ( Page, Paginated Concourse.Build ))
    | JobFetched (Fetched Concourse.Job)
    | JobsFetched (Fetched (List Concourse.Job))
    | PipelineFetched (Fetched Concourse.Pipeline)
    | PipelinesFetched (Fetched (List Concourse.Pipeline))
    | PipelineToggled Concourse.PipelineIdentifier (Fetched ())
    | PipelinesOrdered Concourse.TeamName (Fetched ())
    | UserFetched (Fetched Concourse.User)
    | ResourcesFetched (Fetched (List Concourse.Resource))
    | BuildResourcesFetched (Fetched ( Int, Concourse.BuildResources ))
    | ResourceFetched (Fetched Concourse.Resource)
    | VersionedResourceFetched (Fetched Concourse.VersionedResource)
    | VersionedResourcesFetched (Fetched ( Page, Paginated Concourse.VersionedResource ))
    | VersionedResourceIdFetched (Fetched (Maybe Concourse.VersionedResource))
    | ClusterInfoFetched (Fetched Concourse.ClusterInfo)
    | WallFetched (Fetched Concourse.Wall)
    | PausedToggled (Fetched ())
    | InputToFetched (Fetched ( VersionId, List Concourse.Build ))
    | OutputOfFetched (Fetched ( VersionId, List Concourse.Build ))
    | CausalityFetched (Fetched ( Concourse.CausalityDirection, Maybe Concourse.Causality ))
    | VersionPinned (Fetched ())
    | VersionUnpinned (Fetched ())
    | VersionToggled VersionToggleAction VersionId (Fetched ())
    | Checked (Fetched Concourse.Build)
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
