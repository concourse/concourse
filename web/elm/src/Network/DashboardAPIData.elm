module Network.DashboardAPIData exposing (remoteData)

import Concourse
import Http
import Network.Pipeline
import Network.Team
import Task exposing (Task)


remoteData : Task Http.Error Concourse.APIData
remoteData =
    Network.Team.fetchTeams
        |> Task.andThen
            (\teams ->
                Network.Pipeline.fetchPipelines
                    |> Task.andThen
                        (\pipelines ->
                            Task.succeed
                                { teams = teams
                                , pipelines = pipelines
                                }
                        )
            )
