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
        "/api/v1/teams/"
            ++ teamName
            ++ "/auth/methods"
