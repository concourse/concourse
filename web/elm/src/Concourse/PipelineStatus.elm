module Concourse.PipelineStatus exposing (..)

import Time exposing (Time)


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
