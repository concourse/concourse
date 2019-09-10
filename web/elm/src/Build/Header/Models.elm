module Build.Header.Models exposing
    ( BuildPageType(..)
    , CurrentOutput(..)
    , Model
    )

import Build.Output.Models exposing (OutputModel)
import Concourse
import Concourse.BuildStatus as BuildStatus
import Concourse.Pagination exposing (Page)
import RemoteData exposing (WebData)
import Time


type alias Model r =
    { r
        | page : BuildPageType
        , scrolledToCurrentBuild : Bool
        , history : List Concourse.Build
        , build : WebData Concourse.Build
        , duration : Concourse.BuildDuration
        , status : BuildStatus.BuildStatus
        , disableManualTrigger : Bool
        , now : Maybe Time.Posix
        , fetchingHistory : Bool
        , nextPage : Maybe Page
        , previousTriggerBuildByKey : Bool
        , browsingIndex : Int
    }


type CurrentOutput
    = Empty
    | Cancelled
    | Output OutputModel


type BuildPageType
    = OneOffBuildPage Concourse.BuildId
    | JobBuildPage Concourse.JobBuildIdentifier
