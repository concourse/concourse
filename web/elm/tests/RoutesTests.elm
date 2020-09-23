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
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.Normal "asdf sd" Nothing
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                            )
                        )
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
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.Normal "" Nothing
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                            )
                        )
        , test "parses dashboard with instance group" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Just "team=main&group=my-group"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.Normal "" <| Just { teamName = "main", name = "my-group" }
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                            )
                        )
        , test "parses dashboard with search and instance group" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Just "search=hello+world&team=main&group=my-group"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.Normal "hello world" <| Just { teamName = "main", name = "my-group" }
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                            )
                        )
        , test "parses dashboard instance group respecting space" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Just "team=main+team&group=my+group"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.Normal "" <| Just { teamName = "main team", name = "my group" }
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                            )
                        )
        , test "parses dashboard with incomplete instance group" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Just "team=main"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.Normal "" Nothing
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                            )
                        )
        , test "parses dashboard with 'all' view" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Just "view=all"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.Normal "" Nothing
                                , dashboardView = Routes.ViewAllPipelines
                                }
                            )
                        )
        , test "parses dashboard with unknown view defaults to non archived only" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Just "view=blah"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.Normal "" Nothing
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                            )
                        )
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
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.HighDensity
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                            )
                        )
        , test "dashboard hd view ignores search and instance group query params" <|
            \_ ->
                Routes.parsePath
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/hd"
                    , query = Just "search=abc&team=def&group=ghi"
                    , fragment = Nothing
                    }
                    |> Expect.equal
                        (Just
                            (Routes.Dashboard
                                { searchType = Routes.HighDensity
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                            )
                        )
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
        , test "toString serializes instance group on dashboard" <|
            \_ ->
                ("http://example.com"
                    ++ Routes.toString
                        (Routes.Dashboard
                            { searchType = Routes.Normal "" <| Just { teamName = "team", name = "group" }
                            , dashboardView = Routes.ViewNonArchivedPipelines
                            }
                        )
                )
                    |> Url.fromString
                    |> Maybe.andThen Routes.parsePath
                    |> Expect.equal
                        (Just <|
                            Routes.Dashboard
                                { searchType = Routes.Normal "" <| Just { teamName = "team", name = "group" }
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                        )
        , test "toString serializes 'all' dashboard view" <|
            \_ ->
                ("http://example.com"
                    ++ Routes.toString
                        (Routes.Dashboard
                            { searchType = Routes.Normal "hello world" Nothing
                            , dashboardView = Routes.ViewAllPipelines
                            }
                        )
                )
                    |> Url.fromString
                    |> Maybe.andThen Routes.parsePath
                    |> Expect.equal
                        (Just <|
                            Routes.Dashboard
                                { searchType = Routes.Normal "hello world" Nothing
                                , dashboardView = Routes.ViewAllPipelines
                                }
                        )
        , test "toString doesn't serialize 'non_archived' dashboard view" <|
            \_ ->
                Routes.toString
                    (Routes.Dashboard
                        { searchType = Routes.Normal "" Nothing
                        , dashboardView = Routes.ViewNonArchivedPipelines
                        }
                    )
                    |> Expect.equal "/"
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
