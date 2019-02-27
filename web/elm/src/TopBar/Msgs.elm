module TopBar.Msgs exposing (Msg(..))

import Keyboard
import Routes
import Window


type Msg
    = LogIn
    | LogOut
    | FilterMsg String
    | FocusMsg
    | BlurMsg
    | ToggleUserMenu
    | ShowSearchInput
    | TogglePinIconDropdown
    | GoToPinnedResource Routes.Route
