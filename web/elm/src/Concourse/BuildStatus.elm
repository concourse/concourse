module Concourse.BuildStatus exposing (..)

import Concourse


show : Concourse.BuildStatus -> String
show status =
    case status of
        Concourse.BuildStatusPending ->
            "pending"

        Concourse.BuildStatusStarted ->
            "started"

        Concourse.BuildStatusSucceeded ->
            "succeeded"

        Concourse.BuildStatusFailed ->
            "failed"

        Concourse.BuildStatusErrored ->
            "errored"

        Concourse.BuildStatusAborted ->
            "aborted"


isRunning : Concourse.BuildStatus -> Bool
isRunning status =
    case status of
        Concourse.BuildStatusPending ->
            True

        Concourse.BuildStatusStarted ->
            True

        _ ->
            False
