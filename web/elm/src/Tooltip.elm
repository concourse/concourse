module Tooltip exposing
    ( Alignment(..)
    , Direction(..)
    , Model
    , Tooltip
    , handleCallback
    , view
    )

import Browser.Dom
import EffectTransformer exposing (ET)
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Html.Attributes exposing (style)
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message)


type alias Model m =
    { m | hovered : HoverState.HoverState }


type alias Tooltip =
    { body : Html Message
    , arrow : Maybe Arrow
    , attachPosition : AttachPosition
    }



-- Many tooltips, especially in crowded parts of the UI, have an extra
-- triangular piece sticking out that points to the tooltip's target. Online
-- this element is variously called a 'tail' or an 'arrow', with 'arrow'
-- predominating.


type alias Arrow =
    { size : Float
    , color : String
    }


type TooltipCondition
    = AlwaysShow
    | OnlyShowWhenOverflowing


type alias AttachPosition =
    { direction : Direction
    , alignment : Alignment
    }


type Direction
    = Top
    | Right Float


type Alignment
    = Start
    | Middle Float
    | End


policy : DomID -> TooltipCondition
policy domID =
    case domID of
        SideBarPipeline _ _ ->
            OnlyShowWhenOverflowing

        SideBarTeam _ _ ->
            OnlyShowWhenOverflowing

        SideBarInstanceGroup _ _ _ ->
            OnlyShowWhenOverflowing

        _ ->
            AlwaysShow


position : AttachPosition -> Browser.Dom.Element -> List (Html.Attribute msg)
position { direction, alignment } { element, viewport } =
    let
        target =
            element

        vertical =
            case ( direction, alignment ) of
                ( Top, _ ) ->
                    [ style "bottom" <| String.fromFloat (viewport.height - target.y) ++ "px" ]

                ( Right _, Start ) ->
                    [ style "top" <| String.fromFloat target.y ++ "px" ]

                ( Right _, Middle height ) ->
                    [ style "top" <| String.fromFloat (target.y + (target.height - height) / 2) ++ "px" ]

                ( Right _, End ) ->
                    [ style "bottom" <| String.fromFloat (viewport.height - target.y - target.height) ++ "px" ]

        horizontal =
            case ( direction, alignment ) of
                ( Top, Start ) ->
                    [ style "left" <| String.fromFloat target.x ++ "px" ]

                ( Top, Middle width ) ->
                    [ style "left" <| String.fromFloat (target.x + (target.width - width) / 2) ++ "px" ]

                ( Top, End ) ->
                    [ style "right" <| String.fromFloat (viewport.width - target.x - target.width) ++ "px" ]

                ( Right offset, _ ) ->
                    [ style "left" <| String.fromFloat (target.x + target.width + offset) ++ "px" ]
    in
    [ style "position" "fixed", style "z-index" "100" ] ++ vertical ++ horizontal


handleCallback : Callback -> ET (Model m)
handleCallback callback ( model, effects ) =
    case callback of
        GotViewport _ (Ok { scene, viewport }) ->
            case model.hovered of
                HoverState.Hovered domID ->
                    if policy domID == OnlyShowWhenOverflowing && viewport.width >= scene.width then
                        ( model, effects )

                    else
                        ( { model
                            | hovered =
                                HoverState.TooltipPending domID
                          }
                        , effects ++ [ Effects.GetElement domID ]
                        )

                _ ->
                    ( model, effects )

        GotElement (Ok element) ->
            case model.hovered of
                HoverState.TooltipPending domID ->
                    ( { model | hovered = HoverState.Tooltip domID element }
                    , effects
                    )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


arrowView : AttachPosition -> Browser.Dom.Element -> Arrow -> Html Message
arrowView { direction } target { size, color } =
    Html.div
        ((case direction of
            Top ->
                [ style "border-top" <| String.fromFloat size ++ "px solid " ++ color
                , style "border-left" <| String.fromFloat size ++ "px solid transparent"
                , style "border-right" <| String.fromFloat size ++ "px solid transparent"
                , style "margin-bottom" <| "-" ++ String.fromFloat size ++ "px"
                ]

            Right _ ->
                [ style "border-right" <| String.fromFloat size ++ "px solid " ++ color
                , style "border-top" <| String.fromFloat size ++ "px solid transparent"
                , style "border-bottom" <| String.fromFloat size ++ "px solid transparent"
                , style "margin-left" <| "-" ++ String.fromFloat size ++ "px"
                ]
         )
            ++ position
                { direction = direction, alignment = Middle (2 * size) }
                target
        )
        []


view : Model m -> Tooltip -> Html Message
view { hovered } { body, attachPosition, arrow } =
    case ( hovered, arrow ) of
        ( HoverState.Tooltip _ target, a ) ->
            Html.div (position attachPosition target)
                [ Maybe.map (arrowView attachPosition target) a |> Maybe.withDefault (Html.text "")
                , body
                ]

        _ ->
            Html.text ""
