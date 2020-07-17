module Message.Message exposing
    ( DomID(..)
    , DropTarget(..)
    , Message(..)
    , VersionId
    , VersionToggleAction(..)
    , VisibilityAction(..)
    )

import Concourse
import Concourse.Cli as Cli
import Concourse.Pagination exposing (Page)
import Routes exposing (StepID)
import StrictEvents


type Message
    = -- Top Bar
      FilterMsg String
    | FocusMsg
    | BlurMsg
      -- Pipeline
    | ToggleGroup Concourse.PipelineGroup
    | SetGroups (List String)
      -- Dashboard
    | DragStart String String
    | DragOver DropTarget
    | DragEnd
    | Tooltip String String
    | TooltipHd String String
      -- Resource
    | EditComment String
    | FocusTextArea
    | BlurTextArea
      -- Build
    | ScrollBuilds StrictEvents.WheelEvent
    | RevealCurrentBuildInHistory
    | SetHighlight String Int
    | ExtendHighlight String Int
      -- common
    | Hover (Maybe DomID)
    | Click DomID
    | GoToRoute Routes.Route
    | Scrolled StrictEvents.ScrollState


type DomID
    = ToggleJobButton
    | TriggerBuildButton
    | AbortBuildButton
    | RerunBuildButton
    | PreviousPageButton
    | NextPageButton
    | CheckButton Bool
    | EditButton
    | SaveCommentButton
    | ResourceCommentTextarea
    | FirstOccurrenceGetStepLabel StepID
    | StepState StepID
    | PinIcon
    | PinMenuDropDown String
    | PinButton VersionId
    | PinBar
    | PipelineStatusIcon Concourse.PipelineIdentifier
    | PipelineButton Concourse.PipelineIdentifier
    | VisibilityButton Concourse.PipelineIdentifier
    | FooterCliIcon Cli.Cli
    | WelcomeCardCliIcon Cli.Cli
    | CopyTokenButton
    | SendTokenButton
    | CopyTokenInput
    | JobGroup Int
    | StepTab String Int
    | StepHeader String
    | ShowSearchButton
    | ClearSearchButton
    | LoginButton
    | LogoutButton
    | UserMenu
    | PaginationButton Page
    | VersionHeader VersionId
    | VersionToggle VersionId
    | BuildTab Int String
    | PipelineWrapper Concourse.PipelineIdentifier
    | JobPreview Concourse.JobIdentifier
    | HamburgerMenu
    | SideBarResizeHandle
    | SideBarTeam String
    | SideBarPipeline Concourse.PipelineIdentifier
    | Dashboard
    | DashboardGroup String


type VersionToggleAction
    = Enable
    | Disable


type VisibilityAction
    = Expose
    | Hide


type alias VersionId =
    Concourse.VersionedResourceIdentifier


type DropTarget
    = Before String
    | After String
