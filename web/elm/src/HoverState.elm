module HoverState exposing
    ( HoverState(..)
    , TooltipPosition(..)
    , hoveredElement
    , isHovered
    )

import Browser.Dom
import Message.Message exposing (DomID)


type TooltipPosition
    = Top Float Float
    | Bottom Float Float Float


type HoverState
    = NoHover
    | Hovered DomID
    | TooltipPending DomID
    | Tooltip DomID Browser.Dom.Element


hoveredElement : HoverState -> Maybe DomID
hoveredElement hoverState =
    case hoverState of
        NoHover ->
            Nothing

        Hovered d ->
            Just d

        TooltipPending d ->
            Just d

        Tooltip d _ ->
            Just d


isHovered : DomID -> HoverState -> Bool
isHovered domID hoverState =
    case hoveredElement hoverState of
        Nothing ->
            False

        Just d ->
            d == domID
