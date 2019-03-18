module Message.JobMsgs exposing (Hoverable(..), Msg(..))

import Message.TopBarMsgs
import Routes


type Msg
    = TriggerBuild
    | TogglePaused
    | NavTo Routes.Route
    | Hover Hoverable
    | FromTopBar Message.TopBarMsgs.Msg


type Hoverable
    = Toggle
    | Trigger
    | PreviousPage
    | NextPage
    | None
