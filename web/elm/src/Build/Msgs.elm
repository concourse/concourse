module Build.Msgs exposing (Msg(..))

import Build.Models exposing (Hoverable)
import Concourse
import Routes exposing (StepID)
import StrictEvents
import TopBar.Msgs


type Msg
    = SwitchToBuild Concourse.Build
    | Hover (Maybe Hoverable)
    | TriggerBuild (Maybe Concourse.JobIdentifier)
    | AbortBuild Int
    | ScrollBuilds StrictEvents.MouseWheelEvent
    | RevealCurrentBuildInHistory
    | NavTo Routes.Route
    | ToggleStep String
    | SwitchTab String Int
    | SetHighlight String Int
    | ExtendHighlight String Int
    | FromTopBar TopBar.Msgs.Msg
