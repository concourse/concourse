module Message.Message exposing
    ( DomID(..)
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
    | DragStart String Int
    | DragOver String Int
    | DragEnd
    | Tooltip String String
    | TooltipHd String String
      -- Resource
    | EditComment String
    | FocusTextArea
    | BlurTextArea
      -- Build
    | ScrollBuilds StrictEvents.MouseWheelEvent
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
    | SaveCommentButton
    | FirstOccurrenceIcon StepID
    | StepState StepID
    | PinIcon
    | PinMenuDropDown String
    | PinButton VersionId
    | PinBar
    | PipelineButton Concourse.PipelineIdentifier
    | VisibilityButton Concourse.PipelineIdentifier
    | FooterCliIcon Cli.Cli
    | WelcomeCardCliIcon Cli.Cli
    | CopyTokenButton
    | SendTokenButton
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
    | JobPreview Concourse.JobIdentifier
    | HamburgerMenu
    | SideBarTeam String
    | SideBarPipeline Concourse.PipelineIdentifier


type VersionToggleAction
    = Enable
    | Disable


type VisibilityAction
    = Expose
    | Hide


type alias VersionId =
    Concourse.VersionedResourceIdentifier
