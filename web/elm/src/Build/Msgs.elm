module Build.Msgs exposing (EventsMsg(..), Msg(..))

import Array
import Build.Models exposing (Hoverable)
import Build.StepTree.Models exposing (BuildEvent)
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


type EventsMsg
    = Opened
    | Errored
    | Events (Result String (Array.Array BuildEvent))
