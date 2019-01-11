module Job.Msgs exposing (Hoverable(..), Msg(..))

import Concourse
import Concourse.Pagination exposing (Paginated)
import Http
import Time exposing (Time)


type Msg
    = Noop
    | BuildTriggered (Result Http.Error Concourse.Build)
    | TriggerBuild
    | JobBuildsFetched (Result Http.Error (Paginated Concourse.Build))
    | JobFetched (Result Http.Error Concourse.Job)
    | BuildResourcesFetched Int (Result Http.Error Concourse.BuildResources)
    | ClockTick Time
    | TogglePaused
    | PausedToggled (Result Http.Error ())
    | NavTo String
    | SubscriptionTick Time
    | Hover Hoverable


type Hoverable
    = Toggle
    | Trigger
    | PreviousPage
    | NextPage
    | None
