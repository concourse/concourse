module Tooltip exposing (Model, handleCallback, view)

import Build.Styles
import EffectTransformer exposing (ET)
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Message.Callback exposing (Callback(..), TooltipPolicy(..))
import Message.Effects as Effects
import Message.Message as Message
import SideBar.Styles


type alias Model m =
    { m | hovered : HoverState.HoverState }


handleCallback : Callback -> ET (Model m)
handleCallback callback ( model, effects ) =
    case callback of
        GotViewport _ policy (Ok { scene, viewport }) ->
            case model.hovered of
                HoverState.Hovered domID ->
                    if policy == OnlyShowWhenOverflowing && viewport.width >= scene.width then
                        ( model, effects )

                    else
                        ( { model | hovered = HoverState.TooltipPending domID }
                        , effects ++ [ Effects.GetElement domID ]
                        )

                _ ->
                    ( model, effects )

        GotElement (Ok { element, viewport }) ->
            case model.hovered of
                HoverState.TooltipPending (Message.FirstOccurrenceGetStepLabel stepID) ->
                    ( { model
                        | hovered =
                            HoverState.Tooltip (Message.FirstOccurrenceGetStepLabel stepID) <|
                                Bottom
                                    (viewport.height - element.y)
                                    element.x
                                    element.width
                      }
                    , effects
                    )

                HoverState.TooltipPending domID ->
                    ( { model
                        | hovered =
                            HoverState.Tooltip domID <|
                                Top (element.y + (element.height / 2))
                                    (element.x + element.width)
                      }
                    , effects
                    )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


view : Model m -> Html msg
view { hovered } =
    case hovered of
        HoverState.Tooltip (Message.FirstOccurrenceGetStepLabel _) (Bottom b l w) ->
            Html.div []
                [ Html.div
                    (Build.Styles.firstOccurrenceTooltip b l)
                    [ Html.text "new version" ]
                , Html.div
                    (Build.Styles.firstOccurrenceTooltipArrow b l w)
                    []
                ]

        HoverState.Tooltip (Message.SideBarTeam teamName) (Top t l) ->
            Html.div
                (SideBar.Styles.tooltip t l)
                [ Html.div SideBar.Styles.tooltipArrow []
                , Html.div SideBar.Styles.tooltipBody [ Html.text teamName ]
                ]

        HoverState.Tooltip (Message.SideBarPipeline { pipelineName }) (Top t l) ->
            Html.div
                (SideBar.Styles.tooltip t l)
                [ Html.div SideBar.Styles.tooltipArrow []
                , Html.div SideBar.Styles.tooltipBody [ Html.text pipelineName ]
                ]

        _ ->
            Html.text ""
