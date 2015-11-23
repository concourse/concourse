module Build where

import Ansi
import Ansi.Log
import Array
import Debug
import Dict
import Effects exposing (Effects)
import EventSource exposing (EventSource)
import Focus
import Html exposing (Html)
import Html.Lazy
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

    --Event (Ok (BuildEvent.BuildStatus s)) ->
      --({ model | buildStatus = Just s }, Effects.none)

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
  case tree of
    StepTree.Task step ->
      StepTree.Task { step | error = Just message }

    StepTree.Get step version ->
      StepTree.Get { step | error = Just message } version

    _ ->
      tree

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
  case tree of
    StepTree.Task step ->
      StepTree.Task { step | state = state }

    StepTree.Get step version ->
      StepTree.Get { step | state = state } version

    _ ->
      tree

view : Signal.Address Action -> Model -> Html
view action model =
  case model.stepRoot of
    Nothing ->
      Html.text "loading..."

    Just root ->
      Html.div [class "steps"]
        [ viewStepTree root.tree ]

viewStepTree : StepTree -> Html
viewStepTree tree =
  case tree of
    StepTree.Task step ->
      viewStep step "fa-terminal"

    StepTree.Get step _ ->
      viewStep step "fa-arrow-down"

    StepTree.DependentGet step ->
      viewStep step "fa-arrow-down"

    StepTree.Put step ->
      viewStep step "fa-arrow-up"

    StepTree.Try step ->
      viewStepTree step

    StepTree.Timeout step ->
      viewStepTree step

    StepTree.Aggregate steps ->
      Html.div [class "aggregate"]
        (Array.toList <| Array.map viewStepTree steps)

    StepTree.OnSuccess {step, hook} ->
      Html.div [class "on-success"]
        [ Html.div [class "step"] [viewStepTree step]
        , Html.div [class "children hook-success"] [viewStepTree hook]
        ]

    StepTree.OnFailure {step, hook} ->
      Html.div [class "on-failure"]
        [ Html.div [class "step"] [viewStepTree step]
        , Html.div [class "children hook-failure"] [viewStepTree hook]
        ]

    StepTree.Ensure {step, hook} ->
      Html.div [class "ensure"]
        [ Html.div [class "step"] [viewStepTree step]
        , Html.div [class "children hook-ensure"] [viewStepTree hook]
        ]

viewStep : StepTree.Step -> String -> Html
viewStep {name, log, state, error} icon =
  Html.div [class "build-step"]
    [ Html.div [class "header"]
        [ viewStepState state
        , typeIcon icon
        , Html.h3 [] [Html.text name]
        ]
    , Html.div [class "step-body"]
        [ viewStepLog log
        ]
    , case error of
        Nothing ->
          Html.div [] []
        Just msg ->
          Html.div [class "step-error"]
            [Html.span [class "error"] [Html.text msg]]
    ]


typeIcon : String -> Html
typeIcon fa =
  Html.i [class ("left fa fa-fw " ++ fa)] []

viewStepState : StepTree.StepState -> Html
viewStepState state =
  case state of
    StepTree.StepStatePending ->
      Html.i
        [ class "right fa fa-fw fa-beer"
        ] []

    StepTree.StepStateRunning ->
      Html.i
        [ class "right fa fa-fw fa-spin fa-circle-o-notch"
        ] []

    StepTree.StepStateSucceeded ->
      Html.i
        [ class "right succeeded fa fa-fw fa-check"
        ] []

    StepTree.StepStateFailed ->
      Html.i
        [ class "right failed fa fa-fw fa-times"
        ] []

    StepTree.StepStateErrored ->
      Html.i
        [ class "right errored fa fa-fw fa-exclamation-triangle"
        ] []

viewStepLog : Ansi.Log.Window -> Html.Html
viewStepLog window =
  Html.pre []
    (Array.toList (Array.map lazyLine window.lines))

lazyLine : Ansi.Log.Line -> Html.Html
lazyLine = Html.Lazy.lazy viewLine

viewLine : Ansi.Log.Line -> Html.Html
viewLine line =
  case line of
    [] -> Html.div [] [Html.text "\n"]
    _  -> Html.div [] (List.map viewChunk line)

viewChunk : Ansi.Log.Chunk -> Html.Html
viewChunk chunk =
  Html.span (styleAttributes chunk.style)
    [Html.text chunk.text]

styleAttributes : Ansi.Log.Style -> List Html.Attribute
styleAttributes style =
  [ Html.Attributes.style [("font-weight", if style.bold then "bold" else "normal")]
  , let
      fgClasses =
        colorClasses "-fg"
          style.bold
          (if not style.inverted then style.foreground else style.background)
      bgClasses =
        colorClasses "-bg"
          style.bold
          (if not style.inverted then style.background else style.foreground)
    in
      Html.Attributes.classList (List.map (flip (,) True) (fgClasses ++ bgClasses))
  ]

colorClasses : String -> Bool -> Maybe Ansi.Color -> List String
colorClasses suffix bold mc =
  let
    brightPrefix = "ansi-bright-"

    prefix =
      if bold then
        brightPrefix
      else
        "ansi-"
  in
    case mc of
      Nothing ->
        if bold then
          ["ansi-bold"]
        else
          []
      Just (Ansi.Black) ->   [prefix ++ "black" ++ suffix]
      Just (Ansi.Red) ->     [prefix ++ "red" ++ suffix]
      Just (Ansi.Green) ->   [prefix ++ "green" ++ suffix]
      Just (Ansi.Yellow) ->  [prefix ++ "yellow" ++ suffix]
      Just (Ansi.Blue) ->    [prefix ++ "blue" ++ suffix]
      Just (Ansi.Magenta) -> [prefix ++ "magenta" ++ suffix]
      Just (Ansi.Cyan) ->    [prefix ++ "cyan" ++ suffix]
      Just (Ansi.White) ->   [prefix ++ "white" ++ suffix]
      Just (Ansi.BrightBlack) ->   [brightPrefix ++ "black" ++ suffix]
      Just (Ansi.BrightRed) ->     [brightPrefix ++ "red" ++ suffix]
      Just (Ansi.BrightGreen) ->   [brightPrefix ++ "green" ++ suffix]
      Just (Ansi.BrightYellow) ->  [brightPrefix ++ "yellow" ++ suffix]
      Just (Ansi.BrightBlue) ->    [brightPrefix ++ "blue" ++ suffix]
      Just (Ansi.BrightMagenta) -> [brightPrefix ++ "magenta" ++ suffix]
      Just (Ansi.BrightCyan) ->    [brightPrefix ++ "cyan" ++ suffix]
      Just (Ansi.BrightWhite) ->   [brightPrefix ++ "white" ++ suffix]

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
