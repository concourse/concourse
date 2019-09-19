module FlySuccess.Models exposing
    ( ButtonState(..)
    , Model
    , TokenTransfer
    , TransferFailure(..)
    , hover
    , isClicked
    , isPending
    )

import Login.Login as Login
import RemoteData


type alias Model =
    Login.Model
        { copyTokenButtonState : ButtonState
        , sendTokenButtonState : ButtonState
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
    = NetworkTrouble
    | BlockedByBrowser
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
