module FetchResultTests exposing (..)

import Expect
import FetchResult exposing (FetchResult(..), changedFrom)
import Test exposing (Test, describe, test)


all : Test
all =
    describe "FetchResult"
        [ describe "changedFrom"
            [ test "both fetched, same value, no change" <|
                \_ ->
                    Fetched 1
                        |> changedFrom (Fetched 1)
                        |> Expect.equal False
            , test "both fetched, diff value, yes change" <|
                \_ ->
                    Fetched 2
                        |> changedFrom (Fetched 1)
                        |> Expect.equal True
            , test "both cached, same value, no change" <|
                \_ ->
                    Cached 1
                        |> changedFrom (Cached 1)
                        |> Expect.equal False
            , test "both cached, diff value, yes change" <|
                \_ ->
                    Cached 2
                        |> changedFrom (Cached 1)
                        |> Expect.equal True
            , test "cached to fetched, same value, no change" <|
                \_ ->
                    Fetched 1
                        |> changedFrom (Cached 1)
                        |> Expect.equal False
            , test "cached to fetched, diff value, yes change" <|
                \_ ->
                    Fetched 2
                        |> changedFrom (Cached 1)
                        |> Expect.equal True
            , test "fetched to cached, diff value, no change" <|
                \_ ->
                    Cached 2
                        |> changedFrom (Fetched 1)
                        |> Expect.equal False
            , test "none to cached, yes change" <|
                \_ ->
                    Cached 1
                        |> changedFrom None
                        |> Expect.equal True
            , test "none to fetched, yes change" <|
                \_ ->
                    Fetched 1
                        |> changedFrom None
                        |> Expect.equal True
            ]
        ]
