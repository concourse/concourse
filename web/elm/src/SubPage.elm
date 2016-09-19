port module SubPage exposing (Page, Model, Msg, init, update, view, subscriptions)

import Json.Encode
import Html exposing (Html)
import Html.App
import Login
import Routes
import Pipeline

-- TODO: move ports somewhere else

port renderPipeline : (Json.Encode.Value, Json.Encode.Value) -> Cmd msg
port renderFinished : (Bool -> msg) -> Sub msg

type alias Page =
  { init : Routes.Route -> (Model, Cmd Msg)
  , update : Msg -> Model -> (Model, Cmd Msg)
  , view : Model -> Html Msg
  , subscriptions : Model -> Sub Msg
  }

type Model
  = LoginModel Login.Model
  | PipelineModel Pipeline.Model

type Msg
  = LoginMsg Login.Msg
  | PipelineMsg Pipeline.Msg

init : Routes.Route -> (Model, Cmd Msg)
init route =
  case route of
    Routes.Login ->
      let
        (subModel, subMsg) = Login.init route -- TODO: remove route from Login
      in
        (LoginModel subModel, Cmd.map LoginMsg subMsg)
    Routes.TeamLogin teamName ->
      let
        (subModel, subMsg) = Login.init route
      in
        (LoginModel subModel, Cmd.map LoginMsg subMsg)
    Routes.Pipeline teamName pipelineName ->
      let
        (subModel, subMsg) =
          Pipeline.init { render = renderPipeline, renderFinished = renderFinished } { teamName = teamName, pipelineName = pipelineName, turbulenceImgSrc = ""}
      in
        (PipelineModel subModel, Cmd.map PipelineMsg subMsg)

update : Msg -> Model -> (Model, Cmd Msg)
update msg model =
  case model of
    LoginModel lModel ->
      case msg of
        LoginMsg lMsg ->
          let
            (subModel, subMsg) = Login.update lMsg lModel
          in
            (LoginModel subModel, Cmd.map LoginMsg subMsg)
        _ ->
          (model, Cmd.none)
    PipelineModel pModel ->
      case msg of
        PipelineMsg pMsg ->
          let
            (subModel, subMsg) = Pipeline.update pMsg pModel
          in
            (PipelineModel subModel, Cmd.map PipelineMsg subMsg)
        _ ->
          (model, Cmd.none)

view : Model -> Html Msg
view model =
  case model of
    LoginModel lModel ->
      let
        subMsg = Login.view lModel
      in
        Html.App.map LoginMsg subMsg
    PipelineModel pModel ->
      let
        subMsg = Pipeline.view pModel
      in
        Html.App.map PipelineMsg subMsg

subscriptions : Model -> Sub Msg
subscriptions model =
  case model of
    LoginModel lModel ->
      let
        subMsg = Login.subscriptions lModel
      in
        Sub.map LoginMsg subMsg
    PipelineModel pModel ->
      let
        subMsg = Pipeline.subscriptions pModel
      in
        Sub.map PipelineMsg subMsg
