module Message.Message exposing (Message(..))

import Concourse
import Routes


type Message
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
    | ToggleGroup Concourse.PipelineGroup
    | SetGroups (List String)
    | CopyTokenButtonHover Bool
    | CopyToken
