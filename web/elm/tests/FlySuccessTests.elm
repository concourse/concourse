module FlySuccessTests exposing (all)

import Application.Application as Application
import Common exposing (defineHoverBehaviour, queryView)
import DashboardTests exposing (iconSelector)
import Expect exposing (Expectation)
import FlySuccess.FlySuccess as FlySuccess
import Html.Attributes as Attr
import Http
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message
import Message.Subscription as Subscription
import Message.TopLevelMessage as Msgs
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector
    exposing
        ( attribute
        , containing
        , id
        , style
        , tag
        , text
        )
import Url


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
