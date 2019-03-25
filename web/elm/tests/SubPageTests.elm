module SubPageTests exposing (all)

import Application.Application as Application
import Callback exposing (Callback(..))
import Dict exposing (Dict)
import Effects
import Expect
import Http
import NotFound.Model
import RemoteData
import Routes
import ScreenSize
import SubPage.SubPage exposing (..)
import Test exposing (..)
import TopBar.Model


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
                    Application.init
                        { turbulenceImgSrc = ""
                        , notFoundImgSrc = "notfound.svg"
                        , csrfToken = ""
                        , authToken = ""
                        , pipelineRunningKeyframes = ""
                        }
                        { href = ""
                        , host = ""
                        , hostname = ""
                        , protocol = ""
                        , origin = ""
                        , port_ = ""
                        , pathname = path
                        , search = ""
                        , hash = ""
                        , username = ""
                        , password = ""
                        }
                        |> Tuple.first
            in
            [ test "JobNotFound" <|
                init "/teams/t/pipelines/p/jobs/j"
                    >> Application.handleCallback
                        (Effects.SubPage 1)
                        (JobFetched notFoundResult)
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
                        (Effects.SubPage 1)
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
                    >> Application.handleCallback
                        (Effects.SubPage 0)
                        (BuildFetched notFoundResult)
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
                        (Effects.SubPage 1)
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
    , groups = []
    , screenSize = ScreenSize.Desktop
    , dropdown = TopBar.Model.Hidden
    , isPinMenuExpanded = False
    , route = route
    , shiftDown = False
    }
