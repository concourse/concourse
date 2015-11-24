module Build where

import Ansi.Log
import Debug
import Effects exposing (Effects)
import EventSource exposing (EventSource)
import Html exposing (Html)
import Html.Attributes exposing (class)
import Http
import Json.Decode
import Task
import Time exposing (Time)

import BuildEvent exposing (BuildEvent)
import BuildPlan exposing (BuildPlan)
import StepTree exposing (StepTree)


type alias Model =
  { actions : Signal.Address Action
  , buildId : Int
  , stepRoot : Maybe StepTree.Root
  , eventSource : Maybe EventSource
  , eventsLoaded : Bool
  }

type Action
  = Noop
  | PlanFetched (Result Http.Error BuildPlan)
  | Listening EventSource
  | Opened
  | Errored
  | Event (Result String BuildEvent)
  | EndOfEvents
  | Closed
  | StepTreeAction StepTree.Action

init : Signal.Address Action -> Int -> (Model, Effects Action)
init actions buildId =
  let
    model =
      { actions = actions
      , buildId = buildId
      , stepRoot = Nothing
      , eventSource = Nothing
      , eventsLoaded = False
      }
  in
    (model, fetchBuildPlan 0 buildId)

update : Action -> Model -> (Model, Effects Action)
update action model =
  case action of
    Noop ->
      (model, Effects.none)

    PlanFetched (Err (Http.BadResponse 404 _)) ->
      (model, fetchBuildPlan Time.second model.buildId)

    PlanFetched (Err err) ->
      Debug.log ("failed to fetch plan: " ++ toString err) <|
        (model, Effects.none)

    PlanFetched (Ok plan) ->
      ( { model | stepRoot = Just (StepTree.init plan) }
      , subscribeToEvents model.buildId model.actions
      )

    Listening es ->
      ({ model | eventSource = Just es }, Effects.none)

    Opened ->
      (model, Effects.none)

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

    Event (Ok (BuildEvent.BuildStatus _)) ->
      (model, Effects.none)

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
  case model.stepRoot of
    Nothing ->
      Html.text "loading..."

    Just root ->
      Html.div [class "steps"]
        [ StepTree.view (Signal.forwardTo actions StepTreeAction) root.tree ]

fetchBuildPlan : Time -> Int -> Effects.Effects Action
fetchBuildPlan delay buildId =
  let
    fetchPlan =
      Http.get BuildPlan.decode ("/api/v1/builds/" ++ toString buildId ++ "/plan")
        |> Task.toResult
        |> Task.map PlanFetched
  in
    Effects.task (Task.sleep delay `Task.andThen` \_ -> fetchPlan)

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
