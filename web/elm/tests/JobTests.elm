module JobTests exposing (all)

import Concourse exposing (Build, BuildId, BuildStatus(..), Job)
import Concourse.Pagination exposing (Direction(..))
import DashboardTests
    exposing
        ( darkGrey
        , defineHoverBehaviour
        , iconSelector
        , middleGrey
        )
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
import Test.Html.Selector as Selector
    exposing
        ( attribute
        , class
        , containing
        , id
        , style
        , text
        )


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

                init :
                    { disabled : Bool, paused : Bool }
                    -> ()
                    -> Layout.Model
                init { disabled, paused } _ =
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
                                            , paused = paused
                                            , disableManualTrigger = disabled
                                            , inputs = []
                                            , outputs = []
                                            , groups = []
                                            }
                            )
                        |> Tuple.first

                loadingIndicatorSelector : List Selector.Selector
                loadingIndicatorSelector =
                    [ style [ ( "display", "flex" ) ]
                    , containing
                        [ style
                            [ ( "animation"
                              , "container-rotate 1568ms linear infinite"
                              )
                            , ( "height", "14px" )
                            , ( "width", "14px" )
                            , ( "margin", "7px" )
                            ]
                        ]
                    ]
            in
            [ describe "while page is loading"
                [ test "shows two spinners before anything has loaded" <|
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
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.findAll loadingIndicatorSelector
                            |> Query.count (Expect.equal 2)
                , test "loading build has spinners for inputs and outputs" <|
                    init { disabled = False, paused = False }
                        >> Layout.update
                            (Layout.SubMsg 1 <|
                                SubPage.JobMsg <|
                                    Job.JobBuildsFetched <|
                                        let
                                            jobId =
                                                { jobName = "job"
                                                , pipelineName = "pipeline"
                                                , teamName = "team"
                                                }

                                            status =
                                                BuildStatusSucceeded

                                            builds =
                                                [ { id = 0
                                                  , name = "0"
                                                  , job = Just jobId
                                                  , status = status
                                                  , duration =
                                                        { startedAt = Nothing
                                                        , finishedAt = Nothing
                                                        }
                                                  , reapTime = Nothing
                                                  }
                                                ]
                                        in
                                        Ok
                                            { pagination =
                                                { previousPage = Nothing
                                                , nextPage = Nothing
                                                }
                                            , content = builds
                                            }
                            )
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Expect.all
                            [ Query.find [ class "inputs" ]
                                >> Query.has loadingIndicatorSelector
                            , Query.find [ class "outputs" ]
                                >> Query.has loadingIndicatorSelector
                            ]
                ]
            , test "build header lays out contents horizontally" <|
                init { disabled = False, paused = False }
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find [ class "build-header" ]
                    >> Query.has
                        [ style
                            [ ( "display", "flex" )
                            , ( "justify-content", "space-between" )
                            ]
                        ]
            , test "header has play/pause button at the left" <|
                init { disabled = False, paused = False }
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find [ class "build-header" ]
                    >> Query.has [ id "pause-toggle" ]
            , test "play/pause has grey background" <|
                init { disabled = False, paused = False }
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find [ id "pause-toggle" ]
                    >> Query.has
                        [ style
                            [ ( "padding", "10px" )
                            , ( "border", "none" )
                            , ( "background-color", middleGrey )
                            , ( "outline", "none" )
                            ]
                        ]
            , defineHoverBehaviour
                { name = "play/pause button when job is unpaused"
                , setup =
                    init { disabled = False, paused = False } ()
                , query =
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "pause-toggle" ]
                , updateFunc = \msg -> Layout.update msg >> Tuple.first
                , unhoveredSelector =
                    { description = "grey pause icon"
                    , selector =
                        [ style [ ( "opacity", "0.5" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic-pause-circle-outline-white.svg"
                                }
                    }
                , hoveredSelector =
                    { description = "white pause icon"
                    , selector =
                        [ style [ ( "opacity", "1" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic-pause-circle-outline-white.svg"
                                }
                    }
                , mouseEnterMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.Toggle
                , mouseLeaveMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.None
                }
            , defineHoverBehaviour
                { name = "play/pause button when job is paused"
                , setup =
                    init { disabled = False, paused = True } ()
                , query =
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "pause-toggle" ]
                , updateFunc = \msg -> Layout.update msg >> Tuple.first
                , unhoveredSelector =
                    { description = "grey play icon"
                    , selector =
                        [ style [ ( "opacity", "0.5" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic-play-circle-outline.svg"
                                }
                    }
                , hoveredSelector =
                    { description = "white play icon"
                    , selector =
                        [ style [ ( "opacity", "1" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic-play-circle-outline.svg"
                                }
                    }
                , mouseEnterMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.Toggle
                , mouseLeaveMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.None
                }
            , test "trigger build button has grey background" <|
                init { disabled = False, paused = False }
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Trigger Build"
                        ]
                    >> Query.has
                        [ style
                            [ ( "padding", "10px" )
                            , ( "border", "none" )
                            , ( "background-color", middleGrey )
                            , ( "outline", "none" )
                            ]
                        ]
            , test "trigger build button has 'plus' icon" <|
                init { disabled = False, paused = False }
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Trigger Build"
                        ]
                    >> Query.children []
                    >> Query.first
                    >> Query.has
                        (iconSelector
                            { size = "40px"
                            , image = "ic-add-circle-outline-white.svg"
                            }
                        )
            , defineHoverBehaviour
                { name = "trigger build button"
                , setup =
                    init { disabled = False, paused = False } ()
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
                                , image = "ic-add-circle-outline-white.svg"
                                }
                    }
                , hoveredSelector =
                    { description = "white plus icon"
                    , selector =
                        [ style [ ( "opacity", "1" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic-add-circle-outline-white.svg"
                                }
                    }
                , mouseEnterMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.Trigger
                , mouseLeaveMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.None
                }
            , defineHoverBehaviour
                { name = "disabled trigger build button"
                , setup =
                    init { disabled = True, paused = False } ()
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
                                , image = "ic-add-circle-outline-white.svg"
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
                                    , image = "ic-add-circle-outline-white.svg"
                                    }
                        ]
                    }
                , mouseEnterMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.Trigger
                , mouseLeaveMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.None
                }
            , test "inputs icon on build" <|
                init { disabled = False, paused = False }
                    >> Layout.update
                        (Layout.SubMsg 1 <|
                            SubPage.JobMsg <|
                                Job.JobBuildsFetched <|
                                    let
                                        jobId =
                                            { jobName = "job"
                                            , pipelineName = "pipeline"
                                            , teamName = "team"
                                            }

                                        status =
                                            BuildStatusSucceeded

                                        builds =
                                            [ { id = 0
                                              , name = "0"
                                              , job = Just jobId
                                              , status = status
                                              , duration =
                                                    { startedAt = Nothing
                                                    , finishedAt = Nothing
                                                    }
                                              , reapTime = Nothing
                                              }
                                            ]
                                    in
                                    Ok
                                        { pagination =
                                            { previousPage = Nothing
                                            , nextPage = Nothing
                                            }
                                        , content = builds
                                        }
                        )
                    >> Tuple.first
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find [ class "inputs" ]
                    >> Query.children []
                    >> Query.first
                    >> Expect.all
                        [ Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "align-items", "center" )
                                , ( "padding-bottom", "5px" )
                                ]
                            ]
                        , Query.children []
                            >> Query.first
                            >> Query.has
                                (iconSelector
                                    { size = "12px"
                                    , image = "ic-arrow-downward.svg"
                                    }
                                    ++ [ style
                                            [ ( "background-size"
                                              , "contain"
                                              )
                                            , ( "margin-right", "5px" )
                                            ]
                                       ]
                                )
                        ]
            , test "outputs icon on build" <|
                init { disabled = False, paused = False }
                    >> Layout.update
                        (Layout.SubMsg 1 <|
                            SubPage.JobMsg <|
                                Job.JobBuildsFetched <|
                                    let
                                        jobId =
                                            { jobName = "job"
                                            , pipelineName = "pipeline"
                                            , teamName = "team"
                                            }

                                        status =
                                            BuildStatusSucceeded

                                        builds =
                                            [ { id = 0
                                              , name = "0"
                                              , job = Just jobId
                                              , status = status
                                              , duration =
                                                    { startedAt = Nothing
                                                    , finishedAt = Nothing
                                                    }
                                              , reapTime = Nothing
                                              }
                                            ]
                                    in
                                    Ok
                                        { pagination =
                                            { previousPage = Nothing
                                            , nextPage = Nothing
                                            }
                                        , content = builds
                                        }
                        )
                    >> Tuple.first
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find [ class "outputs" ]
                    >> Query.children []
                    >> Query.first
                    >> Expect.all
                        [ Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "align-items", "center" )
                                , ( "padding-bottom", "5px" )
                                ]
                            ]
                        , Query.children []
                            >> Query.first
                            >> Query.has
                                (iconSelector
                                    { size = "12px"
                                    , image = "ic-arrow-upward.svg"
                                    }
                                    ++ [ style
                                            [ ( "background-size"
                                              , "contain"
                                              )
                                            , ( "margin-right", "5px" )
                                            ]
                                       ]
                                )
                        ]
            , test "pagination header lays out horizontally" <|
                init { disabled = False, paused = False }
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find [ id "pagination-header" ]
                    >> Query.has
                        [ style
                            [ ( "display", "flex" )
                            , ( "justify-content", "space-between" )
                            , ( "align-items", "stretch" )
                            , ( "background-color", darkGrey )
                            , ( "height", "60px" )
                            ]
                        ]
            , test "the word 'builds' is bold and indented" <|
                init { disabled = False, paused = False }
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find [ id "pagination-header" ]
                    >> Query.children []
                    >> Query.first
                    >> Query.has
                        [ containing [ text "builds" ]
                        , style
                            [ ( "margin", "0 18px" )
                            , ( "font-weight", "700" )
                            ]
                        ]
            , test "pagination lays out horizontally" <|
                init { disabled = False, paused = False }
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find [ id "pagination" ]
                    >> Query.has
                        [ style
                            [ ( "display", "flex" )
                            , ( "align-items", "stretch" )
                            ]
                        ]
            , test "pagination chevrons with no pages" <|
                init { disabled = False, paused = False }
                    >> Layout.update
                        (Layout.SubMsg 1 <|
                            SubPage.JobMsg <|
                                Job.JobBuildsFetched <|
                                    let
                                        jobId =
                                            { jobName = "job"
                                            , pipelineName = "pipeline"
                                            , teamName = "team"
                                            }

                                        status =
                                            BuildStatusSucceeded

                                        builds =
                                            [ { id = 0
                                              , name = "0"
                                              , job = Just jobId
                                              , status = status
                                              , duration =
                                                    { startedAt = Nothing
                                                    , finishedAt = Nothing
                                                    }
                                              , reapTime = Nothing
                                              }
                                            ]
                                    in
                                    Ok
                                        { pagination =
                                            { previousPage = Nothing
                                            , nextPage = Nothing
                                            }
                                        , content = builds
                                        }
                        )
                    >> Tuple.first
                    >> Layout.view
                    >> Query.fromHtml
                    >> Query.find [ id "pagination" ]
                    >> Query.children []
                    >> Expect.all
                        [ Query.index 0
                            >> Query.has
                                [ style
                                    [ ( "padding", "5px" )
                                    , ( "display", "flex" )
                                    , ( "align-items", "center" )
                                    , ( "border-left"
                                      , "1px solid " ++ middleGrey
                                      )
                                    ]
                                , containing
                                    (iconSelector
                                        { image =
                                            "baseline-chevron-left-24px.svg"
                                        , size = "24px"
                                        }
                                        ++ [ style
                                                [ ( "padding", "5px" )
                                                , ( "opacity", "0.5" )
                                                ]
                                           ]
                                    )
                                ]
                        , Query.index 1
                            >> Query.has
                                [ style
                                    [ ( "padding", "5px" )
                                    , ( "display", "flex" )
                                    , ( "align-items", "center" )
                                    , ( "border-left"
                                      , "1px solid " ++ middleGrey
                                      )
                                    ]
                                , containing
                                    (iconSelector
                                        { image =
                                            "baseline-chevron-right-24px.svg"
                                        , size = "24px"
                                        }
                                        ++ [ style
                                                [ ( "padding", "5px" )
                                                , ( "opacity", "0.5" )
                                                ]
                                           ]
                                    )
                                ]
                        ]
            , defineHoverBehaviour <|
                let
                    urlPath =
                        "/teams/team/pipelines/pipeline/jobs/job?since=1"
                in
                { name = "left pagination chevron with previous page"
                , setup =
                    let
                        jobId =
                            { jobName = "job"
                            , pipelineName = "pipeline"
                            , teamName = "team"
                            }

                        status =
                            BuildStatusSucceeded

                        builds =
                            [ { id = 0
                              , name = "0"
                              , job = Just jobId
                              , status = status
                              , duration =
                                    { startedAt = Nothing
                                    , finishedAt = Nothing
                                    }
                              , reapTime = Nothing
                              }
                            ]

                        prevPage =
                            { direction = Since 1
                            , limit = 1
                            }
                    in
                    init { disabled = False, paused = False } ()
                        |> Layout.update
                            (Layout.SubMsg 1 <|
                                SubPage.JobMsg <|
                                    Job.JobBuildsFetched <|
                                        Ok
                                            { pagination =
                                                { previousPage =
                                                    Just prevPage
                                                , nextPage = Nothing
                                                }
                                            , content = builds
                                            }
                            )
                        |> Tuple.first
                , query =
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "pagination" ]
                        >> Query.children []
                        >> Query.index 0
                , updateFunc = \msg -> Layout.update msg >> Tuple.first
                , unhoveredSelector =
                    { description = "white left chevron"
                    , selector =
                        [ style
                            [ ( "padding", "5px" )
                            , ( "display", "flex" )
                            , ( "align-items", "center" )
                            , ( "border-left"
                              , "1px solid " ++ middleGrey
                              )
                            ]
                        , containing
                            (iconSelector
                                { image =
                                    "baseline-chevron-left-24px.svg"
                                , size = "24px"
                                }
                                ++ [ style
                                        [ ( "padding", "5px" )
                                        , ( "opacity", "1" )
                                        ]
                                   , attribute <| Attr.href urlPath
                                   ]
                            )
                        ]
                    }
                , hoveredSelector =
                    { description =
                        "left chevron with light grey circular bg"
                    , selector =
                        [ style
                            [ ( "padding", "5px" )
                            , ( "display", "flex" )
                            , ( "align-items", "center" )
                            , ( "border-left"
                              , "1px solid " ++ middleGrey
                              )
                            ]
                        , containing
                            (iconSelector
                                { image =
                                    "baseline-chevron-left-24px.svg"
                                , size = "24px"
                                }
                                ++ [ style
                                        [ ( "padding", "5px" )
                                        , ( "opacity", "1" )
                                        , ( "border-radius", "50%" )
                                        , ( "background-color"
                                          , "#504b4b"
                                          )
                                        ]
                                   , attribute <| Attr.href urlPath
                                   ]
                            )
                        ]
                    }
                , mouseEnterMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.PreviousPage
                , mouseLeaveMsg =
                    Layout.SubMsg 1 <|
                        SubPage.JobMsg <|
                            Job.Hover Job.None
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
