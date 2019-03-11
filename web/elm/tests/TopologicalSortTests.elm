module TopologicalSortTests exposing (all)

import Expect
import Test exposing (..)
import TopologicalSort exposing (flattenToLayers, tsort)


all : Test
all =
    describe "tsort utilities"
        [ describe "topological sort"
            [ test "empty case" <|
                \_ ->
                    []
                        |> tsort
                        |> Expect.equal []
            , test "singleton" <|
                \_ ->
                    [ ( 'a', [] ) ]
                        |> tsort
                        |> Expect.equal [ [ 'a' ] ]
            , test "looping singleton" <|
                \_ ->
                    [ ( 'a', [ 'a' ] ) ]
                        |> tsort
                        |> Expect.equal [ [ 'a' ] ]
            , test "simple DAG" <|
                \_ ->
                    [ ( 'a', [ 'b', 'c' ] ), ( 'b', [ 'c' ] ), ( 'c', [ 'd' ] ), ( 'd', [ 'e', 'f' ] ), ( 'e', [] ), ( 'f', [] ) ]
                        |> tsort
                        |> Expect.equal [ [ 'f' ], [ 'e' ], [ 'd' ], [ 'c' ], [ 'b' ], [ 'a' ] ]
            , test "very flat DAG" <|
                \_ ->
                    [ ( 'a', [ 'b', 'c', 'd', 'e' ] ), ( 'b', [ 'f' ] ), ( 'c', [ 'f' ] ), ( 'd', [ 'e', 'f' ] ), ( 'e', [] ), ( 'f', [] ) ]
                        |> tsort
                        |> Expect.equal [ [ 'f' ], [ 'e' ], [ 'd' ], [ 'c' ], [ 'b' ], [ 'a' ] ]
            , test "simple cyclic graph" <|
                \_ ->
                    [ ( 'a', [ 'b' ] ), ( 'b', [ 'a', 'c' ] ), ( 'c', [ 'd' ] ), ( 'd', [ 'c' ] ) ]
                        |> tsort
                        |> Expect.equal [ [ 'c', 'd' ], [ 'a', 'b' ] ]
            , test "multi-looped cyclic graph" <|
                \_ ->
                    [ ( 'a', [ 'a', 'b' ] ), ( 'b', [ 'a', 'c' ] ), ( 'c', [ 'a', 'b', 'd' ] ), ( 'd', [] ) ]
                        |> tsort
                        |> Expect.equal [ [ 'd' ], [ 'a', 'b', 'c' ] ]

            -- this one is the example gif on wikipedia ;)
            , test "large cyclic graph" <|
                \_ ->
                    [ ( 'a', [ 'b' ] ), ( 'b', [ 'c' ] ), ( 'c', [ 'a' ] ), ( 'd', [ 'b', 'c', 'e' ] ), ( 'e', [ 'd', 'f' ] ), ( 'f', [ 'c', 'g' ] ), ( 'g', [ 'f' ] ), ( 'h', [ 'e', 'g', 'h' ] ) ]
                        |> tsort
                        |> Expect.equal [ [ 'b', 'a', 'c' ], [ 'f', 'g' ], [ 'd', 'e' ], [ 'h' ] ]
            ]
        , describe "flatten to layers"
            [ test "empty case" <|
                \_ ->
                    []
                        |> flattenToLayers
                        |> Expect.equal []
            , test "singleton" <|
                \_ ->
                    [ ( 'a', [] ) ]
                        |> flattenToLayers
                        |> Expect.equal [ [ 'a' ] ]
            , test "looping singleton" <|
                \_ ->
                    [ ( 'a', [ 'a' ] ) ]
                        |> flattenToLayers
                        |> Expect.equal [ [ 'a' ] ]
            , test "simple DAG" <|
                \_ ->
                    [ ( 'a', [ 'b', 'c' ] ), ( 'b', [ 'c' ] ), ( 'c', [ 'd' ] ), ( 'd', [ 'e', 'f' ] ), ( 'e', [] ), ( 'f', [] ) ]
                        |> flattenToLayers
                        |> Expect.equal [ [ 'e', 'f' ], [ 'd' ], [ 'c' ], [ 'b' ], [ 'a' ] ]
            , test "very flat DAG" <|
                \_ ->
                    [ ( 'a', [ 'b', 'c', 'd', 'e' ] ), ( 'b', [ 'f' ] ), ( 'c', [ 'f' ] ), ( 'd', [ 'e', 'f' ] ), ( 'e', [] ), ( 'f', [] ) ]
                        |> flattenToLayers
                        |> Expect.equal [ [ 'e', 'f' ], [ 'b', 'c', 'd' ], [ 'a' ] ]
            , test "multi-looped cyclic graph" <|
                \_ ->
                    [ ( 'a', [ 'a', 'b' ] ), ( 'b', [ 'a', 'c' ] ), ( 'c', [ 'a', 'b', 'd' ] ), ( 'd', [] ) ]
                        |> flattenToLayers
                        |> Expect.equal [ [ 'd' ], [ 'a', 'b', 'c' ] ]
            , test "large cyclic graph" <|
                \_ ->
                    [ ( 'a', [ 'b' ] ), ( 'b', [ 'c' ] ), ( 'c', [ 'a' ] ), ( 'd', [ 'b', 'c', 'e' ] ), ( 'e', [ 'd', 'f' ] ), ( 'f', [ 'c', 'g' ] ), ( 'g', [ 'f' ] ), ( 'h', [ 'e', 'g', 'h' ] ) ]
                        |> flattenToLayers
                        |> Expect.equal [ [ 'a', 'b', 'c' ], [ 'f', 'g' ], [ 'd', 'e' ], [ 'h' ] ]
            ]
        ]
