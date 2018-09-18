module Concourse.PipelineStatus exposing (..)

import Concourse


show : Concourse.PipelineStatus -> String
show status =
    case status of
        Concourse.PipelineStatusAborted ->
            "aborted"

        Concourse.PipelineStatusErrored ->
            "errored"

        Concourse.PipelineStatusFailed ->
            "failed"

        Concourse.PipelineStatusPaused ->
            "paused"

        Concourse.PipelineStatusPending ->
            "pending"

        Concourse.PipelineStatusRunning ->
            "running"

        Concourse.PipelineStatusSucceeded ->
            "succeeded"


isRunning : Concourse.PipelineStatus -> Bool
isRunning status =
    case status of
        Concourse.PipelineStatusPending ->
            True

        Concourse.PipelineStatusRunning ->
            True

        _ ->
            False
