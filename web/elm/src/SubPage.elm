port module SubPage exposing (Model, Msg, init, update, view, subscriptions)

import Dict
import Json.Encode
import Html exposing (Html)
import Html.App
import Login
import Routes
import Pipeline
import TeamSelection

-- TODO: move ports somewhere else

port renderPipeline : (Json.Encode.Value, Json.Encode.Value) -> Cmd msg
port renderFinished : (Bool -> msg) -> Sub msg
--
-- type alias Page =
--   { init : Routes.ConcourseRoute -> (Model, Cmd Msg)
--   , update : Msg -> Model -> (Model, Cmd Msg)
--   , view : Model -> Html Msg
--   , subscriptions : Model -> Sub Msg
--   }

type Model
  = LoginModel Login.Model
  | PipelineModel Pipeline.Model
  | SelectTeamModel TeamSelection.Model

type Msg
  = LoginMsg Login.Msg
  | PipelineMsg Pipeline.Msg
  | SelectTeamMsg TeamSelection.Msg

superDupleWrap : ((a -> b), (c -> d)) -> (a, Cmd c) -> (b, Cmd d)
superDupleWrap (modelFunc, msgFunc) (model, msg) =
  (modelFunc model, Cmd.map msgFunc msg)

init : Routes.ConcourseRoute -> (Model, Cmd Msg)
init route =
  case route.logical of
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
    (LoginMsg message, LoginModel model) ->
      superDupleWrap (LoginModel, LoginMsg) <| Login.update message model
    (PipelineMsg message, PipelineModel model) ->
      superDupleWrap (PipelineModel, PipelineMsg) <| Pipeline.update message model
    (SelectTeamMsg message, SelectTeamModel model) ->
      superDupleWrap (SelectTeamModel, SelectTeamMsg) <| TeamSelection.update message model
    _ ->
      Debug.log "Impossible combination" (mdl, Cmd.none)

view : Model -> Html Msg
view mdl =
  case mdl of
    LoginModel model ->
      Html.App.map LoginMsg <| Login.view model
    PipelineModel model ->
      Html.App.map PipelineMsg <| Pipeline.view model
    SelectTeamModel model ->
      Html.App.map SelectTeamMsg <| TeamSelection.view model

subscriptions : Model -> Sub Msg
subscriptions mdl =
  case mdl of
    LoginModel model ->
      Sub.map LoginMsg <| Login.subscriptions model
    PipelineModel model ->
      Sub.map PipelineMsg <| Pipeline.subscriptions model
    SelectTeamModel model ->
      Sub.map SelectTeamMsg <| TeamSelection.subscriptions model
