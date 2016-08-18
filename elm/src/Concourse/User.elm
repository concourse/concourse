module Concourse.User exposing (..)

import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

import Concourse.Team exposing (Team)

type alias User =
  { team : Team
  }

fetchUser : Task Http.Error User
fetchUser =
  Http.get decodeUser "/api/v1/user"

decodeUser : Json.Decode.Decoder User
decodeUser =
  Json.Decode.object1 User
    ("team" := Concourse.Team.decodeTeam)
