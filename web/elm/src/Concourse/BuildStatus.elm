module Concourse.BuildStatus exposing (..)

import Json.Decode

type BuildStatus
  = Pending
  | Started
  | Succeeded
  | Failed
  | Errored
  | Aborted

decode : Json.Decode.Decoder BuildStatus
decode =
  Json.Decode.customDecoder Json.Decode.string <| \status ->
    case status of
      "pending" -> Ok Pending
      "started" -> Ok Started
      "succeeded" -> Ok Succeeded
      "failed" -> Ok Failed
      "errored" -> Ok Errored
      "aborted" -> Ok Aborted
      unknown -> Err ("unknown build status: " ++ unknown)

show : BuildStatus -> String
show status =
  case status of
    Pending ->
      "pending"

    Started ->
      "started"

    Succeeded ->
      "succeeded"

    Failed ->
      "failed"

    Errored ->
      "errored"

    Aborted ->
      "aborted"

isRunning : BuildStatus -> Bool
isRunning status =
  case status of
    Pending ->
      True

    Started ->
      True

    _ ->
      False
