module Resource.Msgs exposing (Msg(..))

import Concourse.Pagination exposing (Page, Paginated)
import Keyboard
import NewTopBar.Msgs
import Resource.Models as Models
import Routes
import Time exposing (Time)


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
    | TopBarMsg NewTopBar.Msgs.Msg
    | EditComment String
    | SaveComment String
    | FocusTextArea
    | BlurTextArea
    | KeyDowns Keyboard.KeyCode
    | KeyUps Keyboard.KeyCode
