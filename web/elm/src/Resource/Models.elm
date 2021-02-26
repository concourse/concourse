module Resource.Models exposing
    ( CheckStatus(..)
    , Model
    , PageError(..)
    , PinnedVersion
    , Version
    , VersionEnabledState(..)
    , VersionId
    )

import Build.Output.Models exposing (OutputModel)
import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Login.Login as Login
import Pinned exposing (CommentState, ResourcePinState)
import Routes
import Time


type PageError
    = Empty
    | NotFound


type CheckStatus
    = CheckPending
    | CurrentlyChecking Int
    | NotChecking


type alias Model =
    Login.Model
        { pageStatus : Result PageError ()
        , checkStatus : CheckStatus
        , lastChecked : Maybe Time.Posix
        , pinnedVersion : PinnedVersion
        , now : Maybe Time.Posix
        , resourceIdentifier : Concourse.ResourceIdentifier
        , currentPage : Page
        , versions : Paginated Version
        , pinCommentLoading : Bool
        , textAreaFocused : Bool
        , icon : Maybe String
        , isEditing : Bool
        , build : Maybe Concourse.Build
        , authorized : Bool
        , output : Maybe OutputModel
        , highlight : Routes.Highlight
        , highlightVersion : Maybe Concourse.Version
        }


type alias PinnedVersion =
    ResourcePinState Concourse.Version VersionId CommentState


type VersionEnabledState
    = Enabled
    | Changing
    | Disabled


type alias VersionId =
    Concourse.VersionedResourceIdentifier


type alias Version =
    { id : VersionId
    , version : Concourse.Version
    , metadata : Concourse.Metadata
    , enabled : VersionEnabledState
    , expanded : Bool
    , inputTo : List Concourse.Build
    , outputOf : List Concourse.Build
    }
