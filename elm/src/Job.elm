module Job exposing (Flags, init, update, view, Msg(ClockTick))

import Array exposing (Array)
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class, href, id, disabled, attribute)
import Html.Events exposing (onClick)
import Http
import Process
import Task
import Time exposing (Time)

import Concourse.Build exposing (Build, BuildJob, BuildDuration)
import Concourse.Job exposing (Job)
import Concourse.BuildResources exposing (BuildResources, BuildInput, BuildOutput)
import Concourse.BuildStatus exposing (BuildStatus)
import Concourse.Pagination exposing (Pagination, Paginated, Page)
import Concourse.Version exposing (Version)
import BuildDuration
import DictView
import Redirect

type alias Model =
  { jobInfo : BuildJob
  , job : (Maybe Job)
  , pausedChanging : Bool
  , buildsWithResources : Maybe (Array LiveUpdatingBuildWithResources)
  , now : Time
  , page : Page
  , pagination : Pagination
  }

type Msg
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

type alias Flags =
  { jobName : String
  , teamName : String
  , pipelineName : String
  , pageSince : Int
  , pageUntil : Int
  }

init : Flags -> (Model, Cmd Msg)
init flags =
  let
    model =
      { jobInfo =
          { name = flags.jobName
          , teamName = flags.teamName
          , pipelineName = flags.pipelineName
          }
      , job = Nothing
      , pausedChanging = False
      , buildsWithResources = Nothing
      , now = 0
      , page =
          { direction =
              if flags.pageUntil > 0 then
                Concourse.Pagination.Until flags.pageUntil
              else
                Concourse.Pagination.Since flags.pageSince
          , limit = jobBuildsPerPage
          }
      , pagination =
          { previousPage = Nothing
          , nextPage = Nothing
          }
      }
  in
    ( model
    , Cmd.batch
        [fetchJobBuilds 0 model.jobInfo (Just model.page)
        , fetchJob 0 model.jobInfo
        ]
    )

update : Msg -> Model -> (Model, Cmd Msg)
update action model =
  case action of
    Noop ->
      (model, Cmd.none)
    JobBuildsFetched (Ok builds) ->
      handleJobBuildsFetched builds model
    JobBuildsFetched (Err err) ->
      Debug.log ("failed to fetch builds: " ++ toString err) <|
        (model, Cmd.none)
    JobFetched (Ok job) ->
      ( { model | job = Just job }
      , fetchJob (5 * Time.second) model.jobInfo
      )
    JobFetched (Err err) ->
      Debug.log ("failed to fetch job info: " ++ toString err) <|
        (model, Cmd.none)
    BuildResourcesFetched buildResourcesFetched ->
      case buildResourcesFetched.result of
        Ok buildResources ->
          case model.buildsWithResources of
            Nothing -> (model, Cmd.none)
            Just bwr ->
              case Array.get buildResourcesFetched.index bwr of
                Nothing -> (model, Cmd.none)
                Just lubwr ->
                  ( { model
                    | buildsWithResources = Just
                      <| Array.set buildResourcesFetched.index
                                   (addFetchedResources buildResources lubwr)
                                   bwr
                    }
                  , Cmd.none
                  )
        Err err ->
          (model, Cmd.none)
    ClockTick now ->
      ({ model | now = now }, Cmd.none)
    TogglePaused ->
      case model.job of
        Nothing -> (model, Cmd.none)
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
      ( { model | pausedChanging = False} , Cmd.none)
    PausedToggled (Err (Http.BadResponse 401 _)) ->
      (model, redirectToLogin model)
    PausedToggled (Err err) ->
      Debug.log ("failed to pause/unpause job: " ++ toString err) <|
        (model, Cmd.none)

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

handleJobBuildsFetched : Paginated Concourse.Build.Build -> Model -> (Model, Cmd Msg)
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
    , Cmd.batch
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

view : Model -> Html Msg
view model =
  Html.div [class "with-fixed-header"] [
    case model.job of
      Nothing ->
        loadSpinner

      Just job ->
        Html.div [class "fixed-header"]
          [ Html.div [ class ("build-header " ++ headerBuildStatusClass job.finishedBuild)] -- TODO really?
              [ Html.button
                  ( List.append
                    [id "job-state", attribute "aria-label" "Toggle Job Paused State", class <| "btn-pause btn-large fl " ++ (getPausedState job model.pausedChanging)]
                    (if not model.pausedChanging then [onClick TogglePaused] else [])
                  )
                  [ Html.i [ class <| "fa fa-fw fa-play " ++ (getPlayPauseLoadIcon job model.pausedChanging) ] [] ]
              , Html.form
                  [ class "trigger-build"
                  , Html.Attributes.method "post"
                  , Html.Attributes.action <| "/teams/" ++ model.jobInfo.teamName ++ "/pipelines/" ++ model.jobInfo.pipelineName
                    ++ "/jobs/" ++ model.jobInfo.name ++ "/builds"
                  ]
                  [ Html.button [ class "build-action fr", disabled job.disableManualTrigger, attribute "aria-label" "Trigger Build" ]
                    [ Html.i [ class "fa fa-plus-circle" ] []
                    ]
                  ]
              , Html.h1 [] [ Html.text(model.jobInfo.name) ]
              ]
          , Html.div [ class "pagination-header" ]
              [ viewPaginationBar model
              , Html.h1 [] [ Html.text("builds") ]
              ]
          ],
    case model.buildsWithResources of
      Nothing ->
        loadSpinner

      Just bwr ->
        Html.div [class "scrollable-body"]
          [ Html.ul [ class "jobs-builds-list builds-list" ]
            <| List.map (viewBuildWithResources model) <| Array.toList bwr
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

loadSpinner : Html Msg
loadSpinner =
  Html.div [class "build-step"]
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

viewPaginationBar : Model -> Html Msg
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
            , href <| "/teams/" ++ model.jobInfo.teamName ++ "/pipelines/" ++ model.jobInfo.pipelineName ++ "/jobs/"
              ++ model.jobInfo.name ++ "?" ++ paginationParam page
            , attribute "aria-label" "Previous Page"
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
            , href <| "/teams/" ++ model.jobInfo.teamName ++ "/pipelines/" ++ model.jobInfo.pipelineName ++ "/jobs/"
              ++ model.jobInfo.name ++ "?" ++ paginationParam page
            , attribute "aria-label" "Next Page"
            ]
            [ Html.i [ class "fa fa-arrow-right"] []
            ]
          ]
    ]

viewBuildWithResources : Model -> LiveUpdatingBuildWithResources -> Html Msg
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

viewBuildHeader : Model -> Build -> Html Msg
viewBuildHeader model b =
  Html.a
  [class <| Concourse.BuildStatus.show b.status
  , href <| Concourse.Build.url b
  ]
  [ Html.text ("#" ++ b.name)
  ]

viewBuildResources : Model -> (Maybe BuildWithResources) -> List (Html Msg)
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

viewBuildInputs : Model -> BuildInput -> Html Msg
viewBuildInputs model bi =
  Html.tr [class "mbs pas resource fl clearfix"]
  [ Html.td [class "resource-name mrm"]
    [ Html.text(bi.resource)
    ]
  , Html.td [class "resource-version"]
    [ viewVersion bi.version
    ]
  ]

viewBuildOutputs : Model -> BuildOutput -> Html Msg
viewBuildOutputs model bo =
  Html.tr [class "mbs pas resource fl clearfix"]
  [ Html.td [class "resource-name mrm"]
    [ Html.text(bo.resource)
    ]
  , Html.td [class "resource-version"]
    [ viewVersion bo.version
    ]
  ]

viewVersion : Version -> Html Msg
viewVersion version =
  DictView.view << Dict.map (\_ s -> Html.text s) <|
    version

fetchJobBuilds : Time -> Concourse.Build.BuildJob -> Maybe Concourse.Pagination.Page -> Cmd Msg
fetchJobBuilds delay jobInfo page =
  Cmd.map JobBuildsFetched << Task.perform Err Ok <|
    Process.sleep delay `Task.andThen` (always <| Concourse.Build.fetchJobBuilds jobInfo page)

fetchJob : Time -> Concourse.Build.BuildJob -> Cmd Msg
fetchJob delay jobInfo =
  Cmd.map JobFetched << Task.perform Err Ok <|
    Process.sleep delay `Task.andThen` (always <| Concourse.Job.fetchJob jobInfo)

fetchBuildResources : Int -> Concourse.Build.BuildId -> Cmd Msg
fetchBuildResources index buildId =
  Cmd.map (initBuildResourcesFetched index) << Task.perform Err Ok <|
    Concourse.BuildResources.fetch buildId

initBuildResourcesFetched : Int -> Result Http.Error (BuildResources) -> Msg
initBuildResourcesFetched index result = BuildResourcesFetched { index = index, result = result }

paginationParam : Page -> String
paginationParam page =
  case page.direction of
    Concourse.Pagination.Since i -> "since=" ++ toString i
    Concourse.Pagination.Until i -> "until=" ++ toString i

pauseJob : BuildJob -> Cmd Msg
pauseJob jobInfo =
  Cmd.map PausedToggled << Task.perform Err Ok <|
    Concourse.Job.pause jobInfo


unpauseJob : BuildJob -> Cmd Msg
unpauseJob jobInfo =
  Cmd.map PausedToggled << Task.perform Err Ok <|
    Concourse.Job.unpause jobInfo

redirectToLogin : Model -> Cmd Msg
redirectToLogin model =
  Cmd.map (always Noop) << Task.perform Err Ok <|
    Redirect.to "/login"
