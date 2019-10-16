module Tooltip exposing (Model, handleCallback)

import EffectTransformer exposing (ET)
import HoverState
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects


type alias Model m =
    { m | hovered : HoverState.HoverState }


handleCallback : Callback -> ET (Model m)
handleCallback callback ( model, effects ) =
    case callback of
        GotViewport (Ok { scene, viewport }) ->
            case ( model.hovered, scene.width > viewport.width ) of
                ( HoverState.Hovered domID, True ) ->
                    ( { model
                        | hovered =
                            HoverState.TooltipPending domID
                      }
                    , effects ++ [ Effects.GetElement domID ]
                    )

                _ ->
                    ( model, effects )

        GotElement (Ok { element }) ->
            case model.hovered of
                HoverState.TooltipPending domID ->
                    ( { model
                        | hovered =
                            HoverState.Tooltip domID
                                { top = element.y + (element.height / 2)
                                , left = element.x + element.width
                                , arrowSize = 15
                                , marginTop = -15
                                }
                      }
                    , effects
                    )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )
