module Concourse.PipelineStatus exposing
    ( PipelineStatus(..)
    , StatusDetails(..)
    , icon
    , isRunning
    , show
    )

import Html exposing (Html)
import Html.Attributes exposing (style)
import Time exposing (Time)
import Views.Icon as Icon


type StatusDetails
    = Running
    | Since Time


type PipelineStatus
    = PipelineStatusPaused
    | PipelineStatusAborted StatusDetails
    | PipelineStatusErrored StatusDetails
    | PipelineStatusFailed StatusDetails
    | PipelineStatusPending Bool
    | PipelineStatusSucceeded StatusDetails


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

        PipelineStatusPending isRunning ->
            isRunning

        PipelineStatusSucceeded details ->
            details == Running


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
        }
        [ style [ ( "background-size", "contain" ) ] ]
