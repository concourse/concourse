module Build where

import Date exposing (Date)
import Date.Format
import Debug
import Dict exposing (Dict)
import Effects exposing (Effects)
import Html exposing (Html)
import Html.Attributes exposing (action, class, classList, href, id, method, title, disabled, attribute)
import Html.Events exposing (onClick, on, onWithOptions)
import Html.Lazy
import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)
import Time exposing (Time)
import String

import Autoscroll exposing (ScrollBehavior (..))
import BuildOutput
import Concourse.Build exposing (Build, BuildDuration)
import Concourse.BuildPrep exposing (BuildPrep, BuildPrepStatus)
import Concourse.BuildStatus exposing (BuildStatus)
import Concourse.Pagination exposing (Paginated)
import LoadingIndicator
import BuildDuration
import Scroll

import Concourse.Job exposing (Job)

type alias Model =
  { redirect : Signal.Address String
  , actions : Signal.Address Action
  , buildId : Int
  , build : Maybe Build
  , buildPrep: Maybe BuildPrep
  , history : List Build
  , job : Maybe Job
  , status : BuildStatus
  , now : Time.Time
  , duration : BuildDuration
  , output : Maybe BuildOutput.Model
  }

type StepRenderingState
  = StepsLoading
  | StepsLiveUpdating
  | StepsComplete
  | LoginRequired

type Action
  = Noop
  | BuildFetched (Result Http.Error Build)
  | BuildPrepFetched (Result Http.Error BuildPrep)
  | BuildHistoryFetched (Result Http.Error (Paginated Build))
  | BuildJobDetailsFetched (Result Http.Error Job)
  | BuildOutputAction BuildOutput.Action
  | BuildStatus BuildStatus Date
  | ScrollBuilds (Float, Float)
  | ClockTick Time.Time
  | AbortBuild
  | BuildAborted (Result Http.Error ())
  | RevealCurrentBuildInHistory

init : Signal.Address String -> Signal.Address Action -> Int -> (Model, Effects Action)
init redirect actions buildId =
  let
    model =
      { redirect = redirect
      , actions = actions
      , buildId = buildId
      , output = Nothing
      , build = Nothing
      , buildPrep = Nothing
      , history = []
      , job = Nothing
      , status = Concourse.BuildStatus.Pending
      , now = 0
      , duration = BuildDuration Nothing Nothing
      }
  in
    (model, fetchBuild 0 buildId)

update : Action -> Model -> (Model, Effects Action)
update action model =
  case action of
    Noop ->
      (model, Effects.none)

    AbortBuild ->
      (model, abortBuild model.buildId)

    BuildAborted (Ok ()) ->
      (model, Effects.none)

    BuildAborted (Err (Http.BadResponse 401 _)) ->
      (model, redirectToLogin model)

    BuildAborted (Err err) ->
      Debug.log ("failed to abort build: " ++ toString err) <|
        (model, Effects.none)

    BuildFetched (Ok build) ->
      handleBuildFetched build model

    BuildFetched (Err err) ->
      Debug.log ("failed to fetch build: " ++ toString err) <|
        (model, Effects.none)

    BuildPrepFetched (Ok buildPrep) ->
      handleBuildPrepFetched buildPrep model

    BuildPrepFetched (Err err) ->
      Debug.log ("failed to fetch build preparation: " ++ toString err) <|
        (model, Effects.none)

    BuildOutputAction action ->
      case model.output of
        Just output ->
          let
            (newOutput, effects) = BuildOutput.update action output
          in
            ({ model | output = Just newOutput }, Effects.map BuildOutputAction effects)

        Nothing ->
          Debug.crash "impossible (received action for missing BuildOutput)"

    BuildStatus status date ->
      ( updateStartFinishAt status date <|
          if Concourse.BuildStatus.isRunning model.status then
            { model | status = status }
          else
            model
      , Effects.none
      )

    BuildHistoryFetched (Err err) ->
      Debug.log ("failed to fetch build history: " ++ toString err) <|
        (model, Effects.none)

    BuildHistoryFetched (Ok history) ->
      handleHistoryFetched history model

    BuildJobDetailsFetched (Ok job) ->
      handleBuildJobFetched job model

    BuildJobDetailsFetched (Err err) ->
      Debug.log ("failed to fetch build job details: " ++ toString err) <|
        (model, Effects.none)

    RevealCurrentBuildInHistory ->
      (model, scrollToCurrentBuildInHistory)

    ScrollBuilds (0, deltaY) ->
      (model, scrollBuilds deltaY)

    ScrollBuilds (deltaX, _) ->
      (model, scrollBuilds -deltaX)

    ClockTick now ->
      ({ model | now = now }, Effects.none)

handleBuildFetched : Build -> Model -> (Model, Effects Action)
handleBuildFetched build model =
  let
    withBuild =
      { model | build = Just build
              , status = build.status
              , duration = build.duration }

    fetchHistory =
      case (model.build, build.job) of
        (Nothing, Just job) ->
          fetchBuildHistory job Nothing

        _ ->
          Effects.none

    fetchJobDetails =
      case model.job of
        Nothing ->
          fetchBuildJobDetails build
        _ ->
          Effects.none

    (newModel, effects) =
      if build.status == Concourse.BuildStatus.Pending then
        pollUntilStarted withBuild
      else if build.reapTime == Nothing then
        case model.buildPrep of
          Nothing -> initBuildOutput build withBuild
          Just _ ->
            let (newModel, effects) = initBuildOutput build withBuild in
              ( newModel
              , Effects.batch [effects, fetchBuildPrep Time.second model.buildId]
              )
      else (withBuild, Effects.none)
  in
    (newModel, Effects.batch [effects, fetchHistory, fetchJobDetails])

pollUntilStarted : Model -> (Model, Effects Action)
pollUntilStarted model =
  (
    model,
    Effects.batch
      [ (fetchBuild Time.second model.buildId)
      , (fetchBuildPrep Time.second model.buildId)
      ]
  )

initBuildOutput : Build -> Model -> (Model, Effects Action)
initBuildOutput build model =
  let
    (output, outputEffects) =
      BuildOutput.init
        build
        { events = Signal.forwardTo model.actions BuildOutputAction
        , buildStatus = Signal.forwardTo model.actions (uncurry BuildStatus)
        }
  in
    ( { model | output = Just output }
    , Effects.map BuildOutputAction outputEffects
    )

handleBuildJobFetched : Job -> Model -> (Model, Effects Action)
handleBuildJobFetched job model =
  let
    withJobDetails =
      { model | job = Just job }
  in
    (withJobDetails, Effects.none)

handleHistoryFetched : Paginated Build -> Model -> (Model, Effects Action)
handleHistoryFetched history model =
  let
    withBuilds =
      { model | history = List.append model.history history.content }

    loadedCurrentBuild =
      List.any ((==) model.buildId << .id) history.content

    scrollToCurrent =
      if loadedCurrentBuild then
        -- deferred so that UI will render build first, so we can scroll to it
        Effects.tick (always RevealCurrentBuildInHistory)
      else
        Effects.none
  in
    case (history.pagination.nextPage, model.build `Maybe.andThen` .job) of
      (Nothing, _) ->
        (withBuilds, scrollToCurrent)

      (Just page, Just job) ->
        (withBuilds, Effects.batch [fetchBuildHistory job (Just page), scrollToCurrent])

      (Just url, Nothing) ->
        Debug.crash "impossible"

handleBuildPrepFetched : BuildPrep -> Model -> (Model, Effects Action)
handleBuildPrepFetched buildPrep model =
  ({model | buildPrep = Just buildPrep}, Effects.none)

updateStartFinishAt : BuildStatus -> Date -> Model -> Model
updateStartFinishAt status date model =
  let
    duration = model.duration
  in
    case status of
      Concourse.BuildStatus.Started ->
        { model | duration = { duration | startedAt = Just date } }

      _ ->
        { model | duration = { duration | finishedAt = Just date } }

abortBuild : Int -> Effects Action
abortBuild buildId =
  Concourse.Build.abort buildId
    |> Task.toResult
    |> Task.map BuildAborted
    |> Effects.task

view : Signal.Address Action -> Model -> Html
view actions model =
  case model.build of
    Just build ->
      Html.div []
        [ viewBuildHeader actions build model
        , Html.div (id "build-body" :: paddingClass build) <|
          [ viewBuildPrep model.buildPrep
          , Html.Lazy.lazy (viewBuildOutput actions) model.output
          ] ++
            case (build.duration.startedAt, build.reapTime) of
              (Just startedAt, Just reapTime) ->
                [ Html.div
                    [ class "tombstone" ]
                    [ Html.div [ class "heading" ] [ Html.text "RIP" ]
                    , Html.div
                        [ class "job-name" ]
                        [ Html.text <|
                            Maybe.withDefault
                              "one-off build" <|
                              Maybe.map .name build.job
                        ]
                    , Html.div
                        [ class "build-name" ]
                        [ Html.text <|
                            "build #" ++
                              case build.job of
                                Nothing -> toString build.id
                                Just _ -> build.name
                        ]
                    , Html.div
                        [ class "date" ]
                        [ Html.text <|
                            mmDDYY startedAt ++ "-" ++ mmDDYY reapTime
                        ]
                    , Html.div
                        [ class "epitaph" ]
                        [ Html.text <|
                            case build.status of
                              Concourse.BuildStatus.Succeeded -> "It passed, and now it has passed on."
                              Concourse.BuildStatus.Failed -> "It failed, and now has been forgotten."
                              Concourse.BuildStatus.Errored -> "It errored, but has found forgiveness."
                              Concourse.BuildStatus.Aborted -> "It was never given a chance."
                              _ -> "I'm not dead yet."
                        ]
                    ]
                , Html.div
                    [ class "explanation" ]
                    [ Html.text "This log has been "
                    , Html.a
                        [ Html.Attributes.href "http://concourse.ci/configuring-jobs.html#build_logs_to_retain" ]
                        [ Html.text "reaped." ]
                    ]
                ]
              _ -> []
        ]

    _ ->
      LoadingIndicator.view

mmDDYY : Date -> String
mmDDYY d =
  Date.Format.format "%m/%d/" d ++ String.left 2 (Date.Format.format "%Y" d)

paddingClass : Build -> List Html.Attribute
paddingClass build =
  case build.job of
    Just _ ->
      []

    _ ->
      [class "build-body-noSubHeader"]

viewBuildOutput : Signal.Address Action -> Maybe BuildOutput.Model -> Html
viewBuildOutput actions output =
  case output of
    Just o ->
      BuildOutput.view (Signal.forwardTo actions BuildOutputAction) o

    Nothing ->
      Html.div [] []

viewBuildPrep : Maybe BuildPrep -> Html
viewBuildPrep prep =
  case prep of
    Just prep ->
      Html.div [class "build-step"]
        [ Html.div [class "header"]
            [ Html.i [class "left fa fa-fw fa-cogs"] []
            , Html.h3 [] [Html.text "preparing build"]
            ]
        , Html.div []
            [ Html.ul [class "prep-status-list"]
                (
                    [ viewBuildPrepLi "checking pipeline is not paused" prep.pausedPipeline
                    , viewBuildPrepLi "checking job is not paused" prep.pausedJob
                    ] ++
                    (viewBuildPrepInputs prep.inputs) ++
                    [ viewBuildPrepLi "waiting for a suitable set of input versions" prep.inputsSatisfied
                    , viewBuildPrepLi "checking max-in-flight is not reached" prep.maxRunningBuilds
                    ]
                )
            ]
        ]
    Nothing ->
      Html.div [] []

viewBuildPrepInputs : Dict String BuildPrepStatus -> List Html
viewBuildPrepInputs inputs =
  List.map viewBuildPrepInput (Dict.toList inputs)

viewBuildPrepInput : (String, BuildPrepStatus) -> Html
viewBuildPrepInput (name, status) =
  viewBuildPrepLi ("discovering any new versions of " ++ name) status

viewBuildPrepLi : String -> BuildPrepStatus -> Html
viewBuildPrepLi text status =
  Html.li
    [ classList [
        ("prep-status", True),
        ("inactive", status == Concourse.BuildPrep.Unknown)
      ]
    ]
    [ Html.span [class "marker"]
        [ viewBuildPrepStatus status ]
    , Html.span []
        [ Html.text text ]
    ]

viewBuildPrepStatus : BuildPrepStatus -> Html
viewBuildPrepStatus status =
  case status of
    Concourse.BuildPrep.Unknown -> Html.i [class "fa fa-fw fa-circle-o-notch", title "thinking..."] []
    Concourse.BuildPrep.Blocking -> Html.i [class "fa fa-fw fa-spin fa-circle-o-notch inactive", title "blocking"] []
    Concourse.BuildPrep.NotBlocking -> Html.i [class "fa fa-fw fa-check", title "not blocking"] []

viewBuildHeader : Signal.Address Action -> Build -> Model -> Html
viewBuildHeader actions build {status, now, duration, history, job} =
  let
    triggerButton =
      case build.job of
        Just {name, pipelineName} ->
          let
            actionUrl = "/pipelines/" ++ pipelineName ++ "/jobs/" ++ name ++ "/builds"
            buttonDisabled = case job of
              Nothing -> True
              Just job -> job.disableManualTrigger
          in
            Html.form
              [class "trigger-build", method "post", action (actionUrl)]
              [Html.button [class "build-action fr", disabled buttonDisabled, attribute "aria-label" "Trigger Build"] [Html.i [class "fa fa-plus-circle"] []]]

        _ ->
          Html.div [] []

    abortButton =
      if Concourse.BuildStatus.isRunning status then
        Html.span
          [class "build-action build-action-abort fr", onClick actions AbortBuild, attribute "aria-label" "Abort Build"]
          [Html.i [class "fa fa-times-circle"] []]
      else
        Html.span [] []

    buildTitle = case build.job of
      Just {name, pipelineName} ->
        Html.a [href ("/pipelines/" ++ pipelineName ++ "/jobs/" ++ name)]
          [Html.text (name ++ " #" ++ build.name)]

      _ ->
        Html.text ("build #" ++ toString build.id)
  in
    Html.div [id "page-header", class (Concourse.BuildStatus.show status)]
      [ Html.div [class "build-header"]
          [ Html.div [class "build-actions fr"] [triggerButton, abortButton]
          , Html.h1 [] [buildTitle]
          , BuildDuration.view duration now
          ]
      , Html.div
          [ onWithOptions
              "mousewheel"
              { stopPropagation = True, preventDefault = True }
              decodeScrollEvent
              ( scrollEvent actions )
          ]
          [ lazyViewHistory build status history ]
      ]

lazyViewHistory : Build -> BuildStatus -> List Build -> Html
lazyViewHistory currentBuild currentStatus builds =
  Html.Lazy.lazy3 viewHistory currentBuild currentStatus builds

viewHistory : Build -> BuildStatus -> List Build -> Html
viewHistory currentBuild currentStatus builds =
  Html.ul [id "builds"]
    (List.map (viewHistoryItem currentBuild currentStatus) builds)

viewHistoryItem : Build -> BuildStatus -> Build -> Html
viewHistoryItem currentBuild currentStatus build =
  Html.li
    [ classList
        [ ( if build.name == currentBuild.name then
              Concourse.BuildStatus.show currentStatus
            else
              Concourse.BuildStatus.show build.status
          , True
          )
        , ("current", build.name == currentBuild.name)
        ]
    ]
    [Html.a [href (Concourse.Build.url build)] [Html.text (build.name)]]

durationTitle : Date -> List Html -> Html
durationTitle date content =
  Html.div [title (Date.Format.format "%b" date)] content

scrollEvent : Signal.Address Action -> (Float, Float) -> Signal.Message
scrollEvent actions delta =
  Signal.message actions (ScrollBuilds delta)

decodeScrollEvent : Json.Decode.Decoder (Float, Float)
decodeScrollEvent =
  Json.Decode.object2 (,)
    ("deltaX" := Json.Decode.float)
    ("deltaY" := Json.Decode.float)

fetchBuild : Time -> Int -> Effects Action
fetchBuild delay buildId =
  Task.sleep delay `Task.andThen` (always <| Concourse.Build.fetch buildId)
    |> Task.toResult
    |> Task.map BuildFetched
    |> Effects.task

fetchBuildJobDetails : Build -> Effects Action
fetchBuildJobDetails build =
  case build.job of
    Nothing ->
      Effects.none
    Just buildJob ->
      Concourse.Job.fetchJob buildJob
        |> Task.toResult
        |> Task.map BuildJobDetailsFetched
        |> Effects.task

fetchBuildPrep : Time -> Int -> Effects Action
fetchBuildPrep delay buildId =
  Task.sleep delay `Task.andThen` (always <| Concourse.BuildPrep.fetch buildId)
    |> Task.toResult
    |> Task.map BuildPrepFetched
    |> Effects.task

fetchBuildHistory : Concourse.Build.BuildJob -> Maybe Concourse.Pagination.Page -> Effects Action
fetchBuildHistory job page =
  Concourse.Build.fetchJobBuilds job page
    |> Task.toResult
    |> Task.map BuildHistoryFetched
    |> Effects.task

scrollBuilds : Float -> Effects Action
scrollBuilds delta =
  Scroll.scroll "builds" delta
    |> Task.map (always Noop)
    |> Effects.task

scrollToCurrentBuildInHistory : Effects Action
scrollToCurrentBuildInHistory =
  Scroll.scrollIntoView "#builds .current"
    |> Task.map (always Noop)
    |> Effects.task

getScrollBehavior : Model -> Autoscroll.ScrollBehavior
getScrollBehavior model =
  case model.status of
    Concourse.BuildStatus.Failed -> ScrollUntilCancelled
    Concourse.BuildStatus.Errored -> ScrollUntilCancelled
    Concourse.BuildStatus.Aborted -> ScrollUntilCancelled
    Concourse.BuildStatus.Started -> Autoscroll
    Concourse.BuildStatus.Pending -> NoScroll
    Concourse.BuildStatus.Succeeded -> NoScroll

redirectToLogin : Model -> Effects Action
redirectToLogin model =
  Signal.send model.redirect "/login"
    |> Task.map (always Noop)
    |> Effects.task
