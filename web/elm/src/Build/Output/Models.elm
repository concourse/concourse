module Build.Output.Models exposing (OutputModel, OutputState(..))

import Build.StepTree.Models exposing (StepTreeModel)
import Concourse
import Routes exposing (Highlight)


type alias OutputModel =
    { steps : Maybe StepTreeModel
    , state : OutputState
    , eventSourceOpened : Bool
    , eventStreamUrlPath : Maybe String
    , highlight : Highlight
    , buildId : Maybe Concourse.JobBuildIdentifier
    }


type OutputState
    = StepsLoading
    | StepsLiveUpdating
    | StepsComplete
