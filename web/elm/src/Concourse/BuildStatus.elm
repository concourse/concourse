module Concourse.BuildStatus exposing
    ( BuildStatus(..)
    , decodeBuildStatus
    , encodeBuildStatus
    , isRunning
    , ordering
    , show
    )

import Json.Decode
import Json.Encode
import Ordering exposing (Ordering)


type BuildStatus
    = BuildStatusPending
    | BuildStatusStarted
    | BuildStatusSucceeded
    | BuildStatusFailed
    | BuildStatusErrored
    | BuildStatusAborted


ordering : Ordering BuildStatus
ordering =
    Ordering.explicit
        [ BuildStatusFailed
        , BuildStatusErrored
        , BuildStatusAborted
        , BuildStatusSucceeded
        , BuildStatusPending
        ]


show : BuildStatus -> String
show status =
    case status of
        BuildStatusPending ->
            "pending"

        BuildStatusStarted ->
            "started"

        BuildStatusSucceeded ->
            "succeeded"

        BuildStatusFailed ->
            "failed"

        BuildStatusErrored ->
            "errored"

        BuildStatusAborted ->
            "aborted"


encodeBuildStatus : BuildStatus -> Json.Encode.Value
encodeBuildStatus =
    show >> Json.Encode.string


decodeBuildStatus : Json.Decode.Decoder BuildStatus
decodeBuildStatus =
    Json.Decode.string
        |> Json.Decode.andThen
            (\status ->
                case status of
                    "pending" ->
                        Json.Decode.succeed BuildStatusPending

                    "started" ->
                        Json.Decode.succeed BuildStatusStarted

                    "succeeded" ->
                        Json.Decode.succeed BuildStatusSucceeded

                    "failed" ->
                        Json.Decode.succeed BuildStatusFailed

                    "errored" ->
                        Json.Decode.succeed BuildStatusErrored

                    "aborted" ->
                        Json.Decode.succeed BuildStatusAborted

                    unknown ->
                        Json.Decode.fail <| "unknown build status: " ++ unknown
            )


isRunning : BuildStatus -> Bool
isRunning status =
    case status of
        BuildStatusPending ->
            True

        BuildStatusStarted ->
            True

        _ ->
            False
