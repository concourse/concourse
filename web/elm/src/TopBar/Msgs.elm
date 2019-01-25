module TopBar.Msgs exposing (Msg(..))

import Concourse
import Time


type Msg
    = Noop
    | FetchUser Time.Time
    | FetchPipeline Concourse.PipelineIdentifier
    | LogOut
    | LogIn
    | ResetToPipeline String
    | ToggleUserMenu
    | TogglePinIconDropdown
    | GoToPinnedResource String
