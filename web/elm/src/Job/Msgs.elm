module Job.Msgs exposing (Hoverable(..), Msg(..))

import NewTopBar.Msgs
import Routes
import Time exposing (Time)


type Msg
    = TriggerBuild
    | TogglePaused
    | NavTo Routes.Route
    | SubscriptionTick Time
    | Hover Hoverable
    | ClockTick Time
    | FromTopBar NewTopBar.Msgs.Msg


type Hoverable
    = Toggle
    | Trigger
    | PreviousPage
    | NextPage
    | None
