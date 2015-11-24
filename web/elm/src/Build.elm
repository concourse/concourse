module Build where

import Ansi.Log
import Debug
import Dict
import Effects exposing (Effects)
import EventSource exposing (EventSource)
import Focus
import Html exposing (Html)
import Html.Attributes exposing (class)
import Http
import Json.Decode
import Task

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
    (model, fetchBuildPlan buildId)

update : Action -> Model -> (Model, Effects Action)
update action model =
  case action of
    Noop ->
      (model, Effects.none)

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
      ( { model | stepRoot = Maybe.map (appendLog origin.id output) model.stepRoot }
      , Effects.none
      )

    Event (Ok (BuildEvent.Error origin message)) ->
      ( { model | stepRoot = Maybe.map (setError origin.id message) model.stepRoot }
      , Effects.none
      )

    Event (Ok (BuildEvent.FinishTask origin exitStatus)) ->
      ( { model | stepRoot = Maybe.map (finishStep origin.id exitStatus) model.stepRoot }
      , Effects.none
      )

    Event (Ok (BuildEvent.FinishGet origin exitStatus)) ->
      ( { model | stepRoot = Maybe.map (finishStep origin.id exitStatus) model.stepRoot }
      , Effects.none
      )

    Event (Ok (BuildEvent.BuildStatus _)) ->
      (model, Effects.none)

    Event (Err e) ->
      (model, Debug.log e Effects.none)

    EndOfEvents ->
      case model.eventSource of
        Just es ->
          ({ model | eventsLoaded = True }, closeEvents es)

        Nothing ->
          (model, Effects.none)

    Closed ->
      ({ model | eventSource = Nothing }, Effects.none)


appendLog : String -> String -> StepTree.Root -> StepTree.Root
appendLog id output {tree, foci} =
  case Dict.get id foci of
    Nothing ->
      -- unknown step
      {tree = tree, foci = foci}

    Just focus ->
      {tree = Focus.update focus (appendStepLog output) tree, foci = foci}

appendStepLog : String -> StepTree -> StepTree
appendStepLog output tree =
  case tree of
    StepTree.Task step ->
      StepTree.Task { step | log = Ansi.Log.update output step.log }

    StepTree.Get step version ->
      StepTree.Get { step | log = Ansi.Log.update output step.log } version

    _ ->
      tree

setError : String -> String -> StepTree.Root -> StepTree.Root
setError id message {tree, foci} =
  case Dict.get id foci of
    Nothing ->
      -- unknown step
      {tree = tree, foci = foci}

    Just focus ->
      {tree = Focus.update focus (setStepError message) tree, foci = foci}

setStepError : String -> StepTree -> StepTree
setStepError message tree =
  StepTree.map (\step -> { step | error = Just message }) tree

finishStep : String -> Int -> StepTree.Root -> StepTree.Root
finishStep id exitStatus {tree, foci} =
  case Dict.get id foci of
    Nothing ->
      -- unknown step
      {tree = tree, foci = foci}

    Just focus ->
      let
        stepState =
          if exitStatus == 0 then
            StepTree.StepStateSucceeded
          else
            StepTree.StepStateFailed
      in
        {tree = Focus.update focus (setStepState stepState) tree, foci = foci}

setStepState : StepTree.StepState -> StepTree -> StepTree
setStepState state tree =
  StepTree.map (\step -> { step | state = state }) tree

view : Signal.Address Action -> Model -> Html
view action model =
  case model.stepRoot of
    Nothing ->
      Html.text "loading..."

    Just root ->
      Html.div [class "steps"]
        [ StepTree.view root.tree ]

fetchBuildPlan : Int -> Effects.Effects Action
fetchBuildPlan buildId =
  Http.get BuildPlan.decode ("/api/v1/builds/" ++ toString buildId ++ "/plan")
    |> Task.toResult
    |> Task.map PlanFetched
    |> Effects.task

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
