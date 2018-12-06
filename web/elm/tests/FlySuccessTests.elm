module FlySuccessTests exposing (..)

import Layout
import PipelineTests exposing (init)
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (text)


all : Test
all =
    test "says 'you have successfully logged in' on page load" <|
        \_ ->
            init "/fly_success"
                |> Layout.view
                |> Query.fromHtml
                |> Query.has [ text "you have successfully logged in" ]
