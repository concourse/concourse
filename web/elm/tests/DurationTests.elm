module DurationTests exposing (all, start)

import Duration exposing (Duration)
import Expect exposing (..)
import Test exposing (..)
import Time


start : Int
start =
    10 * 1000


second : Int
second =
    1 * 1000


minute : Int
minute =
    60 * 1000


hour : Int
hour =
    60 * 60 * 1000


all : Test
all =
    describe "Duration"
        [ describe "formatting"
            [ test "seconds difference" <|
                \_ ->
                    Expect.equal
                        "1s"
                        (Duration.format <| Duration.between (Time.millisToPosix start) (Time.millisToPosix (start + second)))
            , test "minutes difference" <|
                \_ ->
                    Expect.equal
                        "1m 2s"
                        (Duration.format <| Duration.between (Time.millisToPosix start) (Time.millisToPosix (start + minute + (2 * second))))
            , test "hours difference" <|
                \_ ->
                    Expect.equal
                        "1h 2m"
                        (Duration.format <| Duration.between (Time.millisToPosix start) (Time.millisToPosix (start + hour + (2 * minute) + (3 * second))))
            , test "days difference" <|
                \_ ->
                    Expect.equal
                        "1d 2h"
                        (Duration.format <| Duration.between (Time.millisToPosix start) (Time.millisToPosix (start + (26 * hour) + (3 * minute) + (4 * second))))
            ]
        ]
