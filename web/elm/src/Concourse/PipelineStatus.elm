module Concourse.PipelineStatus exposing (..)


type PipelineStatus
    = PipelineStatusPaused
    | PipelineStatusAborted Bool
    | PipelineStatusErrored Bool
    | PipelineStatusFailed Bool
    | PipelineStatusPending Bool
    | PipelineStatusSucceeded Bool


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

        PipelineStatusAborted isRunning ->
            isRunning

        PipelineStatusErrored isRunning ->
            isRunning

        PipelineStatusFailed isRunning ->
            isRunning

        PipelineStatusPending isRunning ->
            isRunning

        PipelineStatusSucceeded isRunning ->
            isRunning
