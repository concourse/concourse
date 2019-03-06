module Callback exposing (Callback(..))

import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Dashboard.APIData
import Http
import Json.Encode
import Resource.Models exposing (VersionId, VersionToggleAction)
import Time exposing (Time)
import Window


type Callback
    = EmptyCallback
    | GotCurrentTime Time
    | BuildTriggered (Result Http.Error Concourse.Build)
    | JobBuildsFetched (Result Http.Error (Paginated Concourse.Build))
    | JobFetched (Result Http.Error Concourse.Job)
    | JobsFetched (Result Http.Error Json.Encode.Value)
    | PipelineFetched (Result Http.Error Concourse.Pipeline)
    | UserFetched (Result Http.Error Concourse.User)
    | ResourcesFetched (Result Http.Error Json.Encode.Value)
    | BuildResourcesFetched (Result Http.Error ( Int, Concourse.BuildResources ))
    | ResourceFetched (Result Http.Error Concourse.Resource)
    | VersionedResourcesFetched (Result Http.Error ( Maybe Page, Paginated Concourse.VersionedResource ))
    | VersionFetched (Result Http.Error String)
    | PausedToggled (Result Http.Error ())
    | InputToFetched (Result Http.Error ( VersionId, List Concourse.Build ))
    | OutputOfFetched (Result Http.Error ( VersionId, List Concourse.Build ))
    | VersionPinned (Result Http.Error ())
    | VersionUnpinned (Result Http.Error ())
    | VersionToggled VersionToggleAction VersionId (Result Http.Error ())
    | Checked (Result Http.Error ())
    | CommentSet (Result Http.Error ())
    | TokenSentToFly Bool
    | APIDataFetched (Result Http.Error ( Time.Time, Dashboard.APIData.APIData ))
    | LoggedOut (Result Http.Error ())
    | ScreenResized Window.Size
    | BuildJobDetailsFetched (Result Http.Error Concourse.Job)
    | BuildFetched (Result Http.Error ( Int, Concourse.Build ))
    | BuildPrepFetched (Result Http.Error ( Int, Concourse.BuildPrep ))
    | BuildHistoryFetched (Result Http.Error (Paginated Concourse.Build))
    | PlanAndResourcesFetched Int (Result Http.Error ( Concourse.BuildPlan, Concourse.BuildResources ))
    | BuildAborted (Result Http.Error ())
    | BuildsScrolled
