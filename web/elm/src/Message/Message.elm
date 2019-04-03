module Message.Message exposing
    ( DomID(..)
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


type DomID
    = ToggleJobButton
    | TriggerBuildButton
    | PreviousPageButton
    | NextPageButton
    | CheckButton Bool
    | SaveCommentButton
    | AbortBuildButton
    | FirstOccurrenceIcon StepID
    | PinIcon
    | PinButton VersionId
    | PinBar
    | PipelineButton Concourse.PipelineIdentifier
    | VisibilityButton Concourse.PipelineIdentifier
    | FooterCliIcon Cli.Cli
    | WelcomeCardCliIcon Cli.Cli
    | CopyTokenButton
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
    | BuildTab Concourse.Build



-- type Hoverable
--     = HToggleJobButton
--     | HTriggerBuildButton
--     | PreviousPageButton
--     | NextPageButton
--     | HCheckButton
--     | HSaveCommentButton
--     | HAbortBuildButton
--     | FirstOccurrenceIcon StepID
--     | HPinIcon
--     | HPinButton
--     | PinBar
--     | HPipelineButton Concourse.PipelineIdentifier
--     | VisibilityButton Concourse.PipelineIdentifier
--     | FooterCliIcon Cli.Cli
--     | WelcomeCardCliIcon Cli.Cli
--     | HCopyTokenButton
--     | JobGroup Int
-- type Clickable
--     = StepTab String Int
--     | StepHeader String
--     | ShowSearchButton
--     | ClearSearchButton
--     | CCopyTokenButton
--     | CToggleJobButton
--     | LoginButton
--     | LogoutButton
--     | UserMenu
--     | PaginationButton Page
--     | CCheckButton Bool
--     | CSaveCommentButton
--     | CPinIcon
--     | CPinButton VersionId
--     | VersionHeader VersionId
--     | VersionToggle VersionId
--     | CPipelineButton Concourse.PipelineIdentifier
--     | CTriggerBuildButton
--     | CAbortBuildButton
--     | BuildTab Concourse.Build


type VersionToggleAction
    = Enable
    | Disable


type alias VersionId =
    Concourse.VersionedResourceIdentifier
