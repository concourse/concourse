module SubPageTests exposing (all)

import Application.Application as Application
import Common
import Data
import Dict exposing (Dict)
import Expect
import Http
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Subscription exposing (Delivery(..))
import NotFound.Model
import Routes
import SubPage.SubPage exposing (..)
import Test exposing (..)
import Url


all : Test
all =
    describe "SubPage" <|
        let
            init : String -> () -> Application.Model
            init path _ =
                Common.init path
        in
        [ describe "not found"
            [ test "JobNotFound" <|
                init "/teams/t/pipelines/p/jobs/j"
                    >> Application.handleCallback (JobFetched Data.httpNotFound)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel
                            (notFound
                                (Routes.Job
                                    { id = Data.shortJobId
                                    , page = Nothing
                                    }
                                )
                            )
                        )
            , test "Resource not found" <|
                init "/teams/t/pipelines/p/resources/r"
                    >> Application.handleCallback
                        (ResourceFetched Data.httpNotFound)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel
                            (notFound
                                (Routes.Resource
                                    { id = Data.shortResourceId
                                    , page = Nothing
                                    }
                                )
                            )
                        )
            , test "Build not found" <|
                init "/builds/1"
                    >> Application.handleCallback (BuildFetched Data.httpNotFound)
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
                        (PipelineFetched Data.httpNotFound)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel
                            (notFound
                                (Routes.Pipeline
                                    { id = Data.shortPipelineId
                                    , groups = []
                                    }
                                )
                            )
                        )
            ]
        , describe "close event streams on navigation"
            [ test "close BuildEventStream from Build page to dashboard" <|
                init "/teams/t/pipelines/p/jobs/j/builds/1"
                    >> Application.handleDelivery
                        (RouteChanged <|
                            Routes.Dashboard <|
                                Routes.Normal Nothing
                        )
                    >> Tuple.second
                    >> Common.contains CloseBuildEventStream
            , test "close BuildEventStream from Build page to different Build page" <|
                init "/teams/t/pipelines/p/jobs/j/builds/1"
                    >> Application.handleDelivery
                        (RouteChanged <|
                            Routes.Build
                                { id = { teamName = "t", pipelineName = "b", jobName = "j", buildName = "2" }
                                , highlight = Routes.HighlightNothing
                                }
                        )
                    >> Tuple.second
                    >> Common.contains CloseBuildEventStream
            , test "don't close BuildEventStream when navigating to same Build" <|
                init "/teams/t/pipelines/p/jobs/j/builds/1"
                    >> Application.handleDelivery
                        (RouteChanged <|
                            Routes.Build
                                { id = { teamName = "t", pipelineName = "p", jobName = "j", buildName = "1" }
                                , highlight = Routes.HighlightLine "step" 1
                                }
                        )
                    >> Tuple.second
                    >> Common.notContains CloseBuildEventStream
            ]
        ]


notFound : Routes.Route -> NotFound.Model.Model
notFound route =
    { notFoundImgSrc = "notfound.svg"
    , isUserMenuExpanded = False
    , route = route
    }
