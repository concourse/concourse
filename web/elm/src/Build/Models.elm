module Build.Models exposing
    ( Model
    , StepHeaderType(..)
    , toMaybe
    )

import Build.Header.Models exposing (BuildPageType(..), CurrentOutput(..))
import Build.Output.Models exposing (OutputModel)
import Concourse
import Keyboard
import Login.Login as Login
import Routes exposing (Highlight)
import Time



-- Top level build


type alias Model =
    Login.Model
        (Build.Header.Models.Model
            { autoScroll : Bool
            , previousKeyPress : Maybe Keyboard.KeyEvent
            , shiftDown : Bool
            , showHelp : Bool
            , highlight : Highlight
            , authorized : Bool
            , output : CurrentOutput
            , prep : Maybe Concourse.BuildPrep
            , page : BuildPageType
            , hasLoadedYet : Bool
            , notFound : Bool
            , reapTime : Maybe Time.Posix
            }
        )


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
