module HoverState exposing (HoverState(..), TooltipPosition, isHovered, tooltip)

import Message.Message exposing (DomID)


type alias TooltipPosition =
    { x : Float
    , y : Float
    }


type HoverState
    = NoHover
    | Hovered DomID
    | Tooltip DomID TooltipPosition


isHovered : DomID -> HoverState -> Bool
isHovered domID hoverState =
    case hoverState of
        NoHover ->
            False

        Hovered d ->
            d == domID

        Tooltip d _ ->
            d == domID


tooltip : DomID -> HoverState -> Maybe TooltipPosition
tooltip domID hoverState =
    case hoverState of
        NoHover ->
            Nothing

        Hovered _ ->
            Nothing

        Tooltip d t ->
            if d == domID then
                Just t

            else
                Nothing
