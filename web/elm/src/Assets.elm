module Assets exposing
    ( Asset(..)
    , CircleOutlineIcon(..)
    , ComponentType(..)
    , backgroundImage
    , toString
    )

import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Cli exposing (Cli(..))
import Concourse.PipelineStatus exposing (PipelineStatus(..))
import Url.Builder


type Asset
    = CliIcon Cli
    | ChevronLeft
    | ChevronRight
    | ToggleSwitch Bool
    | VisibilityToggleIcon Bool
    | FavoritedToggleIcon Bool
    | BuildFavicon (Maybe BuildStatus)
    | PinIconWhite
    | PinIconGrey
    | CheckmarkIcon
    | BreadcrumbIcon ComponentType
    | ArchivedPipelineIcon
    | PassportOfficerIcon
    | ConcourseLogoWhite
    | CircleOutlineIcon CircleOutlineIcon
    | CogsIcon
    | RunningLegend
    | NotBlockingCheckIcon
    | RerunIcon
    | PendingIcon
    | InterruptedIcon
    | CancelledIcon
    | SuccessCheckIcon
    | FailureTimesIcon
    | ExclamationTriangleIcon
    | PipelineStatusIcon PipelineStatus
    | PipelineStatusIconStale
    | PipelineStatusIconJobsDisabled
    | ClippyIcon
    | UpArrow
    | DownArrow
    | RefreshIcon
    | MessageIcon
    | HamburgerMenuIcon
    | PeopleIcon
    | PlusIcon
    | MinusIcon
    | PlayIcon
    | PauseIcon
    | PencilIcon
    | SearchIcon
    | CloseIcon
    | StarIconUnfilled
    | StarIconFilled


type ComponentType
    = PipelineComponent
    | JobComponent
    | ResourceComponent


type CircleOutlineIcon
    = PlayCircleIcon
    | PauseCircleIcon
    | AddCircleIcon
    | AbortCircleIcon


toString : Asset -> String
toString asset =
    Url.Builder.absolute (toPath asset) []


backgroundImage : Maybe Asset -> String
backgroundImage maybeAsset =
    case maybeAsset of
        Nothing ->
            "none"

        Just asset ->
            "url(" ++ toString asset ++ ")"


toPath : Asset -> List String
toPath asset =
    let
        basePath =
            [ "public", "images" ]
    in
    case asset of
        CliIcon cli ->
            let
                imageName =
                    case cli of
                        OSX ->
                            "apple"

                        Windows ->
                            "windows"

                        Linux ->
                            "linux"
            in
            basePath ++ [ imageName ++ "-logo.svg" ]

        ChevronLeft ->
            basePath ++ [ "baseline-chevron-left.svg" ]

        ChevronRight ->
            basePath ++ [ "baseline-chevron-right.svg" ]

        ToggleSwitch on ->
            let
                imageName =
                    if on then
                        "on"

                    else
                        "off"
            in
            basePath ++ [ "ic-toggle-" ++ imageName ++ ".svg" ]

        VisibilityToggleIcon visible ->
            let
                imageName =
                    if visible then
                        ""

                    else
                        "-off"
            in
            basePath ++ [ "baseline-visibility" ++ imageName ++ ".svg" ]

        FavoritedToggleIcon isFavorited ->
            let
                imageName =
                    if isFavorited then
                        "-filled"

                    else
                        "-unfilled"
            in
            basePath ++ [ "star" ++ imageName ++ ".svg" ]

        BuildFavicon maybeStatus ->
            basePath
                ++ (case maybeStatus of
                        Just status ->
                            let
                                imageName =
                                    Concourse.BuildStatus.show status
                            in
                            [ "favicon-" ++ imageName ++ ".png" ]

                        Nothing ->
                            [ "favicon.png" ]
                   )

        PinIconWhite ->
            basePath ++ [ "pin-ic-white.svg" ]

        PinIconGrey ->
            basePath ++ [ "pin-ic-grey.svg" ]

        PencilIcon ->
            basePath ++ [ "pencil-white.svg" ]

        StarIconUnfilled ->
            basePath ++ [ "star-unfilled.svg" ]

        StarIconFilled ->
            basePath ++ [ "star-filled.svg" ]

        CheckmarkIcon ->
            basePath ++ [ "checkmark-ic.svg" ]

        BreadcrumbIcon component ->
            let
                imageName =
                    case component of
                        PipelineComponent ->
                            "pipeline"

                        JobComponent ->
                            "job"

                        ResourceComponent ->
                            "resource"
            in
            basePath ++ [ "ic-breadcrumb-" ++ imageName ++ ".svg" ]

        ArchivedPipelineIcon ->
            basePath ++ [ "ic-archived-pipeline.svg" ]

        PassportOfficerIcon ->
            basePath ++ [ "passport-officer-ic.svg" ]

        ConcourseLogoWhite ->
            basePath ++ [ "concourse-logo-white.svg" ]

        CircleOutlineIcon icon ->
            let
                imageName =
                    case icon of
                        PlayCircleIcon ->
                            "play"

                        PauseCircleIcon ->
                            "pause"

                        AddCircleIcon ->
                            "add"

                        AbortCircleIcon ->
                            "abort"
            in
            basePath ++ [ "ic-" ++ imageName ++ "-circle-outline-white.svg" ]

        CogsIcon ->
            basePath ++ [ "ic-cogs.svg" ]

        RunningLegend ->
            basePath ++ [ "ic-running-legend.svg" ]

        NotBlockingCheckIcon ->
            basePath ++ [ "ic-not-blocking-check.svg" ]

        RerunIcon ->
            basePath ++ [ "ic-rerun.svg" ]

        PendingIcon ->
            basePath ++ [ "ic-pending.svg" ]

        InterruptedIcon ->
            basePath ++ [ "ic-interrupted.svg" ]

        CancelledIcon ->
            basePath ++ [ "ic-cancelled.svg" ]

        SuccessCheckIcon ->
            basePath ++ [ "ic-success-check.svg" ]

        FailureTimesIcon ->
            basePath ++ [ "ic-failure-times.svg" ]

        ExclamationTriangleIcon ->
            basePath ++ [ "ic-exclamation-triangle.svg" ]

        PipelineStatusIcon status ->
            let
                imageName =
                    case status of
                        PipelineStatusPaused ->
                            "ic-pause-blue.svg"

                        PipelineStatusPending _ ->
                            "ic-pending-grey.svg"

                        PipelineStatusSucceeded _ ->
                            "ic-running-green.svg"

                        PipelineStatusFailed _ ->
                            "ic-failing-red.svg"

                        PipelineStatusAborted _ ->
                            "ic-aborted-brown.svg"

                        PipelineStatusErrored _ ->
                            "ic-error-orange.svg"
            in
            basePath ++ [ imageName ]

        PipelineStatusIconStale ->
            basePath ++ [ "ic-cached-grey.svg" ]

        PipelineStatusIconJobsDisabled ->
            basePath ++ [ "ic-sync.svg" ]

        ClippyIcon ->
            basePath ++ [ "clippy.svg" ]

        UpArrow ->
            basePath ++ [ "ic-arrow-upward.svg" ]

        DownArrow ->
            basePath ++ [ "ic-arrow-downward.svg" ]

        RefreshIcon ->
            basePath ++ [ "baseline-refresh.svg" ]

        MessageIcon ->
            basePath ++ [ "baseline-message.svg" ]

        HamburgerMenuIcon ->
            basePath ++ [ "baseline-menu.svg" ]

        PeopleIcon ->
            basePath ++ [ "baseline-people.svg" ]

        PlusIcon ->
            basePath ++ [ "ic-plus.svg" ]

        MinusIcon ->
            basePath ++ [ "ic-minus.svg" ]

        PlayIcon ->
            basePath ++ [ "ic-play-white.svg" ]

        PauseIcon ->
            basePath ++ [ "ic-pause-white.svg" ]

        SearchIcon ->
            basePath ++ [ "ic-search-white.svg" ]

        CloseIcon ->
            basePath ++ [ "ic-close-white.svg" ]
