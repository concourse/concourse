module RequestBufferTests exposing (all)

import Common
import Dashboard.RequestBuffer exposing (Buffer(..), handleCallback, handleDelivery)
import Expect
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Subscription exposing (Delivery(..), Interval(..))
import Test exposing (Test, describe, test)
import Time


all : Test
all =
    describe "RequestBuffer"
        [ test "auto refreshes on five-second tick after previous request finishes" <|
            \_ ->
                ( False, [] )
                    |> handleCallback callback [ buffer ]
                    |> handleDelivery
                        (ClockTicked FiveSeconds <| Time.millisToPosix 0)
                        [ buffer ]
                    |> Tuple.second
                    |> Common.contains effect
        , test "doesn't fetch until the first request finishes" <|
            \_ ->
                ( False, [] )
                    |> handleDelivery
                        (ClockTicked FiveSeconds <| Time.millisToPosix 0)
                        [ buffer ]
                    |> Tuple.second
                    |> Common.notContains effect
        , test "doesn't fetch until the last request finishes" <|
            \_ ->
                ( False, [] )
                    |> handleCallback callback [ buffer ]
                    |> handleDelivery
                        (ClockTicked FiveSeconds <| Time.millisToPosix 0)
                        [ buffer ]
                    |> handleDelivery
                        (ClockTicked FiveSeconds <| Time.millisToPosix 0)
                        [ buffer ]
                    |> Tuple.second
                    |> Expect.equal [ effect ]
        ]


callback =
    EmptyCallback


effect =
    FetchUser


buffer : Buffer Bool
buffer =
    Buffer effect
        ((==) callback)
        (always False)
        { get = identity
        , set = always
        }
