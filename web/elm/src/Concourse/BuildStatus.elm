module Concourse.BuildStatus exposing (..)

import Concourse
import Ordering exposing (Ordering)


ordering : Ordering Concourse.BuildStatus
ordering =
    Ordering.explicit
        [ Concourse.BuildStatusFailed
        , Concourse.BuildStatusErrored
        , Concourse.BuildStatusAborted
        , Concourse.BuildStatusSucceeded
        , Concourse.BuildStatusPending
        ]


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
