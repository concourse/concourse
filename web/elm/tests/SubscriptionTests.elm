module SubscriptionTests exposing (all)

import Expect
import Message.Subscription as Subscription
import Test exposing (Test, describe, test)


all : Test
all =
    describe "decodeHttpResponse"
        [ test "'timeout' means the request timed out" <|
            \_ ->
                "timeout"
                    |> Subscription.decodeHttpResponse
                    |> Expect.equal Subscription.Timeout
        , test "'networkError' means there was a problem with the network" <|
            \_ ->
                "networkError"
                    |> Subscription.decodeHttpResponse
                    |> Expect.equal Subscription.NetworkError
        , test "'browserError' means the browser blocked the request" <|
            \_ ->
                "browserError"
                    |> Subscription.decodeHttpResponse
                    |> Expect.equal Subscription.BrowserError
        , test "'success' means success" <|
            \_ ->
                "success"
                    |> Subscription.decodeHttpResponse
                    |> Expect.equal Subscription.Success
        , test "anything else means success" <|
            \_ ->
                "banana"
                    |> Subscription.decodeHttpResponse
                    |> Expect.equal Subscription.Success
        ]
