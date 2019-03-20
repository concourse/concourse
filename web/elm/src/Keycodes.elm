module Keycodes exposing (enter, isControlModifier, shift)

import Keyboard


ctrl : Keyboard.KeyCode
ctrl =
    17


leftCommand : Keyboard.KeyCode
leftCommand =
    91


rightCommand : Keyboard.KeyCode
rightCommand =
    93


enter : Keyboard.KeyCode
enter =
    13


shift : Keyboard.KeyCode
shift =
    16


isControlModifier : Keyboard.KeyCode -> Bool
isControlModifier keycode =
    keycode == ctrl || keycode == leftCommand || keycode == rightCommand
