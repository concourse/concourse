module Build where

import Ansi.Log
import Date exposing (Date)
import Date.Format
import Debug
import Effects exposing (Effects)
import EventSource exposing (EventSource)
import Html exposing (Html)
import Html.Events exposing (onClick, on)
import Html.Attributes exposing (action, class, classList, href, id, method, title)
import Http
import Json.Decode exposing ((:=))
import Task
import Time exposing (Time)

import BuildEvent exposing (BuildEvent)
import BuildPlan exposing (BuildPlan)
import BuildResources exposing (BuildResources)
import Scroll
import StepTree exposing (StepTree)
import Pagination exposing (Pagination)
import Duration exposing (Duration)

type alias Model =
  { redirect : Signal.Address String
  , actions : Signal.Address Action
  , buildId : Int
  , errors : Maybe Ansi.Log.Model
  , stepRoot : Maybe StepTree.Root
  , build : Maybe Build
  , history : Maybe (List Build)
  , eventSource : Maybe EventSource
  , stepState : StepRenderingState
  , status : BuildEvent.BuildStatus
  , autoScroll : Bool
  , now : Time.Time
  , duration : BuildDuration
  }

type alias BuildDuration =
  { startedAt : Maybe Date
  , finishedAt : Maybe Date
  }

type alias Build =
  { id : Int
  , name : String
  , status : String
  , job : Maybe Job
  , url : String
  }

type alias Job =
  { name : String
  , pipelineName : String
  }

type StepRenderingState
  = StepsLoading
  | StepsLiveUpdating
  | StepsComplete
  | LoginRequired

type Action
  = Noop
  | PlanAndResourcesFetched (Result Http.Error (BuildPlan, BuildResources))
  | BuildFetched (Result Http.Error Build)
  | BuildHistoryFetched (Result Http.Error BuildHistory)
  | Listening EventSource
  | EventSourceOpened
  | EventSourceErrored
  | Event (Result String BuildEvent)
  | EndOfEvents
  | EventSourceClosed
  | ScrollTick
  | ScrollFromBottom Int
  | ScrollBuilds (Float, Float)
  | StepTreeAction StepTree.Action
  | ClockTick Time.Time
  | AbortBuild
  | BuildAborted (Result Http.RawError Http.Response)
  | Deferred (Effects Action)

type alias BuildHistory =
  { builds : List Build
  , pagination : Pagination
  }

init : Signal.Address String -> Signal.Address Action -> Int -> (Model, Effects Action)
init redirect actions buildId =
  let
    model =
      { redirect = redirect
      , actions = actions
      , buildId = buildId
      , errors = Nothing
      , stepRoot = Nothing
      , build = Nothing
      , history = Nothing
      , eventSource = Nothing
      , stepState = StepsLoading
      , autoScroll = True
      , status = BuildEvent.BuildStatusPending
      , now = 0
      , duration = BuildDuration Nothing Nothing
      }
  in
    ( model
    , Effects.batch
        [ keepScrolling
        , fetchBuild 0 buildId
        ]
    )

update : Action -> Model -> (Model, Effects Action)
update action model =
  case action of
    Noop ->
      (model, Effects.none)

    ScrollTick ->
      if model.stepState == StepsLiveUpdating && model.autoScroll then
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

    BuildAborted (Err err) ->
      Debug.log ("failed to abort build: " ++ toString err) <|
        (model, Effects.none)

    BuildAborted (Ok {status}) ->
      case status of
        401 ->
          (model, redirectToLogin model)

        _ ->
          (model, Effects.none)

    BuildFetched (Err err) ->
      Debug.log ("failed to fetch build: " ++ toString err) <|
        (model, Effects.none)

    BuildFetched (Ok build) ->
      let
        status = toStatus build.status
        pending = status == BuildEvent.BuildStatusPending
        stepState =
          case status of
            BuildEvent.BuildStatusPending ->
              StepsLiveUpdating
            BuildEvent.BuildStatusStarted ->
              StepsLiveUpdating
            _ ->
              StepsLoading
      in
        ( { model
          | build = Just build
          , status = status
          , stepState = stepState
          }
        , let
            fetch =
              if pending then
                fetchBuild Time.second model.buildId
              else if build.job /= Nothing then
                fetchBuildPlanAndResources model.buildId
              else
                fetchBuildPlan model.buildId

            fetchHistory =
              case (model.build, build.job) of
                (Nothing, Just job) ->
                  fetchBuildHistory job Nothing

                _ ->
                  Effects.none
          in
            case build.job of
              Just job ->
                Effects.batch [fetchHistory, fetch]

              _ ->
                fetch
        )

    PlanAndResourcesFetched (Err (Http.BadResponse 404 _)) ->
      ( model
      , subscribeToEvents model.buildId model.actions
      )

    PlanAndResourcesFetched (Err err) ->
      Debug.log ("failed to fetch plan: " ++ toString err) <|
        (model, Effects.none)

    PlanAndResourcesFetched (Ok (plan, resources)) ->
      ( { model | stepRoot = Just (StepTree.init resources plan) }
      , subscribeToEvents model.buildId model.actions
      )

    BuildHistoryFetched (Err err) ->
      Debug.log ("failed to fetch build history: " ++ toString err) <|
        (model, Effects.none)

    BuildHistoryFetched (Ok history) ->
      let
        builds = List.append (Maybe.withDefault [] model.history) history.builds
        withBuilds = { model | history = Just builds }
        loadedCurrentBuild = List.any ((==) model.buildId << .id) history.builds
        scrollToCurrent =
          if loadedCurrentBuild then
            Effects.tick (always (Deferred scrollToCurrentBuildInHistory))
          else
            Effects.none
      in
        case (history.pagination.nextPage, model.build `Maybe.andThen` .job) of
          (Nothing, _) ->
            (withBuilds, scrollToCurrent)

          (Just url, Just job) ->
            (withBuilds, Effects.batch [fetchBuildHistory job (Just url), scrollToCurrent])

          (Just url, Nothing) ->
            Debug.crash "impossible"

    Deferred effects ->
      (model, effects)

    Listening es ->
      ({ model | eventSource = Just es }, Effects.none)

    EventSourceOpened ->
      (model, scrollToBottom)

    EventSourceErrored ->
      let
        newState =
          case model.stepState of
            -- if we're loading and the event source errors, assume we're not
            -- logged in (there's no way to actually tell)
            StepsLoading ->
              LoginRequired

            -- closing the event source causes an error to come in, so ignore
            -- it since that means everything actually worked
            StepsComplete ->
              model.stepState

            -- getting an error in the middle could just be the ATC going away
            -- (i.e. during a deploy). ignore it and let the browser
            -- auto-reconnect
            StepsLiveUpdating ->
              model.stepState

            -- shouldn't ever happen, but...
            LoginRequired ->
              model.stepState
      in
        ({ model | stepState = newState }, Effects.none)

    Event (Ok (BuildEvent.Log origin output)) ->
      ( updateStep origin.id (setRunning << appendStepLog output) model
      , Effects.none
      )

    Event (Ok (BuildEvent.Error origin message)) ->
      ( updateStep origin.id (setStepError message) model
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

    Event (Ok (BuildEvent.FinishGet origin exitStatus version metadata)) ->
      ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
      , Effects.none
      )

    Event (Ok (BuildEvent.FinishPut origin exitStatus version metadata)) ->
      ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
      , Effects.none
      )

    Event (Ok (BuildEvent.BuildStatus status date)) ->
      ( updateStartFinishAt status date <|
          case model.stepState of
            StepsLiveUpdating ->
              { model | status = status }

            _ ->
              model
      , Effects.none
      )

    Event (Ok (BuildEvent.BuildError message)) ->
      ( { model |
          errors =
            Just <|
              Ansi.Log.update message <|
                Maybe.withDefault (Ansi.Log.init Ansi.Log.Cooked) model.errors
        }
      , Effects.none
      )

    Event (Err e) ->
      (model, Debug.log e Effects.none)

    StepTreeAction action ->
      ( { model | stepRoot = Maybe.map (StepTree.update action) model.stepRoot }
      , Effects.none
      )

    ScrollBuilds (0, deltaY) ->
      (model, scrollBuilds deltaY)

    ScrollBuilds (deltaX, _) ->
      (model, scrollBuilds -deltaX)

    ClockTick now ->
      ({ model | now = now }, Effects.none)

    EndOfEvents ->
      case model.eventSource of
        Just es ->
          ({ model | stepState = StepsComplete }, closeEvents es)

        Nothing ->
          (model, Effects.none)

    EventSourceClosed ->
      ({ model | eventSource = Nothing }, Effects.none)

updateStartFinishAt : BuildEvent.BuildStatus -> Date -> Model -> Model
updateStartFinishAt status date model =
  let
    duration = model.duration
  in
    case status of
      BuildEvent.BuildStatusStarted ->
        { model | duration = { duration | startedAt = Just date } }

      _ ->
        { model | duration = { duration | finishedAt = Just date } }

abortBuild : Int -> Effects Action
abortBuild buildId =
  Http.send Http.defaultSettings
    { verb = "POST"
    , headers = []
    , url = "/api/v1/builds/" ++ toString buildId ++ "/abort"
    , body = Http.empty
    }
    |> Task.toResult
    |> Task.map BuildAborted
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

setResourceInfo : BuildEvent.Version -> BuildEvent.Metadata -> StepTree -> StepTree
setResourceInfo version metadata tree =
  StepTree.map (\step -> { step | version = Just version, metadata = metadata }) tree

setStepState : StepTree.StepState -> StepTree -> StepTree
setStepState state tree =
  let
    autoCollapse = state == StepTree.StepStateSucceeded
  in
    StepTree.map (\step ->
      let
        expanded =
          if autoCollapse then
            Just <| Maybe.withDefault False step.expanded
          else
            step.expanded
      in
        { step
        | state = state
        , expanded = expanded
        }) tree

view : Signal.Address Action -> Model -> Html
view actions model =
  case (model.build, model.stepRoot) of
    (Just build, Just root) ->
      Html.div []
        [ viewBuildHeader actions build model.status model.now model.duration (Maybe.withDefault [] model.history)
        , Html.div (id "build-body" :: paddingClass build)
            [ case model.stepState of
                StepsLoading ->
                  loadingIndicator

                StepsLiveUpdating ->
                  viewSteps actions model.errors build root

                StepsComplete ->
                  viewSteps actions model.errors build root

                LoginRequired ->
                  viewLoginButton build
            ]
        ]

    (Just build, Nothing) ->
      Html.div []
        [ viewBuildHeader actions build model.status model.now model.duration (Maybe.withDefault [] model.history)
        , Html.div (id "build-body" :: paddingClass build)
            [Html.div [class "steps"] [viewErrors model.errors]]
        ]

    _ ->
      loadingIndicator

loadingIndicator : Html
loadingIndicator =
  Html.div [class "steps"]
    [ Html.div [class "build-step"]
      [ Html.div [class "header"]
          [ Html.i [class "left fa fa-fw fa-spin fa-circle-o-notch"] []
          , Html.h3 [] [Html.text "loading"]
          ]
      ]
    ]

viewSteps : Signal.Address Action -> Maybe Ansi.Log.Model -> Build -> StepTree.Root -> Html
viewSteps actions errors build root =
  Html.div [class "steps"]
    [ viewErrors errors
    , StepTree.view (Signal.forwardTo actions StepTreeAction) root.tree
    ]

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
        , Html.Attributes.value (buildUrl build)
        ] []
    ]

paddingClass : Build -> List Html.Attribute
paddingClass build =
  case build.job of
    Just _ ->
      []

    _ ->
      [class "build-body-noSubHeader"]


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

viewBuildHeader : Signal.Address Action -> Build -> BuildEvent.BuildStatus -> Time.Time -> BuildDuration -> List Build -> Html
viewBuildHeader actions build status now duration history =
  let
    triggerButton = case build.job of
      Just {name, pipelineName} ->
        let
          actionUrl = "/pipelines/" ++ pipelineName ++ "/jobs/" ++ name ++ "/builds"
        in
          Html.form
            [class "trigger-build", method "post", action (actionUrl) ]
            [ Html.button [ class "build-action fr" ] [ Html.i [class "fa fa-plus-circle" ] [] ] ]

      _ ->
        Html.div [] []

    abortButton =
      if isRunning status then
        Html.span
          [class "build-action build-action-abort fr", onClick actions AbortBuild]
          [ Html.i [class "fa fa-times-circle"] [] ]
      else
        Html.span [] []

    buildTitle = case build.job of
      Just {name, pipelineName} ->
        Html.a [href ("/pipelines/" ++ pipelineName ++ "/jobs/" ++ name)]
          [ Html.text (name ++ " #" ++ build.name) ]

      _ ->
        Html.text ("build #" ++ toString build.id)
  in
    Html.div [id "page-header", class (statusClass status)]
      [ Html.div [class "build-header"]
          [ Html.div [class "build-actions fr"]
              [ triggerButton
              , abortButton
              ]
          , Html.h1 [] [buildTitle]
          , viewBuildDuration now duration
          ]
      , Html.ul
          [ on "mousewheel" decodeScrollEvent (scrollEvent actions)
          , id "builds"
          ]
          (List.map (renderHistory build status) history)
      ]

viewBuildDuration : Time.Time -> BuildDuration -> Html
viewBuildDuration now duration =
  Html.dl [class "build-times"] <|
    case (duration.startedAt, duration.finishedAt) of
      (Nothing, _) ->
        []

      (Just startedAt, Nothing) ->
        labeledRelativeDate "started" now startedAt

      (Just startedAt, Just finishedAt) ->
        labeledRelativeDate "started" now startedAt ++
          labeledRelativeDate "finished" now finishedAt ++
          labeledDuration "duration" (Duration.between (Date.toTime startedAt) (Date.toTime finishedAt))

durationTitle : Date -> List Html -> Html
durationTitle date content =
  Html.div [title (Date.Format.format "%b" date)] content

labeledRelativeDate : String -> Time -> Date -> List Html
labeledRelativeDate label now date =
  let
    ago = Duration.between (Date.toTime date) now
  in
    [ Html.dt [] [Html.text label]
    , Html.dd [title (Date.Format.format "%b %d %Y %I:%M:%S %p" date)]
      [ Html.span [] [Html.text (Duration.format ago ++ " ago")]
      ]
    ]

labeledDuration : String -> Duration ->  List Html
labeledDuration label duration =
  [ Html.dt [] [Html.text label]
  , Html.dd []
    [ Html.span [] [Html.text (Duration.format duration)]
    ]
  ]

scrollEvent : Signal.Address Action -> (Float, Float) -> Signal.Message
scrollEvent actions delta =
  Signal.message actions (ScrollBuilds delta)

decodeScrollEvent : Json.Decode.Decoder (Float, Float)
decodeScrollEvent =
  Json.Decode.object2 (,)
    ("deltaX" := Json.Decode.float)
    ("deltaY" := Json.Decode.float)

renderHistory : Build -> BuildEvent.BuildStatus -> Build -> Html
renderHistory currentBuild currentStatus build =
  Html.li
    [ classList
        [ ( if build.name == currentBuild.name then
              statusClass currentStatus
            else
              build.status
          , True
          )
        , ("current", build.name == currentBuild.name)
        ]
    ]
    [ Html.a [href build.url] [ Html.text ("#" ++ build.name) ] ]

fetchBuildPlanAndResources : Int -> Effects Action
fetchBuildPlanAndResources buildId =
  let
    getPlan =
      Http.get BuildPlan.decode ("/api/v1/builds/" ++ toString buildId ++ "/plan")
    getResources =
      Http.get BuildResources.decode ("/api/v1/builds/" ++ toString buildId ++ "/resources")
  in
    Task.map2 (,) getPlan getResources
      |> Task.toResult
      |> Task.map PlanAndResourcesFetched
      |> Effects.task

fetchBuildPlan : Int -> Effects Action
fetchBuildPlan buildId =
  let
    getPlan =
      Http.get BuildPlan.decode ("/api/v1/builds/" ++ toString buildId ++ "/plan")
  in
    Task.map (flip (,) { inputs = [], outputs = [] }) getPlan
      |> Task.toResult
      |> Task.map PlanAndResourcesFetched
      |> Effects.task

fetchBuild : Time -> Int -> Effects Action
fetchBuild delay buildId =
  let
    fetch =
      Http.get decode ("/api/v1/builds/" ++ toString buildId)
        |> Task.toResult
        |> Task.map BuildFetched
  in
    Effects.task (Task.sleep delay `Task.andThen` \_ -> fetch)

fetchBuildHistory : Job -> Maybe String -> Effects Action
fetchBuildHistory job specificPage =
  let
    firstPage = "/api/v1/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.name ++ "/builds"
    url = Maybe.withDefault firstPage specificPage
  in
    Http.send
      Http.defaultSettings
      { verb = "GET"
      , headers = []
      , url = url
      , body = Http.empty
      }
    |> Task.toResult
    |> Task.map parseBuildHistory
    |> Effects.task

parseBuildHistory : Result Http.RawError Http.Response -> Action
parseBuildHistory result =
  case result of
    Ok response ->
      let
        pagination = Pagination.parse response
        decode = Json.Decode.decodeString decodeBuilds
        history = handleResponse response `Result.andThen`
          (Result.formatError Http.UnexpectedPayload << decode)
      in
        BuildHistoryFetched <|
          Result.map (\builds -> { builds = builds, pagination = pagination }) history

    Err error ->
      BuildHistoryFetched (Err (promoteError error))

handleResponse : Http.Response -> Result Http.Error String
handleResponse response =
  if 200 <= response.status && response.status < 300 then
      case response.value of
        Http.Text str ->
          Ok str

        _ ->
            Err (Http.UnexpectedPayload "Response body is a blob, expecting a string.")
  else
      Err (Http.BadResponse response.status response.statusText)

promoteError : Http.RawError -> Http.Error
promoteError rawError =
  case rawError of
    Http.RawTimeout -> Http.Timeout
    Http.RawNetworkError -> Http.NetworkError

decode : Json.Decode.Decoder Build
decode =
  Json.Decode.object5 Build
    ("id" := Json.Decode.int)
    ("name" := Json.Decode.string)
    ("status" := Json.Decode.string)
    (Json.Decode.maybe (Json.Decode.object2 Job
      ("job_name" := Json.Decode.string)
      ("pipeline_name" := Json.Decode.string)))
    ("url" := Json.Decode.string)

decodeBuilds : Json.Decode.Decoder (List Build)
decodeBuilds =
  Json.Decode.list decode

subscribeToEvents : Int -> Signal.Address Action -> Effects Action
subscribeToEvents build actions =
  let
    settings =
      EventSource.Settings
        (Just <| Signal.forwardTo actions (always EventSourceOpened))
        (Just <| Signal.forwardTo actions (always EventSourceErrored))

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

closeEvents : EventSource.EventSource -> Effects Action
closeEvents eventSource =
  EventSource.close eventSource
    |> Task.map (always EventSourceClosed)
    |> Effects.task

parseEvent : EventSource.Event -> Result String BuildEvent
parseEvent e = Json.Decode.decodeString BuildEvent.decode e.data

scrollToBottom : Effects Action
scrollToBottom =
  Scroll.toBottom
    |> Task.map (always Noop)
    |> Effects.task

scrollBuilds : Float -> Effects Action
scrollBuilds delta =
  Scroll.scroll "builds" delta
    |> Task.map (always Noop)
    |> Effects.task

toStatus : String -> BuildEvent.BuildStatus
toStatus str =
  case str of
    "pending" -> BuildEvent.BuildStatusPending
    "started" -> BuildEvent.BuildStatusStarted
    "succeeded" -> BuildEvent.BuildStatusSucceeded
    "failed" -> BuildEvent.BuildStatusFailed
    "errored" -> BuildEvent.BuildStatusErrored
    "aborted" -> BuildEvent.BuildStatusAborted
    _ -> Debug.crash ("unknown state: " ++ str)

buildUrl : Build -> String
buildUrl build =
  case build.job of
    Nothing ->
      "/builds/" ++ toString build.id

    Just {name, pipelineName} ->
      "/pipelines/" ++ pipelineName ++ "/jobs/" ++ name ++ "/builds/" ++ build.name

redirectToLogin : Model -> Effects Action
redirectToLogin model =
  Signal.send model.redirect "/login"
    |> Task.map (always Noop)
    |> Effects.task

scrollToCurrentBuildInHistory : Effects Action
scrollToCurrentBuildInHistory =
  Scroll.scrollIntoView "#builds .current"
    |> Task.map (always Noop)
    |> Effects.task
