module TopBar.Msgs exposing (Msg(..))

import Concourse
import Routes
import Time


type Msg
    = FetchUser Time.Time
    | FetchPipeline Concourse.PipelineIdentifier
    | LogOut
    | LogIn
    | ResetToPipeline Routes.Route
    | ToggleUserMenu
    | TogglePinIconDropdown
    | GoToPinnedResource Routes.Route
