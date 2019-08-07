module Build.Header.Models exposing
    ( CurrentBuild
    , CurrentOutput(..)
    , Model
    )

import Build.Output.Models exposing (OutputModel)
import Concourse
import RemoteData exposing (WebData)
import Time


type alias Model r =
    { r
        | scrolledToCurrentBuild : Bool
        , history : List Concourse.Build
        , currentBuild : WebData CurrentBuild
        , disableManualTrigger : Bool
        , now : Maybe Time.Posix
    }


type alias CurrentBuild =
    { build : Concourse.Build
    , prep : Maybe Concourse.BuildPrep
    , output : CurrentOutput
    }


type CurrentOutput
    = Empty
    | Cancelled
    | Output OutputModel
