module Pinned exposing (..)


type Pinned a b
    = Off
    | TurningOn b
    | On a
    | TurningOff a
    | Static a


type PinState
    = Enabled
    | Pinned
    | Disabled
    | Pending


startPinningTo : b -> Pinned a b -> Pinned a b
startPinningTo destination pinned =
    case pinned of
        Off ->
            TurningOn destination

        x ->
            x


finishPinning : (b -> Maybe a) -> Pinned a b -> Pinned a b
finishPinning lookup pinned =
    case pinned of
        TurningOn b ->
            lookup b |> Maybe.map On |> Maybe.withDefault Off

        x ->
            x


startUnpinning : Pinned a b -> Pinned a b
startUnpinning pinned =
    case pinned of
        On p ->
            TurningOff p

        x ->
            x


quitUnpinning : Pinned a b -> Pinned a b
quitUnpinning pinned =
    case pinned of
        TurningOff p ->
            On p

        x ->
            x


stable : Pinned a b -> Maybe a
stable pinnable =
    case pinnable of
        Static p ->
            Just p

        On p ->
            Just p

        _ ->
            Nothing


pinState : a -> b -> Pinned a b -> PinState
pinState pinnable index pinned =
    case pinned of
        Static p ->
            if p == pinnable then
                Pinned
            else
                Disabled

        Off ->
            Enabled

        TurningOn destination ->
            if destination == index then
                Pending
            else
                Disabled

        On p ->
            if p == pinnable then
                Pinned
            else
                Disabled

        TurningOff p ->
            if p == pinnable then
                Pending
            else
                Disabled
