module RoutesTests exposing (all)

import Concourse exposing (JsonValue(..))
import Dict
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
                                { searchType = Routes.Normal "asdf sd"
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
                                { searchType = Routes.Normal ""
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
                                { searchType = Routes.Normal ""
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
                                { searchType = Routes.Normal ""
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
        , test "toString serializes 'all' dashboard view" <|
            \_ ->
                ("http://example.com"
                    ++ Routes.toString
                        (Routes.Dashboard
                            { searchType = Routes.Normal "hello world"
                            , dashboardView = Routes.ViewAllPipelines
                            }
                        )
                )
                    |> Url.fromString
                    |> Maybe.andThen Routes.parsePath
                    |> Expect.equal
                        (Just <|
                            Routes.Dashboard
                                { searchType = Routes.Normal "hello world"
                                , dashboardView = Routes.ViewAllPipelines
                                }
                        )
        , test "toString doesn't serialize 'non_archived' dashboard view" <|
            \_ ->
                Routes.toString
                    (Routes.Dashboard
                        { searchType = Routes.Normal ""
                        , dashboardView = Routes.ViewNonArchivedPipelines
                        }
                    )
                    |> Expect.equal "/"
        , test "toString on Pipeline doesn't add empty instance vars" <|
            \_ ->
                Routes.toString
                    (Routes.Pipeline
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , pipelineInstanceVars = Dict.empty
                            }
                        , groups = []
                        }
                    )
                    |> Expect.equal "/teams/team/pipelines/pipeline"
        , test "toString on Pipeline adds instance vars if non-empty" <|
            \_ ->
                Routes.toString
                    (Routes.Pipeline
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , pipelineInstanceVars =
                                Dict.fromList
                                    [ ( "k", JsonString "s" )
                                    , ( "foo"
                                      , JsonObject
                                            [ ( "bar"
                                              , JsonObject
                                                    [ ( "baz.qux", JsonNumber 1 )
                                                    , ( "special_chars", JsonString "/\"'&." )
                                                    ]
                                              )
                                            ]
                                      )
                                    ]
                            }
                        , groups = []
                        }
                    )
                    |> Expect.equal "/teams/team/pipelines/pipeline?vars.foo.bar.%22baz.qux%22=1&vars.foo.bar.special_chars=%22%2F%5C%22'%26.%22&vars.k=%22s%22"
        , test "Pipeline route can be parsed properly" <|
            \_ ->
                ("http://example.com"
                    ++ Routes.toString
                        (Routes.Pipeline
                            { id =
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                , pipelineInstanceVars =
                                    Dict.fromList
                                        [ ( "k1", JsonNumber 1 )
                                        , ( "k2", JsonString "/\"'&." )
                                        ]
                                }
                            , groups = []
                            }
                        )
                )
                    |> Url.fromString
                    |> Maybe.andThen Routes.parsePath
                    |> Expect.equal
                        (Just <|
                            Routes.Pipeline
                                { id =
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    , pipelineInstanceVars =
                                        Dict.fromList
                                            [ ( "k1", JsonNumber 1 )
                                            , ( "k2", JsonString "/\"'&." )
                                            ]
                                    }
                                , groups = []
                                }
                        )
        , test "Pipeline route can be parsed properly given rooted vars" <|
            \_ ->
                "http://example.com/teams/team/pipelines/pipeline?vars=%7B%22foo%22%3A%22bar%22%7D"
                    |> Url.fromString
                    |> Maybe.andThen Routes.parsePath
                    |> Expect.equal
                        (Just <|
                            Routes.Pipeline
                                { id =
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    , pipelineInstanceVars = Dict.fromList [ ( "foo", JsonString "bar" ) ]
                                    }
                                , groups = []
                                }
                        )
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
        , test "resources" <|
            \_ ->
                "http://example.com/teams/team/pipelines/pipeline/resources/resource?filter=version:sha:123abc"
                    |> Url.fromString
                    |> Maybe.andThen Routes.parsePath
                    |> Expect.equal
                        (Just <|
                            Routes.Resource
                                { id =
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    , pipelineInstanceVars = Dict.empty
                                    , resourceName = "resource"
                                    }
                                , page = Nothing
                                , version = Just <| Dict.fromList [ ( "version", "sha:123abc" ) ]
                                , groups = []
                                }
                        )
        ]
