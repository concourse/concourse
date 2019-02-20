module Build.Models exposing
    ( BuildPageType(..)
    , Hoverable(..)
    , Model
    , OutputModel
    , OutputState(..)
    , StepHeaderType(..)
    )

import Ansi.Log
import Array exposing (Array)
import Build.StepTree.Models exposing (StepTreeModel)
import Concourse
import Date exposing (Date)
import Dict exposing (Dict)
import RemoteData exposing (WebData)
import Routes exposing (Highlight, StepID)
import Time exposing (Time)
import TopBar.Model



-- Top level build


type alias Model =
    { page : BuildPageType
    , now : Maybe Time
    , job : Maybe Concourse.Job
    , history : List Concourse.Build
    , currentBuild : WebData CurrentBuild
    , browsingIndex : Int
    , autoScroll : Bool
    , csrfToken : String
    , previousKeyPress : Maybe Char
    , previousTriggerBuildByKey : Bool
    , showHelp : Bool
    , highlight : Highlight
    , hoveredElement : Maybe Hoverable
    , hoveredCounter : Int
    , topBar : TopBar.Model.Model
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


type Hoverable
    = Abort
    | Trigger
    | FirstOccurrence StepID



-- Output


type alias OutputModel =
    { steps : Maybe StepTreeModel
    , errors : Maybe Ansi.Log.Model
    , state : OutputState
    , eventSourceOpened : Bool
    , events : Maybe Int
    , highlight : Highlight
    }


type OutputState
    = StepsLoading
    | StepsLiveUpdating
    | StepsComplete
    | NotAuthorized
