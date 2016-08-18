module Autoscroll exposing
  ( init
  , update
  , urlUpdate
  , view
  , subscriptions
  , ScrollBehavior(..)
  , Msg(SubMsg)
  )

import Html exposing (Html)
import Html.App
import Task
import Time

import Scroll

type alias Model subModel =
  { subModel : subModel
  , shouldScroll : Bool
  , scrollBehaviorFunc : subModel -> ScrollBehavior
  }

type ScrollBehavior
  = Autoscroll
  | ScrollUntilCancelled
  | NoScroll

type Msg subMsg
  = SubMsg subMsg
  | ScrollDown
  | ScrolledDown
  | FromBottom Int

init : (subModel -> ScrollBehavior) -> (subModel, Cmd subMsg) -> (Model subModel, Cmd (Msg subMsg))
init toScrollMsg (subModel, subCmd) =
  (Model subModel True toScrollMsg, Cmd.map SubMsg subCmd)

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
      , if model.shouldScroll && model.scrollBehaviorFunc model.subModel /= NoScroll then
          scrollToBottom
        else
          Cmd.none
      )

    ScrolledDown ->
      (model, Cmd.none)

    FromBottom num ->
      ( { model
        | shouldScroll =
            case model.scrollBehaviorFunc model.subModel of
              Autoscroll -> (num < 16)
              _ -> False
        }
      , Cmd.none
      )

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
  let
    scrolledUp =
      Scroll.fromBottom FromBottom

    pushDown =
      Time.every (100 * Time.millisecond) (always ScrollDown)
  in
    Sub.batch
      [ scrolledUp
      , pushDown
      ]

scrollToBottom : Cmd (Msg x)
scrollToBottom =
  Task.perform (always ScrolledDown) (always ScrolledDown) Scroll.toBottom
