module Message.BuildMsgs exposing (Msg(..))

import Build.Models exposing (Hoverable)
import Concourse
import Message.TopBarMsgs
import Routes exposing (StepID)
import StrictEvents


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
    | FromTopBar Message.TopBarMsgs.Msg
