module Build.Header.Models exposing
    ( BuildPageType(..)
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
        , status : BuildStatus.BuildStatus
        , disableManualTrigger : Bool
        , now : Maybe Time.Posix
        , fetchingHistory : Bool
        , nextPage : Maybe Page
        , hasLoadedYet : Bool
    }


type alias HistoryItem =
    { id : Int
    , name : String
    , status : BuildStatus.BuildStatus
    , duration : Concourse.BuildDuration
    }


type CurrentOutput
    = Empty
    | Cancelled
    | Output OutputModel


type BuildPageType
    = OneOffBuildPage Concourse.BuildId
    | JobBuildPage Concourse.JobBuildIdentifier
