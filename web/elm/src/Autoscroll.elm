module Autoscroll exposing
  ( init
  , update
  , urlUpdate
  , view
  , subscriptions
  , ScrollBehavior(..)
  , Msg(SubMsg)
  , fromBottom
  )

import AnimationFrame
import Html exposing (Html)
import Html.App
import Task

import Scroll

type alias Model subModel =
  { subModel : subModel
  , scrollBehaviorFunc : subModel -> ScrollBehavior
  }

type ScrollBehavior
  = Scroll String
  | NoScroll

type Msg subMsg
  = SubMsg subMsg
  | ScrollDown
  | ScrolledDown

init : (subModel -> ScrollBehavior) -> (subModel, Cmd subMsg) -> (Model subModel, Cmd (Msg subMsg))
init toScrollMsg (subModel, subCmd) =
  (Model subModel toScrollMsg, Cmd.map SubMsg subCmd)

update : (subMsg -> subModel -> (subModel, Cmd subMsg)) -> Msg subMsg -> Model subModel -> (Model subModel, Cmd (Msg subMsg))
update subUpdate action model =
  case action of
    SubMsg subMsg ->
      let
        (subModel, subCmd) = subUpdate subMsg model.subModel
      in
        ({ model | subModel = subModel }, Cmd.map SubMsg subCmd)

    ScrollDown ->
      ( model
      , case model.scrollBehaviorFunc model.subModel of
          Scroll ele ->
            scrollToBottom ele

          NoScroll ->
            Cmd.none
      )

    ScrolledDown ->
      (model, Cmd.none)

fromBottom : String -> (Int -> msg) -> Cmd msg
fromBottom ele cons =
  Task.perform (always (cons 0)) cons <|
    Scroll.fromBottom ele

urlUpdate : (pageResult -> subModel -> (subModel, Cmd subMsg)) -> pageResult -> Model subModel -> (Model subModel, Cmd (Msg subMsg))
urlUpdate subUrlUpdate pageResult model =
  let
    (newSubModel, subMsg) = subUrlUpdate pageResult model.subModel
  in
    ({ model | subModel = newSubModel }, Cmd.map SubMsg subMsg )

view : (subModel -> Html subMsg) -> Model subModel -> Html (Msg subMsg)
view subView model =
  Html.App.map SubMsg (subView model.subModel)

subscriptions : Model subModel -> Sub (Msg subMsg)
subscriptions model =
  if model.scrollBehaviorFunc model.subModel /= NoScroll then
    AnimationFrame.times (always ScrollDown)
  else
    Sub.none

scrollToBottom : String -> Cmd (Msg x)
scrollToBottom ele =
  Task.perform (always ScrolledDown) (always ScrolledDown) (Scroll.toBottom ele)
