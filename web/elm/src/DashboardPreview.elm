module DashboardPreview exposing (init, view, width)

import Concourse
import Concourse.BuildStatus
import Dict exposing (Dict)
import Graph exposing (Graph)
import Grid exposing (Grid)
import Html exposing (Html)
import Html.Attributes exposing (class)


type Node
    = JobNode Concourse.Job
    | ConstrainedInputNode
        { resourceName : String
        , dependentJob : Concourse.Job
        , upstreamJob : Maybe Concourse.Job
        }


type alias ByName a =
    Dict String a


init : List Concourse.Job -> Grid Node ()
init jobs =
    let
        jobNodes =
            List.map JobNode jobs

        jobsByName =
            List.foldl (\job dict -> Dict.insert job.name job dict) Dict.empty jobs

        resourceNodes =
            List.concatMap (jobResourceNodes jobsByName) jobs

        graphNodes =
            List.indexedMap Graph.Node (List.concat [ jobNodes, resourceNodes ])
    in
        Grid.fromGraph <|
            Graph.fromNodesAndEdges
                graphNodes
                (List.concatMap (nodeEdges graphNodes) graphNodes)


jobResourceNodes : ByName Concourse.Job -> Concourse.Job -> List Node
jobResourceNodes jobs job =
    List.concatMap (inputNodes jobs job) job.inputs


inputNodes : ByName Concourse.Job -> Concourse.Job -> Concourse.JobInput -> List Node
inputNodes jobs job { resource, passed } =
    if List.isEmpty passed then
        []
    else
        List.map (constrainedInputNode jobs resource job) passed


constrainedInputNode : ByName Concourse.Job -> String -> Concourse.Job -> String -> Node
constrainedInputNode jobs resourceName dependentJob upstreamJobName =
    ConstrainedInputNode
        { resourceName = resourceName
        , dependentJob = dependentJob
        , upstreamJob = Dict.get upstreamJobName jobs
        }


nodeEdges : List (Graph.Node Node) -> Graph.Node Node -> List (Graph.Edge ())
nodeEdges allNodes { id, label } =
    case label of
        JobNode _ ->
            []

        ConstrainedInputNode { dependentJob, upstreamJob } ->
            Graph.Edge id (jobId allNodes dependentJob) ()
                :: case upstreamJob of
                    Just upstream ->
                        [ Graph.Edge (jobId allNodes upstream) id () ]

                    Nothing ->
                        []


jobId : List (Graph.Node Node) -> Concourse.Job -> Int
jobId nodes job =
    case List.filter ((==) (JobNode job) << .label) nodes of
        { id } :: _ ->
            id

        [] ->
            Debug.crash "impossible: job index not found"


view : Grid Node () -> Html msg
view grid =
    let
        groups =
            Dict.values <| jobGroups grid Dict.empty 0
    in
        Html.div [ class "pipeline-grid" ] <|
            List.map (\jobs -> Html.div [ class "parallel-grid" ] (List.map viewJob jobs)) groups


jobGroups : Grid Node () -> Dict Int (List Concourse.Job) -> Int -> Dict Int (List Concourse.Job)
jobGroups grid dict depth =
    case grid of
        Grid.Cell { node } ->
            case node.label of
                JobNode job ->
                    Dict.update depth (\jobs -> Just (job :: Maybe.withDefault [] jobs)) dict

                _ ->
                    dict

        Grid.Serial prev next ->
            jobGroups next (jobGroups prev dict depth) (depth + 1)

        Grid.Parallel grids ->
            List.foldl (\grid byDepth -> jobGroups grid byDepth depth) dict grids

        Grid.End ->
            dict


width : Grid Node () -> Int
width grid =
    -- TODO: do this more efficiently
    Dict.size (jobGroups grid Dict.empty 0)


viewJob : Concourse.Job -> Html msg
viewJob job =
    let
        linkAttrs =
            case job.finishedBuild of
                Just fb ->
                    Concourse.BuildStatus.show fb.status

                Nothing ->
                    "no-builds"
    in
        Html.div [ class ("node job " ++ linkAttrs) ] [ Html.text "" ]
