module Message.Message exposing
    ( HoverableBuild(..)
    , HoverableJob(..)
    , HoverableRes(..)
    , Message(..)
    , VersionId
    , VersionToggleAction(..)
    )

import Concourse
import Concourse.Cli as Cli
import Concourse.Pagination exposing (Page, Paginated)
import Dashboard.Models
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
    | ExpandVersionedResource VersionId
    | TogglePinBarTooltip
    | ToggleVersionTooltip
    | PinVersion VersionId
    | UnpinVersion
    | ToggleVersion VersionToggleAction VersionId
    | PinIconHover Bool
    | HoverRes (Maybe HoverableRes)
    | CheckRequested Bool
    | EditComment String
    | SaveComment String
    | FocusTextArea
    | BlurTextArea
      -- Job
    | TriggerBuildJob
    | TogglePaused
    | HoverJob (Maybe HoverableJob)
      -- Build
    | SwitchToBuild Concourse.Build
    | HoverBuild (Maybe HoverableBuild)
    | TriggerBuild (Maybe Concourse.JobIdentifier)
    | AbortBuild Int
    | ScrollBuilds StrictEvents.MouseWheelEvent
    | RevealCurrentBuildInHistory
    | ToggleStep String
    | SwitchTab String Int
    | SetHighlight String Int
    | ExtendHighlight String Int


type HoverableJob
    = Toggle
    | TriggerJ
    | PreviousPageJ
    | NextPageJ


type HoverableRes
    = PreviousPageR
    | NextPageR
    | CheckButton
    | SaveCommentR


type HoverableBuild
    = Abort
    | TriggerB
    | FirstOccurrence StepID


type VersionToggleAction
    = Enable
    | Disable


type alias VersionId =
    Concourse.VersionedResourceIdentifier
