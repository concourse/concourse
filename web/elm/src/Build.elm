module Build exposing (..)

import Date exposing (Date)
import Date.Format
import Debug
import Dict exposing (Dict)
import Html exposing (Html)
import Html.App
import Html.Attributes exposing (action, class, classList, href, id, method, title, disabled, attribute)
import Html.Events exposing (onClick, on, onWithOptions)
import Html.Lazy
import Http
import Json.Decode exposing ((:=))
import Navigation
import Process
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
import Redirect
import Scroll

import Concourse.Job exposing (Job)

type BuildOrOutput
  = HasOutput BuildOutput.Model
  | NoOutput Build

type alias CurrentBuild =
  { buildOrOutput : BuildOrOutput
  , prep : Maybe BuildPrep
  }

currentBuildBuild : CurrentBuild -> Build
currentBuildBuild cb =
  case cb.buildOrOutput of
    NoOutput build -> build
    HasOutput output -> output.build

currentBuildOutput : CurrentBuild -> Maybe BuildOutput.Model
currentBuildOutput cb =
  case cb.buildOrOutput of
    NoOutput _ -> Nothing
    HasOutput output -> Just output

type alias Model =
  { now : Time.Time
  , job : Maybe Job
  , history : List Build
  , currentBuild : Maybe CurrentBuild
  }

type StepRenderingState
  = StepsLoading
  | StepsLiveUpdating
  | StepsComplete
  | LoginRequired

type Action
  = Noop
  | FetchBuild Int
  | AbortBuild Int
  | BuildFetched (Result Http.Error Build)
  | BuildPrepFetched (Result Http.Error BuildPrep)
  | BuildHistoryFetched (Result Http.Error (Paginated Build))
  | BuildJobDetailsFetched (Result Http.Error Job)
  | BuildOutputAction BuildOutput.Action
  | ScrollBuilds (Float, Float)
  | ClockTick Time.Time
  | BuildAborted (Result Http.Error ())
  | RevealCurrentBuildInHistory

type alias Flags =
  { buildId : Int
  }

init : Flags -> (Model, Cmd Action)
init flags =
  let
    model =
      { now = 0
      , job = Nothing
      , history = []
      , currentBuild = Nothing
      }
  in
    update (FetchBuild flags.buildId) model

update : Action -> Model -> (Model, Cmd Action)
update action model =
  case action of
    Noop ->
      (model, Cmd.none)

    FetchBuild buildId ->
      (model, fetchBuild 0 buildId)

    BuildFetched (Ok build) ->
      handleBuildFetched build model

    BuildFetched (Err err) ->
      Debug.log ("failed to fetch build: " ++ toString err) <|
        (model, Cmd.none)

    AbortBuild buildId ->
      (model, abortBuild buildId)

    BuildAborted (Ok ()) ->
      (model, Cmd.none)

    BuildAborted (Err (Http.BadResponse 401 _)) ->
      (model, redirectToLogin model)

    BuildAborted (Err err) ->
      Debug.log ("failed to abort build: " ++ toString err) <|
        (model, Cmd.none)

    BuildPrepFetched (Ok buildPrep) ->
      handleBuildPrepFetched buildPrep model

    BuildPrepFetched (Err err) ->
      Debug.log ("failed to fetch build preparation: " ++ toString err) <|
        (model, Cmd.none)

    BuildOutputAction action ->
      case (model.currentBuild, Maybe.map .buildOrOutput model.currentBuild) of
        (Just currentBuild, Just (HasOutput output)) ->
          let
            (newOutput, cmd) = BuildOutput.update action output
          in
            ( { model
              | currentBuild = Just { currentBuild | buildOrOutput = HasOutput newOutput } }
            , Cmd.map BuildOutputAction cmd
            )

        _ ->
          Debug.crash "impossible (received action for missing BuildOutput)"

    BuildHistoryFetched (Err err) ->
      Debug.log ("failed to fetch build history: " ++ toString err) <|
        (model, Cmd.none)

    BuildHistoryFetched (Ok history) ->
      handleHistoryFetched history model

    BuildJobDetailsFetched (Ok job) ->
      handleBuildJobFetched job model

    BuildJobDetailsFetched (Err err) ->
      Debug.log ("failed to fetch build job details: " ++ toString err) <|
        (model, Cmd.none)

    RevealCurrentBuildInHistory ->
      (model, scrollToCurrentBuildInHistory)

    ScrollBuilds (0, deltaY) ->
      (model, scrollBuilds deltaY)

    ScrollBuilds (deltaX, _) ->
      (model, scrollBuilds -deltaX)

    ClockTick now ->
      ({ model | now = now }, Cmd.none)

handleBuildFetched : Build -> Model -> (Model, Cmd Action)
handleBuildFetched build model =
  let
    currentBuild =
      case model.currentBuild of
        Nothing -> { buildOrOutput = NoOutput build, prep = Nothing }
        Just currentBuild -> { currentBuild | buildOrOutput = NoOutput build }

    withBuild =
      { model | currentBuild = Just currentBuild }

    fetchJobAndHistory =
      case (model.job, build.job) of
        (Nothing, Just buildJob) ->
          Cmd.batch [fetchBuildJobDetails buildJob, fetchBuildHistory buildJob Nothing]

        _ ->
          Cmd.none

    updateBuildURL =
      Navigation.newUrl <| Concourse.Build.url build

    (newModel, cmd) =
      if build.status == Concourse.BuildStatus.Pending then
        (withBuild, pollUntilStarted build.id)
      else if build.reapTime == Nothing then
        case model.currentBuild `Maybe.andThen` .prep of
          Nothing ->
            initBuildOutput build withBuild
          Just _ ->
            let
              (newModel, cmd) = initBuildOutput build withBuild
            in
              ( newModel
              , Cmd.batch [cmd, fetchBuildPrep Time.second build.id]
              )
      else (withBuild, Cmd.none)
  in
    (newModel, Cmd.batch [cmd, updateBuildURL, fetchJobAndHistory])

pollUntilStarted : Int -> Cmd Action
pollUntilStarted buildId =
  Cmd.batch
    [ (fetchBuild Time.second buildId)
    , (fetchBuildPrep Time.second buildId)
    ]

initBuildOutput : Build -> Model -> (Model, Cmd Action)
initBuildOutput build model =
  let
    (output, outputCmd) = BuildOutput.init build
  in
    ( { model
      | currentBuild =
          Maybe.map
            (\info -> { info | buildOrOutput = HasOutput output })
            model.currentBuild
      }
    , Cmd.map BuildOutputAction outputCmd
    )

handleBuildJobFetched : Job -> Model -> (Model, Cmd Action)
handleBuildJobFetched job model =
  let
    withJobDetails =
      { model | job = Just job }
  in
    (withJobDetails, Cmd.none)

handleHistoryFetched : Paginated Build -> Model -> (Model, Cmd Action)
handleHistoryFetched history model =
  let
    withBuilds =
      { model | history = List.append model.history history.content }
  in
    case (history.pagination.nextPage, model.currentBuild `Maybe.andThen` (.job << currentBuildBuild)) of
      (Nothing, _) ->
        (withBuilds, Cmd.none)

      (Just page, Just job) ->
        (withBuilds, Cmd.batch [fetchBuildHistory job (Just page)])

      (Just url, Nothing) ->
        Debug.crash "impossible"

handleBuildPrepFetched : BuildPrep -> Model -> (Model, Cmd Action)
handleBuildPrepFetched buildPrep model =
  ({ model | currentBuild = Maybe.map (\info -> { info | prep = Just buildPrep }) model.currentBuild }, Cmd.none)

abortBuild : Int -> Cmd Action
abortBuild buildId =
  Cmd.map BuildAborted << Task.perform Err Ok <|
    Concourse.Build.abort buildId

view : Model -> Html Action
view model =
  case model.currentBuild of
    Just currentBuild ->
      Html.div [class "with-fixed-header"]
        [ viewBuildHeader (currentBuildBuild currentBuild) model
        , Html.div [class "scrollable-body"] <|
          [ viewBuildPrep currentBuild.prep
          , Html.Lazy.lazy viewBuildOutput <| currentBuildOutput currentBuild
          ] ++
            let
              build = currentBuildBuild currentBuild
            in let
              maybeBirthDate =
                Maybe.oneOf [build.duration.startedAt, build.duration.finishedAt]
            in
              case (maybeBirthDate, build.reapTime) of
                (Just birthDate, Just reapTime) ->
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
                              mmDDYY birthDate ++ "-" ++ mmDDYY reapTime
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
  Date.Format.format "%m/%d/" d ++ String.right 2 (Date.Format.format "%Y" d)

viewBuildOutput : Maybe BuildOutput.Model -> Html Action
viewBuildOutput output =
  case output of
    Just o ->
      Html.App.map BuildOutputAction (BuildOutput.view o)

    Nothing ->
      Html.div [] []

viewBuildPrep : Maybe BuildPrep -> Html Action
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
                    [ viewBuildPrepLi "checking pipeline is not paused" prep.pausedPipeline Dict.empty
                    , viewBuildPrepLi "checking job is not paused" prep.pausedJob Dict.empty
                    ] ++
                    (viewBuildPrepInputs prep.inputs) ++
                    [ viewBuildPrepLi "waiting for a suitable set of input versions" prep.inputsSatisfied prep.missingInputReasons
                    , viewBuildPrepLi "checking max-in-flight is not reached" prep.maxRunningBuilds Dict.empty
                    ]
                )
            ]
        ]
    Nothing ->
      Html.div [] []

viewBuildPrepInputs : Dict String BuildPrepStatus -> List (Html Action)
viewBuildPrepInputs inputs =
  List.map viewBuildPrepInput (Dict.toList inputs)

viewBuildPrepInput : (String, BuildPrepStatus) -> Html Action
viewBuildPrepInput (name, status) =
  viewBuildPrepLi ("discovering any new versions of " ++ name) status Dict.empty

viewBuildPrepDetails : Dict String String -> Html Action
viewBuildPrepDetails details =
  Html.ul [class "details"]
    (List.map (viewDetailItem) (Dict.toList details))

viewDetailItem : (String, String) -> Html Action
viewDetailItem (name, status) =
    Html.li []
      [Html.text (name ++ " - " ++ status)]

viewBuildPrepLi : String -> BuildPrepStatus -> Dict String String -> Html Action
viewBuildPrepLi text status details =
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
    ,
      (viewBuildPrepDetails details)
    ]

viewBuildPrepStatus : BuildPrepStatus -> Html Action
viewBuildPrepStatus status =
  case status of
    Concourse.BuildPrep.Unknown -> Html.i [class "fa fa-fw fa-circle-o-notch", title "thinking..."] []
    Concourse.BuildPrep.Blocking -> Html.i [class "fa fa-fw fa-spin fa-circle-o-notch inactive", title "blocking"] []
    Concourse.BuildPrep.NotBlocking -> Html.i [class "fa fa-fw fa-check", title "not blocking"] []

viewBuildHeader : Build -> Model -> Html Action
viewBuildHeader build {now, job, history} =
  let
    triggerButton =
      case job of
        Just {name, teamName, pipelineName} ->
          let
            actionUrl = "/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/jobs/" ++ name ++ "/builds"
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
      if Concourse.BuildStatus.isRunning build.status then
        Html.span
          [class "build-action build-action-abort fr", onClick (AbortBuild build.id), attribute "aria-label" "Abort Build"]
          [Html.i [class "fa fa-times-circle"] []]
      else
        Html.span [] []

    buildTitle = case build.job of
      Just {name, teamName, pipelineName} ->
        Html.a [href ("/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/jobs/" ++ name)]
          [Html.text (name ++ " #" ++ build.name)]

      _ ->
        Html.text ("build #" ++ toString build.id)
  in
    Html.div [class "fixed-header"]
      [ Html.div [class ("build-header " ++ Concourse.BuildStatus.show build.status)]
          [ Html.div [class "build-actions fr"] [triggerButton, abortButton]
          , Html.h1 [] [buildTitle]
          , BuildDuration.view build.duration now
          ]
      , Html.div
          [ onWithOptions
              "mousewheel"
              { stopPropagation = True, preventDefault = True }
              (Json.Decode.map ScrollBuilds decodeScrollEvent)
          ]
          [ lazyViewHistory build history ]
      ]

lazyViewHistory : Build -> List Build -> Html Action
lazyViewHistory currentBuild builds =
  Html.Lazy.lazy2 viewHistory currentBuild builds

viewHistory : Build -> List Build -> Html Action
viewHistory currentBuild builds =
  Html.ul [id "builds"]
    (List.map (viewHistoryItem currentBuild) builds)

viewHistoryItem : Build -> Build -> Html Action
viewHistoryItem currentBuild build =
  Html.li
    [ classList
        [ ( if build.name == currentBuild.name then
              Concourse.BuildStatus.show currentBuild.status
            else
              Concourse.BuildStatus.show build.status
          , True
          )
        , ("current", build.name == currentBuild.name)
        ]
    ]
    [Html.a [overrideClick (FetchBuild build.id),  href (Concourse.Build.url build)] [Html.text (build.name)]]

overrideClick : Action -> Html.Attribute Action
overrideClick action =
  Html.Events.onWithOptions "click"
    { stopPropagation = True, preventDefault = True } <|
      Json.Decode.customDecoder
      ("button" := Json.Decode.int) <|
        assertLeftButton action

assertLeftButton : Action -> Int -> Result String Action
assertLeftButton action button =
  if button == 0 then Ok action
  else Err "placeholder error, nothing is wrong"

durationTitle : Date -> List (Html Action) -> Html Action
durationTitle date content =
  Html.div [title (Date.Format.format "%b" date)] content

decodeScrollEvent : Json.Decode.Decoder (Float, Float)
decodeScrollEvent =
  Json.Decode.object2 (,)
    ("deltaX" := Json.Decode.float)
    ("deltaY" := Json.Decode.float)

fetchBuild : Time -> Int -> Cmd Action
fetchBuild delay buildId =
  Cmd.map BuildFetched << Task.perform Err Ok <|
    Process.sleep delay `Task.andThen` (always <| Concourse.Build.fetch buildId)

fetchBuildJobDetails : Concourse.Build.BuildJob -> Cmd Action
fetchBuildJobDetails buildJob =
  Cmd.map BuildJobDetailsFetched << Task.perform Err Ok <|
    Concourse.Job.fetchJob buildJob

fetchBuildPrep : Time -> Int -> Cmd Action
fetchBuildPrep delay buildId =
  Cmd.map BuildPrepFetched << Task.perform Err Ok <|
    Process.sleep delay `Task.andThen` (always <| Concourse.BuildPrep.fetch buildId)

fetchBuildHistory : Concourse.Build.BuildJob -> Maybe Concourse.Pagination.Page -> Cmd Action
fetchBuildHistory job page =
  Cmd.map BuildHistoryFetched << Task.perform Err Ok <|
    Concourse.Build.fetchJobBuilds job page

scrollBuilds : Float -> Cmd Action
scrollBuilds delta =
  Cmd.map (always Noop) << Task.perform Err Ok <|
    Scroll.scroll "builds" delta

scrollToCurrentBuildInHistory : Cmd Action
scrollToCurrentBuildInHistory =
  Cmd.map (always Noop) << Task.perform Err Ok <|
    Scroll.scrollIntoView "#builds .current"

getScrollBehavior : Model -> Autoscroll.ScrollBehavior
getScrollBehavior model =
  case Maybe.withDefault Concourse.BuildStatus.Pending (Maybe.map (.status << currentBuildBuild) model.currentBuild) of
    Concourse.BuildStatus.Failed -> ScrollUntilCancelled
    Concourse.BuildStatus.Errored -> ScrollUntilCancelled
    Concourse.BuildStatus.Aborted -> ScrollUntilCancelled
    Concourse.BuildStatus.Started -> Autoscroll
    Concourse.BuildStatus.Pending -> NoScroll
    Concourse.BuildStatus.Succeeded -> NoScroll

redirectToLogin : Model -> Cmd Action
redirectToLogin model =
  Cmd.map (always Noop) << Task.perform Err Ok <|
    Redirect.to "/login"
