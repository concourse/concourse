module Network.DashboardAPIData exposing (remoteData)

import Concourse
import Http
import Network.Team
import Task exposing (Task)


remoteData : Task Http.Error Concourse.APIData
remoteData =
    Network.Team.fetchTeams
        |> Task.andThen
            (\teams ->
                Task.succeed
                    { teams = teams }
            )
