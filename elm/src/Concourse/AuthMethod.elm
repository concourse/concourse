module Concourse.AuthMethod exposing (fetchAll)

import Http
import Json.Decode
import Task exposing (Task)
import Concourse


fetchAll : String -> Task Http.Error (List Concourse.AuthMethod)
fetchAll teamName =
    Http.toTask
        << flip Http.get (Json.Decode.list Concourse.decodeAuthMethod)
    <|
        "/auth/list_methods?team_name="
            ++ teamName
