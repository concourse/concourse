module BuildOutput exposing (..)

import Ansi.Log
import Html exposing (Html)
import Html.App
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
import StepTree exposing (StepTree)

type alias Model =
  { build : Build
  , steps : Maybe StepTree.Model
  , errors : Maybe Ansi.Log.Model
  , state : OutputState
  , eventSourceOpened : Bool
  , events : Sub Action
  }

type OutputState
  = StepsLoading
  | StepsLiveUpdating
  | StepsComplete
  | LoginRequired

type Action
  = Noop
  | PlanAndResourcesFetched (Result Http.Error (BuildPlan, BuildResources))
  | BuildEventsAction Concourse.BuildEvents.Action
  | StepTreeAction StepTree.Action

init : Build -> (Model, Cmd Action)
init build =
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
      , events = Sub.none
      , eventSourceOpened = False
      }

    fetch =
      if build.job /= Nothing then
        fetchBuildPlanAndResources model.build.id
      else
        fetchBuildPlan model.build.id
  in
    (model, fetch)

update : Action -> Model -> (Model, Cmd Action)
update action model =
  case action of
    Noop ->
      (model, Cmd.none)

    PlanAndResourcesFetched (Err (Http.BadResponse 404 _)) ->
      ( { model | events = subscribeToEvents model.build.id }
      , Cmd.none
      )

    PlanAndResourcesFetched (Err err) ->
      Debug.log ("failed to fetch plan: " ++ toString err) <|
        (model, Cmd.none)

    PlanAndResourcesFetched (Ok (plan, resources)) ->
      ( { model | steps = Just (StepTree.init resources plan)
                , events = subscribeToEvents model.build.id }
      , Cmd.none
      )

    BuildEventsAction action ->
      handleEventsAction action model

    StepTreeAction action ->
      ( { model | steps = Maybe.map (StepTree.update action) model.steps }
      , Cmd.none
      )

handleEventsAction : Concourse.BuildEvents.Action -> Model -> (Model, Cmd Action)
handleEventsAction action model =
  case action of
    Concourse.BuildEvents.Opened ->
      ({ model | eventSourceOpened = True }, Cmd.none)

    Concourse.BuildEvents.Errored ->
      if model.eventSourceOpened then
        -- connection could have dropped out of the blue; just let the browser
        -- handle reconnecting
        (model, Cmd.none)
      else
        -- assume request was rejected because auth is required; no way to
        -- really tell
        ({ model | state = LoginRequired }, Cmd.none)

    Concourse.BuildEvents.Event (Ok event) ->
      handleEvent event model

    Concourse.BuildEvents.Event (Err err) ->
      (model, Debug.log err Cmd.none)

    Concourse.BuildEvents.End ->
      ({ model | state = StepsComplete, events = Sub.none }, Cmd.none)

handleEvent : Concourse.BuildEvents.BuildEvent -> Model -> (Model, Cmd Action)
handleEvent event model =
  case event of
    Concourse.BuildEvents.Log origin output ->
      ( updateStep origin.id (setRunning << appendStepLog output) model
      , Cmd.none
      )

    Concourse.BuildEvents.Error origin message ->
      ( updateStep origin.id (setStepError message) model
      , Cmd.none
      )

    Concourse.BuildEvents.InitializeTask origin ->
      ( updateStep origin.id setRunning model
      , Cmd.none
      )

    Concourse.BuildEvents.StartTask origin ->
      ( updateStep origin.id setRunning model
      , Cmd.none
      )

    Concourse.BuildEvents.FinishTask origin exitStatus ->
      ( updateStep origin.id (finishStep exitStatus) model
      , Cmd.none
      )

    Concourse.BuildEvents.InitializeGet origin ->
      ( updateStep origin.id setRunning model
      , Cmd.none
      )

    Concourse.BuildEvents.FinishGet origin exitStatus version metadata ->
      ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
      , Cmd.none
      )

    Concourse.BuildEvents.InitializePut origin ->
      ( updateStep origin.id setRunning model
      , Cmd.none
      )

    Concourse.BuildEvents.FinishPut origin exitStatus version metadata ->
      ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
      , Cmd.none
      )

    Concourse.BuildEvents.BuildStatus status date ->
      ( { model
        | steps =
            if not <| Concourse.BuildStatus.isRunning status then
              Maybe.map (StepTree.update StepTree.Finished) model.steps
            else
              model.steps
        , build =
            case (status, model.build, model.build.duration) of
              (Concourse.BuildStatus.Started, _, _) -> model.build
              (Concourse.BuildStatus.Pending, _, _) -> model.build
              (_, build, duration) ->
                { build
                | status = status
                , duration = { duration | finishedAt = Just date }
                }
        }
      , Cmd.none
      )

    Concourse.BuildEvents.BuildError message ->
      ( { model |
          errors =
            Just <|
              Ansi.Log.update message <|
                Maybe.withDefault (Ansi.Log.init Ansi.Log.Cooked) model.errors
        }
      , Cmd.none
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

fetchBuildPlanAndResources : Int -> Cmd Action
fetchBuildPlanAndResources buildId =
  Cmd.map PlanAndResourcesFetched << Task.perform Err Ok <|
    Task.map2 (,) (Concourse.BuildPlan.fetch buildId) (Concourse.BuildResources.fetch buildId)

fetchBuildPlan : Int -> Cmd Action
fetchBuildPlan buildId =
  Cmd.map PlanAndResourcesFetched << Task.perform Err Ok <|
    Task.map (flip (,) Concourse.BuildResources.empty) (Concourse.BuildPlan.fetch buildId)

subscribeToEvents : Int -> Sub Action
subscribeToEvents build =
  Sub.map BuildEventsAction (Concourse.BuildEvents.subscribe build)

view : Model -> Html Action
view {build, steps, errors, state} =
  Html.div [class "steps"]
    [ viewErrors errors
    , viewStepTree build steps state
    ]

viewStepTree : Build -> Maybe StepTree.Model -> OutputState -> Html Action
viewStepTree build steps state =
  case (state, steps) of
    (StepsLoading, _) ->
      LoadingIndicator.view

    (LoginRequired, _) ->
      viewLoginButton build

    (StepsLiveUpdating, Just root) ->
      Html.App.map StepTreeAction (StepTree.view root)

    (StepsComplete, Just root) ->
      Html.App.map StepTreeAction (StepTree.view root)

    (_, Nothing) ->
      Html.div [] []

viewErrors : Maybe Ansi.Log.Model -> Html msg
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

viewLoginButton : Build -> Html msg
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

