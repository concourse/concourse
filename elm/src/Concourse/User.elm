module Concourse.User exposing (..)

import HttpBuilder
import Json.Decode exposing ((:=))
import Task exposing (Task)

import Concourse.Team exposing (Team)

type alias User =
  { team : Team
  }

type alias Error =
  HttpBuilder.Error String

fetchUser : Task Error User
fetchUser =
  HttpBuilder.get "/api/v1/user"
    |> HttpBuilder.send (HttpBuilder.jsonReader decodeUser) HttpBuilder.stringReader
    |> Task.map .data

logOut : Task Error ()
logOut =
  HttpBuilder.get "/auth/logout"
    |> HttpBuilder.send HttpBuilder.stringReader HttpBuilder.stringReader
    |> Task.map (always ())

decodeUser : Json.Decode.Decoder User
decodeUser =
  Json.Decode.object1 User
    ("team" := Concourse.Team.decodeTeam)
