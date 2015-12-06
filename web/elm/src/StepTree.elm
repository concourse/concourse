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
import Ansi.Log
import Array exposing (Array)
import Dict exposing (Dict)
import Focus exposing (Focus, (=>))
import Html exposing (Html)
import Html.Events exposing (onClick)
import Html.Attributes exposing (class, classList)

import Concourse.BuildPlan exposing (BuildPlan)
import Concourse.BuildResources exposing (BuildResources)

type StepTree
  = Task Step
  | Get Step
  | Put Step
  | DependentGet Step
  | Aggregate (Array StepTree)
  | Do (Array StepTree)
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
  , log : Ansi.Log.Model
  , error : Maybe String
  , expanded : Maybe Bool
  , version : Maybe Version
  , metadata : List MetadataField
  , firstOccurrence : Bool
  }

type alias StepName = String

type alias StepID = String

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

type alias Version =
  Dict String String

type alias MetadataField =
  { name : String
  , value : String
  }

init : BuildResources -> BuildPlan -> Root
init resources plan =
  case plan.step of
    Concourse.BuildPlan.Task name ->
      initBottom Task plan.id name

    Concourse.BuildPlan.Get name version ->
      initBottom (Get << setupGetStep resources name version) plan.id name

    Concourse.BuildPlan.Put name ->
      initBottom Put plan.id name

    Concourse.BuildPlan.DependentGet name ->
      initBottom DependentGet plan.id name

    Concourse.BuildPlan.Aggregate plans ->
      let
        inited = Array.map (init resources) plans
        trees = Array.map .tree inited
        subFoci = Array.map .foci inited
        wrappedSubFoci = Array.indexedMap wrapMultiStep subFoci
        foci = Array.foldr Dict.union Dict.empty wrappedSubFoci
      in
        Root (Aggregate trees) foci

    Concourse.BuildPlan.Do plans ->
      let
        inited = Array.map (init resources) plans
        trees = Array.map .tree inited
        subFoci = Array.map .foci inited
        wrappedSubFoci = Array.indexedMap wrapMultiStep subFoci
        foci = Array.foldr Dict.union Dict.empty wrappedSubFoci
      in
        Root (Do trees) foci

    Concourse.BuildPlan.OnSuccess hookedPlan ->
      initHookedStep resources OnSuccess hookedPlan

    Concourse.BuildPlan.OnFailure hookedPlan ->
      initHookedStep resources OnFailure hookedPlan

    Concourse.BuildPlan.Ensure hookedPlan ->
      initHookedStep resources Ensure hookedPlan

    Concourse.BuildPlan.Try plan ->
      initWrappedStep resources Try plan

    Concourse.BuildPlan.Timeout plan ->
      initWrappedStep resources Timeout plan

setupGetStep : BuildResources -> StepName -> Maybe Version -> Step -> Step
setupGetStep resources name version step =
  { step
  | version = version
  , firstOccurrence = isFirstOccurrence resources.inputs name
  }

isFirstOccurrence : List Concourse.BuildResources.BuildInput -> StepName -> Bool
isFirstOccurrence resources step =
  case resources of
    [] ->
      False

    {name, firstOccurrence} :: rest ->
      if name == step then
        firstOccurrence
      else
        isFirstOccurrence rest step


update : Action -> Root -> Root
update action root =
  case action of
    ToggleStep id ->
      updateAt id (map (\step -> { step | expanded = Just <| not <| Maybe.withDefault True step.expanded })) root

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

    Get step ->
      Get (f step)

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
      , expanded = Nothing
      , version = Nothing
      , metadata = []
      , firstOccurrence = False
      }
  in
    { tree = create step
    , foci = Dict.singleton id (Focus.create identity identity)
    }

initWrappedStep : BuildResources -> (StepTree -> StepTree) -> BuildPlan -> Root
initWrappedStep resources create plan =
  let
    {tree, foci} = init resources plan
  in
    { tree = create tree
    , foci = Dict.map wrapStep foci
    }

initHookedStep : BuildResources -> (HookedStep -> StepTree) -> Concourse.BuildPlan.HookedPlan -> Root
initHookedStep resources create hookedPlan =
  let
    stepRoot = init resources hookedPlan.step
    hookRoot = init resources hookedPlan.hook
  in
    { tree = create { step = stepRoot.tree, hook = hookRoot.tree }
    , foci = Dict.union
        (Dict.map wrapStep stepRoot.foci)
        (Dict.map wrapHook hookRoot.foci)
    }

wrapMultiStep : Int -> Dict StepID StepFocus -> Dict StepID StepFocus
wrapMultiStep i = Dict.map (\_ focus -> Focus.create (getMultiStepIndex i) (setMultiStepIndex i) => focus)

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

getMultiStepIndex : Int -> StepTree -> StepTree
getMultiStepIndex idx tree =
  let
    steps =
      case tree of
        Aggregate trees ->
          trees

        Do trees ->
          trees

        _ ->
          Debug.crash "impossible"
  in
    case Array.get idx steps of
      Just sub ->
        sub

      Nothing ->
        Debug.crash "impossible"

setMultiStepIndex : Int -> (StepTree -> StepTree) -> StepTree -> StepTree
setMultiStepIndex idx update tree =
  case tree of
    Aggregate trees ->
      Aggregate (Array.set idx (update (getMultiStepIndex idx tree)) trees)

    Do trees ->
      Do (Array.set idx (update (getMultiStepIndex idx tree)) trees)

    _ ->
      Debug.crash "impossible"

view : Signal.Address Action -> StepTree -> Html
view actions tree =
  case tree of
    Task step ->
      viewStep actions step "fa-terminal"

    Get step ->
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

    Do steps ->
      Html.div [class "do"]
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

isActive : StepState -> Bool
isActive = (/=) StepStatePending

viewStep : Signal.Address Action -> Step -> String -> Html
viewStep actions {id, name, log, state, error, expanded, version, metadata, firstOccurrence} icon =
  Html.div
    [ classList
      [ ("build-step", True)
      , ("inactive", not <| isActive state)
      , ("first-occurrence", firstOccurrence)
      ]
    ]
    [ Html.div [class "header", onClick actions (ToggleStep id)]
        [ viewStepState state
        , typeIcon icon
        , Html.dl [class "version"] <|
            List.concatMap (uncurry viewPair) << Dict.toList <|
              Maybe.withDefault Dict.empty version
        , Html.h3 [] [Html.text name]
        ]
    , Html.div
        [ classList
            [ ("step-body", True)
            , ("clearfix", True)
            , ("step-collapsed", not <| Maybe.withDefault (isActive state) expanded)
            ]
        ]
        [ Html.dl [class "build-metadata fr"]
            (List.concatMap (\{name, value} -> viewPair name value) metadata)
        , Ansi.Log.view log
        , case error of
            Nothing ->
              Html.span [] []
            Just msg ->
              Html.span [class "error"] [Html.text msg]
        ]
    ]

viewPair : String -> String -> List Html
viewPair name value =
  [ Html.dt [] [Html.text name]
  , Html.dd [] [Html.text value]
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
