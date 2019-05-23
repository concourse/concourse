module TopologicalSortTests exposing (all)

import Dashboard.DashboardPreview exposing (groupByRank)
import Expect
import Test exposing (..)


all : Test
all =
    describe "DashboardPreview.groupByRank"
        [ test "empty case" <|
            \_ ->
                []
                    |> groupByRank
                    |> Expect.equal []
        , test "singleton" <|
            \_ ->
                [ { name = "a", inputs = [] } ]
                    |> groupByRank
                    |> Expect.equal [ [ { name = "a", inputs = [] } ] ]
        , test "looping singleton" <|
            \_ ->
                [ { name = "a", inputs = [ { passed = [ "a" ] } ] } ]
                    |> groupByRank
                    |> Expect.equal [ [ { name = "a", inputs = [ { passed = [ "a" ] } ] } ] ]
        , test "very simple DAG" <|
            \_ ->
                [ { name = "c", inputs = [ { passed = [ "b" ] } ] }
                , { name = "b", inputs = [ { passed = [ "a" ] } ] }
                , { name = "a", inputs = [ { passed = [] } ] }
                ]
                    |> groupByRank
                    |> Expect.equal
                        [ [ { name = "a", inputs = [ { passed = [] } ] } ]
                        , [ { name = "b", inputs = [ { passed = [ "a" ] } ] } ]
                        , [ { name = "c", inputs = [ { passed = [ "b" ] } ] } ]
                        ]
        , test "simple DAG" <|
            \_ ->
                [ { name = "a", inputs = [ { passed = [ "b", "c" ] } ] }
                , { name = "b", inputs = [ { passed = [ "c" ] } ] }
                , { name = "c", inputs = [ { passed = [ "d" ] } ] }
                , { name = "d", inputs = [ { passed = [ "e", "f" ] } ] }
                , { name = "e", inputs = [ { passed = [] } ] }
                , { name = "f", inputs = [ { passed = [] } ] }
                ]
                    |> groupByRank
                    |> Expect.equal
                        [ [ { name = "e", inputs = [ { passed = [] } ] }
                          , { name = "f", inputs = [ { passed = [] } ] }
                          ]
                        , [ { name = "d", inputs = [ { passed = [ "e", "f" ] } ] } ]
                        , [ { name = "c", inputs = [ { passed = [ "d" ] } ] } ]
                        , [ { name = "b", inputs = [ { passed = [ "c" ] } ] } ]
                        , [ { name = "a", inputs = [ { passed = [ "b", "c" ] } ] } ]
                        ]
        , test "very flat DAG" <|
            \_ ->
                [ { name = "a", inputs = [ { passed = [ "b", "c", "d", "e" ] } ] }
                , { name = "b", inputs = [ { passed = [ "f" ] } ] }
                , { name = "c", inputs = [ { passed = [ "f" ] } ] }
                , { name = "d", inputs = [ { passed = [ "e", "f" ] } ] }
                , { name = "e", inputs = [ { passed = [] } ] }
                , { name = "f", inputs = [ { passed = [] } ] }
                ]
                    |> groupByRank
                    |> Expect.equal
                        [ [ { name = "e", inputs = [ { passed = [] } ] }
                          , { name = "f", inputs = [ { passed = [] } ] }
                          ]
                        , [ { name = "b", inputs = [ { passed = [ "f" ] } ] }
                          , { name = "c", inputs = [ { passed = [ "f" ] } ] }
                          , { name = "d", inputs = [ { passed = [ "e", "f" ] } ] }
                          ]
                        , [ { name = "a", inputs = [ { passed = [ "b", "c", "d", "e" ] } ] } ]
                        ]
        , test "two cycles and a bridge" <|
            \_ ->
                [ { name = "a", inputs = [ { passed = [ "b" ] } ] }
                , { name = "b", inputs = [ { passed = [ "a" ] } ] }
                , { name = "c", inputs = [ { passed = [ "b", "d" ] } ] }
                , { name = "d", inputs = [ { passed = [ "c" ] } ] }
                ]
                    |> groupByRank
                    |> Expect.equal
                        [ [ { name = "a", inputs = [ { passed = [ "b" ] } ] } ]
                        , [ { name = "b", inputs = [ { passed = [ "a" ] } ] } ]
                        , [ { name = "c", inputs = [ { passed = [ "b", "d" ] } ] } ]
                        , [ { name = "d", inputs = [ { passed = [ "c" ] } ] } ]
                        ]
        , test "multi-looped cyclic graph" <|
            \_ ->
                [ { name = "a", inputs = [ { passed = [ "a", "b" ] } ] }
                , { name = "b", inputs = [ { passed = [ "a", "c" ] } ] }
                , { name = "c", inputs = [ { passed = [ "a", "b", "d" ] } ] }
                , { name = "d", inputs = [ { passed = [] } ] }
                ]
                    |> groupByRank
                    |> Expect.equal
                        [ [ { name = "a", inputs = [ { passed = [ "a", "b" ] } ] }
                          , { name = "d", inputs = [ { passed = [] } ] }
                          ]
                        , [ { name = "b", inputs = [ { passed = [ "a", "c" ] } ] } ]
                        , [ { name = "c", inputs = [ { passed = [ "a", "b", "d" ] } ] } ]
                        ]
        , test "large cyclic graph" <|
            \_ ->
                [ { name = "a", inputs = [ { passed = [ "b" ] } ] }
                , { name = "b", inputs = [ { passed = [ "c" ] } ] }
                , { name = "c", inputs = [ { passed = [ "a" ] } ] }
                , { name = "d", inputs = [ { passed = [ "b", "c", "e" ] } ] }
                , { name = "e", inputs = [ { passed = [ "d", "f" ] } ] }
                , { name = "f", inputs = [ { passed = [ "c", "g" ] } ] }
                , { name = "g", inputs = [ { passed = [ "f" ] } ] }
                , { name = "h", inputs = [ { passed = [ "e", "g", "h" ] } ] }
                ]
                    |> groupByRank
                    |> Expect.equal
                        [ [ { name = "a", inputs = [ { passed = [ "b" ] } ] } ]
                        , [ { name = "c", inputs = [ { passed = [ "a" ] } ] } ]
                        , [ { name = "b", inputs = [ { passed = [ "c" ] } ] }
                          , { name = "f", inputs = [ { passed = [ "c", "g" ] } ] }
                          ]
                        , [ { name = "d", inputs = [ { passed = [ "b", "c", "e" ] } ] }
                          , { name = "g", inputs = [ { passed = [ "f" ] } ] }
                          ]
                        , [ { name = "e", inputs = [ { passed = [ "d", "f" ] } ] } ]
                        , [ { name = "h", inputs = [ { passed = [ "e", "g", "h" ] } ] } ]
                        ]
        ]
