module SubPageTests exposing (all)

import Autoscroll
import Build
import Build.Msgs
import Dict exposing (Dict)
import Expect
import Http
import Job exposing (..)
import Pipeline
import QueryString
import Resource
import Resource.Msgs
import Routes
import SubPage exposing (..)
import Test exposing (..)


all : Test
all =
    describe "SubPage"
        [ describe "not found" <|
            let
                turbulenceAsset =
                    ""

                notfoundAsset =
                    "notfound.svg"

                csrfToken =
                    ""

                notFoundStatus : Http.Response String
                notFoundStatus =
                    { url = ""
                    , status = { code = 404, message = "" }
                    , headers = Dict.empty
                    , body = ""
                    }
            in
            [ test "JobNotFound" <|
                \_ ->
                    let
                        msg =
                            Job.JobFetched <| Err <| Http.BadStatus notFoundStatus

                        ( model, _ ) =
                            Job.init
                                { title = \_ -> Cmd.none }
                                { jobName = "some-job"
                                , teamName = "some-team"
                                , pipelineName = "some-pipeline"
                                , paging = Nothing
                                , csrfToken = csrfToken
                                }
                    in
                    Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (JobMsg msg) (JobModel model)
            , test "Resource not found" <|
                \_ ->
                    let
                        msg =
                            Resource.Msgs.ResourceFetched <| Err <| Http.BadStatus notFoundStatus

                        ( model, _ ) =
                            Resource.init
                                { teamName = ""
                                , pipelineName = ""
                                , resourceName = ""
                                , paging = Nothing
                                , csrfToken = csrfToken
                                }
                    in
                    Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (ResourceMsg msg) (ResourceModel model)
            , test "Build not found" <|
                \_ ->
                    let
                        msg =
                            Build.Msgs.BuildFetched 1 <| Err <| Http.BadStatus notFoundStatus

                        ( subModel, _ ) =
                            Build.init
                                { title = \_ -> Cmd.none }
                                { csrfToken = csrfToken
                                , hash = ""
                                }
                                (Build.BuildPage 1)

                        model =
                            { subModel = subModel
                            , scrollBehaviorFunc = \_ -> Autoscroll.NoScroll
                            }
                    in
                    Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (BuildMsg <| Autoscroll.SubMsg msg) (BuildModel model)
            , test "Pipeline not found" <|
                \_ ->
                    let
                        msg : Pipeline.Msg
                        msg =
                            Pipeline.PipelineFetched <| Err <| Http.BadStatus notFoundStatus

                        pipelineLocator =
                            { teamName = ""
                            , pipelineName = ""
                            }

                        ( model, _ ) =
                            Pipeline.init { title = \_ -> Cmd.none, render = \( _, _ ) -> Cmd.none }
                                { teamName = ""
                                , pipelineName = ""
                                , route =
                                    { logical = Routes.Pipeline "" ""
                                    , queries = QueryString.empty
                                    , page = Nothing
                                    , hash = ""
                                    }
                                , turbulenceImgSrc = turbulenceAsset
                                }
                    in
                    Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (PipelineMsg msg) (PipelineModel model)
            ]
        ]
