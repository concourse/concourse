module Resource.Msgs exposing (Msg(..))

import Concourse.Pagination exposing (Page, Paginated)
import Resource.Models as Models
import Time exposing (Time)
import TopBar


type Msg
    = Noop
    | AutoupdateTimerTicked Time
    | LoadPage Page
    | ClockTick Time.Time
    | ExpandVersionedResource Int
    | NavTo String
    | TogglePinBarTooltip
    | ToggleVersionTooltip
    | PinVersion Int
    | UnpinVersion
    | ToggleVersion Models.VersionToggleAction Int
    | PinIconHover Bool
    | Hover Models.Hoverable
    | Check
    | TopBarMsg TopBar.Msg
