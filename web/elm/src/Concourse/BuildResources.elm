module Concourse.BuildResources exposing (..)

import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

import Concourse.Metadata exposing (Metadata)
import Concourse.Version exposing (Version)

type alias BuildResources =
  { inputs : List BuildInput
  , outputs : List BuildOutput
  }

type alias BuildInput =
  { name : String
  , resource : String
  , type' : String
  , version : Version
  , metadata : Metadata
  , firstOccurrence : Bool
  }

type alias BuildOutput =
  { resource : String
  , version : Version
  }

type alias BuildId =
  Int

empty : BuildResources
empty =
  { inputs = []
  , outputs = []
  }

fetch : BuildId -> Task Http.Error BuildResources
fetch buildId =
  Http.get decode ("/api/v1/builds/" ++ toString buildId ++ "/resources")

decode : Json.Decode.Decoder BuildResources
decode =
  Json.Decode.object2 BuildResources
    ("inputs" := Json.Decode.list decodeInput)
    ("outputs" := Json.Decode.list decodeOutput)

decodeInput : Json.Decode.Decoder BuildInput
decodeInput =
  Json.Decode.object6 BuildInput
    ("name" := Json.Decode.string)
    ("resource" := Json.Decode.string)
    ("type" := Json.Decode.string)
    ("version" := Concourse.Version.decode)
    ("metadata" := Concourse.Metadata.decode)
    ("first_occurrence" := Json.Decode.bool)

decodeOutput : Json.Decode.Decoder BuildOutput
decodeOutput =
  Json.Decode.object2 BuildOutput
    ("resource" := Json.Decode.string)
    ("version" := Json.Decode.dict Json.Decode.string)
