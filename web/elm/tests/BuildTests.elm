module BuildTests exposing (all)

import Application.Application as Application
import Application.Msgs as Msgs
import Array
import Build.Build as Build
import Build.Models as Models
import Build.Msgs
import Build.StepTree.Models as STModels
import Callback
import Char
import Concourse exposing (BuildPrepStatus(..))
import Concourse.Pagination exposing (Direction(..))
import DashboardTests
    exposing
        ( defineHoverBehaviour
        , iconSelector
        , middleGrey
        )
import Date
import Dict
import Effects
import Expect
import Html.Attributes as Attr
import Keycodes
import Routes
import Subscription exposing (Delivery(..), Interval(..))
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
import Time
import UserState


all : Test
all =
    describe "build page" <|
        let
            buildId =
                { teamName = "team"
                , pipelineName = "pipeline"
                , jobName = "job"
                , buildName = "1"
                }

            pageLoad =
                Build.init
                    { highlight = Routes.HighlightNothing
                    , pageType = Models.JobBuildPage buildId
                    }

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
                    { startedAt = Just (Date.fromTime 0)
                    , finishedAt = Just (Date.fromTime 0)
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
                flip (,) []
                    >> (Build.handleCallback <| Callback.BuildFetched <| Ok ( 1, theBuild ))

            fetchStartedBuild :
                Models.Model
                -> ( Models.Model, List Effects.Effect )
            fetchStartedBuild =
                flip (,) []
                    >> (Build.handleCallback <| Callback.BuildFetched <| Ok ( 1, startedBuild ))

            fetchJobDetails :
                Models.Model
                -> ( Models.Model, List Effects.Effect )
            fetchJobDetails =
                flip (,) []
                    >> (Build.handleCallback <|
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
                       )

            fetchJobDetailsNoTrigger :
                Models.Model
                -> ( Models.Model, List Effects.Effect )
            fetchJobDetailsNoTrigger =
                flip (,) []
                    >> (Build.handleCallback <|
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
                       )

            fetchHistory : Models.Model -> ( Models.Model, List Effects.Effect )
            fetchHistory =
                flip (,) []
                    >> Build.handleCallback
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

            csrfToken : String
            csrfToken =
                "csrf_token"

            initFromApplication : Application.Model
            initFromApplication =
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = ""
                    , csrfToken = csrfToken
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
        in
        [ test "converts URL hash to highlighted line in view" <|
            \_ ->
                Application.init
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
                    , pathname = "/teams/t/pipelines/p/jobs/j/builds/307"
                    , search = ""
                    , hash = "#Lstepid:1"
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <|
                            Ok
                                ( 1
                                , { id = 307
                                  , name = "307"
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
                    |> Application.handleCallback
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
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout"
                                                , id = "stepid"
                                                }
                                      }
                                    ]
                        )
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.Log
                                                { source = "stdout"
                                                , id = "stepid"
                                                }
                                                "log message"
                                                Nothing
                                      }
                                    ]
                        )
                    |> Tuple.first
                    |> Application.view
                    |> Query.fromHtml
                    |> Query.find
                        [ class "timestamped-line"
                        , containing [ text "log message" ]
                        ]
                    |> Query.has [ class "highlighted-line" ]
        , test "events from a different build are discarded" <|
            \_ ->
                Application.init
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
                    |> Application.handleCallback
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
                    |> Application.handleCallback
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
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" }
                        }
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.Log { id = "stepid", source = "stdout" } "log message" Nothing
                        }
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/2/events"
                        , data = STModels.Log { id = "stepid", source = "stdout" } "bad message" Nothing
                        }
                    |> Tuple.first
                    |> Application.view
                    |> Query.fromHtml
                    |> Query.hasNot [ text "bad message" ]
        , test "when build is running it scrolls every build event" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" }
                        }
                    |> Tuple.second
                    |> Expect.equal [ ( Effects.SubPage 1, csrfToken, Effects.Scroll Effects.ToWindowBottom ) ]
        , test "when build is not running it does not scroll on build event" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <| Ok ( 1, theBuild ))
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" }
                        }
                    |> Tuple.second
                    |> Expect.equal []
        , test "when build is running but the user is not scrolled to the bottom it does not scroll on build event" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived (ScrolledToBottom False))
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" }
                        }
                    |> Tuple.second
                    |> Expect.equal []
        , test "when build is running but the user scrolls back to the bottom it scrolls on build event" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived (ScrolledToBottom False))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived (ScrolledToBottom True))
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" }
                        }
                    |> Tuple.second
                    |> Expect.equal [ ( Effects.SubPage 1, csrfToken, Effects.Scroll Effects.ToWindowBottom ) ]
        , test "pressing 'T' twice triggers two builds" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildJobDetailsFetched <|
                            Ok
                                { pipeline =
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                                , name = ""
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
                        )
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Keycodes.shift)
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Char.toCode 'T')
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyUp <| Char.toCode 'T')
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Char.toCode 'T')
                    |> Tuple.second
                    |> Expect.equal
                        [ ( Effects.SubPage 1
                          , csrfToken
                          , Effects.DoTriggerBuild
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                , jobName = "job"
                                }
                          )
                        ]
        , test "pressing 'gg' scrolls to the top" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Char.toCode 'G')
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Char.toCode 'G')
                    |> Tuple.second
                    |> Expect.equal
                        [ ( Effects.SubPage 1
                          , csrfToken
                          , Effects.Scroll Effects.ToWindowTop
                          )
                        ]
        , test "pressing 'G' scrolls to the bottom" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Keycodes.shift)
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Char.toCode 'G')
                    |> Tuple.second
                    |> Expect.equal
                        [ ( Effects.SubPage 1
                          , csrfToken
                          , Effects.Scroll Effects.ToWindowBottom
                          )
                        ]
        , test "pressing and releasing shift, then 'g', does nothing" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Keycodes.shift)
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyUp <| Keycodes.shift)
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Char.toCode 'G')
                    |> Tuple.second
                    |> Expect.equal []
        , test "pressing '?' shows the keyboard help" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Effects.SubPage 1)
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Keycodes.shift)
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| KeyDown <| Char.toCode '¿')
                    |> Tuple.first
                    |> Application.view
                    |> Query.fromHtml
                    |> Query.find [ class "keyboard-help" ]
                    |> Query.hasNot [ class "hidden" ]
        , test "says 'loading' on page load" <|
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
                        [ Effects.GetScreenSize
                        , Effects.GetCurrentTime
                        , Effects.CloseBuildEventStream
                        , Effects.FetchJobBuild 1
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = "job"
                            , buildName = "1"
                            }
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
            , test "when less than 24h old, shows relative time since build" <|
                \_ ->
                    initFromApplication
                        |> Application.handleCallback (Effects.SubPage 1) (Callback.BuildFetched <| Ok ( 1, theBuild ))
                        |> Tuple.first
                        |> Application.update (Msgs.DeliveryReceived <| ClockTicked OneSecond (2 * Time.second))
                        |> Tuple.first
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "build-header" ]
                        |> Query.has [ text "2s ago" ]
            , test "when at least 24h old, shows absolute time of build" <|
                \_ ->
                    initFromApplication
                        |> Application.handleCallback (Effects.SubPage 1) (Callback.BuildFetched <| Ok ( 1, theBuild ))
                        |> Tuple.first
                        |> Application.update (Msgs.DeliveryReceived <| ClockTicked OneSecond (24 * Time.hour))
                        |> Tuple.first
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "build-header" ]
                        |> Query.hasNot [ text "1d" ]
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
                [ test "trigger build button on right side of header " <|
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
                , test "if build is present in history, fetches no more" <|
                    givenBuildFetched
                        >> Tuple.mapSecond (always [])
                        >> Build.handleCallback
                            (Callback.BuildHistoryFetched
                                (Ok
                                    { pagination =
                                        { previousPage = Nothing
                                        , nextPage =
                                            Just
                                                { direction = Until 1
                                                , limit = 100
                                                }
                                        }
                                    , content = [ theBuild ]
                                    }
                                )
                            )
                        >> Tuple.second
                        >> Expect.equal []
                , test "if build is not present in history, fetches more" <|
                    givenBuildFetched
                        >> Tuple.mapSecond (always [])
                        >> Build.handleCallback
                            (Callback.BuildHistoryFetched
                                (Ok
                                    { pagination =
                                        { previousPage = Nothing
                                        , nextPage =
                                            Just
                                                { direction = Until 2
                                                , limit = 100
                                                }
                                        }
                                    , content =
                                        [ { id = 2
                                          , name = "2"
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
                                        ]
                                    }
                                )
                            )
                        >> Tuple.second
                        >> Expect.equal
                            [ Effects.FetchBuildHistory
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                , jobName = "job"
                                }
                                (Just { direction = Until 2, limit = 100 })
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
                    , updateFunc = \msg -> flip (,) [] >> Build.update msg >> Tuple.first
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
            , describe "when history and details fetched with manual triggering disabled" <|
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
                    , updateFunc = \msg -> flip (,) [] >> Build.update msg >> Tuple.first
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
                , updateFunc = \msg -> flip (,) [] >> Build.update msg >> Tuple.first
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
                        >> flip (,) []
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
                        >> flip (,) []
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
            , describe "build events subscription" <|
                let
                    buildPlanReceived _ =
                        pageLoad
                            |> Tuple.first
                            |> fetchStartedBuild
                            |> Tuple.first
                            |> fetchHistory
                            |> Tuple.first
                            |> fetchJobDetails
                            |> Tuple.first
                            |> flip (,) []
                            |> Build.handleCallback
                                (Callback.PlanAndResourcesFetched 1 <|
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
                in
                [ test "after build plan is received, opens event stream" <|
                    buildPlanReceived
                        >> Expect.all
                            [ Tuple.second
                                >> Expect.equal
                                    [ Effects.OpenBuildEventStream
                                        { url = "/api/v1/builds/1/events"
                                        , eventTypes = [ "end", "event" ]
                                        }
                                    ]
                            , Tuple.first
                                >> Build.subscriptions
                                >> List.member
                                    (Subscription.FromEventSource
                                        ( "/api/v1/builds/1/events"
                                        , [ "end", "event" ]
                                        )
                                    )
                                >> Expect.true
                                    "why aren't we listening for build events!?"
                            ]
                ]
            , describe "step header" <|
                let
                    fetchPlanWithGetStep : () -> Models.Model
                    fetchPlanWithGetStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> flip (,) []
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
                            >> flip (,) []
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
                            >> flip (,) []
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
                            >> flip (,) []
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
                            >> flip (,) []
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
                            >> flip (,) []
                            >> Build.handleDelivery (ClockTicked OneSecond 0)
                            >> Tuple.first
                            >> flip (,) []
                            >> Build.update
                                (Build.Msgs.Hover <| Just <| Models.FirstOccurrence "foo")
                            >> Tuple.first
                            >> flip (,) []
                            >> Build.handleDelivery (ClockTicked OneSecond 1)
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
                            >> flip (,) []
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
                            >> flip (,) []
                            >> Build.handleDelivery (ClockTicked OneSecond 0)
                            >> Tuple.first
                            >> flip (,) []
                            >> Build.update
                                (Build.Msgs.Hover <| Just <| Models.FirstOccurrence "foo")
                            >> Tuple.first
                            >> flip (,) []
                            >> Build.handleDelivery (ClockTicked OneSecond 1)
                            >> Tuple.first
                            >> flip (,) []
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
                        >> flip (,) []
                        >> Build.handleDelivery (ClockTicked OneSecond 0)
                        >> Tuple.first
                        >> flip (,) []
                        >> Build.update (Build.Msgs.Hover <| Just <| Models.FirstOccurrence "foo")
                        >> Tuple.first
                        >> flip (,) []
                        >> Build.handleDelivery (ClockTicked OneSecond 1)
                        >> Tuple.first
                        >> Build.view UserState.UserStateLoggedOut
                        >> Query.fromHtml
                        >> Query.findAll [ text "new version" ]
                        >> Query.count (Expect.equal 1)
                , test "successful step has a checkmark at the far right" <|
                    fetchPlanWithGetStep
                        >> flip (,) []
                        >> Build.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.FinishGet
                                                { source = "stdout", id = "plan" }
                                                0
                                                Dict.empty
                                                []
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
                            (iconSelector
                                { size = "28px"
                                , image = "ic-success-check.svg"
                                }
                                ++ [ style [ ( "background-size", "14px 14px" ) ] ]
                            )
                , test "get step lists resource version on the right" <|
                    fetchPlanWithGetStep
                        >> flip (,) []
                        >> Build.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.FinishGet
                                                { source = "stdout", id = "plan" }
                                                0
                                                (Dict.fromList [ ( "version", "v3.1.4" ) ])
                                                []
                                      }
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
                        >> flip (,) []
                        >> Build.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout"
                                                , id = "plan"
                                                }
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
                        >> flip (,) []
                        >> Build.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.FinishGet
                                                { source = "stdout", id = "plan" }
                                                1
                                                Dict.empty
                                                []
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
                            (iconSelector
                                { size = "28px"
                                , image = "ic-failure-times.svg"
                                }
                                ++ [ style
                                        [ ( "background-size", "14px 14px" ) ]
                                   ]
                            )
                , test "erroring step has orange exclamation triangle at right" <|
                    fetchPlanWithGetStep
                        >> flip (,) []
                        >> Build.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.Error
                                                { source = "stderr", id = "plan" }
                                                "error message"
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
                            (iconSelector
                                { size = "28px"
                                , image = "ic-exclamation-triangle.svg"
                                }
                                ++ [ style
                                        [ ( "background-size", "14px 14px" ) ]
                                   ]
                            )
                , describe "erroring build" <|
                    [ test "has orange exclamation triangle at left" <|
                        fetchPlanWithGetStep
                            >> flip (,) []
                            >> Build.handleDelivery
                                (EventsReceived <|
                                    Ok <|
                                        [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                          , data = STModels.Opened
                                          }
                                        ]
                                )
                            >> Build.handleDelivery
                                (EventsReceived <|
                                    Ok <|
                                        [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                          , data =
                                                STModels.BuildError
                                                    "error message"
                                          }
                                        ]
                                )
                            >> Tuple.first
                            >> Build.view UserState.UserStateLoggedOut
                            >> Query.fromHtml
                            >> Query.findAll [ class "header" ]
                            >> Query.first
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
                        fetchPlanWithGetStep
                            >> flip (,) []
                            >> Build.handleDelivery (EventsReceived <| Err "server burned down")
                            >> Tuple.first
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


receiveEvent :
    STModels.BuildEventEnvelope
    -> Application.Model
    -> ( Application.Model, List ( Effects.LayoutDispatch, Concourse.CSRFToken, Effects.Effect ) )
receiveEvent envelope =
    Application.update (Msgs.DeliveryReceived <| EventsReceived <| Ok [ envelope ])
