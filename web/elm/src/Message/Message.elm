module Message.Message exposing
    ( Hoverable(..)
    , Message(..)
    , VersionId
    , VersionToggleAction(..)
    )

import Concourse
import Concourse.Cli as Cli
import Concourse.Pagination exposing (Page)
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
    | TogglePipelinePaused Concourse.PipelineIdentifier Bool
      -- Pipeline
    | ToggleGroup Concourse.PipelineGroup
    | SetGroups (List String)
      -- Fly Success
    | CopyToken
      -- Dashboard
    | DragStart String Int
    | DragOver String Int
    | DragEnd
    | Tooltip String String
    | TooltipHd String String
      -- Resource
    | LoadPage Page
    | ExpandVersionedResource VersionId
    | PinVersion VersionId
    | UnpinVersion
    | ToggleVersion VersionToggleAction VersionId
    | CheckRequested Bool
    | EditComment String
    | SaveComment String
    | FocusTextArea
    | BlurTextArea
      -- Job
    | TriggerBuildJob
    | TogglePaused
      -- Build
    | SwitchToBuild Concourse.Build
    | TriggerBuild (Maybe Concourse.JobIdentifier)
    | AbortBuild Int
    | ScrollBuilds StrictEvents.MouseWheelEvent
    | RevealCurrentBuildInHistory
    | ToggleStep String
    | SwitchTab String Int
    | SetHighlight String Int
    | ExtendHighlight String Int
      -- common
    | Hover (Maybe Hoverable)
    | GoToRoute Routes.Route


type Hoverable
    = ToggleJobButton
    | TriggerBuildButton
    | PreviousPageButton
    | NextPageButton
    | CheckButton
    | SaveCommentButton
    | AbortBuildButton
    | FirstOccurrenceIcon StepID
    | PinIcon
    | PinButton
    | PinBar
    | PipelineButton Concourse.PipelineIdentifier
    | FooterCliIcon Cli.Cli
    | WelcomeCardCliIcon Cli.Cli
    | CopyTokenButton
    | JobGroup Int


type VersionToggleAction
    = Enable
    | Disable


type alias VersionId =
    Concourse.VersionedResourceIdentifier
