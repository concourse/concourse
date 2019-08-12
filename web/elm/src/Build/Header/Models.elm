module Build.Header.Models exposing
    ( CurrentBuild
    , CurrentOutput(..)
    , Model
    )

import Build.Output.Models exposing (OutputModel)
import Concourse
import Concourse.Pagination exposing (Page)
import RemoteData exposing (WebData)
import Time


type alias Model r =
    { r
        | scrolledToCurrentBuild : Bool
        , history : List Concourse.Build
        , currentBuild : WebData CurrentBuild
        , build : WebData Concourse.Build
        , disableManualTrigger : Bool
        , now : Maybe Time.Posix
        , fetchingHistory : Bool
        , nextPage : Maybe Page
        , previousTriggerBuildByKey : Bool
        , browsingIndex : Int
    }


type alias CurrentBuild =
    { prep : Maybe Concourse.BuildPrep }


type CurrentOutput
    = Empty
    | Cancelled
    | Output OutputModel
