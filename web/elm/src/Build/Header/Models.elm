module Build.Header.Models exposing
    ( BuildComment(..)
    , BuildPageType(..)
    , CurrentOutput(..)
    , HistoryItem
    , Model
    )

import Build.Output.Models exposing (OutputModel)
import Concourse
import Concourse.BuildStatus as BuildStatus
import Concourse.Pagination exposing (Page)
import Time


type alias Model r =
    { r
        | id : Int
        , name : String
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
        , comment : BuildComment
        , shortcutsEnabled : Bool
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


type BuildComment
    = Viewing String
    | Editing ( String, String )
    | Saving String


type BuildPageType
    = OneOffBuildPage Concourse.BuildId
    | JobBuildPage Concourse.JobBuildIdentifier
