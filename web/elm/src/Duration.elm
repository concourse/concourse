module Duration exposing (Duration, between, format)

import Time


type alias Duration =
    Int


between : Time.Posix -> Time.Posix -> Duration
between a b =
    Time.posixToMillis b - Time.posixToMillis a


format : Duration -> String
format duration =
    let
        seconds =
            duration // 1000

        remainingSeconds =
            remainderBy 60 seconds

        minutes =
            seconds // 60

        remainingMinutes =
            remainderBy 60 minutes

        hours =
            minutes // 60

        remainingHours =
            remainderBy 24 hours

        days =
            hours // 24
    in
    case ( ( days, remainingHours ), remainingMinutes, remainingSeconds ) of
        ( ( 0, 0 ), 0, s ) ->
            String.fromInt s ++ "s"

        ( ( 0, 0 ), m, s ) ->
            String.fromInt m ++ "m " ++ String.fromInt s ++ "s"

        ( ( 0, h ), m, _ ) ->
            String.fromInt h ++ "h " ++ String.fromInt m ++ "m"

        ( ( d, h ), _, _ ) ->
            String.fromInt d ++ "d " ++ String.fromInt h ++ "h"
