module Concourse.Team exposing (..)

import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

type alias Team = { name : String }

fetchTeams : Task Http.Error (List Team)
fetchTeams = Http.get decodeTeams "/api/v1/teams"

decodeTeams : Json.Decode.Decoder (List Team)
decodeTeams =
  Json.Decode.list <|
    Json.Decode.object1 Team
      ("name" := Json.Decode.string)
