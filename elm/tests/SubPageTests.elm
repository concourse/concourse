module SubPageTests exposing (..)

import Autoscroll
import Build
import Concourse.Pagination exposing (Paginated)
import Dict exposing (Dict)
import Expect
import Http
import Pipeline
import Test exposing (..)
import SubPage exposing (..)
import Job exposing (..)
import RemoteData exposing (WebData)
import Resource


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

                someJobInfo =
                    { jobName = "some-job"
                    , pipelineName = "some-pipeline"
                    , teamName = "some-team"
                    }

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
                            msg : Job.Msg
                            msg =
                                (Job.JobFetched <| Err <| Http.BadStatus notFoundStatus)

                            model : Job.Model
                            model =
                                { ports = { title = (\_ -> Cmd.none) }
                                , jobIdentifier = someJobInfo
                                , job = RemoteData.NotAsked
                                , pausedChanging = False
                                , buildsWithResources = { content = [], pagination = { previousPage = Nothing, nextPage = Nothing } }
                                , currentPage = Nothing
                                , now = 0
                                , csrfToken = ""
                                }
                        in
                            Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (JobMsg msg) (JobModel model)
                , test "Resource not found" <|
                    \_ ->
                        let
                            msg : Resource.Msg
                            msg =
                                (Resource.ResourceFetched <| Err <| Http.BadStatus notFoundStatus)

                            model : Resource.Model
                            model =
                                { ports = { title = (\_ -> Cmd.none) }
                                , resourceIdentifier = { teamName = "", pipelineName = "", resourceName = "" }
                                , resource = RemoteData.Success { name = "", paused = False, failingToCheck = False, checkError = "" }
                                , pausedChanging = Resource.Stable
                                , versionedResources = { content = [], pagination = { previousPage = Nothing, nextPage = Nothing } }
                                , currentPage = Nothing
                                , versionedUIStates = Dict.empty
                                , csrfToken = ""
                                }
                        in
                            Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (ResourceMsg msg) (ResourceModel model)
                , test "Build not found" <|
                    \_ ->
                        let
                            msg : Build.Msg
                            msg =
                                (Build.BuildFetched 1 <| Err <| Http.BadStatus notFoundStatus)

                            subModel : Build.Model
                            subModel =
                                { ports = { title = (\_ -> Cmd.none) }
                                , now = Nothing
                                , job = Nothing
                                , history = []
                                , currentBuild = RemoteData.NotAsked
                                , browsingIndex = 0
                                , autoScroll = False
                                , csrfToken = ""
                                }

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
                                (Pipeline.PipelineFetched <| Err <| Http.BadStatus notFoundStatus)

                            pipelineLocator =
                                { teamName = ""
                                , pipelineName = ""
                                }

                            model : Pipeline.Model
                            model =
                                { ports = { title = (\_ -> Cmd.none), render = (\( _, _ ) -> Cmd.none) }
                                , concourseVersion = ""
                                , turbulenceImgSrc = ""
                                , pipelineLocator = pipelineLocator
                                , pipeline = RemoteData.NotAsked
                                , fetchedJobs = Nothing
                                , fetchedResources = Nothing
                                , renderedJobs = Nothing
                                , renderedResources = Nothing
                                , experiencingTurbulence = False
                                , selectedGroups = []
                                , hideLegend = False
                                , hideLegendCounter = 0
                                }
                        in
                            Expect.equal (NotFoundModel { notFoundImgSrc = "notfound.svg" }) <| Tuple.first <| SubPage.update turbulenceAsset notfoundAsset csrfToken (PipelineMsg msg) (PipelineModel model)
                ]
        ]
