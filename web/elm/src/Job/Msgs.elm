module Job.Msgs exposing (Hoverable(..), Msg(..))

import Routes
import TopBar.Msgs


type Msg
    = TriggerBuild
    | TogglePaused
    | NavTo Routes.Route
    | Hover Hoverable
    | FromTopBar TopBar.Msgs.Msg


type Hoverable
    = Toggle
    | Trigger
    | PreviousPage
    | NextPage
    | None
