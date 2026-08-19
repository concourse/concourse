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
        (Json.Decode.field "key" decodeKeyCode)


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
    | QuestionMark
    | ArrowUp
    | ArrowDown
    | Enter
    | Escape


{-| Decodes the DOM `KeyboardEvent.key` field (the character the active
keyboard layout actually produces), not `KeyboardEvent.code` (the physical
key position). This keeps shortcuts working on non-US layouts, where the
character and the physical position diverge.

Letters fold case ("t" and "T" both map to `T`); the Shift distinction is
carried separately by `keyEvent.shiftKey`, which is reliable for a-z on every
Latin layout. Punctuation like "?" must be matched as the character itself,
because the modifier that produces it varies by layout.

-}
decodeKeyCode : Json.Decode.Decoder KeyCode
decodeKeyCode =
    Concourse.customDecoder Json.Decode.string <|
        \key ->
            case key of
                "?" ->
                    Ok QuestionMark

                "/" ->
                    Ok Slash

                "ArrowUp" ->
                    Ok ArrowUp

                "ArrowDown" ->
                    Ok ArrowDown

                "Enter" ->
                    Ok Enter

                "Escape" ->
                    Ok Escape

                other ->
                    case String.toUpper other of
                        "A" ->
                            Ok A

                        "T" ->
                            Ok T

                        "R" ->
                            Ok R

                        "G" ->
                            Ok G

                        "H" ->
                            Ok H

                        "J" ->
                            Ok J

                        "K" ->
                            Ok K

                        "L" ->
                            Ok L

                        "F" ->
                            Ok F

                        _ ->
                            Err <|
                                Json.Decode.Failure "unknown keycode" <|
                                    Json.Encode.string other


hasControlModifier : KeyEvent -> Bool
hasControlModifier keyEvent =
    keyEvent.ctrlKey || keyEvent.metaKey
