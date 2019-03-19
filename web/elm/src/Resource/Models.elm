module Resource.Models exposing
    ( CheckStatus(..)
    , Model
    , PageError(..)
    , PinnedVersion
    , Version
    , VersionEnabledState(..)
    , VersionId
    )

import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Date exposing (Date)
import Message.Message exposing (HoverableRes)
import Pinned exposing (CommentState, ResourcePinState)
import Time
import TopBar.Model


type PageError
    = Empty
    | NotFound


type CheckStatus
    = CheckingSuccessfully
    | CurrentlyChecking
    | FailingToCheck


type alias Model =
    TopBar.Model.Model
        { pageStatus : Result PageError ()
        , checkStatus : CheckStatus
        , checkError : String
        , checkSetupError : String
        , lastChecked : Maybe Date
        , pinnedVersion : PinnedVersion
        , now : Maybe Time.Time
        , resourceIdentifier : Concourse.ResourceIdentifier
        , currentPage : Maybe Page
        , hovered : Maybe HoverableRes
        , versions : Paginated Version
        , showPinBarTooltip : Bool
        , pinIconHover : Bool
        , pinCommentLoading : Bool
        , ctrlDown : Bool
        , textAreaFocused : Bool
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
    , showTooltip : Bool
    }
