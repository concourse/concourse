module Concourse.BuildStatus where

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
