module Keyboard exposing
    ( KeyCode(..)
    , KeyEvent
    , decodeKeyCode
    , decodeKeyEvent
    , hasControlModifier
    )

import Concourse
import Json.Decode
import Json.Encode


type alias KeyEvent =
    { ctrlKey : Bool
    , shiftKey : Bool
    , metaKey : Bool
    , code : KeyCode
    }


decodeKeyEvent : Json.Decode.Decoder KeyEvent
decodeKeyEvent =
    Json.Decode.map4 KeyEvent
        (Json.Decode.field "ctrlKey" Json.Decode.bool)
        (Json.Decode.field "shiftKey" Json.Decode.bool)
        (Json.Decode.field "metaKey" Json.Decode.bool)
        (Json.Decode.field "code" decodeKeyCode)


type KeyCode
    = A
    | T
    | R
    | G
    | H
    | J
    | K
    | L
    | F
    | Slash
    | ArrowUp
    | ArrowDown
    | Enter
    | Escape


decodeKeyCode : Json.Decode.Decoder KeyCode
decodeKeyCode =
    Concourse.customDecoder Json.Decode.string <|
        \code ->
            case code of
                "KeyA" ->
                    Ok A

                "KeyT" ->
                    Ok T

                "KeyR" ->
                    Ok R

                "KeyG" ->
                    Ok G

                "KeyH" ->
                    Ok H

                "KeyJ" ->
                    Ok J

                "KeyK" ->
                    Ok K

                "KeyL" ->
                    Ok L

                "KeyF" ->
                    Ok F

                "Slash" ->
                    Ok Slash

                "ArrowUp" ->
                    Ok ArrowUp

                "ArrowDown" ->
                    Ok ArrowDown

                "Enter" ->
                    Ok Enter

                "Escape" ->
                    Ok Escape

                unknown ->
                    Err <|
                        Json.Decode.Failure "unknown keycode" <|
                            Json.Encode.string unknown


hasControlModifier : KeyEvent -> Bool
hasControlModifier keyEvent =
    keyEvent.ctrlKey || keyEvent.metaKey
