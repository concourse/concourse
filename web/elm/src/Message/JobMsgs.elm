module Message.JobMsgs exposing (Hoverable(..), Msg(..))

import Message.Message
import Routes


type Msg
    = TriggerBuild
    | TogglePaused
    | NavTo Routes.Route
    | Hover Hoverable
    | FromTopBar Message.Message.Message


type Hoverable
    = Toggle
    | Trigger
    | PreviousPage
    | NextPage
    | None
