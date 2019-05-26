module ResourceVersion.Models exposing
    ( CheckStatus(..)
    , Model
    , PageError(..)
    , PinnedVersion
    , Version
    , VersionEnabledState(..)
    , VersionId
    )

import Concourse
import Login.Login as Login
import Pinned exposing (CommentState, ResourcePinState)
import Time


type PageError
    = Empty
    | NotFound


type CheckStatus
    = CheckingSuccessfully
    | CurrentlyChecking
    | FailingToCheck


type alias Model =
    Login.Model
        { pageStatus : Result PageError ()
        , checkStatus : CheckStatus
        , checkError : String
        , checkSetupError : String
        , lastChecked : Maybe Time.Posix
        , pinnedVersion : PinnedVersion
        , now : Maybe Time.Posix
        , resourceVersionIdentifier : Concourse.VersionedResourceIdentifier
        , version : Maybe Version
        , pinCommentLoading : Bool
        , textAreaFocused : Bool
        , icon : Maybe String
        , timeZone : Time.Zone
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
