module Concourse.PipelineStatus exposing
    ( PipelineStatus(..)
    , StatusDetails(..)
    , equal
    , isRunning
    , show
    )

import Time


type StatusDetails
    = Running
    | Since Time.Posix


type PipelineStatus
    = PipelineStatusPaused
    | PipelineStatusArchived
    | PipelineStatusAborted StatusDetails
    | PipelineStatusErrored StatusDetails
    | PipelineStatusFailed StatusDetails
    | PipelineStatusPending Bool
    | PipelineStatusSucceeded StatusDetails


equal : PipelineStatus -> PipelineStatus -> Bool
equal ps1 ps2 =
    case ( ps1, ps2 ) of
        ( PipelineStatusPaused, PipelineStatusPaused ) ->
            True

        ( PipelineStatusArchived, PipelineStatusArchived ) ->
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

        PipelineStatusArchived ->
            "archived"

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

        PipelineStatusArchived ->
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
