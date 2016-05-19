module Autoscroll exposing (..)

import Html exposing (Html)
import Html.App
import Task

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

type Action subAction
  = SubAction subAction
  | ScrollDown
  | ScrolledDown
  | FromBottom Int

init : (subModel -> ScrollBehavior) -> (subModel, Cmd subAction) -> (Model subModel, Cmd (Action subAction))
init toScrollAction (subModel, subCmd) =
  (Model subModel True toScrollAction, Cmd.map SubAction subCmd)

update : (subAction -> subModel -> (subModel, Cmd subAction)) -> Action subAction -> Model subModel -> (Model subModel, Cmd (Action subAction))
update subUpdate action model =
  case action of
    SubAction subAction ->
      let
        (subModel, subCmd) = subUpdate subAction model.subModel
      in
        ({ model | subModel = subModel }, Cmd.map SubAction subCmd)

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


view : (subModel -> Html subAction) -> Model subModel -> Html (Action subAction)
view subView model =
  Html.App.map SubAction (subView model.subModel)

scrollToBottom : Cmd (Action x)
scrollToBottom =
  Task.perform (always ScrolledDown) (always ScrolledDown) Scroll.toBottom
