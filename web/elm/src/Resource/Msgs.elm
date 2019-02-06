module Resource.Msgs exposing (Msg(..))

import Concourse.Pagination exposing (Page, Paginated)
import NewTopBar.Msgs
import Resource.Models as Models
import Time exposing (Time)


type Msg
    = AutoupdateTimerTicked Time
    | LoadPage Page
    | ClockTick Time.Time
    | ExpandVersionedResource Models.VersionId
    | NavTo String
    | TogglePinBarTooltip
    | ToggleVersionTooltip
    | PinVersion Models.VersionId
    | UnpinVersion
    | ToggleVersion Models.VersionToggleAction Models.VersionId
    | PinIconHover Bool
    | Hover Models.Hoverable
    | CheckRequested Bool
    | TopBarMsg NewTopBar.Msgs.Msg
