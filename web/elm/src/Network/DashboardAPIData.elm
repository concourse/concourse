module Network.DashboardAPIData exposing (remoteData)

import Concourse
import Http
import Network.Info
import Network.Job
import Network.Pipeline
import Network.Resource
import Network.Team
import Network.User
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
                                |> Task.map (Maybe.withDefault [] >> Just)
                                |> Task.onError
                                    (\err ->
                                        if isStatusCode 501 err then
                                            Task.succeed Nothing

                                        else
                                            Task.fail err
                                    )
                                |> Task.andThen
                                    (\jobs ->
                                        Network.Resource.fetchAllResources
                                            |> Task.map (Maybe.withDefault [])
                                            |> Task.andThen
                                                (\resources ->
                                                    Network.Info.fetch
                                                        |> Task.andThen
                                                            (\clusterInfo ->
                                                                Network.User.fetchUser
                                                                    |> Task.map Just
                                                                    |> Task.onError
                                                                        (always <| Task.succeed Nothing)
                                                                    |> Task.andThen
                                                                        (\user ->
                                                                            Task.succeed
                                                                                { teams = teams
                                                                                , pipelines = pipelines
                                                                                , jobs = jobs
                                                                                , resources = resources
                                                                                , user = user
                                                                                , version = clusterInfo.version
                                                                                }
                                                                        )
                                                            )
                                                )
                                    )
                        )
            )


isStatusCode : Int -> Http.Error -> Bool
isStatusCode code err =
    case err of
        Http.BadStatus { status } ->
            status.code == code

        _ ->
            False
