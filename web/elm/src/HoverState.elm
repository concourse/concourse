module HoverState exposing
    ( HoverState(..)
    , TooltipPosition(..)
    , isHovered
    )

import Message.Message exposing (DomID)


type TooltipPosition
    = Top Float Float
    | Bottom Float Float Float


type HoverState
    = NoHover
    | Hovered DomID
    | TooltipPending DomID
    | Tooltip DomID TooltipPosition


isHovered : DomID -> HoverState -> Bool
isHovered domID hoverState =
    case hoverState of
        NoHover ->
            False

        Hovered d ->
            d == domID

        TooltipPending d ->
            d == domID

        Tooltip d _ ->
            d == domID
