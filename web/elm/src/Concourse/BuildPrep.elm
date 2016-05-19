module Concourse.BuildPrep exposing (..)

import Dict exposing (Dict)
import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

import Concourse.Build exposing (BuildId)

type BuildPrepStatus = Unknown
  | Blocking
  | NotBlocking

type alias BuildPrep =
  { pausedPipeline : BuildPrepStatus
  , pausedJob : BuildPrepStatus
  , maxRunningBuilds : BuildPrepStatus
  , inputs : Dict String BuildPrepStatus
  , inputsSatisfied : BuildPrepStatus
  , missingInputReasons : Dict String String
  }

fetch : BuildId -> Task Http.Error BuildPrep
fetch buildId =
  Http.get decode ("/api/v1/builds/" ++ toString buildId ++ "/preparation")

decodeStatus : Json.Decode.Decoder BuildPrepStatus
decodeStatus =
  Json.Decode.customDecoder Json.Decode.string <| \status ->
    case status of
      "unknown" -> Ok Unknown
      "blocking" -> Ok Blocking
      "not_blocking" -> Ok NotBlocking
      unknown -> Err ("unknown build preparation status: " ++ unknown)

replaceMaybeWithEmptyDict : Maybe (Dict a b) -> Dict a b
replaceMaybeWithEmptyDict maybeDict =
    case maybeDict of
      Just dict -> dict
      Nothing -> Dict.empty

decode : Json.Decode.Decoder BuildPrep
decode =
  Json.Decode.object6 BuildPrep
    ("paused_pipeline" := decodeStatus)
    ("paused_job" := decodeStatus)
    ("max_running_builds" := decodeStatus)
    ("inputs" := Json.Decode.dict decodeStatus)
    ("inputs_satisfied" := decodeStatus)
    (Json.Decode.map replaceMaybeWithEmptyDict (Json.Decode.maybe ("missing_input_reasons" := Json.Decode.dict Json.Decode.string)))
