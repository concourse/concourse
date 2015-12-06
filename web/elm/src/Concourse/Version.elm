module Concourse.Version where

import Dict exposing (Dict)

import Json.Decode exposing ((:=))

type alias Version =
  Dict String String

decode : Json.Decode.Decoder (Dict String String)
decode =
  Json.Decode.dict Json.Decode.string
