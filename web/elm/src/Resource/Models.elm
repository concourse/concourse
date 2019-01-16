module Resource.Models exposing
    ( CheckStatus(..)
    , Hoverable(..)
    , Model
    , PageError(..)
    , Version
    , VersionToggleAction(..)
    )

import BoolTransitionable
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
    , checkStatus : CheckStatus
    , checkError : String
    , checkSetupError : String
    , lastChecked : Maybe Date
    , pinnedVersion : ResourcePinState Concourse.Version Int
    , now : Maybe Time.Time
    , resourceIdentifier : Concourse.ResourceIdentifier
    , currentPage : Maybe Page
    , hovered : Hoverable
    , versions : Paginated Version
    , csrfToken : String
    , showPinBarTooltip : Bool
    , pinIconHover : Bool
    , route : Routes.ConcourseRoute
    , pipeline : Maybe Concourse.Pipeline
    , userState : UserState
    , userMenuVisible : Bool
    , pinnedResources : List ( String, Concourse.Version )
    , showPinIconDropDown : Bool
    }


type alias Version =
    { id : Int
    , version : Concourse.Version
    , metadata : Concourse.Metadata
    , enabled : BoolTransitionable.BoolTransitionable
    , expanded : Bool
    , inputTo : List Concourse.Build
    , outputOf : List Concourse.Build
    , showTooltip : Bool
    }
