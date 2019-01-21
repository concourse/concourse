module SubPageTests exposing (all)

import Autoscroll
import Build
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
                        callback =
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
                    SubPage.handleCallback
                        notfoundAsset
                        csrfToken
                        callback
                        (JobModel model)
                        |> Tuple.first
                        |> Expect.equal
                            (NotFoundModel { notFoundImgSrc = "notfound.svg" })
            , test "Resource not found" <|
                \_ ->
                    let
                        callback =
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
                    SubPage.handleCallback
                        notfoundAsset
                        csrfToken
                        callback
                        (ResourceModel model)
                        |> Tuple.first
                        |> Expect.equal
                            (NotFoundModel { notFoundImgSrc = "notfound.svg" })
            , test "Build not found" <|
                \_ ->
                    let
                        callback =
                            BuildFetched 1 <| Err <| Http.BadStatus notFoundStatus

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
                    SubPage.handleCallback
                        notfoundAsset
                        csrfToken
                        callback
                        (BuildModel model)
                        |> Tuple.first
                        |> Expect.equal
                            (NotFoundModel { notFoundImgSrc = "notfound.svg" })
            , test "Pipeline not found" <|
                \_ ->
                    let
                        callback : Effects.Callback
                        callback =
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
                    SubPage.handleCallback
                        notfoundAsset
                        csrfToken
                        callback
                        (PipelineModel model)
                        |> Tuple.first
                        |> Expect.equal
                            (NotFoundModel { notFoundImgSrc = "notfound.svg" })
            ]
        ]
