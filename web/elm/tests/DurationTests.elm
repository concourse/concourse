module DurationTests exposing (..)

import ElmTest exposing (..)
import Time exposing (Time)

import Duration exposing (Duration)

start : Time
start = 10 * Time.second

all : Test
all =
  suite "Duration"
    [ suite "formatting"
        [ test "seconds difference" <|
            assertEqual
              "1s"
              (Duration.format <| Duration.between start (start + 1 * Time.second))
        , test "minutes difference" <|
            assertEqual
              "1m 2s"
              (Duration.format <| Duration.between start (start + Time.minute + (2 * Time.second)))
        , test "hours difference" <|
            assertEqual
              "1h 2m"
              (Duration.format <| Duration.between start (start + Time.hour + (2 * Time.minute) + (3 * Time.second)))
        , test "days difference" <|
            assertEqual
              "1d 2h"
              (Duration.format <| Duration.between start (start + (26 * Time.hour) + (3 * Time.minute) + (4 * Time.second)))
        ]
    ]
