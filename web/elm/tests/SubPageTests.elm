module SubPageTests exposing (all)

import Autoscroll
import Build
import Build.Msgs
import Dict exposing (Dict)
import Effects exposing (Callback(..))
import Expect
import Http
import Job exposing (..)
import Pipeline
import QueryString
import Resource
import Routes
import SubPage exposing (..)
import SubPage.Msgs exposing (Msg(..))
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
                            Effects.JobFetched <| Err <| Http.BadStatus notFoundStatus

                        ( model, _ ) =
                            Job.init
                                { jobName = "some-job"
                                , teamName = "some-team"
                                , pipelineName = "some-pipeline"
                                , paging = Nothing
                                , csrfToken = csrfToken
                                }
                    in
                    Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (Callback msg) (JobModel model)
            , test "Resource not found" <|
                \_ ->
                    let
                        msg =
                            ResourceFetched <| Err <| Http.BadStatus notFoundStatus

                        ( model, _ ) =
                            Resource.init
                                { teamName = ""
                                , pipelineName = ""
                                , resourceName = ""
                                , paging = Nothing
                                , csrfToken = csrfToken
                                }
                    in
                    Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (Callback msg) (ResourceModel model)
            , test "Build not found" <|
                \_ ->
                    let
                        msg =
                            Build.Msgs.BuildFetched 1 <| Err <| Http.BadStatus notFoundStatus

                        ( subModel, _ ) =
                            Build.init
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
                        msg : Effects.Callback
                        msg =
                            Effects.PipelineFetched <| Err <| Http.BadStatus notFoundStatus

                        pipelineLocator =
                            { teamName = ""
                            , pipelineName = ""
                            }

                        ( model, _ ) =
                            Pipeline.init
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
                    Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (Callback msg) (PipelineModel model)
            ]
        ]
