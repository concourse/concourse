module Build.Models exposing
    ( BuildPageType(..)
    , CurrentBuild
    , Model
    , StepHeaderType(..)
    )

import Build.Output.Models exposing (OutputModel)
import Concourse
import Concourse.Pagination exposing (Page)
import Keyboard
import Login.Login as Login
import RemoteData exposing (WebData)
import Routes exposing (Highlight)
import Time



-- Top level build


type alias Model =
    Login.Model
        { page : BuildPageType
        , now : Maybe Time.Posix
        , disableManualTrigger : Bool
        , history : List Concourse.Build
        , nextPage : Maybe Page
        , currentBuild : WebData CurrentBuild
        , browsingIndex : Int
        , autoScroll : Bool
        , previousKeyPress : Maybe Keyboard.KeyEvent
        , shiftDown : Bool
        , previousTriggerBuildByKey : Bool
        , showHelp : Bool
        , highlight : Highlight
        , hoveredCounter : Int
        , fetchingHistory : Bool
        , scrolledToCurrentBuild : Bool
        , authorized : Bool
        , timeZone : Time.Zone
        }


type alias CurrentBuild =
    { build : Concourse.Build
    , prep : Maybe Concourse.BuildPrep
    , output : Maybe OutputModel
    }


type BuildPageType
    = OneOffBuildPage Concourse.BuildId
    | JobBuildPage Concourse.JobBuildIdentifier


type StepHeaderType
    = StepHeaderPut
    | StepHeaderGet Bool
    | StepHeaderTask
