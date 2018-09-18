module Dashboard.Group exposing (APIData, Group, groups, apiData, ordering, remoteData, view, headerView)

import Concourse
import Concourse.Info
import Concourse.Job
import Concourse.Pipeline
import Concourse.Resource
import Concourse.Team
import Dashboard.Pipeline as Pipeline
import Html exposing (..)
import Html.Attributes exposing (..)
import Http
import Ordering exposing (Ordering)
import Set
import Task
import Time exposing (Time)


type alias Group =
    { pipelines : List Pipeline.PipelineWithJobs
    , teamName : String
    }


type alias APIData =
    { teams : List Concourse.Team
    , pipelines : List Concourse.Pipeline
    , jobs : List Concourse.Job
    , resources : List Concourse.Resource
    , version : String
    }


allPipelines : APIData -> List Pipeline.PipelineWithJobs
allPipelines data =
    data.pipelines
        |> List.map
            (\p ->
                { pipeline = p
                , jobs =
                    data.jobs
                        |> List.filter
                            (\j ->
                                (j.teamName == p.teamName)
                                    && (j.pipelineName == p.name)
                            )
                , resourceError =
                    data.resources
                        |> List.any
                            (\r ->
                                (r.teamName == p.teamName)
                                    && (r.pipelineName == p.name)
                                    && r.failingToCheck
                            )
                }
            )


allTeamNames : APIData -> List String
allTeamNames apiData =
    Set.union
        (Set.fromList (List.map .teamName apiData.pipelines))
        (Set.fromList (List.map .name apiData.teams))
        |> Set.toList


remoteData : Task.Task Http.Error APIData
remoteData =
    Task.map5 APIData
        Concourse.Team.fetchTeams
        Concourse.Pipeline.fetchPipelines
        (Concourse.Job.fetchAllJobs |> Task.map (Maybe.withDefault []))
        (Concourse.Resource.fetchAllResources |> Task.map (Maybe.withDefault []))
        (Concourse.Info.fetch |> Task.map .version)


groups : APIData -> List Group
groups apiData =
    (List.map << group)
        (allPipelines apiData)
        (allTeamNames apiData)


apiData : List Group -> APIData
apiData groups =
    let
        pipelines =
            groups |> List.concatMap .pipelines
    in
        { teams = groups |> List.map (\g -> { id = 0, name = g.teamName })
        , pipelines = pipelines |> List.map .pipeline
        , jobs = pipelines |> List.concatMap .jobs
        , resources = []
        , version = ""
        }


group : List Pipeline.PipelineWithJobs -> String -> Group
group allPipelines teamName =
    { pipelines = (List.filter ((==) teamName << .teamName << .pipeline) allPipelines)
    , teamName = teamName
    }


ordering : Ordering Group
ordering =
    Ordering.byField .teamName


view : List (Html Pipeline.Msg) -> Pipeline.DragState -> Pipeline.DropState -> Time -> Group -> Html Pipeline.Msg
view header dragState dropState now group =
    let
        pipelines =
            if List.isEmpty group.pipelines then
                [ Pipeline.pipelineNotSetView ]
            else
                List.append
                    (List.indexedMap
                        (\i pipeline ->
                            Html.div [ class "pipeline-wrapper" ] [ Pipeline.pipelineDropAreaView dragState dropState group.teamName i, Pipeline.pipelineView dragState now pipeline i ]
                        )
                        group.pipelines
                    )
                    [ Pipeline.pipelineDropAreaView dragState dropState group.teamName (List.length group.pipelines) ]
    in
        Html.div [ id group.teamName, class "dashboard-team-group", attribute "data-team-name" group.teamName ]
            [ Html.div [ class "pin-wrapper" ]
                [ Html.div [ class "dashboard-team-header" ] header ]
            , Html.div [ class "dashboard-team-pipelines" ] pipelines
            ]


headerView : Group -> List (Html Pipeline.Msg)
headerView group =
    [ Html.div [ class "dashboard-team-name" ] [ Html.text group.teamName ] ]
