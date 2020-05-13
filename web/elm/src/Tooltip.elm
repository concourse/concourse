module Tooltip exposing (Model, handleCallback, view)

import Browser.Dom
import Build.Styles
import Dashboard.Styles
import EffectTransformer exposing (ET)
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Html.Attributes exposing (style)
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message as Message exposing (DomID(..))
import SideBar.Styles


type alias Model m =
    { m | hovered : HoverState.HoverState }


type TooltipCondition
    = AlwaysShow
    | OnlyShowWhenOverflowing


type alias AttachPosition =
    { direction : Direction
    , alignment : Alignment
    }


type Direction
    = Top
    | Right


type Alignment
    = Start
    | Middle


policy : DomID -> TooltipCondition
policy domID =
    case domID of
        SideBarPipeline _ ->
            OnlyShowWhenOverflowing

        SideBarTeam _ ->
            OnlyShowWhenOverflowing

        _ ->
            AlwaysShow


position : AttachPosition -> Browser.Dom.Element -> Maybe Float -> Maybe Float -> List (Html.Attribute msg)
position { direction, alignment } { element, viewport } w h =
    let
        target =
            element

        vertical =
            case ( direction, alignment, h ) of
                ( Top, _, _ ) ->
                    [ style "bottom" <| String.fromFloat (viewport.height - target.y) ++ "px" ]

                ( Right, Start, _ ) ->
                    [ style "top" <| String.fromFloat target.y ++ "px" ]

                ( Right, Middle, Just height ) ->
                    [ style "top" <| String.fromFloat (target.y + (target.height - height) / 2) ++ "px" ]

                -- ( Right, End, _ ) ->
                --     [ style "bottom" <| String.fromFloat (viewport.height - target.y - target.height) ++ "px" ]
                -- ( Bottom, _, _ ) ->
                --     [ style "top" <| String.fromFloat (target.y + target.height) ++ "px" ]
                -- ( Left, Start, _ ) ->
                --     [ style "top" <| String.fromFloat target.y ++ "px" ]
                -- ( Left, Middle, Just height ) ->
                --     [ style "top" <| String.fromFloat (target.y + (target.height - height) / 2) ++ "px" ]
                -- ( Left, End, _ ) ->
                --     [ style "bottom" <| String.fromFloat (viewport.height - target.y - target.height) ++ "px" ]
                _ ->
                    []

        horizontal =
            case ( direction, alignment, w ) of
                ( Top, Start, _ ) ->
                    [ style "left" <| String.fromFloat target.x ++ "px" ]

                ( Top, Middle, Just width ) ->
                    [ style "left" <| String.fromFloat (target.x + (target.width - width) / 2) ++ "px" ]

                -- ( Top, End, _ ) ->
                --     [ style "right" <| String.fromFloat (target.x + target.width) ++ "px" ]
                ( Right, _, _ ) ->
                    [ style "left" <| String.fromFloat (target.x + target.width) ++ "px" ]

                -- ( Bottom, Start, _ ) ->
                --     [ style "left" <| String.fromFloat target.x ++ "px" ]
                -- ( Bottom, Middle, Just width ) ->
                --     [ style "left" <| String.fromFloat (target.x + (target.width - width) / 2) ++ "px" ]
                -- ( Bottom, End, _ ) ->
                --     [ style "right" <| String.fromFloat (target.x + target.width) ++ "px" ]
                -- ( Left, _, _ ) ->
                --     [ style "right" <| String.fromFloat (viewport.width - target.x) ++ "px" ]
                _ ->
                    []
    in
    style "position" "fixed" :: vertical ++ horizontal


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


view : Model m -> Html msg
view { hovered } =
    case hovered of
        HoverState.Tooltip (Message.FirstOccurrenceGetStepLabel _) target ->
            Html.div []
                [ Html.div
                    (Build.Styles.firstOccurrenceTooltip
                        ++ position { direction = Top, alignment = Start } target Nothing Nothing
                    )
                    [ Html.text "new version" ]
                , Html.div
                    (Build.Styles.firstOccurrenceTooltipArrow
                        ++ position { direction = Top, alignment = Middle } target (Just 5) Nothing
                    )
                    []
                ]

        HoverState.Tooltip (Message.SideBarTeam teamName) target ->
            Html.div
                (SideBar.Styles.tooltip
                    ++ position { direction = Right, alignment = Middle } target Nothing (Just 30)
                )
                [ Html.div SideBar.Styles.tooltipArrow []
                , Html.div SideBar.Styles.tooltipBody [ Html.text teamName ]
                ]

        HoverState.Tooltip (Message.SideBarPipeline pipelineID) target ->
            Html.div
                (SideBar.Styles.tooltip
                    ++ position { direction = Right, alignment = Middle } target Nothing (Just 30)
                )
                [ Html.div SideBar.Styles.tooltipArrow []
                , Html.div SideBar.Styles.tooltipBody [ Html.text pipelineID.pipelineName ]
                ]

        HoverState.Tooltip (Message.PipelineStatusIcon _) target ->
            Html.div
                (Dashboard.Styles.jobsDisabledTooltip
                    ++ position { direction = Top, alignment = Start } target Nothing Nothing
                )
                [ Html.text "automatic job monitoring disabled" ]

        _ ->
            Html.text ""
