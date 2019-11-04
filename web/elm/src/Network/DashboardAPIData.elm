module Network.DashboardAPIData exposing (remoteData)

import Concourse
import Http
import Network.Info
import Network.Job
import Network.Pipeline
import Network.Resource
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
                            Network.Job.fetchAllJobs
                                |> Task.map (Maybe.withDefault [])
                                |> Task.andThen
                                    (\jobs ->
                                        Network.Resource.fetchAllResources
                                            |> Task.map (Maybe.withDefault [])
                                            |> Task.andThen
                                                (\resources ->
                                                    Network.Info.fetch
                                                        |> Task.andThen
                                                            (\clusterInfo ->
                                                                Task.succeed
                                                                    { teams = teams
                                                                    , pipelines = pipelines
                                                                    , jobs = jobs
                                                                    , resources = resources
                                                                    , version = clusterInfo.version
                                                                    }
                                                            )
                                                )
                                    )
                        )
            )
