module Pipeline exposing (Flags, init, update, view, subscriptions)

import Dict exposing (Dict)
import Graph exposing (Graph)
import Html exposing (Html)
import Html.Attributes exposing (class, href, style, rowspan)
import Http
import Set exposing (Set)
import Task

import Concourse
import Concourse.Job
import Concourse.BuildStatus

import Grid exposing (Grid)

type alias Model =
  { ports : Ports
  , pipelineLocator : Concourse.PipelineIdentifier
  , groups : Set String
  , jobs : List Concourse.Job
  , error : Maybe String
  }

type Node
  = JobNode Concourse.Job
  | InputNode
      { resourceName : String
      , dependentJob : Concourse.Job
      }
  | OutputNode
      { resourceName : String
      , upstreamJob : Concourse.Job
      }
  | ConstrainedInputNode
      { resourceName : String
      , dependentJob : Concourse.Job
      , upstreamJob : Maybe Concourse.Job
      }

type alias Ports =
  { setGroups : (List String -> Msg) -> Sub Msg
  }

type alias Flags =
  { teamName : String
  , pipelineName : String
  , groups : List String
  }

type Msg
  = Noop
  | JobsFetched (Result Http.Error (List Concourse.Job))
  | SetGroups (List String)

init : Ports -> Flags -> (Model, Cmd Msg)
init ports flags =
  let
    model =
      { ports = ports
      , pipelineLocator =
          { teamName = flags.teamName
          , pipelineName = flags.pipelineName
          }
      , groups = Set.fromList flags.groups
      , jobs = []
      , error = Nothing
      }
  in
    (model, fetchJobs model.pipelineLocator)

update : Msg -> Model -> (Model, Cmd Msg)
update msg model =
  case msg of
    Noop ->
      (model, Cmd.none)

    JobsFetched (Ok jobs) ->
      ({ model | jobs = jobs }, Cmd.none)

    JobsFetched (Err msg) ->
      ({ model | error = Just (toString msg) }, Cmd.none)

    SetGroups groups ->
      ({ model | groups = Set.fromList groups }, Cmd.none)

subscriptions : Model -> Sub Msg
subscriptions model =
  model.ports.setGroups SetGroups

view : Model -> Html Msg
view model =
  case model.error of
    Just msg ->
      Html.text ("error: " ++ msg)

    Nothing ->
      let
        filtered =
          if Set.isEmpty model.groups then
            model.jobs
          else
            List.filter (List.any (flip Set.member model.groups ) << .groups) model.jobs

        graph =
          initGraph filtered
      in
        -- Html.table [class "pipeline-table"] (
        --   model.graph
        --     |> Grid.fromGraph
        --     |> Grid.toMatrix nodeHeight
        --     |> Matrix.toList
        --     |> List.map viewRow
        -- )
        Html.div [class "pipeline-grid"] [
          viewGrid (Grid.fromGraph graph)
        ]

nodeHeight : Graph.Node Node -> Int
nodeHeight {label} =
  case label of
    JobNode job ->
      max 1 (jobResources job)

    _ ->
      1

viewRow : List (Grid.MatrixCell Node ()) -> Html Msg
viewRow row =
  Html.tr [] <|
    List.map viewMatrixCell row

viewMatrixCell : Grid.MatrixCell Node () -> Html Msg
viewMatrixCell mnode =
  case mnode of
    Grid.MatrixSpacer ->
      Html.td [class "spacer"] []

    Grid.MatrixNode {node} ->
      Html.td [rowspan (nodeHeight node)] [
        viewNode node
      ]

    Grid.MatrixFilled ->
      Html.text ""

viewGrid : Grid Node () -> Html Msg
viewGrid grid =
  case grid of
    Grid.Cell {node} ->
      viewNode node

    Grid.Serial prev next ->
      Html.div [class "serial-grid"]
        (viewSerial prev ++ viewSerial next)

    Grid.Parallel grids ->
      Html.div [class "parallel-grid"] <|
        List.map viewGrid grids

    Grid.End ->
      Html.text ""

viewSerial : Grid Node () -> List (Html Msg)
viewSerial grid =
  case grid of
    Grid.Serial prev next ->
      viewSerial prev ++ viewSerial next

    _ ->
      [viewGrid grid]

viewNode : Graph.Node Node -> Html Msg
viewNode {id,label} =
  let
    idAttr =
      Html.Attributes.id ("node-" ++ toString id)
  in
    case label of
      JobNode job ->
        Html.div [class "node job", idAttr] [
          viewJobNode job
        ]

      InputNode {resourceName} ->
        Html.div [class "node input", idAttr] [
          viewInputNode resourceName
        ]

      ConstrainedInputNode {resourceName} ->
        Html.div [class "node input constrained", idAttr] [
          viewConstrainedInputNode resourceName
        ]

      OutputNode {resourceName} ->
        Html.div [class "node output", idAttr] [
          viewOutputNode resourceName
        ]

viewJobNode : Concourse.Job -> Html Msg
viewJobNode job =
  let
    linkAttrs =
      case (job.finishedBuild, job.nextBuild) of
        (Just fb, Just nb) ->
          [ class (Concourse.BuildStatus.show fb.status ++ " started")
          , href nb.url
          ]

        (Just fb, Nothing) ->
          [ class (Concourse.BuildStatus.show fb.status)
          , href fb.url
          ]

        (Nothing, Just nb) ->
          [ class "no-builds started"
          , href nb.url
          ]

        (Nothing, Nothing) ->
          [ class "no-builds"
          , href job.url
          ]
  in
    Html.a linkAttrs [ --(style [("line-height", toString (30 * jobResources job - 10) ++ "px")] :: linkAttrs) [
      Html.text job.name
    ]

jobResources : Concourse.Job -> Int
jobResources {inputs,outputs} =
  Set.size (Set.fromList (List.map .resource inputs ++ List.map .resource outputs))

viewInputNode : String -> Html Msg
viewInputNode resourceName =
  Html.a [href "#"] [Html.text resourceName]

viewConstrainedInputNode : String -> Html Msg
viewConstrainedInputNode resourceName =
  Html.a [href "#"] [Html.text resourceName]

viewOutputNode : String -> Html Msg
viewOutputNode resourceName =
  Html.a [href "#"] [Html.text resourceName]

fetchJobs : Concourse.PipelineIdentifier -> Cmd Msg
fetchJobs pid =
  Concourse.Job.fetchJobs pid
    |> Task.perform Err Ok
    |> Cmd.map JobsFetched

type alias ByName a =
  Dict String a

initGraph : List Concourse.Job -> Graph Node ()
initGraph jobs =
  let
    jobNodes =
      List.map JobNode jobs

    jobsByName =
      List.foldl (\job dict -> Dict.insert job.name job dict) Dict.empty jobs

    resourceNodes =
      List.concatMap (jobResourceNodes jobsByName) jobs

    graphNodes =
      List.indexedMap Graph.Node (List.concat [jobNodes, resourceNodes])
  in
    Graph.fromNodesAndEdges
      graphNodes
      (List.concatMap (nodeEdges graphNodes) graphNodes)

jobResourceNodes : ByName Concourse.Job -> Concourse.Job -> List Node
jobResourceNodes jobs job =
  List.concatMap (inputNodes jobs job) job.inputs ++
    List.concatMap (outputNodes job) job.outputs

inputNodes : ByName Concourse.Job -> Concourse.Job -> Concourse.JobInput -> List Node
inputNodes jobs job {resource,passed} =
  if List.isEmpty passed then
    [InputNode { resourceName = resource, dependentJob = job }]
  else
    List.map (constrainedInputNode jobs resource job) passed

outputNodes : Concourse.Job -> Concourse.JobOutput -> List Node
outputNodes job {resource} =
  []
  -- [OutputNode { resourceName = resource, upstreamJob = job }]

constrainedInputNode : ByName Concourse.Job -> String -> Concourse.Job -> String -> Node
constrainedInputNode jobs resourceName dependentJob upstreamJobName =
  ConstrainedInputNode
    { resourceName = resourceName
    , dependentJob = dependentJob
    , upstreamJob = Dict.get upstreamJobName jobs
    }

nodeEdges : List (Graph.Node Node) -> Graph.Node Node -> List (Graph.Edge ())
nodeEdges allNodes {id,label} =
  case label of
    JobNode _ ->
      []

    InputNode {dependentJob} ->
      [Graph.Edge id (jobId allNodes dependentJob) ()]

    ConstrainedInputNode {dependentJob,upstreamJob} ->
      Graph.Edge id (jobId allNodes dependentJob) () ::
        case upstreamJob of
          Just upstream ->
            [Graph.Edge (jobId allNodes upstream) id ()]

          Nothing ->
            []

    OutputNode {upstreamJob} ->
      [Graph.Edge (jobId allNodes upstreamJob) id ()]

jobId : List (Graph.Node Node) -> Concourse.Job -> Int
jobId nodes job =
  case List.filter ((==) (JobNode job) << .label) nodes of
    {id} :: _ ->
      id

    [] ->
      Debug.crash "impossible: job index not found"
