module Resource.Models exposing
    ( CheckStatus(..)
    , Hoverable(..)
    , Model
    , PageError(..)
    , PinnedVersion
    , Version
    , VersionEnabledState(..)
    , VersionId
    , VersionToggleAction(..)
    )

import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Date exposing (Date)
import Pinned exposing (ResourcePinState)
import Routes
import Time
import UserState exposing (UserState)


type Hoverable
    = PreviousPage
    | NextPage
    | CheckButton
    | None


type VersionToggleAction
    = Enable
    | Disable


type PageError
    = Empty
    | NotFound


type CheckStatus
    = CheckingSuccessfully
    | CurrentlyChecking
    | FailingToCheck


type alias Model =
    { pageStatus : Result PageError ()
    , teamName : String
    , pipelineName : String
    , name : String
    , type_ : String
    , checkStatus : CheckStatus
    , checkError : String
    , checkSetupError : String
    , lastChecked : Maybe Date
    , pinnedVersion : PinnedVersion
    , now : Maybe Time.Time
    , resourceIdentifier : Concourse.ResourceIdentifier
    , currentPage : Maybe Page
    , hovered : Hoverable
    , versions : Paginated Version
    , csrfToken : String
    , showPinBarTooltip : Bool
    , pinIconHover : Bool
    , route : Routes.Route
    , pipeline : Maybe Concourse.Pipeline
    , userState : UserState
    , userMenuVisible : Bool
    , pinnedResources : List ( String, Concourse.Version )
    , showPinIconDropDown : Bool
    , pinComment : Maybe String
    }


type alias PinnedVersion =
    ResourcePinState Concourse.Version VersionId


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
    , showTooltip : Bool
    }
