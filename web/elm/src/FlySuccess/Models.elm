module FlySuccess.Models exposing
    ( ButtonState(..)
    , Model
    , TokenTransfer
    , TransferFailure(..)
    , hover
    , isClicked
    , isPending
    )

import Http
import Login.Login as Login
import RemoteData


type alias Model =
    Login.Model
        { buttonState : ButtonState
        , authToken : String
        , tokenTransfer : TokenTransfer
        , flyPort : Maybe Int
        }


type ButtonState
    = Unhovered
    | Hovered
    | Clicked


type alias TokenTransfer =
    RemoteData.RemoteData TransferFailure ()


type TransferFailure
    = NetworkTrouble String
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
