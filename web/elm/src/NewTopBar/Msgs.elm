module NewTopBar.Msgs exposing (Msg(..))

import Keyboard
import Window


type Msg
    = LogIn
    | LogOut
    | FilterMsg String
    | FocusMsg
    | BlurMsg
    | KeyDown Keyboard.KeyCode
    | KeyPressed Keyboard.KeyCode
    | ToggleUserMenu
    | ShowSearchInput
    | ResizeScreen Window.Size
    | Noop
