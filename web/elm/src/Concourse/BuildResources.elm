module Concourse.BuildResources where

import Dict exposing (Dict)
import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

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
  , pipelineName : String
  , firstOccurrence : Bool
  }

type alias BuildOutput =
  { resource : String
  , version : Version
  }

type alias Version =
  Dict String String

type alias Metadata =
  List MetadataField

type alias MetadataField =
  { name : String
  , value : String
  }

type alias BuildId =
  Int

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
  Json.Decode.object7 BuildInput
    ("name" := Json.Decode.string)
    ("resource" := Json.Decode.string)
    ("type" := Json.Decode.string)
    ("version" := Json.Decode.dict Json.Decode.string)
    ("metadata" := Json.Decode.list decodeMetadataField)
    ("pipeline_name" := Json.Decode.string)
    ("first_occurrence" := Json.Decode.bool)

decodeMetadataField : Json.Decode.Decoder MetadataField
decodeMetadataField =
  Json.Decode.object2 MetadataField
    ("name" := Json.Decode.string)
    ("value" := Json.Decode.string)

decodeOutput : Json.Decode.Decoder BuildOutput
decodeOutput =
  Json.Decode.object2 BuildOutput
    ("resource" := Json.Decode.string)
    ("version" := Json.Decode.dict Json.Decode.string)
