port module SubPage exposing (Model(..), Msg(..), init, urlUpdate, update, view, subscriptions)

import Dict
import Json.Encode
import Html exposing (Html)
import Html.App
import Login
import String

import Autoscroll
import Job
import Build
import Routes
import Pipeline
import TeamSelection

-- TODO: move ports somewhere else

port renderPipeline : (Json.Encode.Value, Json.Encode.Value) -> Cmd msg
port renderFinished : (Bool -> msg) -> Sub msg
-- port setTitle : String -> Cmd msg

type Model
  = BuildModel (Autoscroll.Model Build.Model)
  | JobModel Job.Model
  | LoginModel Login.Model
  | PipelineModel Pipeline.Model
  | SelectTeamModel TeamSelection.Model

type Msg
  = BuildMsg (Autoscroll.Msg Build.Msg)
  | JobMsg Job.Msg
  | LoginMsg Login.Msg
  | PipelineMsg Pipeline.Msg
  | SelectTeamMsg TeamSelection.Msg

superDupleWrap : ((a -> b), (c -> d)) -> (a, Cmd c) -> (b, Cmd d)
superDupleWrap (modelFunc, msgFunc) (model, msg) =
  (modelFunc model, Cmd.map msgFunc msg)

init : Routes.ConcourseRoute -> (Model, Cmd Msg)
init route =
  case route.logical of
    Routes.Build teamName pipelineName jobName buildName ->
      superDupleWrap (BuildModel, BuildMsg) <|
        Autoscroll.init
          Build.getScrollBehavior <<
            Build.init <|
              Build.JobBuildPage
                { teamName = teamName
                , pipelineName = pipelineName
                , jobName = jobName
                , buildName = buildName
                }
    Routes.OneOffBuild buildId ->
      superDupleWrap (BuildModel, BuildMsg) <|
        Autoscroll.init
          Build.getScrollBehavior <<
            Build.init <|
              Build.BuildPage <|
                Result.withDefault 0 (String.toInt buildId)
    Routes.Job teamName pipelineName jobName ->
      let
        pageSince =
          case Dict.get "since" route.parsed.query of
            Nothing ->
              0
            Just since ->
              Result.withDefault 0 (String.toInt since)
        pageUntil =
          case Dict.get "until" route.parsed.query of
            Nothing ->
              0
            Just until ->
              Result.withDefault 0 (String.toInt until)
      in
        superDupleWrap (JobModel, JobMsg) <|
          Job.init
            { jobName = jobName
            , teamName = teamName
            , pipelineName = pipelineName
            , pageSince = pageSince
            , pageUntil = pageUntil
            }
    Routes.SelectTeam ->
      let
        redirect =
          case Dict.get "redirect" route.parsed.query of
            Nothing ->
              ""
            Just path ->
              path
      in
        superDupleWrap (SelectTeamModel, SelectTeamMsg) <| TeamSelection.init redirect
    Routes.TeamLogin teamName ->
      let
        redirect =
          case Dict.get "redirect" route.parsed.query of
            Nothing ->
              ""
            Just path ->
              path
      in
        superDupleWrap (LoginModel, LoginMsg) <| Login.init teamName redirect
    Routes.Pipeline teamName pipelineName ->
      superDupleWrap (PipelineModel, PipelineMsg) <|
        Pipeline.init
          { render = renderPipeline
          , renderFinished = renderFinished
          }
          { teamName = teamName
          , pipelineName = pipelineName
          , turbulenceImgSrc = "" -- TODO this needs to be a real thing
          }

update : Msg -> Model -> (Model, Cmd Msg)
update msg mdl =
  case (msg, mdl) of
    (BuildMsg message, BuildModel scrollModel) ->
      superDupleWrap (BuildModel, BuildMsg) <| Autoscroll.update Build.update message scrollModel
    (JobMsg message, JobModel model) ->
      superDupleWrap (JobModel, JobMsg) <| Job.update message model
    (LoginMsg message, LoginModel model) ->
      superDupleWrap (LoginModel, LoginMsg) <| Login.update message model
    (PipelineMsg message, PipelineModel model) ->
      superDupleWrap (PipelineModel, PipelineMsg) <| Pipeline.update message model
    (SelectTeamMsg message, SelectTeamModel model) ->
      superDupleWrap (SelectTeamModel, SelectTeamMsg) <| TeamSelection.update message model
    _ ->
      Debug.log "Impossible combination" (mdl, Cmd.none)

urlUpdate : Routes.ConcourseRoute -> Model -> (Model, Cmd Msg)
urlUpdate route model =
  -- TODO update this so that pages which require url update handling have a special Msg for it
  (model, Cmd.none)

view : Model -> Html Msg
view mdl =
  case mdl of
    BuildModel model ->
      Html.App.map BuildMsg <| Autoscroll.view Build.view model
    JobModel model ->
      Html.App.map JobMsg <| Job.view model
    LoginModel model ->
      Html.App.map LoginMsg <| Login.view model
    PipelineModel model ->
      Html.App.map PipelineMsg <| Pipeline.view model
    SelectTeamModel model ->
      Html.App.map SelectTeamMsg <| TeamSelection.view model

subscriptions : Model -> Sub Msg
subscriptions mdl =
  case mdl of
    BuildModel model ->
      Sub.map BuildMsg <| Autoscroll.subscriptions Build.subscriptions model
    JobModel model ->
      Sub.map JobMsg <| Job.subscriptions model
    LoginModel model ->
      Sub.map LoginMsg <| Login.subscriptions model
    PipelineModel model ->
      Sub.map PipelineMsg <| Pipeline.subscriptions model
    SelectTeamModel model ->
      Sub.map SelectTeamMsg <| TeamSelection.subscriptions model
