module Concourse.Team exposing (fetchTeams)

import Concourse
import Http
import Json.Decode
import Task exposing (Task)

fetchTeams : Task Http.Error (List Concourse.Team)
fetchTeams =
  Http.get (Json.Decode.list Concourse.decodeTeam) "/api/v1/teams"
