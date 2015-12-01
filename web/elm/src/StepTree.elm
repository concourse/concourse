module StepTree
  ( StepTree(..)
  , Root
  , HookedStep
  , Step
  , StepID
  , StepName
  , StepState(..)
  , Action
  , init
  , map
  , view
  , update
  , updateAt
  ) where

import Debug
import Ansi
import Ansi.Log
import Array exposing (Array)
import Dict exposing (Dict)
import Focus exposing (Focus, (=>))
import Html exposing (Html)
import Html.Events exposing (onClick)
import Html.Lazy
import Html.Attributes exposing (class, classList)

import BuildPlan exposing (BuildPlan)

type StepTree
  = Task Step
  | Get Step (Maybe Version)
  | Put Step
  | DependentGet Step
  | Aggregate (Array StepTree)
  | OnSuccess HookedStep
  | OnFailure HookedStep
  | Ensure HookedStep
  | Try StepTree
  | Timeout StepTree

type Action = ToggleStep StepID

type alias HookedStep =
  { step : StepTree
  , hook : StepTree
  }

type alias Step =
  { id : StepID
  , name : StepName
  , state : StepState
  , log : Ansi.Log.Window
  , error : Maybe String
  , expanded : Bool
  }

type alias StepName = String

type alias StepID = String

type alias Version = Dict String String

type StepState
  = StepStatePending
  | StepStateRunning
  | StepStateSucceeded
  | StepStateFailed
  | StepStateErrored

type alias StepFocus =
  Focus StepTree StepTree

type alias Root =
  { tree : StepTree
  , foci : Dict StepID StepFocus
  }

init : BuildPlan -> Root
init plan =
  case plan.step of
    BuildPlan.Task name ->
      initBottom Task plan.id name

    BuildPlan.Get name version ->
      initBottom (flip Get version) plan.id name

    BuildPlan.Put name ->
      initBottom Put plan.id name

    BuildPlan.DependentGet name ->
      initBottom DependentGet plan.id name

    BuildPlan.Aggregate plans ->
      let
        inited = Array.map init plans
        trees = Array.map .tree inited
        subFoci = Array.map .foci inited
        wrappedSubFoci = Array.indexedMap wrapAgg subFoci
        foci = Array.foldr Dict.union Dict.empty wrappedSubFoci
      in
        Root (Aggregate trees) foci

    BuildPlan.OnSuccess hookedPlan ->
      initHookedStep OnSuccess hookedPlan

    BuildPlan.OnFailure hookedPlan ->
      initHookedStep OnFailure hookedPlan

    BuildPlan.Ensure hookedPlan ->
      initHookedStep Ensure hookedPlan

    BuildPlan.Try plan ->
      initWrappedStep Try plan

    BuildPlan.Timeout plan ->
      initWrappedStep Timeout plan

update : Action -> Root -> Root
update action root =
  case action of
    ToggleStep id ->
      updateAt id (map (\step -> { step | expanded = not step.expanded })) root

updateAt : StepID -> (StepTree -> StepTree) -> Root -> Root
updateAt id update root =
  case Dict.get id root.foci of
    Nothing ->
      root

    Just focus ->
      { root | tree = Focus.update focus update root.tree }

map : (Step -> Step) -> StepTree -> StepTree
map f tree =
  case tree of
    Task step ->
      Task (f step)

    Get step version ->
      Get (f step) version

    Put step ->
      Put (f step)

    DependentGet step ->
      DependentGet (f step)

    _ ->
      tree


initBottom : (Step -> StepTree) -> StepID -> StepName -> Root
initBottom create id name =
  let
    step =
      { id = id
      , name = name
      , state = StepStatePending
      , log = Ansi.Log.init Ansi.Log.Cooked
      , error = Nothing
      , expanded = True
      }
  in
    { tree = create step
    , foci = Dict.singleton id (Focus.create identity identity)
    }

initWrappedStep : (StepTree -> StepTree) -> BuildPlan -> Root
initWrappedStep create plan =
  let
    {tree, foci} = init plan
  in
    { tree = create tree
    , foci = Dict.map wrapStep foci
    }

initHookedStep : (HookedStep -> StepTree) -> BuildPlan.HookedPlan -> Root
initHookedStep create hookedPlan =
  let
    stepRoot = init hookedPlan.step
    hookRoot = init hookedPlan.hook
  in
    { tree = create { step = stepRoot.tree, hook = hookRoot.tree }
    , foci = Dict.union
        (Dict.map wrapStep stepRoot.foci)
        (Dict.map wrapHook hookRoot.foci)
    }

wrapAgg : Int -> Dict StepID StepFocus -> Dict StepID StepFocus
wrapAgg i = Dict.map (\_ focus -> Focus.create (getAggIndex i) (setAggIndex i) => focus)

wrapStep : StepID -> StepFocus -> StepFocus
wrapStep id subFocus =
  Focus.create getStep updateStep => subFocus

getStep : StepTree -> StepTree
getStep tree =
  case tree of
    OnSuccess {step} ->
      step

    OnFailure {step} ->
      step

    Ensure {step} ->
      step

    Try step ->
      step

    Timeout step ->
      step

    _ ->
      Debug.crash "impossible"

updateStep : (StepTree -> StepTree) -> StepTree -> StepTree
updateStep update tree =
  case tree of
    OnSuccess hookedStep ->
      OnSuccess { hookedStep | step = update hookedStep.step }

    OnFailure hookedStep ->
      OnFailure { hookedStep | step = update hookedStep.step }

    Ensure hookedStep ->
      Ensure { hookedStep | step = update hookedStep.step }

    Try step ->
      Try (update step)

    Timeout step ->
      Timeout (update step)

    _ ->
      Debug.crash "impossible"

wrapHook : StepID -> StepFocus -> StepFocus
wrapHook id subFocus =
  Focus.create getHook updateHook => subFocus

getHook : StepTree -> StepTree
getHook tree =
  case tree of
    OnSuccess {hook} ->
      hook

    OnFailure {hook} ->
      hook

    Ensure {hook} ->
      hook

    _ ->
      Debug.crash "impossible"

updateHook : (StepTree -> StepTree) -> StepTree -> StepTree
updateHook update tree =
  case tree of
    OnSuccess hookedStep ->
      OnSuccess { hookedStep | hook = update hookedStep.hook }

    OnFailure hookedStep ->
      OnFailure { hookedStep | hook = update hookedStep.hook }

    Ensure hookedStep ->
      Ensure { hookedStep | hook = update hookedStep.hook }

    _ ->
      Debug.crash "impossible"

getAggIndex : Int -> StepTree -> StepTree
getAggIndex idx tree =
  case tree of
    Aggregate trees ->
      case Array.get idx trees of
        Just sub ->
          sub

        Nothing ->
          Debug.crash "impossible"

    _ ->
      Debug.crash "impossible"

setAggIndex : Int -> (StepTree -> StepTree) -> StepTree -> StepTree
setAggIndex idx update tree =
  case tree of
    Aggregate trees ->
      Aggregate (Array.set idx (update (getAggIndex idx tree)) trees)

    _ ->
      Debug.crash "impossible"

view : Signal.Address Action -> StepTree -> Html
view actions tree =
  case tree of
    Task step ->
      viewStep actions step "fa-terminal"

    Get step _ ->
      viewStep actions step "fa-arrow-down"

    DependentGet step ->
      viewStep actions step "fa-arrow-down"

    Put step ->
      viewStep actions step "fa-arrow-up"

    Try step ->
      view actions step

    Timeout step ->
      view actions step

    Aggregate steps ->
      Html.div [class "aggregate"]
        (Array.toList <| Array.map (viewSeq actions) steps)

    OnSuccess {step, hook} ->
      viewHooked "success" actions step hook

    OnFailure {step, hook} ->
      viewHooked "failure" actions step hook

    Ensure {step, hook} ->
      viewHooked "ensure" actions step hook

viewSeq : Signal.Address Action -> StepTree -> Html
viewSeq actions tree =
  Html.div [class "seq"] [view actions tree]

viewHooked : String -> Signal.Address Action -> StepTree -> StepTree -> Html
viewHooked name actions step hook =
  Html.div [class "hooked"]
    [ Html.div [class "step"] [view actions step]
    , Html.div [class "children"]
        [ Html.div [class ("hook hook-" ++ name)] [view actions hook]
        ]
    ]

isInactive : StepState -> Bool
isInactive = (==) StepStatePending

viewStep : Signal.Address Action -> Step -> String -> Html
viewStep actions {id, name, log, state, error, expanded} icon =
  Html.div
    [ classList
      [ ("build-step", True)
      , ("inactive", isInactive state)
      ]
    ]
    [ Html.div [class "header", onClick actions (ToggleStep id)]
        [ viewStepState state
        , typeIcon icon
        , Html.h3 [] [Html.text name]
        ]
    , Html.div
        [ classList
            [ ("step-body", True)
            , ("step-collapsed", isInactive state || not expanded)
            ]
        ]
        [ viewStepLog log
        , case error of
            Nothing ->
              Html.span [] []
            Just msg ->
              Html.span [class "error"] [Html.text msg]
        ]
    ]

typeIcon : String -> Html
typeIcon fa =
  Html.i [class ("left fa fa-fw " ++ fa)] []

viewStepState : StepState -> Html
viewStepState state =
  case state of
    StepStatePending ->
      Html.i
        [ class "right fa fa-fw fa-circle-o-notch"
        ] []

    StepStateRunning ->
      Html.i
        [ class "right fa fa-fw fa-spin fa-circle-o-notch"
        ] []

    StepStateSucceeded ->
      Html.i
        [ class "right succeeded fa fa-fw fa-check"
        ] []

    StepStateFailed ->
      Html.i
        [ class "right failed fa fa-fw fa-times"
        ] []

    StepStateErrored ->
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
  Html.div [] (List.map viewChunk line)

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
