module Job where

import Array exposing (Array)
import Dict exposing (Dict)
import Effects exposing (Effects)
import Html exposing (Html)
import Html.Attributes exposing (class, href, id, disabled)
import Html.Events exposing (onClick)
import Http
import Task exposing (Task)
import Time exposing (Time)

import Concourse.Build exposing (Build, BuildJob, BuildDuration)
import Concourse.Job exposing (Job)
import Concourse.BuildResources exposing (BuildResources, BuildInput, BuildOutput)
import Concourse.BuildStatus exposing (BuildStatus)
import Concourse.Pagination exposing (Pagination, Paginated, Page)
import Concourse.Version exposing (Version)
import BuildDuration
import DictView

type alias Model =
  { redirect : Signal.Address String
  , jobInfo : BuildJob
  , job : (Maybe Job)
  , pausedChanging : Bool
  , buildsWithResources : Maybe (Array LiveUpdatingBuildWithResources)
  , now : Time
  , page : Page
  , pagination : Pagination
  }

type Action
  = Noop
  | JobBuildsFetched (Result Http.Error (Paginated Build))
  | JobFetched (Result Http.Error Job)
  | BuildResourcesFetched FetchedBuildResources
  | ClockTick Time
  | TogglePaused
  | PausedToggled (Result Http.Error ())

type alias FetchedBuildResources =
  { index : Int
  , result : (Result Http.Error BuildResources)
  }

type alias BuildWithResources =
  { build : Build
  , resources : BuildResources
  }

type alias LiveUpdatingBuildWithResources =
  { buildWithResources : Maybe BuildWithResources
  , nextBuild : Build
  }

jobBuildsPerPage : Int
jobBuildsPerPage = 100

addFetchedResources : BuildResources -> LiveUpdatingBuildWithResources -> LiveUpdatingBuildWithResources
addFetchedResources resources lubwr =
  { lubwr | buildWithResources = Just {build = lubwr.nextBuild, resources = resources} }

addNextBuild : Build -> LiveUpdatingBuildWithResources -> LiveUpdatingBuildWithResources
addNextBuild nextBuild buildWithResources =
  { buildWithResources | nextBuild = nextBuild }

addNextBuildFromArray : Array Build -> Int -> LiveUpdatingBuildWithResources -> LiveUpdatingBuildWithResources
addNextBuildFromArray newBuilds i lubwr =
  case (Array.get i newBuilds) of
    Nothing -> lubwr
    Just newBuild -> addNextBuild newBuild lubwr

initLiveUpdatingBuildWithResources : Build -> LiveUpdatingBuildWithResources
initLiveUpdatingBuildWithResources nextBuild = {buildWithResources = Nothing, nextBuild = nextBuild}

init : Signal.Address String -> String -> String -> Int -> Int -> (Model, Effects Action)
init redirect jobName pipelineName pageSince pageUntil =
  let
    model =
      { redirect = redirect
      , jobInfo = {name = jobName, pipelineName = pipelineName}
      , job = Nothing
      , pausedChanging = False
      , buildsWithResources = Nothing
      , now = 0
      , page =
        { direction =
          if pageUntil > 0 then
            Concourse.Pagination.Until pageUntil
          else
            Concourse.Pagination.Since pageSince
        , limit = jobBuildsPerPage}
      , pagination = {previousPage = Nothing, nextPage = Nothing}
      }
  in
    ( model
    , Effects.batch
        [fetchJobBuilds 0 model.jobInfo (Just model.page)
        , fetchJob 0 model.jobInfo
        ]
    )

update : Action -> Model -> (Model, Effects Action)
update action model =
  case action of
    Noop ->
      (model, Effects.none)
    JobBuildsFetched (Ok builds) ->
      handleJobBuildsFetched builds model
    JobBuildsFetched (Err err) ->
      Debug.log ("failed to fetch builds: " ++ toString err) <|
        (model, Effects.none)
    JobFetched (Ok job) ->
      ( { model | job = Just job }
      , fetchJob (5 * Time.second) model.jobInfo
      )
    JobFetched (Err err) ->
      Debug.log ("failed to fetch job info: " ++ toString err) <|
        (model, Effects.none)
    BuildResourcesFetched buildResourcesFetched ->
      case buildResourcesFetched.result of
        Ok buildResources ->
          case model.buildsWithResources of
            Nothing -> (model, Effects.none)
            Just bwr ->
              case Array.get buildResourcesFetched.index bwr of
                Nothing -> (model, Effects.none)
                Just lubwr ->
                  ( { model
                    | buildsWithResources = Just
                      <| Array.set buildResourcesFetched.index
                                   (addFetchedResources buildResources lubwr)
                                   bwr
                    }
                  , Effects.none
                  )
        Err err ->
          (model, Effects.none)
    ClockTick now ->
      ({ model | now = now }, Effects.none)
    TogglePaused ->
      case model.job of
        Nothing -> (model, Effects.none)
        Just j ->
          ( { model
            | pausedChanging = True
            , job = Just { j | paused = not j.paused }
            }
          , if j.paused
            then unpauseJob model.jobInfo
            else pauseJob model.jobInfo
          )
    PausedToggled (Ok ()) ->
      ( { model | pausedChanging = False} , Effects.none)
    PausedToggled (Err (Http.BadResponse 401 _)) ->
      (model, redirectToLogin model)
    PausedToggled (Err err) ->
      Debug.log ("failed to pause/unpause job: " ++ toString err) <|
        (model, Effects.none)

permalink : List Build -> Page
permalink builds =
  case List.head builds of
    Nothing ->
      { direction = Concourse.Pagination.Since 0
      , limit = jobBuildsPerPage
      }
    Just build ->
      { direction = Concourse.Pagination.Since (build.id + 1)
      , limit = List.length builds
      }

handleJobBuildsFetched : Paginated Concourse.Build.Build -> Model -> (Model, Effects Action)
handleJobBuildsFetched paginatedBuilds model =
  let
    fetchedBuilds = Array.fromList paginatedBuilds.content
    newPage = permalink paginatedBuilds.content
  in
    ( { model
        | buildsWithResources =
            Just <| case model.buildsWithResources of
              Nothing -> Array.map initLiveUpdatingBuildWithResources fetchedBuilds
              Just lubwrs ->
                Array.indexedMap (addNextBuildFromArray fetchedBuilds) lubwrs
        , page = newPage
        , pagination = paginatedBuilds.pagination
      }
    , Effects.batch
        <| (fetchJobBuilds (5 * Time.second) model.jobInfo (Just newPage))
        :: ( Array.toList
             <| Array.indexedMap fetchBuildResources
             <| Array.map .id
             <| case model.buildsWithResources of
               Nothing -> fetchedBuilds
               Just lubwrs ->
                 Array.filter isRunning
                 <| Array.map .nextBuild lubwrs )
    )

isRunning : Build -> Bool
isRunning build = Concourse.BuildStatus.isRunning build.status

view : Signal.Address Action -> Model -> Html
view actions model = Html.div[]
  [ case model.job of
    Nothing -> loadSpinner
    Just job ->
      Html.div [ id "page-header", class (headerBuildStatusClass job.finishedBuild) ]
      [ Html.div [ class "build-header" ]
        [ Html.button
          ( List.append
            [id "job-state", class <| "btn-pause btn-large fl " ++ (getPausedState job model.pausedChanging)]
            (if not model.pausedChanging then [onClick actions TogglePaused] else [])
          )
          [ Html.i [ class <| "fa fa-fw fa-play " ++ (getPlayPauseLoadIcon job model.pausedChanging) ] [] ]
        , Html.form
          [class "trigger-build"
          , Html.Attributes.method "post"
          , Html.Attributes.action <| "/pipelines/" ++ model.jobInfo.pipelineName
            ++ "/jobs/" ++ model.jobInfo.name ++ "/builds"
          ]
          [ Html.button [ class "build-action fr", disabled job.disableManualTrigger ]
            [ Html.i [ class "fa fa-plus-circle" ] []
            ]
          ]
        , Html.h1 [] [ Html.text(model.jobInfo.name) ]
        ]
      ]
  , case model.buildsWithResources of
    Nothing -> loadSpinner
    Just bwr ->
      Html.div [ id "job-body" ]
      [ Html.div [ class "pagination-header" ]
        [ viewPaginationBar model
        , Html.h1 [] [ Html.text("builds") ]
        ]
      , Html.ul [ class "jobs-builds-list builds-list" ]
        <| List.map (viewBuildWithResources model) <| Array.toList bwr
      , Html.div [ class "pagination-footer" ]
        [ viewPaginationBar model ]
      ]
  ]

getPlayPauseLoadIcon : Job -> Bool -> String
getPlayPauseLoadIcon job pausedChanging =
  if pausedChanging
    then "fa-circle-o-notch fa-spin"
    else
      if job.paused
        then ""
        else "fa-pause"

getPausedState : Job -> Bool -> String
getPausedState job pausedChanging =
  if pausedChanging
    then
      "loading"
  else
    if job.paused
      then "enabled"
      else "disabled"

loadSpinner : Html
loadSpinner = Html.div [class "build-step"]
  [ Html.div [class "header"]
    [ Html.i [class "left fa fa-fw fa-spin fa-circle-o-notch"] []
    , Html.h3 [] [Html.text "Loading..."]
    ]
  ]

headerBuildStatusClass : (Maybe Build) -> String
headerBuildStatusClass finishedBuild =
  case finishedBuild of
    Nothing -> ""
    Just build -> Concourse.BuildStatus.show build.status

viewPaginationBar : Model -> Html
viewPaginationBar model =
  Html.div [ class "pagination fr"]
    [ case model.pagination.previousPage of
        Nothing ->
          Html.div [ class "btn-page-link disabled"]
          [ Html.span [class "arrow"]
            [ Html.i [ class "fa fa-arrow-left"] []
            ]
          ]
        Just page ->
          Html.div [ class "btn-page-link"]
          [ Html.a
            [ class "arrow"
            , href <| "/pipelines/" ++ model.jobInfo.pipelineName ++ "/jobs/"
              ++ model.jobInfo.name ++ "?" ++ paginationParam page
            ]
            [ Html.i [ class "fa fa-arrow-left"] []
            ]
          ]
    , case model.pagination.nextPage of
        Nothing ->
          Html.div [ class "btn-page-link disabled"]
          [ Html.span [class "arrow"]
            [ Html.i [ class "fa fa-arrow-right"] []
            ]
          ]
        Just page ->
          Html.div [ class "btn-page-link"]
          [ Html.a
            [ class "arrow"
            , href <| "/pipelines/" ++ model.jobInfo.pipelineName ++ "/jobs/"
              ++ model.jobInfo.name ++ "?" ++ paginationParam page
            ]
            [ Html.i [ class "fa fa-arrow-right"] []
            ]
          ]
    ]

viewBuildWithResources : Model -> LiveUpdatingBuildWithResources -> Html
viewBuildWithResources model lubwr =
  Html.li [class "js-build"]
    <| let
      build =
        case lubwr.buildWithResources of
          Nothing -> lubwr.nextBuild
          Just bwr -> bwr.build
      buildResourcesView = viewBuildResources model lubwr.buildWithResources
    in
      [ viewBuildHeader model build
      , Html.div [class "pam clearfix"]
        <| (BuildDuration.view build.duration model.now) :: buildResourcesView
      ]

viewBuildHeader : Model -> Build -> Html
viewBuildHeader model b =
  Html.a
  [class <| Concourse.BuildStatus.show b.status
  , href <| Concourse.Build.url b
  ]
  [ Html.text ("#" ++ b.name)
  ]

viewBuildResources : Model -> (Maybe BuildWithResources) -> List Html
viewBuildResources model buildWithResources =
  let
    inputsTable =
      case buildWithResources of
        Nothing -> loadSpinner
        Just bwr -> Html.table [class "build-resources"] <| List.map (viewBuildInputs model) bwr.resources.inputs
    outputsTable =
      case buildWithResources of
        Nothing -> loadSpinner
        Just bwr -> Html.table [class "build-resources"] <| List.map (viewBuildOutputs model) bwr.resources.outputs
  in
    [ Html.div [class "inputs mrl"]
      [ Html.div [class "resource-title pbs"]
        [ Html.i [class "fa fa-fw fa-arrow-down prs"] []
        , Html.text("inputs")
        ]
      , inputsTable
      ]
    , Html.div [class "outputs mrl"]
      [ Html.div [class "resource-title pbs"]
        [ Html.i [class "fa fa-fw fa-arrow-up prs"] []
        , Html.text("outputs")
        ]
      , outputsTable
      ]
    ]

viewBuildInputs : Model -> BuildInput -> Html
viewBuildInputs model bi =
  Html.tr [class "mbs pas resource fl clearfix"]
  [ Html.td [class "resource-name mrm"]
    [ Html.text(bi.resource)
    ]
  , Html.td [class "resource-version"]
    [ viewVersion bi.version
    ]
  ]

viewBuildOutputs : Model -> BuildOutput -> Html
viewBuildOutputs model bo =
  Html.tr [class "mbs pas resource fl clearfix"]
  [ Html.td [class "resource-name mrm"]
    [ Html.text(bo.resource)
    ]
  , Html.td [class "resource-version"]
    [ viewVersion bo.version
    ]
  ]

viewVersion : Version -> Html
viewVersion version =
  DictView.view << Dict.map (\_ s -> Html.text s) <|
    version

fetchJobBuilds : Time -> Concourse.Build.BuildJob -> Maybe Concourse.Pagination.Page -> Effects Action
fetchJobBuilds delay jobInfo page =
  Effects.task
    <| Task.map JobBuildsFetched
    <| Task.toResult
    <| Task.sleep delay `Task.andThen` (always <| Concourse.Build.fetchJobBuilds jobInfo page)

fetchJob : Time -> Concourse.Build.BuildJob -> Effects Action
fetchJob delay jobInfo =
  Effects.task
    <| Task.map JobFetched
    <| Task.toResult
    <| Task.sleep delay `Task.andThen` (always <| Concourse.Job.fetchJob jobInfo)

fetchBuildResources : Int -> Concourse.Build.BuildId -> Effects Action
fetchBuildResources index buildId =
  Effects.task (Task.map (initBuildResourcesFetched index) (Task.toResult (Concourse.BuildResources.fetch buildId)))

initBuildResourcesFetched : Int -> Result Http.Error (BuildResources) -> Action
initBuildResourcesFetched index result = BuildResourcesFetched { index = index, result = result }

paginationParam : Page -> String
paginationParam page =
  case page.direction of
    Concourse.Pagination.Since i -> "since=" ++ toString i
    Concourse.Pagination.Until i -> "until=" ++ toString i

pauseJob : BuildJob -> Effects Action
pauseJob jobInfo =
  Concourse.Job.pause jobInfo
    |> Task.toResult
    |> Task.map PausedToggled
    |> Effects.task


unpauseJob : BuildJob -> Effects Action
unpauseJob jobInfo =
  Concourse.Job.unpause jobInfo
    |> Task.toResult
    |> Task.map PausedToggled
    |> Effects.task

redirectToLogin : Model -> Effects Action
redirectToLogin model =
  Signal.send model.redirect "/login"
    |> Task.map (always Noop)
    |> Effects.task
