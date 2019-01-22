module Job.Msgs exposing (Hoverable(..), Msg(..))

import Time exposing (Time)


type Msg
    = Noop
    | TriggerBuild
    | TogglePaused
    | NavTo String
    | SubscriptionTick Time
    | Hover Hoverable
    | ClockTick Time


type Hoverable
    = Toggle
    | Trigger
    | PreviousPage
    | NextPage
    | None
