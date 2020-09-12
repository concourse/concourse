module SubPageTests exposing (all)

import Application.Application as Application
import Common
import Data
import Dict exposing (Dict)
import Expect
import Http
import Message.Callback exposing (Callback(..))
import NotFound.Model
import Routes
import SubPage.SubPage exposing (..)
import Test exposing (..)
import Url


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
        ]


notFound : Routes.Route -> NotFound.Model.Model
notFound route =
    { notFoundImgSrc = "notfound.svg"
    , isUserMenuExpanded = False
    , route = route
    }
