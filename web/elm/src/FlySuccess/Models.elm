module FlySuccess.Models exposing
    ( ButtonState(..)
    , Model
    , TokenTransfer(..)
    , hover
    , isClicked
    )

import Login.Login as Login


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


type TokenTransfer
    = Pending
    | Success
    | NetworkTrouble
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
