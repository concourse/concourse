module Build.Output.Models exposing (OutputModel, OutputState(..))

import Ansi.Log
import Build.StepTree.Models exposing (StepTreeModel)
import Routes exposing (Highlight)


type alias OutputModel =
    { steps : Maybe StepTreeModel
    , errors : Maybe Ansi.Log.Model
    , state : OutputState
    , eventSourceOpened : Bool
    , eventStreamUrlPath : Maybe String
    , highlight : Highlight
    }


type OutputState
    = StepsLoading
    | StepsLiveUpdating
    | StepsComplete
    | NotAuthorized
