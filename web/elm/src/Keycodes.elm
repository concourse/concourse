module Keycodes exposing (KeyCode, enter, isControlModifier, shift)


type alias KeyCode =
    Int


ctrl : KeyCode
ctrl =
    17


leftCommand : KeyCode
leftCommand =
    91


rightCommand : KeyCode
rightCommand =
    93


enter : KeyCode
enter =
    13


shift : KeyCode
shift =
    16


isControlModifier : KeyCode -> Bool
isControlModifier keycode =
    keycode == ctrl || keycode == leftCommand || keycode == rightCommand
