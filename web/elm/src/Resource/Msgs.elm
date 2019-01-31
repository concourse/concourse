module Resource.Msgs exposing (Msg(..))

import Concourse.Pagination exposing (Page, Paginated)
import Keyboard
import Resource.Models as Models
import Time exposing (Time)
import TopBar.Msgs


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
    | Check
    | TopBarMsg TopBar.Msgs.Msg
    | EditComment String
    | SaveComment String
    | FocusTextArea
    | KeyDowns Keyboard.KeyCode
