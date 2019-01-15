module Build.Msgs exposing (HoveredButton(..), Msg(..))

import Concourse
import Concourse.BuildEvents
import Concourse.Pagination exposing (Paginated)
import Http
import Keyboard
import Scroll
import StrictEvents
import Time


type Msg
    = Noop
    | SwitchToBuild Concourse.Build
    | Hover HoveredButton
    | TriggerBuild (Maybe Concourse.JobIdentifier)
    | BuildTriggered (Result Http.Error Concourse.Build)
    | AbortBuild Int
    | BuildFetched Int (Result Http.Error Concourse.Build)
    | BuildPrepFetched Int (Result Http.Error Concourse.BuildPrep)
    | BuildHistoryFetched (Result Http.Error (Paginated Concourse.Build))
    | BuildJobDetailsFetched (Result Http.Error Concourse.Job)
    | ScrollBuilds StrictEvents.MouseWheelEvent
    | ClockTick Time.Time
    | BuildAborted (Result Http.Error ())
    | RevealCurrentBuildInHistory
    | WindowScrolled Scroll.FromBottom
    | NavTo String
    | NewCSRFToken String
    | KeyPressed Keyboard.KeyCode
    | KeyUped Keyboard.KeyCode
    | PlanAndResourcesFetched (Result Http.Error ( Concourse.BuildPlan, Concourse.BuildResources ))
    | BuildEventsMsg Concourse.BuildEvents.Msg
    | ToggleStep String
    | SwitchTab String Int
    | SetHighlight String Int
    | ExtendHighlight String Int


type HoveredButton
    = Neither
    | Abort
    | Trigger
