module Build.Models exposing
    ( Model
    , ShortcutsModel
    , StepHeaderType(..)
    , toMaybe
    )

import Build.Header.Models exposing (BuildPageType(..), CurrentOutput(..), HistoryItem)
import Build.Output.Models exposing (OutputModel)
import Concourse
import Concourse.BuildStatus as BuildStatus
import Keyboard
import Login.Login as Login
import RemoteData
import Routes exposing (Highlight)



-- Top level build


type alias Model =
    Login.Model
        (Build.Header.Models.Model
            (ShortcutsModel
                { build : RemoteData.WebData Concourse.Build
                , shiftDown : Bool
                , highlight : Highlight
                , authorized : Bool
                , output : CurrentOutput
                , prep : Maybe Concourse.BuildPrep
                , page : BuildPageType
                }
            )
        )


type alias ShortcutsModel r =
    { r
        | previousKeyPress : Maybe Keyboard.KeyEvent
        , autoScroll : Bool
        , showHelp : Bool
        , id : Int
        , history : List HistoryItem
        , name : String
        , job : Maybe Concourse.JobIdentifier
        , status : BuildStatus.BuildStatus
        , isTriggerBuildKeyDown : Bool
    }


toMaybe : CurrentOutput -> Maybe OutputModel
toMaybe currentOutput =
    case currentOutput of
        Empty ->
            Nothing

        Cancelled ->
            Nothing

        Output outputModel ->
            Just outputModel


type StepHeaderType
    = StepHeaderPut
    | StepHeaderGet Bool
    | StepHeaderTask
    | StepHeaderSetPipeline
