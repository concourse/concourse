module MainTests exposing (all)

import Expect
import Main
import Test exposing (..)


all : Test
all =
    test "Main compiles" <|
        \_ -> Expect.equal (2 + 2) 4
