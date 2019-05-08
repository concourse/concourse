module Message.Callback exposing (Callback(..))

import Browser.Dom
import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Http
import Json.Encode
import Message.Message
    exposing
        ( VersionId
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
    | JobBuildsFetched (Fetched (Paginated Concourse.Build))
    | JobFetched (Fetched Concourse.Job)
    | JobsFetched (Fetched Json.Encode.Value)
    | PipelineFetched (Fetched Concourse.Pipeline)
    | PipelineToggled Concourse.PipelineIdentifier (Fetched ())
    | UserFetched (Fetched Concourse.User)
    | ResourcesFetched (Fetched Json.Encode.Value)
    | BuildResourcesFetched (Fetched ( Int, Concourse.BuildResources ))
    | ResourceFetched (Fetched Concourse.Resource)
    | VersionedResourcesFetched (Fetched ( Maybe Page, Paginated Concourse.VersionedResource ))
    | VersionFetched (Fetched String)
    | PausedToggled (Fetched ())
    | InputToFetched (Fetched ( VersionId, List Concourse.Build ))
    | OutputOfFetched (Fetched ( VersionId, List Concourse.Build ))
    | VersionPinned (Fetched ())
    | VersionUnpinned (Fetched ())
    | VersionToggled VersionToggleAction VersionId (Fetched ())
    | Checked (Fetched ())
    | CommentSet (Fetched ())
    | TokenSentToFly (Fetched ())
    | APIDataFetched (Fetched ( Time.Posix, Concourse.APIData ))
    | LoggedOut (Fetched ())
    | ScreenResized Browser.Dom.Viewport
    | BuildJobDetailsFetched (Fetched Concourse.Job)
    | BuildFetched (Fetched ( Int, Concourse.Build ))
    | BuildPrepFetched (Fetched ( Int, Concourse.BuildPrep ))
    | BuildHistoryFetched (Fetched (Paginated Concourse.Build))
    | PlanAndResourcesFetched Int (Fetched ( Concourse.BuildPlan, Concourse.BuildResources ))
    | BuildAborted (Fetched ())
    | VisibilityChanged VisibilityAction Concourse.PipelineIdentifier (Fetched ())
    | PipelinesFetched (Fetched (List Concourse.Pipeline))
