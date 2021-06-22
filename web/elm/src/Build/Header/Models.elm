module Build.Header.Models exposing
    ( BuildPageType(..)
    , CommentBarVisibility(..)
    , CurrentOutput(..)
    , HistoryItem
    , Model
    , commentBarIsVisible
    )

import Build.Output.Models exposing (OutputModel)
import Concourse
import Concourse.BuildStatus as BuildStatus
import Concourse.Pagination exposing (Page)
import Time
import Views.CommentBar as CommentBar


type alias Model r =
    { r
        | id : Int
        , name : String
        , authorized : Bool
        , job : Maybe Concourse.JobIdentifier
        , scrolledToCurrentBuild : Bool
        , history : List HistoryItem
        , duration : Concourse.BuildDuration
        , createdBy : Concourse.BuildCreatedBy
        , status : BuildStatus.BuildStatus
        , disableManualTrigger : Bool
        , now : Maybe Time.Posix
        , fetchingHistory : Bool
        , nextPage : Maybe Page
        , hasLoadedYet : Bool
        , comment : CommentBarVisibility
    }


type alias HistoryItem =
    { id : Int
    , name : String
    , status : BuildStatus.BuildStatus
    , duration : Concourse.BuildDuration
    , createdBy : Concourse.BuildCreatedBy
    , comment : String
    }


type CurrentOutput
    = Empty
    | Cancelled
    | Output OutputModel


type BuildPageType
    = OneOffBuildPage Concourse.BuildId
    | JobBuildPage Concourse.JobBuildIdentifier


type CommentBarVisibility
    = Hidden String
    | Visible CommentBar.Model


commentBarIsVisible : CommentBarVisibility -> Maybe CommentBar.Model
commentBarIsVisible comment =
    case comment of
        Visible commentBar ->
            Just commentBar

        _ ->
            Nothing
