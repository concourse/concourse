module Concourse.Team exposing (..)

import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

type alias Team = { id : Int, name : String }

fetchTeams : Task Http.Error (List Team)
fetchTeams = Http.get (Json.Decode.list decodeTeam) "/api/v1/teams"

decodeTeam : Json.Decode.Decoder Team
decodeTeam =
  Json.Decode.object2 Team
    ("id" := Json.Decode.int)
    ("name" := Json.Decode.string)
