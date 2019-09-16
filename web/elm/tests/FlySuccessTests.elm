module FlySuccessTests exposing (all)

import Common
import Expect
import FlySuccess.FlySuccess as FlySuccess
import Message.Effects as Effects
import Test exposing (Test, test)


all : Test
all =
    test "does not send token when 'noop' is passed" <|
        \_ ->
            { authToken = ""
            , flyPort = Just 1234
            , noop = True
            }
                |> FlySuccess.init
                |> Tuple.second
                |> Common.notContains (Effects.SendTokenToFly "" 1234)
