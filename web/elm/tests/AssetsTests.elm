module AssetsTests exposing (backgroundImageTests, toStringTests)

import Assets
    exposing
        ( Asset(..)
        , CircleOutlineIcon(..)
        , ComponentType(..)
        , backgroundImage
        , toString
        )
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Cli exposing (Cli(..))
import Concourse.PipelineStatus exposing (PipelineStatus(..), StatusDetails(..))
import Expect
import Test exposing (Test, describe, test)


toStringTests : Test
toStringTests =
    describe "Assets"
        [ describe "CliIcon"
            [ test "OSX" <|
                \_ ->
                    CliIcon OSX
                        |> toString
                        |> Expect.equal "/public/images/apple-logo.svg"
            , test "Windows" <|
                \_ ->
                    CliIcon Windows
                        |> toString
                        |> Expect.equal "/public/images/windows-logo.svg"
            , test "Linux" <|
                \_ ->
                    CliIcon Linux
                        |> toString
                        |> Expect.equal "/public/images/linux-logo.svg"
            ]
        , test "ChevronLeft" <|
            \_ ->
                ChevronLeft
                    |> toString
                    |> Expect.equal "/public/images/baseline-chevron-left.svg"
        , test "ChevronRight" <|
            \_ ->
                ChevronRight
                    |> toString
                    |> Expect.equal "/public/images/baseline-chevron-right.svg"
        , describe "ToggleSwitch"
            [ test "On" <|
                \_ ->
                    ToggleSwitch True
                        |> toString
                        |> Expect.equal "/public/images/ic-toggle-on.svg"
            , test "Off" <|
                \_ ->
                    ToggleSwitch False
                        |> toString
                        |> Expect.equal "/public/images/ic-toggle-off.svg"
            ]
        , describe "VisibilityToggleIcon"
            [ test "Visible" <|
                \_ ->
                    VisibilityToggleIcon True
                        |> toString
                        |> Expect.equal "/public/images/baseline-visibility.svg"
            , test "Not Visible" <|
                \_ ->
                    VisibilityToggleIcon False
                        |> toString
                        |> Expect.equal "/public/images/baseline-visibility-off.svg"
            ]
        , describe "BuildFavicon"
            [ test "Nothing" <|
                \_ ->
                    BuildFavicon Nothing
                        |> toString
                        |> Expect.equal "/public/images/favicon.png"
            , test "Pending" <|
                \_ ->
                    BuildFavicon (Just BuildStatusPending)
                        |> toString
                        |> Expect.equal "/public/images/favicon-pending.png"
            , test "Started" <|
                \_ ->
                    BuildFavicon (Just BuildStatusStarted)
                        |> toString
                        |> Expect.equal "/public/images/favicon-started.png"
            , test "Succeeded" <|
                \_ ->
                    BuildFavicon (Just BuildStatusSucceeded)
                        |> toString
                        |> Expect.equal "/public/images/favicon-succeeded.png"
            , test "Failed" <|
                \_ ->
                    BuildFavicon (Just BuildStatusFailed)
                        |> toString
                        |> Expect.equal "/public/images/favicon-failed.png"
            , test "Errored" <|
                \_ ->
                    BuildFavicon (Just BuildStatusErrored)
                        |> toString
                        |> Expect.equal "/public/images/favicon-errored.png"
            , test "Aborted" <|
                \_ ->
                    BuildFavicon (Just BuildStatusAborted)
                        |> toString
                        |> Expect.equal "/public/images/favicon-aborted.png"
            ]
        , test "PinIconWhite" <|
            \_ ->
                PinIconWhite
                    |> toString
                    |> Expect.equal "/public/images/pin-ic-white.svg"
        , test "PinIconGrey" <|
            \_ ->
                PinIconGrey
                    |> toString
                    |> Expect.equal "/public/images/pin-ic-grey.svg"
        , test "CheckmarkIcon" <|
            \_ ->
                CheckmarkIcon
                    |> toString
                    |> Expect.equal "/public/images/checkmark-ic.svg"
        , describe "BreadcrumbIcon"
            [ test "Pipeline" <|
                \_ ->
                    BreadcrumbIcon PipelineComponent
                        |> toString
                        |> Expect.equal "/public/images/ic-breadcrumb-pipeline.svg"
            , test "Job" <|
                \_ ->
                    BreadcrumbIcon JobComponent
                        |> toString
                        |> Expect.equal "/public/images/ic-breadcrumb-job.svg"
            , test "Resource" <|
                \_ ->
                    BreadcrumbIcon ResourceComponent
                        |> toString
                        |> Expect.equal "/public/images/ic-breadcrumb-resource.svg"
            ]
        , test "ArchivedPipelineIcon" <|
            \_ ->
                ArchivedPipelineIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-archived-pipeline.svg"
        , test "PassportOfficerIcon" <|
            \_ ->
                PassportOfficerIcon
                    |> toString
                    |> Expect.equal "/public/images/passport-officer-ic.svg"
        , test "ConcourseLogoWhite" <|
            \_ ->
                ConcourseLogoWhite
                    |> toString
                    |> Expect.equal "/public/images/concourse-logo-white.svg"
        , describe "CircleOutlineIcon"
            [ test "Play" <|
                \_ ->
                    CircleOutlineIcon PlayCircleIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-play-circle-outline-white.svg"
            , test "Pause" <|
                \_ ->
                    CircleOutlineIcon PauseCircleIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-pause-circle-outline-white.svg"
            , test "Add" <|
                \_ ->
                    CircleOutlineIcon AddCircleIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-add-circle-outline-white.svg"
            , test "Abort" <|
                \_ ->
                    CircleOutlineIcon AbortCircleIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-abort-circle-outline-white.svg"
            ]
        , test "CogsIcon" <|
            \_ ->
                CogsIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-cogs.svg"
        , test "RunningLegend" <|
            \_ ->
                RunningLegend
                    |> toString
                    |> Expect.equal "/public/images/ic-running-legend.svg"
        , test "NotBlockingCheckIcon" <|
            \_ ->
                NotBlockingCheckIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-not-blocking-check.svg"
        , test "RerunIcon" <|
            \_ ->
                RerunIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-rerun.svg"
        , test "PendingIcon" <|
            \_ ->
                PendingIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-pending.svg"
        , test "InterruptedIcon" <|
            \_ ->
                InterruptedIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-interrupted.svg"
        , test "CancelledIcon" <|
            \_ ->
                CancelledIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-cancelled.svg"
        , test "SuccessCheckIcon" <|
            \_ ->
                SuccessCheckIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-success-check.svg"
        , test "FailureTimesIcon" <|
            \_ ->
                FailureTimesIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-failure-times.svg"
        , test "ExclamationTriangleIcon" <|
            \_ ->
                ExclamationTriangleIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-exclamation-triangle.svg"
        , describe "PipelineStatusIcon"
            [ test "Paused" <|
                \_ ->
                    PipelineStatusPaused
                        |> PipelineStatusIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-pause-blue.svg"
            , test "Pending" <|
                \_ ->
                    PipelineStatusPending True
                        |> PipelineStatusIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-pending-grey.svg"
            , test "Succeeded" <|
                \_ ->
                    PipelineStatusSucceeded Running
                        |> PipelineStatusIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-running-green.svg"
            , test "Failed" <|
                \_ ->
                    PipelineStatusFailed Running
                        |> PipelineStatusIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-failing-red.svg"
            , test "Aborted" <|
                \_ ->
                    PipelineStatusAborted Running
                        |> PipelineStatusIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-aborted-brown.svg"
            , test "Errored" <|
                \_ ->
                    PipelineStatusErrored Running
                        |> PipelineStatusIcon
                        |> toString
                        |> Expect.equal "/public/images/ic-error-orange.svg"
            ]
        , test "PipelineStatusIconStale" <|
            \_ ->
                PipelineStatusIconStale
                    |> toString
                    |> Expect.equal "/public/images/ic-cached-grey.svg"
        , test "ClippyIcon" <|
            \_ ->
                ClippyIcon
                    |> toString
                    |> Expect.equal "/public/images/clippy.svg"
        , test "UpArrow" <|
            \_ ->
                UpArrow
                    |> toString
                    |> Expect.equal "/public/images/ic-arrow-upward.svg"
        , test "DownArrow" <|
            \_ ->
                DownArrow
                    |> toString
                    |> Expect.equal "/public/images/ic-arrow-downward.svg"
        , test "RefreshIcon" <|
            \_ ->
                RefreshIcon
                    |> toString
                    |> Expect.equal "/public/images/baseline-refresh.svg"
        , test "MessageIcon" <|
            \_ ->
                MessageIcon
                    |> toString
                    |> Expect.equal "/public/images/baseline-message.svg"
        , test "HamburgerMenuIcon" <|
            \_ ->
                HamburgerMenuIcon
                    |> toString
                    |> Expect.equal "/public/images/baseline-menu.svg"
        , test "PeopleIcon" <|
            \_ ->
                PeopleIcon
                    |> toString
                    |> Expect.equal "/public/images/baseline-people.svg"
        , test "PlusIcon" <|
            \_ ->
                PlusIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-plus.svg"
        , test "MinusIcon" <|
            \_ ->
                MinusIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-minus.svg"
        , test "PlayIcon" <|
            \_ ->
                PlayIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-play-white.svg"
        , test "PauseIcon" <|
            \_ ->
                PauseIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-pause-white.svg"
        , test "SearchIcon" <|
            \_ ->
                SearchIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-search-white.svg"
        , test "CloseIcon" <|
            \_ ->
                CloseIcon
                    |> toString
                    |> Expect.equal "/public/images/ic-close-white.svg"
        , test "PencilIcon" <|
            \_ ->
                PencilIcon
                    |> toString
                    |> Expect.equal "/public/images/pencil-white.svg"
        ]


backgroundImageTests : Test
backgroundImageTests =
    describe "backgroundImage"
        [ test "Just" <|
            \_ ->
                CliIcon OSX
                    |> Just
                    |> Assets.backgroundImage
                    |> Expect.equal "url(/public/images/apple-logo.svg)"
        , test "Nothing" <|
            \_ ->
                Nothing
                    |> Assets.backgroundImage
                    |> Expect.equal "none"
        ]
