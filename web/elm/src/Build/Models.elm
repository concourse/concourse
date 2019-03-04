module Build.Models exposing
    ( BuildEvent(..)
    , BuildPageType(..)
    , HookedStep
    , Hoverable(..)
    , MetadataField
    , Model
    , Origin
    , OutputModel
    , OutputState(..)
    , Step
    , StepFocus
    , StepHeaderType(..)
    , StepName
    , StepState(..)
    , StepTree(..)
    , StepTreeModel
    , TabFocus(..)
    , Version
    )

import Ansi.Log
import Array exposing (Array)
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



-- StepTree


type alias StepTreeModel =
    { tree : StepTree
    , foci : Dict StepID StepFocus
    , finished : Bool
    , highlight : Highlight
    , tooltip : Maybe StepID
    }


type StepTree
    = Task Step
    | Get Step
    | Put Step
    | Aggregate (Array StepTree)
    | Do (Array StepTree)
    | OnSuccess HookedStep
    | OnFailure HookedStep
    | OnAbort HookedStep
    | Ensure HookedStep
    | Try StepTree
    | Retry StepID Int TabFocus (Array StepTree)
    | Timeout StepTree


type alias StepFocus =
    { update : (StepTree -> StepTree) -> StepTree -> StepTree }


type alias Step =
    { id : StepID
    , name : StepName
    , state : StepState
    , log : Ansi.Log.Model
    , error : Maybe String
    , expanded : Maybe Bool
    , version : Maybe Version
    , metadata : List MetadataField
    , firstOccurrence : Bool
    , timestamps : Dict Int Date
    }


type alias StepName =
    String


type StepState
    = StepStatePending
    | StepStateRunning
    | StepStateSucceeded
    | StepStateFailed
    | StepStateErrored


type alias Version =
    Dict String String


type alias MetadataField =
    { name : String
    , value : String
    }


type alias HookedStep =
    { step : StepTree
    , hook : StepTree
    }


type TabFocus
    = Auto
    | User


type BuildEvent
    = BuildStatus Concourse.BuildStatus Date
    | Initialize Origin
    | StartTask Origin
    | FinishTask Origin Int
    | FinishGet Origin Int Concourse.Version Concourse.Metadata
    | FinishPut Origin Int Concourse.Version Concourse.Metadata
    | Log Origin String (Maybe Date)
    | Error Origin String
    | BuildError String
    | End


type alias Origin =
    { source : String
    , id : String
    }
