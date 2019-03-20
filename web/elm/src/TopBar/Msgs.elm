module TopBar.Msgs exposing (Msg(..))

import Concourse
import Routes


type Msg
    = LogIn
    | LogOut
    | FilterMsg String
    | FocusMsg
    | BlurMsg
    | ToggleUserMenu
    | ShowSearchInput
    | TogglePinIconDropdown
    | TogglePipelinePaused Concourse.PipelineIdentifier Bool
    | GoToRoute Routes.Route
