module Concourse.Metadata exposing (..)

import Json.Decode exposing ((:=))

type alias Metadata =
  List MetadataField

type alias MetadataField =
  { name : String
  , value : String
  }

decode : Json.Decode.Decoder (List MetadataField)
decode =
  Json.Decode.list decodeMetadataField

decodeMetadataField : Json.Decode.Decoder MetadataField
decodeMetadataField =
  Json.Decode.object2 MetadataField
    ("name" := Json.Decode.string)
    ("value" := Json.Decode.string)

