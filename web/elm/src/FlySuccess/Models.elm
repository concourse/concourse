module FlySuccess.Models exposing
    ( ButtonState(..)
    , TokenTransfer
    , TransferResult
    , hover
    , isClicked
    , isPending
    )


type ButtonState
    = Unhovered
    | Hovered
    | Clicked


type alias TokenTransfer =
    Maybe TransferResult


type alias TransferResult =
    Result () Bool


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
    (==) Nothing
