module RoutesTests exposing (all)

import Expect
import Routes
import Test exposing (Test, describe, test)
import Url


all : Test
all =
    describe "Routes"
        [ test "parses dashboard search query respecting space" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Just "search=asdf+sd"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just (Routes.Dashboard { searchType = Routes.Normal (Just "asdf sd") }))
        , test "parses dashboard without search" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just (Routes.Dashboard { searchType = Routes.Normal Nothing }))
        , test "parses dashboard in hd view" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/hd"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just (Routes.Dashboard { searchType = Routes.HighDensity }))
        , test "fly success has noop parameter" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/fly_success"
                    , query = Just "fly_port=1234&noop=true"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just <| Routes.FlySuccess True (Just 1234))
        , test "fly noop parameter defaults to False" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/fly_success"
                    , query = Just "fly_port=1234"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just <| Routes.FlySuccess False (Just 1234))
        , test "toString respects noop parameter with a fly port" <|
            \_ ->
                ("http://example.com"
                    ++ Routes.toString (Routes.FlySuccess True (Just 1234))
                )
                    |> Url.fromString
                    |> Maybe.andThen Routes.parsePath
                    |> Expect.equal (Just <| Routes.FlySuccess True (Just 1234))
        , test "toString respects noop parameter without a fly port" <|
            \_ ->
                ("http://example.com"
                    ++ Routes.toString (Routes.FlySuccess True Nothing)
                )
                    |> Url.fromString
                    |> Maybe.andThen Routes.parsePath
                    |> Expect.equal (Just <| Routes.FlySuccess True Nothing)
        ]
