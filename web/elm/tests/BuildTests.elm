module BuildTests exposing (all)

import Array
import Build
import Build.Models as Models
import Build.Msgs
import Callback
import Char
import Concourse exposing (BuildPrepStatus(..))
import DashboardTests
    exposing
        ( defineHoverBehaviour
        , iconSelector
        , middleGrey
        )
import Dict
import Effects
import Expect
import Html.Attributes as Attr
import Layout
import Msgs
import Routes
import SubPage.Msgs
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector
    exposing
        ( attribute
        , class
        , containing
        , id
        , style
        , tag
        , text
        )
import UserState


all : Test
all =
    describe "build page" <|
        let
            pageLoad =
                Build.init
                    { csrfToken = ""
                    , highlight = Routes.HighlightNothing
                    , route = Routes.Build "team" "pipeline" "job" "1" Routes.HighlightNothing
                    }
                    (Models.JobBuildPage
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "job"
                        , buildName = "1"
                        }
                    )

            theBuild : Concourse.Build
            theBuild =
                { id = 1
                , name = "1"
                , job =
                    Just
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "job"
                        }
                , status = Concourse.BuildStatusSucceeded
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Nothing
                    }
                , reapTime = Nothing
                }

            startedBuild : Concourse.Build
            startedBuild =
                { id = 1
                , name = "1"
                , job =
                    Just
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "job"
                        }
                , status = Concourse.BuildStatusStarted
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Nothing
                    }
                , reapTime = Nothing
                }

            fetchBuild : Models.Model -> ( Models.Model, List Effects.Effect )
            fetchBuild =
                Build.handleCallback <| Callback.BuildFetched <| Ok ( 1, theBuild )

            fetchStartedBuild :
                Models.Model
                -> ( Models.Model, List Effects.Effect )
            fetchStartedBuild =
                Build.handleCallback <| Callback.BuildFetched <| Ok ( 1, startedBuild )

            fetchJobDetails : Models.Model -> ( Models.Model, List Effects.Effect )
            fetchJobDetails =
                Build.handleCallback <|
                    Callback.BuildJobDetailsFetched <|
                        Ok
                            { pipeline =
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                }
                            , name = "job"
                            , pipelineName = "pipeline"
                            , teamName = "team"
                            , nextBuild = Nothing
                            , finishedBuild = Nothing
                            , transitionBuild = Nothing
                            , paused = False
                            , disableManualTrigger = False
                            , inputs = []
                            , outputs = []
                            , groups = []
                            }

            fetchJobDetailsNoTrigger :
                Models.Model
                -> ( Models.Model, List Effects.Effect )
            fetchJobDetailsNoTrigger =
                Build.handleCallback <|
                    Callback.BuildJobDetailsFetched <|
                        Ok
                            { pipeline =
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                }
                            , name = "job"
                            , pipelineName = "pipeline"
                            , teamName = "team"
                            , nextBuild = Nothing
                            , finishedBuild = Nothing
                            , transitionBuild = Nothing
                            , paused = False
                            , disableManualTrigger = True
                            , inputs = []
                            , outputs = []
                            , groups = []
                            }

            fetchHistory : Models.Model -> ( Models.Model, List Effects.Effect )
            fetchHistory =
                Build.handleCallback
                    (Callback.BuildHistoryFetched
                        (Ok
                            { pagination =
                                { previousPage = Nothing
                                , nextPage = Nothing
                                }
                            , content = [ theBuild ]
                            }
                        )
                    )
        in
        [ test "converts URL hash to highlighted line in view" <|
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
                    , pathname = "/builds/1"
                    , search = ""
                    , hash = "#Lstepid:1"
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Layout.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <|
                            Ok
                                ( 1
                                , { id = 1
                                  , name = "1"
                                  , job = Nothing
                                  , status = Concourse.BuildStatusStarted
                                  , duration =
                                        { startedAt = Nothing
                                        , finishedAt = Nothing
                                        }
                                  , reapTime = Nothing
                                  }
                                )
                        )
                    |> Tuple.first
                    |> Layout.handleCallback
                        (Effects.SubPage 1)
                        (Callback.PlanAndResourcesFetched 307 <|
                            Ok <|
                                ( { id = "stepid"
                                  , step =
                                        Concourse.BuildStepTask
                                            "step"
                                  }
                                , { inputs = [], outputs = [] }
                                )
                        )
                    |> Tuple.first
                    |> Layout.update
                        (Msgs.SubMsg 1
                            (SubPage.Msgs.BuildMsg
                                (Build.Msgs.BuildEventsMsg Build.Msgs.Opened)
                            )
                        )
                    |> Tuple.first
                    |> Layout.update
                        (Msgs.SubMsg 1
                            (SubPage.Msgs.BuildMsg
                                (Build.Msgs.BuildEventsMsg <|
                                    Build.Msgs.Events <|
                                        Ok <|
                                            Array.fromList
                                                [ Models.StartTask
                                                    { source = "stdout"
                                                    , id = "stepid"
                                                    }
                                                , Models.Log
                                                    { source = "stdout"
                                                    , id = "stepid"
                                                    }
                                                    "log message"
                                                    Nothing
                                                ]
                                )
                            )
                        )
                    |> Tuple.first
                    |> Layout.view
                    |> Query.fromHtml
                    |> Query.find
                        [ class "timestamped-line"
                        , containing [ text "log message" ]
                        ]
                    |> Query.has [ class "highlighted-line" ]
        , test "pressing 'T' twice triggers two builds" <|
            \_ ->
                Layout.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = ""
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , pipelineRunningKeyframes = ""
                    }
                    { href = ""
                    , host = ""
                    , hostname = ""
                    , protocol = ""
                    , origin = ""
                    , port_ = ""
                    , pathname = "/teams/t/pipelines/p/jobs/j/builds/1"
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Layout.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <|
                            Ok
                                ( 1
                                , { id = 1
                                  , name = "1"
                                  , job =
                                        Just
                                            { teamName = "t"
                                            , pipelineName = "p"
                                            , jobName = "j"
                                            }
                                  , status = Concourse.BuildStatusStarted
                                  , duration =
                                        { startedAt = Nothing
                                        , finishedAt = Nothing
                                        }
                                  , reapTime = Nothing
                                  }
                                )
                        )
                    |> Tuple.first
                    |> Layout.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildJobDetailsFetched <|
                            Ok
                                { pipeline =
                                    { teamName = "t"
                                    , pipelineName = "p"
                                    }
                                , name = ""
                                , pipelineName = "p"
                                , teamName = "t"
                                , nextBuild = Nothing
                                , finishedBuild = Nothing
                                , transitionBuild = Nothing
                                , paused = False
                                , disableManualTrigger = False
                                , inputs = []
                                , outputs = []
                                , groups = []
                                }
                        )
                    |> Tuple.first
                    |> Layout.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.BuildMsg <|
                                Build.Msgs.KeyPressed <|
                                    Char.toCode 'T'
                        )
                    |> Tuple.first
                    |> Layout.update (Msgs.KeyUp <| Char.toCode 'T')
                    |> Tuple.first
                    |> Layout.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.BuildMsg <|
                                Build.Msgs.KeyPressed <|
                                    Char.toCode 'T'
                        )
                    |> Tuple.second
                    |> Expect.equal
                        [ ( Effects.SubPage 1
                          , Effects.DoTriggerBuild
                                { teamName = "t"
                                , pipelineName = "p"
                                , jobName = "j"
                                }
                                "csrf_token"
                          )
                        ]
        , test "says loading on page load" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> Build.view UserState.UserStateLoggedOut
                    |> Query.fromHtml
                    |> Query.has [ text "loading" ]
        , test "fetches build on page load" <|
            \_ ->
                pageLoad
                    |> Tuple.second
                    |> Expect.equal
                        [ Effects.FetchJobBuild 1
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = "job"
                            , buildName = "1"
                            }
                        , Effects.GetScreenSize
                        , Effects.GetCurrentTime
                        ]
        , describe "top bar" <|
            [ test "has a top bar" <|
                \_ ->
                    pageLoad
                        |> Tuple.first
                        |> Build.view UserState.UserStateLoggedOut
                        |> Query.fromHtml
                        |> Query.has [ id "top-bar-app" ]
            , test "has a concourse icon" <|
                \_ ->
                    pageLoad
                        |> Tuple.first
                        |> Build.view UserState.UserStateLoggedOut
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style [ ( "background-image", "url(/public/images/concourse-logo-white.svg)" ) ] ]
            , test "has the breadcrumbs" <|
                \_ ->
                    pageLoad
                        |> Tuple.first
                        |> Build.view UserState.UserStateLoggedOut
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Expect.all
                            [ Query.has [ id "breadcrumb-pipeline" ]
                            , Query.has [ text "pipeline" ]
                            , Query.has [ id "breadcrumb-job" ]
                            , Query.has [ text "job" ]
                            ]
            , test "has a user section" <|
                \_ ->
                    pageLoad
                        |> Tuple.first
                        |> Build.view UserState.UserStateLoggedOut
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ id "login-component" ]
            ]
        , describe "after build is fetched" <|
            let
                givenBuildFetched _ =
                    pageLoad |> Tuple.first |> fetchBuild
            in
            [ test "has a header after the build is fetched" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Build.view UserState.UserStateLoggedOut
                    >> Query.fromHtml
                    >> Query.has [ id "build-header" ]
            , test "fetches build history and job details after build is fetched" <|
                givenBuildFetched
                    >> Tuple.second
                    >> Expect.all
                        [ List.member
                            (Effects.FetchBuildHistory
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                , jobName = "job"
                                }
                                Nothing
                            )
                            >> Expect.true
                                "expected effect was not in the list"
                        , List.member
                            (Effects.FetchBuildJobDetails
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                , jobName = "job"
                                }
                            )
                            >> Expect.true
                                "expected effect was not in the list"
                        ]
            , test "header lays out horizontally" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Build.view UserState.UserStateLoggedOut
                    >> Query.fromHtml
                    >> Query.find [ id "build-header" ]
                    >> Query.has
                        [ style [ ( "display", "flex" ) ] ]
            , test "header spreads out contents" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Build.view UserState.UserStateLoggedOut
                    >> Query.fromHtml
                    >> Query.find [ id "build-header" ]
                    >> Query.has
                        [ style [ ( "justify-content", "space-between" ) ] ]
            , describe "after history and details get fetched" <|
                let
                    givenHistoryAndDetailsFetched =
                        givenBuildFetched
                            >> Tuple.first
                            >> fetchHistory
                            >> Tuple.first
                            >> fetchJobDetails
                in
                [ test
                    ("trigger build button on right side of header "
                        ++ "after history and job details fetched"
                    )
                  <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ id "build-header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                , test "trigger build button is styled as a plain grey box" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
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
                                , ( "margin", "0" )
                                ]
                            ]
                , test "trigger build button has pointer cursor" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                        >> Query.has [ style [ ( "cursor", "pointer" ) ] ]
                , test "trigger build button has 'plus' icon" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
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
                        givenHistoryAndDetailsFetched () |> Tuple.first
                    , query =
                        Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.find
                                [ attribute <|
                                    Attr.attribute "aria-label" "Trigger Build"
                                ]
                    , updateFunc = \msg -> Build.update msg >> Tuple.first
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
                    , mouseEnterMsg = Build.Msgs.Hover <| Just Models.Trigger
                    , mouseLeaveMsg = Build.Msgs.Hover Nothing
                    }
                ]
            , describe "when history and details witche dwith maual triggering disabled" <|
                let
                    givenHistoryAndDetailsFetched =
                        givenBuildFetched
                            >> Tuple.first
                            >> fetchHistory
                            >> Tuple.first
                            >> fetchJobDetailsNoTrigger
                in
                [ test "when manual triggering is disabled, trigger build button has default cursor" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                        >> Query.has [ style [ ( "cursor", "default" ) ] ]
                , defineHoverBehaviour
                    { name = "disabled trigger build button"
                    , setup =
                        givenHistoryAndDetailsFetched () |> Tuple.first
                    , query =
                        Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.find
                                [ attribute <|
                                    Attr.attribute "aria-label" "Trigger Build"
                                ]
                    , updateFunc = \msg -> Build.update msg >> Tuple.first
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
                    , mouseEnterMsg = Build.Msgs.Hover <| Just Models.Trigger
                    , mouseLeaveMsg = Build.Msgs.Hover Nothing
                    }
                ]
            ]
        , describe "given build started and history and details fetched" <|
            let
                givenBuildStarted _ =
                    pageLoad
                        |> Tuple.first
                        |> fetchStartedBuild
                        |> Tuple.first
                        |> fetchHistory
                        |> Tuple.first
                        |> fetchJobDetails
            in
            [ test "build action section lays out horizontally" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Build.view UserState.UserStateLoggedOut
                    >> Query.fromHtml
                    >> Query.find [ id "build-header" ]
                    >> Query.children []
                    >> Query.index -1
                    >> Query.has [ style [ ( "display", "flex" ) ] ]
            , test "abort build button is to the left of the trigger button" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Build.view UserState.UserStateLoggedOut
                    >> Query.fromHtml
                    >> Query.find [ id "build-header" ]
                    >> Query.children []
                    >> Query.index -1
                    >> Query.children []
                    >> Query.first
                    >> Query.has
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
            , test "abort build button is styled as a plain grey box" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Build.view UserState.UserStateLoggedOut
                    >> Query.fromHtml
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
                    >> Query.has
                        [ style
                            [ ( "padding", "10px" )
                            , ( "border", "none" )
                            , ( "background-color", middleGrey )
                            , ( "outline", "none" )
                            , ( "margin", "0" )
                            ]
                        ]
            , test "abort build button has pointer cursor" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Build.view UserState.UserStateLoggedOut
                    >> Query.fromHtml
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
                    >> Query.has [ style [ ( "cursor", "pointer" ) ] ]
            , test "abort build button has 'X' icon" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Build.view UserState.UserStateLoggedOut
                    >> Query.fromHtml
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
                    >> Query.children []
                    >> Query.first
                    >> Query.has
                        (iconSelector
                            { size = "40px"
                            , image = "ic-abort-circle-outline-white.svg"
                            }
                        )
            , defineHoverBehaviour
                { name = "abort build button"
                , setup =
                    givenBuildStarted ()
                        |> Tuple.first
                , query =
                    Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Abort Build"
                            ]
                , updateFunc = \msg -> Build.update msg >> Tuple.first
                , unhoveredSelector =
                    { description = "grey abort icon"
                    , selector =
                        [ style [ ( "opacity", "0.5" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic-abort-circle-outline-white.svg"
                                }
                    }
                , hoveredSelector =
                    { description = "white abort icon"
                    , selector =
                        [ style [ ( "opacity", "1" ) ] ]
                            ++ iconSelector
                                { size = "40px"
                                , image = "ic-abort-circle-outline-white.svg"
                                }
                    }
                , mouseEnterMsg = Build.Msgs.Hover <| Just Models.Abort
                , mouseLeaveMsg = Build.Msgs.Hover Nothing
                }
            , describe "build prep section"
                [ test "when pipeline is not paused, shows a check" <|
                    let
                        prep =
                            { pausedPipeline = BuildPrepStatusNotBlocking
                            , pausedJob = BuildPrepStatusNotBlocking
                            , maxRunningBuilds = BuildPrepStatusNotBlocking
                            , inputs = Dict.empty
                            , inputsSatisfied = BuildPrepStatusNotBlocking
                            , missingInputReasons = Dict.empty
                            }

                        icon =
                            "url(/public/images/ic-not-blocking-check.svg)"
                    in
                    givenBuildStarted
                        >> Tuple.first
                        >> Build.handleCallback (Callback.BuildPrepFetched <| Ok ( 1, prep ))
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "prep-status-list" ]
                        >> Expect.all
                            [ Query.children []
                                >> Query.each
                                    (Query.children []
                                        >> Query.first
                                        >> Query.has
                                            [ style
                                                [ ( "display", "flex" )
                                                , ( "align-items", "center" )
                                                ]
                                            ]
                                    )
                            , Query.has
                                [ style
                                    [ ( "background-image", icon )
                                    , ( "background-position", "50% 50%" )
                                    , ( "background-repeat", "no-repeat" )
                                    , ( "background-size", "contain" )
                                    , ( "width", "12px" )
                                    , ( "height", "12px" )
                                    , ( "margin-right", "5px" )
                                    ]
                                , attribute <| Attr.title "not blocking"
                                ]
                            ]
                , test "when pipeline is paused, shows a spinner" <|
                    let
                        prep =
                            { pausedPipeline = BuildPrepStatusBlocking
                            , pausedJob = BuildPrepStatusNotBlocking
                            , maxRunningBuilds = BuildPrepStatusNotBlocking
                            , inputs = Dict.empty
                            , inputsSatisfied = BuildPrepStatusNotBlocking
                            , missingInputReasons = Dict.empty
                            }
                    in
                    givenBuildStarted
                        >> Tuple.first
                        >> Build.handleCallback (Callback.BuildPrepFetched <| Ok ( 1, prep ))
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "prep-status-list" ]
                        >> Expect.all
                            [ Query.children []
                                >> Query.each
                                    (Query.children []
                                        >> Query.first
                                        >> Query.has
                                            [ style
                                                [ ( "display", "flex" )
                                                , ( "align-items", "center" )
                                                ]
                                            ]
                                    )
                            , Query.has
                                [ style
                                    [ ( "animation"
                                      , "container-rotate 1568ms linear infinite"
                                      )
                                    , ( "height", "12px" )
                                    , ( "width", "12px" )
                                    , ( "margin-right", "5px" )
                                    ]
                                , attribute <| Attr.title "blocking"
                                ]
                            ]
                ]
            , describe "step header" <|
                let
                    fetchPlanWithGetStep : () -> Models.Model
                    fetchPlanWithGetStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Build.handleCallback
                                (Callback.PlanAndResourcesFetched 307 <|
                                    Ok <|
                                        ( { id = "plan"
                                          , step =
                                                Concourse.BuildStepGet
                                                    "step"
                                                    Nothing
                                          }
                                        , { inputs = [], outputs = [] }
                                        )
                                )
                            >> Tuple.first

                    fetchPlanWithTaskStep : () -> Models.Model
                    fetchPlanWithTaskStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Build.handleCallback
                                (Callback.PlanAndResourcesFetched 307 <|
                                    Ok <|
                                        ( { id = "plan"
                                          , step =
                                                Concourse.BuildStepTask
                                                    "step"
                                          }
                                        , { inputs = [], outputs = [] }
                                        )
                                )
                            >> Tuple.first

                    fetchPlanWithPutStep : () -> Models.Model
                    fetchPlanWithPutStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Build.handleCallback
                                (Callback.PlanAndResourcesFetched 307 <|
                                    Ok <|
                                        ( { id = "plan"
                                          , step =
                                                Concourse.BuildStepPut
                                                    "step"
                                          }
                                        , { inputs = [], outputs = [] }
                                        )
                                )
                            >> Tuple.first

                    fetchPlanWithGetStepWithFirstOccurrence : () -> Models.Model
                    fetchPlanWithGetStepWithFirstOccurrence =
                        givenBuildStarted
                            >> Tuple.first
                            >> Build.handleCallback
                                (Callback.PlanAndResourcesFetched 307 <|
                                    let
                                        version =
                                            Dict.fromList
                                                [ ( "ref", "abc123" ) ]
                                    in
                                    Ok <|
                                        ( { id = "plan"
                                          , step =
                                                Concourse.BuildStepDo <|
                                                    Array.fromList
                                                        [ { id = "foo"
                                                          , step =
                                                                Concourse.BuildStepGet "step"
                                                                    (Just version)
                                                          }
                                                        , { id = "bar"
                                                          , step =
                                                                Concourse.BuildStepGet "step2"
                                                                    (Just version)
                                                          }
                                                        , { id = "baz"
                                                          , step =
                                                                Concourse.BuildStepGet "step3"
                                                                    (Just version)
                                                          }
                                                        ]
                                          }
                                        , { inputs =
                                                [ { name = "step"
                                                  , version = version
                                                  , firstOccurrence = True
                                                  }
                                                , { name = "step2"
                                                  , version = version
                                                  , firstOccurrence = True
                                                  }
                                                , { name = "step3"
                                                  , version = version
                                                  , firstOccurrence = False
                                                  }
                                                ]
                                          , outputs = []
                                          }
                                        )
                                )
                            >> Tuple.first
                in
                [ test "build step header lays out horizontally" <|
                    fetchPlanWithGetStep
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "header" ]
                        >> Query.has [ style [ ( "display", "flex" ) ] ]
                , test "has two children spread apart" <|
                    fetchPlanWithGetStep
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "header" ]
                        >> Expect.all
                            [ Query.has
                                [ style
                                    [ ( "justify-content", "space-between" ) ]
                                ]
                            , Query.children [] >> Query.count (Expect.equal 2)
                            ]
                , test "both children lay out horizontally" <|
                    fetchPlanWithGetStep
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.each
                            (Query.has [ style [ ( "display", "flex" ) ] ])
                , test "resource get step shows downward arrow" <|
                    fetchPlanWithGetStep
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-arrow-downward.svg"
                                }
                                ++ [ style [ ( "background-size", "14px 14px" ) ] ]
                            )
                , test "task step shows terminal icon" <|
                    fetchPlanWithTaskStep
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-terminal.svg"
                                }
                                ++ [ style
                                        [ ( "background-size", "14px 14px" ) ]
                                   ]
                            )
                , test "put step shows upward arrow" <|
                    fetchPlanWithPutStep
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-arrow-upward.svg"
                                }
                                ++ [ style
                                        [ ( "background-size", "14px 14px" ) ]
                                   ]
                            )
                , test "get step on first occurrence shows yellow downward arrow" <|
                    fetchPlanWithGetStepWithFirstOccurrence
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-arrow-downward-yellow.svg"
                                }
                                ++ [ style
                                        [ ( "background-size", "14px 14px" ) ]
                                   ]
                            )
                , test "hovering over a grey down arrow does nothing" <|
                    fetchPlanWithGetStep
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find
                            (iconSelector
                                { size = "28px"
                                , image = "ic-arrow-downward.svg"
                                }
                            )
                        >> Event.simulate Event.mouseEnter
                        >> Event.toResult
                        >> Expect.err
                , describe "yellow resource down arrow hover behaviour"
                    [ test "yellow resource down arrow has no tooltip" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.findAll
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-arrow-downward-yellow.svg"
                                    }
                                )
                            >> Query.first
                            >> Query.children []
                            >> Query.count (Expect.equal 0)
                    , test "hovering over yellow arrow triggers Hover message" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.findAll
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-arrow-downward-yellow.svg"
                                    }
                                )
                            >> Query.first
                            >> Event.simulate Event.mouseEnter
                            >> Event.expect
                                (Build.Msgs.Hover <| Just <| Models.FirstOccurrence "foo")
                    , test "no tooltip before 1 second has passed" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Build.update
                                (Build.Msgs.Hover <| Just <| Models.FirstOccurrence "foo")
                            >> Tuple.first
                            >> Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.findAll
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-arrow-downward-yellow.svg"
                                    }
                                )
                            >> Query.first
                            >> Query.children []
                            >> Query.count (Expect.equal 0)
                    , test "1 second after hovering, tooltip appears" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Build.update (Build.Msgs.ClockTick 0)
                            >> Tuple.first
                            >> Build.update
                                (Build.Msgs.Hover <| Just <| Models.FirstOccurrence "foo")
                            >> Tuple.first
                            >> Build.update (Build.Msgs.ClockTick 1)
                            >> Tuple.first
                            >> Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.findAll
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-arrow-downward-yellow.svg"
                                    }
                                )
                            >> Query.first
                            >> Query.has
                                [ style [ ( "position", "relative" ) ]
                                , containing
                                    [ containing
                                        [ text "new version" ]
                                    , style
                                        [ ( "position", "absolute" )
                                        , ( "left", "0" )
                                        , ( "bottom", "100%" )
                                        , ( "background-color", tooltipGreyHex )
                                        , ( "padding", "5px" )
                                        , ( "z-index", "100" )
                                        , ( "width", "6em" )
                                        , ( "pointer-events", "none" )
                                        , ( "cursor", "default" )
                                        , ( "user-select", "none" )
                                        , ( "-ms-user-select", "none" )
                                        , ( "-moz-user-select", "none" )
                                        , ( "-khtml-user-select", "none" )
                                        , ( "-webkit-user-select", "none" )
                                        , ( "-webkit-touch-callout", "none" )
                                        ]
                                    ]
                                , containing
                                    [ style
                                        [ ( "width", "0" )
                                        , ( "height", "0" )
                                        , ( "left", "50%" )
                                        , ( "margin-left", "-5px" )
                                        , ( "border-top"
                                          , "5px solid " ++ tooltipGreyHex
                                          )
                                        , ( "border-left", "5px solid transparent" )
                                        , ( "border-right", "5px solid transparent" )
                                        , ( "position", "absolute" )
                                        ]
                                    ]
                                ]
                    , test "mousing off yellow arrow triggers Hover message" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Build.update
                                (Build.Msgs.Hover <| Just <| Models.FirstOccurrence "foo")
                            >> Tuple.first
                            >> Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.findAll
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-arrow-downward-yellow.svg"
                                    }
                                )
                            >> Query.first
                            >> Event.simulate Event.mouseLeave
                            >> Event.expect
                                (Build.Msgs.Hover Nothing)
                    , test "unhovering after tooltip appears dismisses" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Build.update (Build.Msgs.ClockTick 0)
                            >> Tuple.first
                            >> Build.update
                                (Build.Msgs.Hover <| Just <| Models.FirstOccurrence "foo")
                            >> Tuple.first
                            >> Build.update (Build.Msgs.ClockTick 1)
                            >> Tuple.first
                            >> Build.update (Build.Msgs.Hover Nothing)
                            >> Tuple.first
                            >> Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.findAll
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-arrow-downward-yellow.svg"
                                    }
                                )
                            >> Query.first
                            >> Query.children []
                            >> Query.count (Expect.equal 0)
                    ]
                , test "hovering one resource of several produces only a single tooltip" <|
                    fetchPlanWithGetStepWithFirstOccurrence
                        >> Build.update (Build.Msgs.ClockTick 0)
                        >> Tuple.first
                        >> Build.update (Build.Msgs.Hover <| Just <| Models.FirstOccurrence "foo")
                        >> Tuple.first
                        >> Build.update (Build.Msgs.ClockTick 1)
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.findAll [ text "new version" ]
                        >> Query.count (Expect.equal 1)
                , test "successful step has a checkmark at the far right" <|
                    fetchPlanWithGetStep
                        >> Build.update (Build.Msgs.BuildEventsMsg Build.Msgs.Opened)
                        >> Tuple.first
                        >> Build.update
                            (Build.Msgs.BuildEventsMsg <|
                                Build.Msgs.Events <|
                                    Ok <|
                                        Array.fromList
                                            [ Models.FinishGet
                                                { source = "stdout", id = "plan" }
                                                0
                                                Dict.empty
                                                []
                                            ]
                            )
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-success-check.svg"
                                }
                                ++ [ style [ ( "background-size", "14px 14px" ) ] ]
                            )
                , test "get step lists resource version on the right" <|
                    fetchPlanWithGetStep
                        >> Build.update (Build.Msgs.BuildEventsMsg Build.Msgs.Opened)
                        >> Tuple.first
                        >> Build.update
                            (Build.Msgs.BuildEventsMsg <|
                                Build.Msgs.Events <|
                                    Ok <|
                                        Array.fromList
                                            [ Models.FinishGet
                                                { source = "stdout", id = "plan" }
                                                0
                                                (Dict.fromList [ ( "version", "v3.1.4" ) ])
                                                []
                                            ]
                            )
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has [ text "v3.1.4" ]
                , test "running step has loading spinner at the right" <|
                    fetchPlanWithTaskStep
                        >> Build.update
                            (Build.Msgs.BuildEventsMsg <|
                                Build.Msgs.Events <|
                                    Ok <|
                                        Array.fromList
                                            [ Models.StartTask
                                                { source = "stdout"
                                                , id = "plan"
                                                }
                                            ]
                            )
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            [ style
                                [ ( "animation"
                                  , "container-rotate 1568ms linear infinite"
                                  )
                                ]
                            ]
                , test "failing step has an X at the far right" <|
                    fetchPlanWithGetStep
                        >> Build.update (Build.Msgs.BuildEventsMsg Build.Msgs.Opened)
                        >> Tuple.first
                        >> Build.update
                            (Build.Msgs.BuildEventsMsg <|
                                Build.Msgs.Events <|
                                    Ok <|
                                        Array.fromList
                                            [ Models.FinishGet
                                                { source = "stdout", id = "plan" }
                                                1
                                                Dict.empty
                                                []
                                            ]
                            )
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-failure-times.svg"
                                }
                                ++ [ style [ ( "background-size", "14px 14px" ) ] ]
                            )
                , test "erroring step has orange exclamation triangle at right" <|
                    fetchPlanWithGetStep
                        >> Build.update (Build.Msgs.BuildEventsMsg Build.Msgs.Opened)
                        >> Tuple.first
                        >> Build.update
                            (Build.Msgs.BuildEventsMsg <|
                                Build.Msgs.Events <|
                                    Ok <|
                                        Array.fromList
                                            [ Models.Error
                                                { source = "stderr", id = "plan" }
                                                "error message"
                                            ]
                            )
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-exclamation-triangle.svg"
                                }
                                ++ [ style [ ( "background-size", "14px 14px" ) ] ]
                            )
                , describe "erroring build" <|
                    let
                        erroringBuild : () -> Models.Model
                        erroringBuild =
                            fetchPlanWithGetStep
                                >> Build.update
                                    (Build.Msgs.BuildEventsMsg Build.Msgs.Errored)
                                >> Tuple.first
                    in
                    [ test "has orange exclamation triangle at left" <|
                        erroringBuild
                            >> Build.update
                                (Build.Msgs.BuildEventsMsg <|
                                    Build.Msgs.Events <|
                                        Ok <|
                                            Array.fromList
                                                [ Models.BuildError
                                                    "error message"
                                                ]
                                )
                            >> Tuple.first
                            >> Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.find [ class "header" ]
                            >> Query.children []
                            >> Query.first
                            >> Query.has
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-exclamation-triangle.svg"
                                    }
                                    ++ [ style [ ( "background-size", "14px 14px" ) ] ]
                                )
                    , test "has passport officer icon" <|
                        let
                            url =
                                "/public/images/passport-officer-ic.svg"
                        in
                        erroringBuild
                            >> Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.find [ class "not-authorized" ]
                            >> Query.find [ tag "img" ]
                            >> Query.has [ attribute <| Attr.src url ]
                    ]
                ]
            ]
        ]


tooltipGreyHex : String
tooltipGreyHex =
    "#9b9b9b"
