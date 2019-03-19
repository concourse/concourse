module Message.Message exposing (Message(..))

import Concourse
import Concourse.Cli as Cli
import Concourse.Pagination exposing (Page, Paginated)
import Dashboard.Models
import Resource.Models
import Routes


type Message
    = -- Top Bar
      LogIn
    | LogOut
    | FilterMsg String
    | FocusMsg
    | BlurMsg
    | ToggleUserMenu
    | ShowSearchInput
    | TogglePinIconDropdown
    | TogglePipelinePaused Concourse.PipelineIdentifier Bool
    | GoToRoute Routes.Route
      -- Pipeline
    | ToggleGroup Concourse.PipelineGroup
    | SetGroups (List String)
      -- Fly Success
    | CopyTokenButtonHover Bool
    | CopyToken
      -- Dashboard
    | DragStart String Int
    | DragOver String Int
    | DragEnd
    | Tooltip String String
    | TooltipHd String String
    | PipelineButtonHover (Maybe Dashboard.Models.Pipeline)
    | CliHover (Maybe Cli.Cli)
    | TopCliHover (Maybe Cli.Cli)
      -- Resource
    | LoadPage Page
    | ExpandVersionedResource Resource.Models.VersionId
    | TogglePinBarTooltip
    | ToggleVersionTooltip
    | PinVersion Resource.Models.VersionId
    | UnpinVersion
    | ToggleVersion Resource.Models.VersionToggleAction Resource.Models.VersionId
    | PinIconHover Bool
    | Hover Resource.Models.Hoverable
    | CheckRequested Bool
    | EditComment String
    | SaveComment String
    | FocusTextArea
    | BlurTextArea
