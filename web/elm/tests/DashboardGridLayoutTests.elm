module DashboardGridLayoutTests exposing (all)

import Dashboard.Grid.Layout exposing (cardSize, layout)
import Expect
import Test exposing (Test, describe, test, todo)


all : Test
all =
    describe "dashboard grid layout"
        [ describe "card size"
            [ test "is wide when pipeline has more than 12 layers of jobs" <|
                \_ ->
                    cardSize ( 13, 1 )
                        |> Tuple.first
                        |> Expect.equal 2
            , test "is super wide when pipeline has more than 24 layers of jobs" <|
                \_ ->
                    cardSize ( 25, 1 )
                        |> Tuple.first
                        |> Expect.equal 3
            , test "is narrow when pipeline has no more than 12 layers of jobs" <|
                \_ ->
                    cardSize ( 12, 1 )
                        |> Tuple.first
                        |> Expect.equal 1
            , test "is tall when pipeline is more than 12 jobs tall" <|
                \_ ->
                    cardSize ( 1, 13 )
                        |> Tuple.second
                        |> Expect.equal 2
            , test "is short when pipeline is very few jobs tall" <|
                \_ ->
                    cardSize ( 1, 3 )
                        |> Tuple.second
                        |> Expect.equal 1
            ]
        , describe "layout"
            [ test "unit cards fill available columns" <|
                \_ ->
                    layout 2 [ ( 1, 1 ), ( 1, 1 ) ]
                        |> Expect.equal
                            [ { spannedColumns = 1
                              , spannedRows = 1
                              , column = 1
                              , row = 1
                              }
                            , { spannedColumns = 1
                              , spannedRows = 1
                              , column = 2
                              , row = 1
                              }
                            ]
            , test "unit cards wrap rows" <|
                \_ ->
                    layout 1 [ ( 1, 1 ), ( 1, 1 ) ]
                        |> Expect.equal
                            [ { spannedColumns = 1
                              , spannedRows = 1
                              , column = 1
                              , row = 1
                              }
                            , { spannedColumns = 1
                              , spannedRows = 1
                              , column = 1
                              , row = 2
                              }
                            ]
            , test "wide cards take up two columns" <|
                \_ ->
                    layout 2 [ ( 2, 1 ), ( 1, 1 ) ]
                        |> Expect.equal
                            [ { spannedColumns = 2
                              , spannedRows = 1
                              , column = 1
                              , row = 1
                              }
                            , { spannedColumns = 1
                              , spannedRows = 1
                              , column = 1
                              , row = 2
                              }
                            ]
            , test "wide cards break rows" <|
                \_ ->
                    layout 2 [ ( 1, 1 ), ( 2, 1 ) ]
                        |> Expect.equal
                            [ { spannedColumns = 1
                              , spannedRows = 1
                              , column = 1
                              , row = 1
                              }
                            , { spannedColumns = 2
                              , spannedRows = 1
                              , column = 1
                              , row = 2
                              }
                            ]
            , test "overflowing cards don't break rows" <|
                \_ ->
                    layout 1 [ ( 2, 1 ) ]
                        |> Expect.equal
                            [ { spannedColumns = 2
                              , spannedRows = 1
                              , column = 1
                              , row = 1
                              }
                            ]
            , test "tall cards take up multiple rows" <|
                \_ ->
                    layout 1 [ ( 1, 2 ), ( 1, 1 ) ]
                        |> Expect.equal
                            [ { spannedColumns = 1
                              , spannedRows = 2
                              , column = 1
                              , row = 1
                              }
                            , { spannedColumns = 1
                              , spannedRows = 1
                              , column = 1
                              , row = 3
                              }
                            ]
            , test "the spannedRows of a row is the spannedRows of the tallest card in the row" <|
                \_ ->
                    layout 2 [ ( 1, 2 ), ( 1, 1 ), ( 1, 1 ) ]
                        |> Expect.equal
                            [ { spannedColumns = 1
                              , spannedRows = 2
                              , column = 1
                              , row = 1
                              }
                            , { spannedColumns = 1
                              , spannedRows = 1
                              , column = 2
                              , row = 1
                              }
                            , { spannedColumns = 1
                              , spannedRows = 1
                              , column = 1
                              , row = 3
                              }
                            ]
            ]
        ]
