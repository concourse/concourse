module Build.Models exposing
    ( BuildPageType(..)
    , Hoverable(..)
    , Model
    , StepHeaderType(..)
    )

import Build.Output.Models exposing (OutputModel)
import Concourse
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
