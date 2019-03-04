module Resource.Msgs exposing (Msg(..))

import Concourse.Pagination exposing (Page, Paginated)
import Keyboard
import Resource.Models as Models
import Routes
import Time exposing (Time)
import TopBar.Msgs


type Msg
    = AutoupdateTimerTicked Time
    | LoadPage Page
    | ClockTick Time.Time
    | ExpandVersionedResource Models.VersionId
    | NavTo Routes.Route
    | TogglePinBarTooltip
    | ToggleVersionTooltip
    | PinVersion Models.VersionId
    | UnpinVersion
    | ToggleVersion Models.VersionToggleAction Models.VersionId
    | PinIconHover Bool
    | Hover Models.Hoverable
    | CheckRequested Bool
    | TopBarMsg TopBar.Msgs.Msg
    | EditComment String
    | SaveComment String
    | FocusTextArea
    | BlurTextArea
    | KeyDowns Keyboard.KeyCode
    | KeyUps Keyboard.KeyCode
