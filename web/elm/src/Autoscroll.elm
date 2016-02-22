module Autoscroll where

import Effects exposing (Effects)
import Html exposing (Html)
import Task

import Scroll

type alias Model subModel =
  { subModel : subModel
  , shouldScroll : Bool
  , shouldAutoscroll : subModel -> Bool
  }

type Action subAction
  = SubAction subAction
  | ScrollDown
  | ScrolledDown
  | FromBottom Int

init : (subModel -> Bool) -> (subModel, Effects subAction) -> (Model subModel, Effects (Action subAction))
init toScrollAction (subModel, subEffects) =
  (Model subModel True toScrollAction, Effects.map SubAction subEffects)

update : (subAction -> subModel -> (subModel, Effects subAction)) -> Action subAction -> Model subModel -> (Model subModel, Effects (Action subAction))
update subUpdate action model =
  case action of
    SubAction subAction ->
      let
        (subModel, subEffects) = subUpdate subAction model.subModel
      in
        ({ model | subModel = subModel }, Effects.map SubAction subEffects)

    ScrollDown ->
      ( model
      , if model.shouldScroll && model.shouldAutoscroll model.subModel then
          scrollToBottom
        else
          Effects.none
      )

    ScrolledDown ->
      (model, Effects.none)

    FromBottom num ->
      ({ model | shouldScroll = Debug.log ("num: " ++ toString num) (num < 16) }, Effects.none)


view : (Signal.Address subAction -> subModel -> Html) -> Signal.Address (Action subAction) -> Model subModel -> Html
view subView actions model =
  subView (Signal.forwardTo actions SubAction) model.subModel

scrollToBottom : Effects (Action x)
scrollToBottom =
  Scroll.toBottom
    |> Task.map (always ScrolledDown)
    |> Effects.task
