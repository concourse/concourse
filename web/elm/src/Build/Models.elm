module Build.Models exposing
    ( BuildPageType(..)
    , Model
    , StepHeaderType(..)
    )

import Build.Output.Models exposing (OutputModel)
import Concourse
import Message.Message exposing (HoverableBuild)
import RemoteData exposing (WebData)
import Routes exposing (Highlight, StepID)
import Time exposing (Time)
import TopBar.Model



-- Top level build


type alias Model =
    TopBar.Model.Model
        { page : BuildPageType
        , now : Maybe Time
        , job : Maybe Concourse.Job
        , history : List Concourse.Build
        , currentBuild : WebData CurrentBuild
        , browsingIndex : Int
        , autoScroll : Bool
        , previousKeyPress : Maybe Char
        , shiftDown : Bool
        , previousTriggerBuildByKey : Bool
        , showHelp : Bool
        , highlight : Highlight
        , hoveredElement : Maybe HoverableBuild
        , hoveredCounter : Int
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
