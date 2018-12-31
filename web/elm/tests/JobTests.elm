module JobTests exposing (all)

import Concourse exposing (Build, BuildId, BuildStatus(..), Job)
import Concourse.Pagination exposing (Direction(..))
import DashboardTests exposing (defineHoverBehaviour, iconSelector, middleGrey)
import Date
import Dict
import Expect exposing (..)
import Html.Attributes as Attr
import Http
import Job exposing (Msg(..), update)
import Layout
import RemoteData
import SubPage
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, class, containing, style, text)


all : Test
all =
    describe "Job"
        [ describe "update" <|
            let
                someJobInfo =
                    { jobName = "some-job"
                    , pipelineName = "some-pipeline"
                    , teamName = "some-team"
                    }
            in
            let
                someBuild : Build
                someBuild =
                    { id = 123
                    , name = "45"
                    , job = Just someJobInfo
                    , status = BuildStatusSucceeded
                    , duration =
                        { startedAt = Just (Date.fromTime 0)
                        , finishedAt = Just (Date.fromTime 0)
                        }
                    , reapTime = Just (Date.fromTime 0)
                    }
            in
            let
                someJob : Concourse.Job
                someJob =
                    { name = "some-job"
                    , pipelineName = "some-pipeline"
                    , teamName = "some-team"
                    , pipeline =
                        { pipelineName = "some-pipeline"
                        , teamName = "some-team"
                        }
                    , nextBuild = Nothing
                    , finishedBuild = Just someBuild
                    , transitionBuild = Nothing
                    , paused = False
                    , disableManualTrigger = False
                    , inputs = []
                    , outputs = []
                    , groups = []
                    }

                defaultModel : Job.Model
                defaultModel =
                    Job.init
                        { title = \_ -> Cmd.none }
                        { jobName = "some-job"
                        , teamName = "some-team"
                        , pipelineName = "some-pipeline"
                        , paging = Nothing
                        , csrfToken = ""
                        }
                        |> Tuple.first
            in
            [ test "build header lays out contents horizontally" <|
                \_ ->
                    Layout.init
                        { turbulenceImgSrc = ""
                        , notFoundImgSrc = ""
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
                        , pathname = "/teams/team/pipelines/pipeline/jobs/job"
                        , search = ""
                        , hash = ""
                        , username = ""
                        , password = ""
                        }
                        |> Tuple.first
                        |> Layout.update
                            (Layout.SubMsg 1 <|
                                SubPage.JobMsg <|
                                    Job.JobFetched <|
                                        Ok
                                            { name = "job"
                                            , pipelineName = "pipeline"
                                            , teamName = "team"
                                            , pipeline =
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                            , nextBuild = Nothing
                                            , finishedBuild = Just someBuild
                                            , transitionBuild = Nothing
                                            , paused = False
                                            , disableManualTrigger = False
                                            , inputs = []
                                            , outputs = []
                                            , groups = []
                                            }
                            )
                        |> Tuple.first
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ class "build-header" ]
                        |> Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "justify-content", "space-between" )
                                ]
                            ]
            , test "trigger build button has grey background" <|
                \_ ->
                    Layout.init
                        { turbulenceImgSrc = ""
                        , notFoundImgSrc = ""
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
                        , pathname = "/teams/team/pipelines/pipeline/jobs/job"
                        , search = ""
                        , hash = ""
                        , username = ""
                        , password = ""
                        }
                        |> Tuple.first
                        |> Layout.update
                            (Layout.SubMsg 1 <|
                                SubPage.JobMsg <|
                                    Job.JobFetched <|
                                        Ok
                                            { name = "job"
                                            , pipelineName = "pipeline"
                                            , teamName = "team"
                                            , pipeline =
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                            , nextBuild = Nothing
                                            , finishedBuild = Just someBuild
                                            , transitionBuild = Nothing
                                            , paused = False
                                            , disableManualTrigger = False
                                            , inputs = []
                                            , outputs = []
                                            , groups = []
                                            }
                            )
                        |> Tuple.first
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                        |> Query.has
                            [ style
                                [ ( "padding", "10px" )
                                , ( "border", "none" )
                                , ( "background-color", middleGrey )
                                , ( "outline", "none" )
                                ]
                            ]
            , test "trigger build button has 'plus' icon" <|
                \_ ->
                    Layout.init
                        { turbulenceImgSrc = ""
                        , notFoundImgSrc = ""
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
                        , pathname = "/teams/team/pipelines/pipeline/jobs/job"
                        , search = ""
                        , hash = ""
                        , username = ""
                        , password = ""
                        }
                        |> Tuple.first
                        |> Layout.update
                            (Layout.SubMsg 1 <|
                                SubPage.JobMsg <|
                                    Job.JobFetched <|
                                        Ok
                                            { name = "job"
                                            , pipelineName = "pipeline"
                                            , teamName = "team"
                                            , pipeline =
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                            , nextBuild = Nothing
                                            , finishedBuild = Just someBuild
                                            , transitionBuild = Nothing
                                            , paused = False
                                            , disableManualTrigger = False
                                            , inputs = []
                                            , outputs = []
                                            , groups = []
                                            }
                            )
                        |> Tuple.first
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has
                            (iconSelector
                                { size = "40px"
                                , image = "ic_add_circle_outline_white.svg"
                                }
                            )
            , defineHoverBehaviour
                { name = "trigger build button"
                , setup =
                    Layout.init
                        { turbulenceImgSrc = ""
                        , notFoundImgSrc = ""
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
                        , pathname = "/teams/team/pipelines/pipeline/jobs/job"
                        , search = ""
                        , hash = ""
                        , username = ""
                        , password = ""
                        }
                        |> Tuple.first
                        |> Layout.update
                            (Layout.SubMsg 1 <|
                                SubPage.JobMsg <|
                                    Job.JobFetched <|
                                        Ok
                                            { name = "job"
                                            , pipelineName = "pipeline"
                                            , teamName = "team"
                                            , pipeline =
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                            , nextBuild = Nothing
                                            , finishedBuild = Just someBuild
                                            , transitionBuild = Nothing
                                            , paused = False
                                            , disableManualTrigger = False
                                            , inputs = []
                                            , outputs = []
                                            , groups = []
                                            }
                            )
                        |> Tuple.first
                , query =
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                , updateFunc = \msg -> Layout.update msg >> Tuple.first
                , unhoveredSelector =
                    { description = "grey plus icon"
                    , selector =
                        [ style [ ( "opacity", "0.5" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic_add_circle_outline_white.svg"
                                }
                    }
                , hoveredSelector =
                    { description = "white plus icon"
                    , selector =
                        [ style [ ( "opacity", "1" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic_add_circle_outline_white.svg"
                                }
                    }
                , mouseEnterMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.HoverTrigger True
                , mouseLeaveMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.HoverTrigger False
                }
            , defineHoverBehaviour
                { name = "disabled trigger build button"
                , setup =
                    Layout.init
                        { turbulenceImgSrc = ""
                        , notFoundImgSrc = ""
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
                        , pathname = "/teams/team/pipelines/pipeline/jobs/job"
                        , search = ""
                        , hash = ""
                        , username = ""
                        , password = ""
                        }
                        |> Tuple.first
                        |> Layout.update
                            (Layout.SubMsg 1 <|
                                SubPage.JobMsg <|
                                    Job.JobFetched <|
                                        Ok
                                            { name = "job"
                                            , pipelineName = "pipeline"
                                            , teamName = "team"
                                            , pipeline =
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                            , nextBuild = Nothing
                                            , finishedBuild = Just someBuild
                                            , transitionBuild = Nothing
                                            , paused = False
                                            , disableManualTrigger = True
                                            , inputs = []
                                            , outputs = []
                                            , groups = []
                                            }
                            )
                        |> Tuple.first
                , query =
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                , updateFunc = \msg -> Layout.update msg >> Tuple.first
                , unhoveredSelector =
                    { description = "grey plus icon"
                    , selector =
                        [ style [ ( "opacity", "0.5" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic_add_circle_outline_white.svg"
                                }
                    }
                , hoveredSelector =
                    { description = "grey plus icon with tooltip"
                    , selector =
                        [ style [ ( "position", "relative" ) ]
                        , containing
                            [ containing
                                [ text "manual triggering disabled in job config" ]
                            , style
                                [ ( "position", "absolute" )
                                , ( "right", "100%" )
                                , ( "top", "15px" )
                                , ( "width", "300px" )
                                , ( "color", "#ecf0f1" )
                                , ( "font-size", "12px" )
                                , ( "font-family", "Inconsolata,monospace" )
                                , ( "padding", "10px" )
                                , ( "text-align", "right" )
                                ]
                            ]
                        , containing <|
                            [ style
                                [ ( "opacity", "0.5" )
                                ]
                            ]
                                ++ iconSelector
                                    { size = "40px"
                                    , image = "ic_add_circle_outline_white.svg"
                                    }
                        ]
                    }
                , mouseEnterMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.HoverTrigger True
                , mouseLeaveMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.HoverTrigger False
                }
            , test "JobBuildsFetched" <|
                \_ ->
                    let
                        bwr =
                            defaultModel.buildsWithResources
                    in
                    Expect.equal
                        { defaultModel
                            | currentPage =
                                Just
                                    { direction = Concourse.Pagination.Since 124
                                    , limit = 1
                                    }
                            , buildsWithResources =
                                { bwr
                                    | content =
                                        [ { build = someBuild
                                          , resources = Nothing
                                          }
                                        ]
                                }
                        }
                    <|
                        Tuple.first <|
                            update
                                (JobBuildsFetched <|
                                    Ok
                                        { content = [ someBuild ]
                                        , pagination =
                                            { previousPage = Nothing
                                            , nextPage = Nothing
                                            }
                                        }
                                )
                                defaultModel
            , test "JobBuildsFetched error" <|
                \_ ->
                    Expect.equal
                        defaultModel
                    <|
                        Tuple.first <|
                            update
                                (JobBuildsFetched <| Err Http.NetworkError)
                                defaultModel
            , test "JobFetched" <|
                \_ ->
                    Expect.equal
                        { defaultModel
                            | job = RemoteData.Success someJob
                        }
                    <|
                        Tuple.first <|
                            update (JobFetched <| Ok someJob) defaultModel
            , test "JobFetched error" <|
                \_ ->
                    Expect.equal
                        defaultModel
                    <|
                        Tuple.first <|
                            update
                                (JobFetched <| Err Http.NetworkError)
                                defaultModel
            , test "BuildResourcesFetched" <|
                \_ ->
                    let
                        buildInput =
                            { name = "some-input"
                            , version = Dict.fromList [ ( "version", "v1" ) ]
                            , firstOccurrence = True
                            }

                        buildOutput =
                            { name = "some-resource"
                            , version = Dict.fromList [ ( "version", "v2" ) ]
                            }
                    in
                    let
                        buildResources =
                            { inputs = [ buildInput ]
                            , outputs = [ buildOutput ]
                            }
                    in
                    Expect.equal
                        defaultModel
                    <|
                        Tuple.first <|
                            update (BuildResourcesFetched 1 (Ok buildResources))
                                defaultModel
            , test "BuildResourcesFetched error" <|
                \_ ->
                    Expect.equal
                        defaultModel
                    <|
                        Tuple.first <|
                            update
                                (BuildResourcesFetched 1 (Err Http.NetworkError))
                                defaultModel
            , test "TogglePaused" <|
                \_ ->
                    Expect.equal
                        { defaultModel
                            | job = RemoteData.Success { someJob | paused = True }
                            , pausedChanging = True
                        }
                    <|
                        Tuple.first <|
                            update
                                TogglePaused
                                { defaultModel | job = RemoteData.Success someJob }
            , test "PausedToggled" <|
                \_ ->
                    Expect.equal
                        { defaultModel
                            | job = RemoteData.Success someJob
                            , pausedChanging = False
                        }
                    <|
                        Tuple.first <|
                            update
                                (PausedToggled <| Ok ())
                                { defaultModel | job = RemoteData.Success someJob }
            , test "PausedToggled error" <|
                \_ ->
                    Expect.equal
                        { defaultModel | job = RemoteData.Success someJob }
                    <|
                        Tuple.first <|
                            update
                                (PausedToggled <| Err Http.NetworkError)
                                { defaultModel | job = RemoteData.Success someJob }
            , test "PausedToggled unauthorized" <|
                \_ ->
                    Expect.equal
                        { defaultModel | job = RemoteData.Success someJob }
                    <|
                        Tuple.first <|
                            update
                                (PausedToggled <|
                                    Err <|
                                        Http.BadStatus
                                            { url = "http://example.com"
                                            , status =
                                                { code = 401
                                                , message = ""
                                                }
                                            , headers = Dict.empty
                                            , body = ""
                                            }
                                )
                                { defaultModel | job = RemoteData.Success someJob }
            ]
        ]
