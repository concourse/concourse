module Concourse.Version exposing (..)

import Dict exposing (Dict)

import Json.Decode exposing ((:=))

type alias Version =
  Dict String String

decode : Json.Decode.Decoder (Dict String String)
decode =
  Json.Decode.dict Json.Decode.string
