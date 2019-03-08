module FlySuccess.Models exposing
    ( ButtonState(..)
    , TokenTransfer
    , TransferFailure(..)
    , hover
    , isClicked
    , isPending
    )

import Http
import RemoteData


type ButtonState
    = Unhovered
    | Hovered
    | Clicked


type alias TokenTransfer =
    RemoteData.RemoteData TransferFailure ()


type TransferFailure
    = NetworkTrouble Http.Error
    | NoFlyPort


hover : Bool -> ButtonState -> ButtonState
hover hovered buttonState =
    case buttonState of
        Clicked ->
            Clicked

        _ ->
            if hovered then
                Hovered

            else
                Unhovered


isClicked : ButtonState -> Bool
isClicked =
    (==) Clicked


isPending : TokenTransfer -> Bool
isPending =
    (==) RemoteData.Loading
