module Build.Msgs exposing (Hoverable(..), Msg(..), fromBuildMessage)

import Concourse
import Concourse.BuildEvents
import Keyboard
import NewTopBar.Msgs
import Routes exposing (StepID)
import Scroll
import StrictEvents
import Time


type Msg
    = SwitchToBuild Concourse.Build
    | Hover (Maybe Hoverable)
    | TriggerBuild (Maybe Concourse.JobIdentifier)
    | AbortBuild Int
    | ScrollBuilds StrictEvents.MouseWheelEvent
    | ClockTick Time.Time
    | RevealCurrentBuildInHistory
    | WindowScrolled Scroll.FromBottom
    | NavTo String
    | NewCSRFToken String
    | KeyPressed Keyboard.KeyCode
    | KeyUped Keyboard.KeyCode
    | BuildEventsMsg Concourse.BuildEvents.Msg
    | ToggleStep String
    | SwitchTab String Int
    | SetHighlight String Int
    | ExtendHighlight String Int
    | ScrollDown
    | FromTopBar NewTopBar.Msgs.Msg


type Hoverable
    = Abort
    | Trigger
    | FirstOccurrence StepID


fromBuildMessage : Msg -> NewTopBar.Msgs.Msg
fromBuildMessage msg =
    case msg of
        KeyPressed k ->
            NewTopBar.Msgs.KeyPressed k

        FromTopBar m ->
            m

        _ ->
            NewTopBar.Msgs.Noop
