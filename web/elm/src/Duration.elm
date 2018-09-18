module Duration exposing (Duration, between, format)

import Time exposing (Time)


type alias Duration =
    Float


between : Time -> Time -> Duration
between a b =
    b - a


format : Duration -> String
format duration =
    let
        seconds =
            truncate (duration / 1000)

        remainingSeconds =
            rem seconds 60

        minutes =
            seconds // 60

        remainingMinutes =
            rem minutes 60

        hours =
            minutes // 60

        remainingHours =
            rem hours 24

        days =
            hours // 24
    in
        case ( days, remainingHours, remainingMinutes, remainingSeconds ) of
            ( 0, 0, 0, s ) ->
                toString s ++ "s"

            ( 0, 0, m, s ) ->
                toString m ++ "m " ++ toString s ++ "s"

            ( 0, h, m, _ ) ->
                toString h ++ "h " ++ toString m ++ "m"

            ( d, h, _, _ ) ->
                toString d ++ "d " ++ toString h ++ "h"
