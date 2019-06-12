module HoverState exposing (HoverState(..), isHovered)

import Message.Message exposing (DomID)


type HoverState
    = NoHover
    | Hovered DomID
    | Tooltip DomID


isHovered : DomID -> HoverState -> Bool
isHovered domID hoverState =
    case hoverState of
        NoHover ->
            False

        Hovered d ->
            d == domID

        Tooltip d ->
            d == domID
