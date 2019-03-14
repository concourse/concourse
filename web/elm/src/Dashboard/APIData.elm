module Dashboard.APIData exposing (APIData, remoteData)

import Concourse
import Concourse.Info
import Concourse.Job
import Concourse.Pipeline
import Concourse.Resource
import Concourse.Team
import Concourse.User
import Http
import Task exposing (Task)


type alias APIData =
    { teams : List Concourse.Team
    , pipelines : List Concourse.Pipeline
    , jobs : List Concourse.Job
    , resources : List Concourse.Resource
    , user : Maybe Concourse.User
    , version : String
    }


remoteData : Task Http.Error APIData
remoteData =
    Concourse.Team.fetchTeams
        |> Task.andThen
            (\teams ->
                Concourse.Pipeline.fetchPipelines
                    |> Task.andThen
                        (\pipelines ->
                            Concourse.Job.fetchAllJobs
                                |> Task.map (Maybe.withDefault [])
                                |> Task.andThen
                                    (\jobs ->
                                        Concourse.Resource.fetchAllResources
                                            |> Task.map (Maybe.withDefault [])
                                            |> Task.andThen
                                                (\resources ->
                                                    Concourse.Info.fetch
                                                        |> Task.map .version
                                                        |> Task.andThen
                                                            (\version ->
                                                                Concourse.User.fetchUser
                                                                    |> Task.map Just
                                                                    |> Task.onError
                                                                        (\err -> Task.succeed Nothing)
                                                                    |> Task.andThen
                                                                        (\user ->
                                                                            Task.succeed
                                                                                { teams = teams
                                                                                , pipelines = pipelines
                                                                                , jobs = jobs
                                                                                , resources = resources
                                                                                , user = user
                                                                                , version = version
                                                                                }
                                                                        )
                                                            )
                                                )
                                    )
                        )
            )
