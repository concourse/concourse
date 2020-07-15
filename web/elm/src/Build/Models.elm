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
import Routes exposing (Highlight)
import Time



-- Top level build


type alias Model =
    Login.Model
        (Build.Header.Models.Model
            (ShortcutsModel
                { shiftDown : Bool
                , highlight : Highlight
                , isScrollToIdInProgress : Bool
                , authorized : Bool
                , output : CurrentOutput
                , prep : Maybe Concourse.BuildPrep
                , page : BuildPageType
                , hasLoadedYet : Bool
                , notFound : Bool
                , reapTime : Maybe Time.Posix
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
        , duration : Concourse.BuildDuration
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
    | StepHeaderLoadVar
    | StepHeaderAcross
