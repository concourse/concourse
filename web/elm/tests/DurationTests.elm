module DurationTests exposing (..)

import Test exposing (..)
import Expect exposing (..)
import Time exposing (Time)
import Duration exposing (Duration)


start : Time
start =
    10 * Time.second


all : Test
all =
    describe "Duration"
        [ describe "formatting"
            [ test "seconds difference" <|
                \_ ->
                    Expect.equal
                        "1s"
                        (Duration.format <| Duration.between start (start + 1 * Time.second))
            , test "minutes difference" <|
                \_ ->
                    Expect.equal
                        "1m 2s"
                        (Duration.format <| Duration.between start (start + Time.minute + (2 * Time.second)))
            , test "hours difference" <|
                \_ ->
                    Expect.equal
                        "1h 2m"
                        (Duration.format <| Duration.between start (start + Time.hour + (2 * Time.minute) + (3 * Time.second)))
            , test "days difference" <|
                \_ ->
                    Expect.equal
                        "1d 2h"
                        (Duration.format <| Duration.between start (start + (26 * Time.hour) + (3 * Time.minute) + (4 * Time.second)))
            ]
        ]
