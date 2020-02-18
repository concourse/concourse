module DragTests exposing (all)

import Dashboard.Drag as Drag
import Expect
import Test exposing (Test, describe, test)


all : Test
all =
    describe "drag"
        [ test "forwards" <|
            \_ ->
                Drag.drag 0 2 [ "a", "b" ]
                    |> Expect.equal [ "b", "a" ]
        , test "forwards from the middle" <|
            \_ ->
                Drag.drag 1 4 [ "a", "b", "c" ]
                    |> Expect.equal [ "a", "c", "b" ]
        , test "backwards" <|
            \_ ->
                Drag.drag 1 0 [ "a", "b" ]
                    |> Expect.equal [ "b", "a" ]
        ]
