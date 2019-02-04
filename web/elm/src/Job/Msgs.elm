module Job.Msgs exposing (Hoverable(..), Msg(..))

import Routes
import Time exposing (Time)


type Msg
    = Noop
    | TriggerBuild
    | TogglePaused
    | NavTo Routes.Route
    | SubscriptionTick Time
    | Hover Hoverable
    | ClockTick Time


type Hoverable
    = Toggle
    | Trigger
    | PreviousPage
    | NextPage
    | None
