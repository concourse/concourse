module Job.Msgs exposing (Hoverable(..), Msg(..))

import Routes
import Time exposing (Time)
import TopBar.Msgs


type Msg
    = TriggerBuild
    | TogglePaused
    | NavTo Routes.Route
    | SubscriptionTick
    | Hover Hoverable
    | ClockTick Time
    | FromTopBar TopBar.Msgs.Msg


type Hoverable
    = Toggle
    | Trigger
    | PreviousPage
    | NextPage
    | None
