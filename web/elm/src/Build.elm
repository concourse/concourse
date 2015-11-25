module Build where

import Ansi.Log
import Debug
import Effects exposing (Effects)
import EventSource exposing (EventSource)
import Html exposing (Html)
import Html.Events exposing (onClick)
import Html.Attributes exposing (action, class, classList, href, id, method)
import Http
import Json.Decode exposing ((:=))
import Task
import Time exposing (Time)

import BuildEvent exposing (BuildEvent)
import BuildPlan exposing (BuildPlan)
import Scroll
import StepTree exposing (StepTree)


type alias Model =
  { actions : Signal.Address Action
  , buildId : Int
  , stepRoot : Maybe StepTree.Root
  , build : Maybe Build
  , history : Maybe (List Build)
  , eventSource : Maybe EventSource
  , status : BuildEvent.BuildStatus
  , autoScroll : Bool
  , buildRunning : Bool
  , eventsLoaded : Bool
  }

type alias Build =
  { id : Int
  , name : String
  , status : String
  , jobName : String
  , pipelineName : String
  , url : String
  }

type Action
  = Noop
  | PlanFetched (Result Http.Error BuildPlan)
  | BuildFetched (Result Http.Error Build)
  | BuildHistoryFetched (Result Http.Error (List Build))
  | Listening EventSource
  | Opened
  | Errored
  | Event (Result String BuildEvent)
  | EndOfEvents
  | Closed
  | ScrollTick
  | ScrollFromBottom Int
  | StepTreeAction StepTree.Action
  | AbortBuild

init : Signal.Address Action -> Int -> (Model, Effects Action)
init actions buildId =
  let
    model =
      { actions = actions
      , buildId = buildId
      , stepRoot = Nothing
      , build = Nothing
      , history = Nothing
      , eventSource = Nothing
      , eventsLoaded = False
      , autoScroll = True
      , status = BuildEvent.BuildStatusPending
      , buildRunning = False
      }
  in
    (model, Effects.batch [ keepScrolling
                          , fetchBuildPlan 0 buildId
                          , fetchBuild buildId
                          ]
    )

update : Action -> Model -> (Model, Effects Action)
update action model =
  case action of
    Noop ->
      (model, Effects.none)

    ScrollTick ->
      if not model.eventsLoaded && model.autoScroll then
        (model, Effects.batch [keepScrolling, scrollToBottom])
      else
        (model, Effects.none)

    ScrollFromBottom fb ->
      if fb == 0 then
        ({ model | autoScroll = True }, Effects.tick (always ScrollTick))
      else
        ({ model | autoScroll = False }, Effects.none)

    AbortBuild ->
      (model, abortBuild model.buildId)

    PlanFetched (Err (Http.BadResponse 404 _)) ->
      (model, fetchBuildPlan Time.second model.buildId)

    PlanFetched (Err err) ->
      Debug.log ("failed to fetch plan: " ++ toString err) <|
        (model, Effects.none)

    PlanFetched (Ok plan) ->
      ( { model | stepRoot = Just (StepTree.init plan) }
      , subscribeToEvents model.buildId model.actions
      )

    BuildFetched (Err err) ->
      Debug.log ("failed to fetch build: " ++ toString err) <|
        (model, Effects.none)

    BuildFetched (Ok build) ->
      let
        status = toStatus build.status
        running =
          case status of
            BuildEvent.BuildStatusPending ->
              True
            BuildEvent.BuildStatusStarted ->
              True
            _ ->
              False
      in
        ( { model
          | build = Just build
          , status = status
          , buildRunning = running
          }
        , fetchBuildHistory build.pipelineName build.jobName
        )

    BuildHistoryFetched (Err err) ->
      Debug.log ("failed to fetch build history: " ++ toString err) <|
        (model, Effects.none)

    BuildHistoryFetched (Ok history) ->
      ( { model | history = Just history }, Effects.none)

    Listening es ->
      ({ model | eventSource = Just es }, Effects.none)

    Opened ->
      (model, scrollToBottom)

    Errored ->
      (model, Effects.none)

    Event (Ok (BuildEvent.Log origin output)) ->
      ( updateStep origin.id (setRunning << appendStepLog output) model
      , Effects.none
      )

    Event (Ok (BuildEvent.Error origin message)) ->
      ( updateStep origin.id (setRunning << setStepError message) model
      , Effects.none
      )

    Event (Ok (BuildEvent.InitializeTask origin)) ->
      ( updateStep origin.id setRunning model
      , Effects.none
      )

    Event (Ok (BuildEvent.StartTask origin)) ->
      ( updateStep origin.id setRunning model
      , Effects.none
      )

    Event (Ok (BuildEvent.FinishTask origin exitStatus)) ->
      ( updateStep origin.id (finishStep exitStatus) model
      , Effects.none
      )

    Event (Ok (BuildEvent.FinishGet origin exitStatus)) ->
      ( updateStep origin.id (finishStep exitStatus) model
      , Effects.none
      )

    Event (Ok (BuildEvent.FinishPut origin exitStatus)) ->
      ( updateStep origin.id (finishStep exitStatus) model
      , Effects.none
      )

    Event (Ok (BuildEvent.BuildStatus status)) ->
      ( if model.buildRunning then
          { model | status = status }
        else
          model
      , Effects.none
      )

    Event (Err e) ->
      (model, Debug.log e Effects.none)

    StepTreeAction action ->
      ( { model | stepRoot = Maybe.map (StepTree.update action) model.stepRoot }
      , Effects.none
      )

    EndOfEvents ->
      case model.eventSource of
        Just es ->
          ({ model | eventsLoaded = True }, closeEvents es)

        Nothing ->
          (model, Effects.none)

    Closed ->
      ({ model | eventSource = Nothing }, Effects.none)

abortBuild : Int -> Effects.Effects Action
abortBuild buildId =
  Http.send Http.defaultSettings
    { verb = "POST"
    , headers = []
    , url = "/api/v1/builds/" ++ toString buildId ++ "/abort"
    , body = Http.empty
    }
    |> Task.toMaybe
    |> Task.map (always Noop)
    |> Effects.task

keepScrolling : Effects Action
keepScrolling = Effects.tick (always ScrollTick)

updateStep : StepTree.StepID -> (StepTree -> StepTree) -> Model -> Model
updateStep id update model =
  { model | stepRoot = Maybe.map (StepTree.updateAt id update) model.stepRoot }

setRunning : StepTree -> StepTree
setRunning = setStepState StepTree.StepStateRunning

appendStepLog : String -> StepTree -> StepTree
appendStepLog output tree =
  StepTree.map (\step -> { step | log = Ansi.Log.update output step.log }) tree

setStepError : String -> StepTree -> StepTree
setStepError message tree =
  StepTree.map (\step -> { step | error = Just message }) tree

finishStep : Int -> StepTree -> StepTree
finishStep exitStatus tree =
  let
    stepState =
      if exitStatus == 0 then
        StepTree.StepStateSucceeded
      else
        StepTree.StepStateFailed
  in
    setStepState stepState tree

setStepState : StepTree.StepState -> StepTree -> StepTree
setStepState state tree =
  let
    expanded = state /= StepTree.StepStateSucceeded
  in
    StepTree.map (\step -> { step | state = state, expanded = expanded }) tree

view : Signal.Address Action -> Model -> Html
view actions model =
  case (model.build, model.stepRoot) of
    (Just build, Just root) ->
      Html.div []
        [ viewBuildHeader actions build model.status (Maybe.withDefault [] model.history)
        , Html.div [id "build-body"]
            [ if model.buildRunning || model.eventsLoaded then
                Html.div [class "steps"]
                  [ StepTree.view (Signal.forwardTo actions StepTreeAction) root.tree ]
              else
                Html.text "loading..."
            ]
        ]

    (Just build, Nothing) ->
      Html.div []
        [ viewBuildHeader actions build model.status (Maybe.withDefault [] model.history)
        , Html.div [id "build-body"] []
        ]

    _ ->
      Html.text "loading..."

statusClass : BuildEvent.BuildStatus -> String
statusClass status =
  case status of
    BuildEvent.BuildStatusPending ->
      "pending"

    BuildEvent.BuildStatusStarted ->
      "started"

    BuildEvent.BuildStatusSucceeded ->
      "succeeded"

    BuildEvent.BuildStatusFailed ->
      "failed"

    BuildEvent.BuildStatusErrored ->
      "errored"

    BuildEvent.BuildStatusAborted ->
      "aborted"

isRunning : BuildEvent.BuildStatus -> Bool
isRunning status =
  case status of
    BuildEvent.BuildStatusPending ->
      True

    BuildEvent.BuildStatusStarted ->
      True

    _ ->
      False

viewBuildHeader : Signal.Address Action -> Build -> BuildEvent.BuildStatus -> List Build -> Html
viewBuildHeader actions build status history =
  Html.div [id "page-header", class (statusClass status)]
    [ Html.div [class "build-header"]
        [ Html.div [class "build-actions fr"]
            [ Html.form [class "trigger-build", method "post", action ("/pipelines/" ++ build.pipelineName ++ "/jobs/" ++ build.jobName ++ "/builds") ]
                [ Html.button [ class "build-action fr" ] [ Html.i [class "fa fa-plus-circle" ] [] ] ]
            , if isRunning status then
                Html.span [class "build-action build-action-abort fr", onClick actions AbortBuild] [ Html.i [class "fa fa-times-circle"] [] ]
              else
                Html.span [] []
            ]
        , Html.h1 []
            [ Html.a [href ("/pipelines/" ++ build.pipelineName ++ "/jobs/" ++ build.jobName)] [ Html.text (build.jobName ++ " #" ++ build.name) ] ]
        , Html.dl [class "build-times"] []
        ]
    , Html.ul [id "builds"] (List.map (renderHistory build) history)
    ]


renderHistory : Build -> Build -> Html
renderHistory currentBuild build =
  Html.li
    [
      classList [
        (build.status, True),
        ("current", build.name == currentBuild.name)
      ]
    ]
    [ Html.a [href build.url] [ Html.text ("#" ++ build.name) ] ]

fetchBuildPlan : Time -> Int -> Effects.Effects Action
fetchBuildPlan delay buildId =
  let
    fetchPlan =
      Http.get BuildPlan.decode ("/api/v1/builds/" ++ toString buildId ++ "/plan")
        |> Task.toResult
        |> Task.map PlanFetched
  in
    Effects.task (Task.sleep delay `Task.andThen` \_ -> fetchPlan)

fetchBuild : Int -> Effects.Effects Action
fetchBuild buildId =
  Http.get decode ("/api/v1/builds/" ++ toString buildId)
    |> Task.toResult
    |> Task.map BuildFetched
    |> Effects.task

fetchBuildHistory : String -> String -> Effects.Effects Action
fetchBuildHistory pipelineName jobName =
  Http.get decodeBuilds ("/api/v1/pipelines/" ++ pipelineName ++ "/jobs/" ++ jobName ++ "/builds")
    |> Task.toResult
    |> Task.map BuildHistoryFetched
    |> Effects.task

decode : Json.Decode.Decoder Build
decode =
  Json.Decode.object6 Build
    ("id" := Json.Decode.int)
    ("name" := Json.Decode.string)
    ("status" := Json.Decode.string)
    ("job_name" := Json.Decode.string)
    ("pipeline_name" := Json.Decode.string)
    ("url" := Json.Decode.string)

decodeBuilds : Json.Decode.Decoder (List Build)
decodeBuilds =
  Json.Decode.list decode

subscribeToEvents : Int -> Signal.Address Action -> Effects.Effects Action
subscribeToEvents build actions =
  let
    settings =
      EventSource.Settings
        (Just <| Signal.forwardTo actions (always Opened))
        (Just <| Signal.forwardTo actions (always Errored))

    connect =
      EventSource.connect ("/api/v1/builds/" ++ toString build ++ "/events") settings

    eventsSub =
      EventSource.on "event" <|
        Signal.forwardTo actions (Event << parseEvent)

    endSub =
      EventSource.on "end" <|
        Signal.forwardTo actions (always EndOfEvents)
  in
    connect `Task.andThen` eventsSub `Task.andThen` endSub
      |> Task.map Listening
      |> Effects.task

closeEvents : EventSource.EventSource -> Effects.Effects Action
closeEvents eventSource =
  EventSource.close eventSource
    |> Task.map (always Closed)
    |> Effects.task

parseEvent : EventSource.Event -> Result String BuildEvent
parseEvent e = Json.Decode.decodeString BuildEvent.decode e.data

scrollToBottom : Effects.Effects Action
scrollToBottom =
  Scroll.toBottom
    |> Task.map (always Noop)
    |> Effects.task

toStatus : String -> BuildEvent.BuildStatus
toStatus str =
  case str of
    "started" -> BuildEvent.BuildStatusStarted
    "succeeded" -> BuildEvent.BuildStatusSucceeded
    "failed" -> BuildEvent.BuildStatusFailed
    "errored" -> BuildEvent.BuildStatusErrored
    "aborted" -> BuildEvent.BuildStatusAborted
    _ -> Debug.crash ("unknown state: " ++ str)
