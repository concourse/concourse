module Concourse.Team exposing (fetchTeams)

import Concourse
import Http
import Json.Decode
import Task exposing (Task)


fetchTeams : Task Http.Error (List Concourse.Team)
fetchTeams =
    Http.toTask <|
        Http.get
            "/api/v1/teams"
            (Json.Decode.list Concourse.decodeTeam)
