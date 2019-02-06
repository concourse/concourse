module Job.Msgs exposing (Hoverable(..), Msg(..))

import NewTopBar.Msgs
import Time exposing (Time)


type Msg
    = TriggerBuild
    | TogglePaused
    | NavTo String
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
