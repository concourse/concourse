module Message.Message exposing (Hoverable(..), Message(..))

import Build.Models
import Concourse
import Concourse.Cli as Cli
import Concourse.Pagination exposing (Page, Paginated)
import Dashboard.Models
import Resource.Models
import Routes exposing (StepID)
import StrictEvents


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
      -- Job
    | TriggerBuildJob
    | TogglePaused
    | HoverJob Hoverable
      -- Build
    | SwitchToBuild Concourse.Build
    | HoverBuild (Maybe Build.Models.Hoverable)
    | TriggerBuild (Maybe Concourse.JobIdentifier)
    | AbortBuild Int
    | ScrollBuilds StrictEvents.MouseWheelEvent
    | RevealCurrentBuildInHistory
    | ToggleStep String
    | SwitchTab String Int
    | SetHighlight String Int
    | ExtendHighlight String Int


type Hoverable
    = Toggle
    | Trigger
    | PreviousPage
    | NextPage
    | None
