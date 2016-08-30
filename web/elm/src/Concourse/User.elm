module Concourse.User exposing (Error, fetchUser, logOut)

import HttpBuilder
import Task exposing (Task)

import Concourse

type alias Error =
  HttpBuilder.Error String

fetchUser : Task Error Concourse.User
fetchUser =
  HttpBuilder.get "/api/v1/user"
    |> HttpBuilder.send (HttpBuilder.jsonReader Concourse.decodeUser) HttpBuilder.stringReader
    |> Task.map .data

logOut : Task Error ()
logOut =
  HttpBuilder.get "/auth/logout"
    |> HttpBuilder.send HttpBuilder.stringReader HttpBuilder.stringReader
    |> Task.map (always ())
