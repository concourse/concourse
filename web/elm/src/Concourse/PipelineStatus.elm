module Concourse.PipelineStatus exposing
    ( PipelineStatus(..)
    , StatusDetails(..)
    , equal
    , icon
    , isRunning
    , show
    )

import Html exposing (Html)
import Html.Attributes exposing (style)
import Time
import Views.Icon as Icon


type StatusDetails
    = Running
    | Since Time.Posix


type PipelineStatus
    = PipelineStatusPaused
    | PipelineStatusAborted StatusDetails
    | PipelineStatusErrored StatusDetails
    | PipelineStatusFailed StatusDetails
    | PipelineStatusPending Bool
    | PipelineStatusSucceeded StatusDetails
    | PipelineStatusUnknown


equal : PipelineStatus -> PipelineStatus -> Bool
equal ps1 ps2 =
    case ( ps1, ps2 ) of
        ( PipelineStatusPaused, PipelineStatusPaused ) ->
            True

        ( PipelineStatusAborted _, PipelineStatusAborted _ ) ->
            True

        ( PipelineStatusErrored _, PipelineStatusErrored _ ) ->
            True

        ( PipelineStatusFailed _, PipelineStatusFailed _ ) ->
            True

        ( PipelineStatusPending _, PipelineStatusPending _ ) ->
            True

        ( PipelineStatusSucceeded _, PipelineStatusSucceeded _ ) ->
            True

        _ ->
            False


show : PipelineStatus -> String
show status =
    case status of
        PipelineStatusPaused ->
            "paused"

        PipelineStatusAborted _ ->
            "aborted"

        PipelineStatusErrored _ ->
            "errored"

        PipelineStatusFailed _ ->
            "failed"

        PipelineStatusPending _ ->
            "pending"

        PipelineStatusSucceeded _ ->
            "succeeded"

        PipelineStatusUnknown ->
            "unknown"


isRunning : PipelineStatus -> Bool
isRunning status =
    case status of
        PipelineStatusPaused ->
            False

        PipelineStatusAborted details ->
            details == Running

        PipelineStatusErrored details ->
            details == Running

        PipelineStatusFailed details ->
            details == Running

        PipelineStatusPending bool ->
            bool

        PipelineStatusSucceeded details ->
            details == Running

        PipelineStatusUnknown ->
            False


icon : PipelineStatus -> Html msg
icon status =
    Icon.icon
        { sizePx = 20
        , image =
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

                PipelineStatusUnknown ->
                    "ic-cached-grey.svg"
        }
        [ style "background-size" "contain" ]
