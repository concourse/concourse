module RoutesTests exposing (all)

import Expect
import Routes
import Test exposing (Test, test)
import Url


all : Test
all =
    test "parses dashboard search query" <|
        \_ ->
            Routes.parsePath
                { protocol = Url.Http
                , host = ""
                , port_ = Nothing
                , path = "/"
                , query = Just "search=asdf"
                , fragment = Nothing
                }
                |> Expect.equal
                    (Just (Routes.Dashboard (Routes.Normal (Just "asdf"))))
