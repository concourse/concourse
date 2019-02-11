module Build.Msgs exposing (EventsMsg(..), Msg(..), fromBuildMessage)

import Array
import Build.Models exposing (BuildEvent, Hoverable)
import Concourse
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
    | NavTo Routes.Route
    | NewCSRFToken String
    | KeyPressed Keyboard.KeyCode
    | KeyUped Keyboard.KeyCode
    | BuildEventsMsg EventsMsg
    | ToggleStep String
    | SwitchTab String Int
    | SetHighlight String Int
    | ExtendHighlight String Int
    | ScrollDown
    | FromTopBar NewTopBar.Msgs.Msg


type EventsMsg
    = Opened
    | Errored
    | Events (Result String (Array.Array BuildEvent))


fromBuildMessage : Msg -> NewTopBar.Msgs.Msg
fromBuildMessage msg =
    case msg of
        KeyPressed k ->
            NewTopBar.Msgs.KeyPressed k

        FromTopBar m ->
            m

        _ ->
            NewTopBar.Msgs.Noop
