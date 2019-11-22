module SubPageTests exposing (all)

import Application.Application as Application
import Common
import Dict exposing (Dict)
import Expect
import Http
import Message.Callback exposing (Callback(..), Route(..))
import NotFound.Model
import Routes
import SubPage.SubPage exposing (..)
import Test exposing (..)
import Url


notFoundResult : Result Http.Error a
notFoundResult =
    Err <|
        Http.BadStatus
            { url = ""
            , status = { code = 404, message = "" }
            , headers = Dict.empty
            , body = ""
            }


all : Test
all =
    describe "SubPage"
        [ describe "not found" <|
            let
                init : String -> () -> Application.Model
                init path _ =
                    Common.init path
            in
            [ test "JobNotFound" <|
                init "/teams/t/pipelines/p/jobs/j"
                    >> Application.handleCallback
                        (ApiResponse
                            (RouteJob
                                { teamName = "t"
                                , pipelineName = "p"
                                , jobName = "j"
                                }
                            )
                            notFoundResult
                        )
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel
                            (notFound
                                (Routes.Job
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        , jobName = "j"
                                        }
                                    , page = Nothing
                                    }
                                )
                            )
                        )
            , test "Resource not found" <|
                init "/teams/t/pipelines/p/resources/r"
                    >> Application.handleCallback
                        (ResourceFetched notFoundResult)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel
                            (notFound
                                (Routes.Resource
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        , resourceName = "r"
                                        }
                                    , page = Nothing
                                    }
                                )
                            )
                        )
            , test "Build not found" <|
                init "/builds/1"
                    >> Application.handleCallback (BuildFetched notFoundResult)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel
                            (notFound
                                (Routes.OneOffBuild
                                    { id = 1
                                    , highlight = Routes.HighlightNothing
                                    }
                                )
                            )
                        )
            , test "Pipeline not found" <|
                init "/teams/t/pipelines/p"
                    >> Application.handleCallback
                        (PipelineFetched notFoundResult)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel
                            (notFound
                                (Routes.Pipeline
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        }
                                    , groups = []
                                    }
                                )
                            )
                        )
            ]
        ]


notFound : Routes.Route -> NotFound.Model.Model
notFound route =
    { notFoundImgSrc = "notfound.svg"
    , isUserMenuExpanded = False
    , route = route
    }
