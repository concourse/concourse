module BuildOutput where

import Ansi.Log
import Date exposing (Date)
import Effects exposing (Effects)
import Html exposing (Html)
import Html.Attributes exposing (action, class, classList, href, id, method, title)
import Http
import Task exposing (Task)

import Concourse.Build exposing (Build)
import Concourse.BuildEvents
import Concourse.BuildPlan exposing (BuildPlan)
import Concourse.BuildResources exposing (BuildResources)
import Concourse.BuildStatus exposing (BuildStatus)
import Concourse.Metadata exposing (Metadata)
import Concourse.Version exposing (Version)
import LoadingIndicator
import EventSource exposing (EventSource)
import StepTree exposing (StepTree)

type alias Model =
  { build : Build
  , steps : Maybe StepTree.Model
  , errors : Maybe Ansi.Log.Model
  , state : OutputState
  , context : Context
  , eventSource : Maybe EventSource
  }

type alias Context =
  { events : Signal.Address Action
  , buildStatus : Signal.Address (BuildStatus, Date)
  }

type OutputState
  = StepsLoading
  | StepsLiveUpdating
  | StepsComplete
  | LoginRequired

type Action
  = Noop
  | PlanAndResourcesFetched (Result Http.Error (BuildPlan, BuildResources))
  | BuildEventsListening EventSource
  | BuildEventsAction Concourse.BuildEvents.Action
  | BuildEventsClosed
  | StepTreeAction StepTree.Action

init : Build -> Context -> (Model, Effects Action)
init build ctx =
  let
    outputState =
      if Concourse.BuildStatus.isRunning build.status then
        StepsLiveUpdating
      else
        StepsLoading

    model =
      { build = build
      , steps = Nothing
      , errors = Nothing
      , state = outputState
      , context = ctx
      , eventSource = Nothing
      }

    fetch =
      if build.job /= Nothing then
        fetchBuildPlanAndResources model.build.id
      else
        fetchBuildPlan model.build.id
  in
    (model, fetch)

update : Action -> Model -> (Model, Effects Action)
update action model =
  case action of
    Noop ->
      (model, Effects.none)

    PlanAndResourcesFetched (Err (Http.BadResponse 404 _)) ->
      (model, subscribeToEvents model.build.id model.context.events)

    PlanAndResourcesFetched (Err err) ->
      Debug.log ("failed to fetch plan: " ++ toString err) <|
        (model, Effects.none)

    PlanAndResourcesFetched (Ok (plan, resources)) ->
      ( { model | steps = Just (StepTree.init resources plan) }
      , subscribeToEvents model.build.id model.context.events
      )

    BuildEventsListening es ->
      ({ model | eventSource = Just es }, Effects.none)

    BuildEventsAction action ->
      handleEventsAction action model

    BuildEventsClosed ->
      ({ model | eventSource = Nothing }, Effects.none)

    StepTreeAction action ->
      ( { model | steps = Maybe.map (StepTree.update action) model.steps }
      , Effects.none
      )

handleEventsAction : Concourse.BuildEvents.Action -> Model -> (Model, Effects Action)
handleEventsAction action model =
  case action of
    Concourse.BuildEvents.Opened ->
      (model, Effects.none)

    Concourse.BuildEvents.Errored ->
      let
        newState =
          case model.state of
            -- if we're loading and the event source errors, assume we're not
            -- logged in (there's no way to actually tell)
            StepsLoading ->
              LoginRequired

            -- closing the event source causes an error to come in, so ignore
            -- it since that means everything actually worked
            StepsComplete ->
              model.state

            -- getting an error in the middle could just be the ATC going away
            -- (i.e. during a deploy). ignore it and let the browser
            -- auto-reconnect
            StepsLiveUpdating ->
              model.state

            -- shouldn't ever happen, but...
            LoginRequired ->
              model.state
      in
        ({ model | state = newState }, Effects.none)

    Concourse.BuildEvents.Event (Ok event) ->
      handleEvent event model

    Concourse.BuildEvents.Event (Err err) ->
      (model, Debug.log err Effects.none)

    Concourse.BuildEvents.End ->
      case model.eventSource of
        Just es ->
          ({ model | state = StepsComplete }, closeEvents es)

        Nothing ->
          (model, Effects.none)

handleEvent : Concourse.BuildEvents.BuildEvent -> Model -> (Model, Effects Action)
handleEvent event model =
  case event of
    Concourse.BuildEvents.Log origin output ->
      ( updateStep origin.id (setRunning << appendStepLog output) model
      , Effects.none
      )

    Concourse.BuildEvents.Error origin message ->
      ( updateStep origin.id (setStepError message) model
      , Effects.none
      )

    Concourse.BuildEvents.InitializeTask origin ->
      ( updateStep origin.id setRunning model
      , Effects.none
      )

    Concourse.BuildEvents.StartTask origin ->
      ( updateStep origin.id setRunning model
      , Effects.none
      )

    Concourse.BuildEvents.FinishTask origin exitStatus ->
      ( updateStep origin.id (finishStep exitStatus) model
      , Effects.none
      )

    Concourse.BuildEvents.InitializeGet origin ->
      ( updateStep origin.id setRunning model
      , Effects.none
      )

    Concourse.BuildEvents.FinishGet origin exitStatus version metadata ->
      ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
      , Effects.none
      )

    Concourse.BuildEvents.InitializePut origin ->
      ( updateStep origin.id setRunning model
      , Effects.none
      )

    Concourse.BuildEvents.FinishPut origin exitStatus version metadata ->
      ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
      , Effects.none
      )

    Concourse.BuildEvents.BuildStatus status date ->
      let
        finishSteps =
          if not <| Concourse.BuildStatus.isRunning status then
            { model | steps = Maybe.map (StepTree.update StepTree.Finished) model.steps }
          else
            model

        notifyStatus =
          Effects.task << Task.map (always Noop) <|
            Signal.send model.context.buildStatus (status, date)
      in
        (finishSteps, notifyStatus)

    Concourse.BuildEvents.BuildError message ->
      ( { model |
          errors =
            Just <|
              Ansi.Log.update message <|
                Maybe.withDefault (Ansi.Log.init Ansi.Log.Cooked) model.errors
        }
      , Effects.none
      )

updateStep : StepTree.StepID -> (StepTree -> StepTree) -> Model -> Model
updateStep id update model =
  { model | steps = Maybe.map (StepTree.updateAt id update) model.steps }

setRunning : StepTree -> StepTree
setRunning = setStepState StepTree.StepStateRunning

appendStepLog : String -> StepTree -> StepTree
appendStepLog output tree =
  StepTree.map (\step -> { step | log = Ansi.Log.update output step.log }) tree

setStepError : String -> StepTree -> StepTree
setStepError message tree =
  StepTree.map
    (\step ->
      { step
      | state = StepTree.StepStateErrored
      , error = Just message
      })
    tree

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

setResourceInfo : Version -> Metadata -> StepTree -> StepTree
setResourceInfo version metadata tree =
  StepTree.map (\step -> { step | version = Just version, metadata = metadata }) tree

setStepState : StepTree.StepState -> StepTree -> StepTree
setStepState state tree =
  StepTree.map (\step -> { step | state = state }) tree

fetchBuildPlanAndResources : Int -> Effects Action
fetchBuildPlanAndResources buildId =
  Task.map2 (,) (Concourse.BuildPlan.fetch buildId) (Concourse.BuildResources.fetch buildId)
    |> Task.toResult
    |> Task.map PlanAndResourcesFetched
    |> Effects.task

fetchBuildPlan : Int -> Effects Action
fetchBuildPlan buildId =
  Task.map (flip (,) Concourse.BuildResources.empty) (Concourse.BuildPlan.fetch buildId)
    |> Task.toResult
    |> Task.map PlanAndResourcesFetched
    |> Effects.task

subscribeToEvents : Int -> Signal.Address Action -> Effects Action
subscribeToEvents build actions =
  Concourse.BuildEvents.subscribe build (Signal.forwardTo actions BuildEventsAction)
    |> Task.map BuildEventsListening
    |> Effects.task

closeEvents : EventSource.EventSource -> Effects Action
closeEvents eventSource =
  EventSource.close eventSource
    |> Task.map (always BuildEventsClosed)
    |> Effects.task

view : Signal.Address Action -> Model -> Html
view actions {build, steps, errors, state} =
  Html.div (id "build-body" :: paddingClass build)
    [ Html.div [class "steps"]
        [ viewErrors errors
        , viewStepTree actions build steps state
        ]
    ]

viewStepTree : Signal.Address Action -> Build -> Maybe StepTree.Model -> OutputState -> Html
viewStepTree actions build steps state =
  case (state, steps) of
    (StepsLoading, _) ->
      LoadingIndicator.view

    (LoginRequired, _) ->
      viewLoginButton build

    (StepsLiveUpdating, Just root) ->
      StepTree.view (Signal.forwardTo actions StepTreeAction) root

    (StepsComplete, Just root) ->
      StepTree.view (Signal.forwardTo actions StepTreeAction) root

    (_, Nothing) ->
      Html.div [] []

viewErrors : Maybe Ansi.Log.Model -> Html
viewErrors errors =
  case errors of
    Nothing ->
      Html.div [] []

    Just log ->
      Html.div [class "build-step"]
        [ Html.div [class "header"]
            [ Html.i [class "left fa fa-fw fa-exclamation-triangle"] []
            , Html.h3 [] [Html.text "error"]
            ]
        , Html.div [class "step-body build-errors-body"] [Ansi.Log.view log]
        ]

viewLoginButton : Build -> Html
viewLoginButton build =
  Html.form
    [ class "build-login"
    , Html.Attributes.method "get"
    , Html.Attributes.action "/login"
    ]
    [ Html.input
        [ Html.Attributes.type' "submit"
        , Html.Attributes.value "log in to view"
        ] []
    , Html.input
        [ Html.Attributes.type' "hidden"
        , Html.Attributes.name "redirect"
        , Html.Attributes.value (Concourse.Build.url build)
        ] []
    ]

paddingClass : Build -> List Html.Attribute
paddingClass build =
  case build.job of
    Just _ ->
      []

    _ ->
      [class "build-body-noSubHeader"]
