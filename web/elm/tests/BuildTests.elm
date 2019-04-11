module BuildTests exposing (all)

import Application.Application as Application
import Array
import Build.Build as Build
import Build.Models as Models
import Build.StepTree.Models as STModels
import Char
import Common
import Concourse exposing (BuildPrepStatus(..))
import Concourse.Pagination exposing (Direction(..))
import DashboardTests
    exposing
        ( defineHoverBehaviour
        , iconSelector
        , isColorWithStripes
        , middleGrey
        )
import Dict
import Expect
import Html.Attributes as Attr
import Http
import Keyboard
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message
import Message.Subscription as Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as Msgs
import Routes
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
import Url
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

            flags =
                { turbulenceImgSrc = ""
                , notFoundImgSrc = ""
                , csrfToken = csrfToken
                , authToken = ""
                , instanceName = ""
                , pipelineRunningKeyframes = ""
                }

            pageLoadJobBuild =
                Common.init
                    "/teams/team/pipelines/pipeline/jobs/job/builds/1"

            pageLoadOneOffBuild =
                Common.init "/builds/1"

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
                    { startedAt = Just <| Time.millisToPosix 0
                    , finishedAt = Just <| Time.millisToPosix 0
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
                    { startedAt = Just <| Time.millisToPosix 0
                    , finishedAt = Just <| Time.millisToPosix 0
                    }
                , reapTime = Nothing
                }

            fetchBuild : Application.Model -> ( Application.Model, List Effects.Effect )
            fetchBuild =
                Application.handleCallback <|
                    Callback.BuildFetched <|
                        Ok ( 1, theBuild )

            fetchBuildWithStatus :
                Concourse.BuildStatus
                -> Application.Model
                -> Application.Model
            fetchBuildWithStatus status =
                Application.handleCallback
                    (Callback.BuildFetched
                        (Ok
                            ( 1
                            , { id = 1
                              , name = "1"
                              , job = Nothing
                              , status = status
                              , duration =
                                    { startedAt = Nothing
                                    , finishedAt = Nothing
                                    }
                              , reapTime = Nothing
                              }
                            )
                        )
                    )
                    >> Tuple.first
                    >> Application.handleCallback
                        (Callback.BuildHistoryFetched
                            (Ok
                                { pagination =
                                    { previousPage = Nothing
                                    , nextPage = Nothing
                                    }
                                , content =
                                    [ { id = 0
                                      , name = "0"
                                      , job = Nothing
                                      , status = status
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
                    >> Tuple.first

            fetchStartedBuild :
                Application.Model
                -> ( Application.Model, List Effects.Effect )
            fetchStartedBuild =
                Application.handleCallback <|
                    Callback.BuildFetched <|
                        Ok ( 1, startedBuild )

            fetchJobDetails :
                Application.Model
                -> ( Application.Model, List Effects.Effect )
            fetchJobDetails =
                Application.handleCallback <|
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
                Application.Model
                -> ( Application.Model, List Effects.Effect )
            fetchJobDetailsNoTrigger =
                Application.handleCallback <|
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

            fetchHistory :
                Application.Model
                -> ( Application.Model, List Effects.Effect )
            fetchHistory =
                Application.handleCallback
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
                Common.init "/teams/t/pipelines/p/jobs/j/builds/1"
        in
        [ test "converts URL hash to highlighted line in view" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/teams/t/pipelines/p/jobs/j/builds/307"
                    , query = Nothing
                    , fragment = Just "Lstepid:1"
                    }
                    |> Tuple.first
                    |> Application.handleCallback
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
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = "http://localhost:8080/api/v1/builds/307/events"
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
                    |> Common.queryView
                    |> Query.find
                        [ class "timestamped-line"
                        , containing [ text "log message" ]
                        ]
                    |> Query.has [ class "highlighted-line" ]
        , describe "page title"
            [ test "with a job build" <|
                \_ ->
                    Application.init
                        flags
                        { protocol = Url.Http
                        , host = ""
                        , port_ = Nothing
                        , path = "/builds/1"
                        , query = Nothing
                        , fragment = Just "Lstepid:1"
                        }
                        |> Tuple.first
                        |> fetchBuild
                        |> Tuple.first
                        |> Application.view
                        |> .title
                        |> Expect.equal "job #1 - Concourse"
            , test "with a one-off-build" <|
                \_ ->
                    Application.init
                        flags
                        { protocol = Url.Http
                        , host = ""
                        , port_ = Nothing
                        , path = "/builds/1"
                        , query = Nothing
                        , fragment = Just "Lstepid:1"
                        }
                        |> Tuple.first
                        |> fetchBuildWithStatus Concourse.BuildStatusFailed
                        |> Application.view
                        |> .title
                        |> Expect.equal "#1 - Concourse"
            , test "with just the page name" <|
                \_ ->
                    Application.init
                        flags
                        { protocol = Url.Http
                        , host = ""
                        , port_ = Nothing
                        , path = "/teams/team/pipelines/pipeline/jobs/routejob/builds/1"
                        , query = Nothing
                        , fragment = Just "Lstepid:1"
                        }
                        |> Tuple.first
                        |> Application.view
                        |> .title
                        |> Expect.equal "routejob #1 - Concourse"
            ]
        , test "shows tombstone for reaped build with date in current zone" <|
            \_ ->
                let
                    buildTime =
                        Just <|
                            Time.millisToPosix
                                (12 * 60 * 60 * 1000)
                in
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                ( 1
                                , { id = 1
                                  , name = "1"
                                  , job =
                                        Just
                                            { teamName = "team"
                                            , pipelineName = "pipeline"
                                            , jobName = "job"
                                            }
                                  , status = Concourse.BuildStatusSucceeded
                                  , duration =
                                        { startedAt = buildTime
                                        , finishedAt = buildTime
                                        }
                                  , reapTime = buildTime
                                  }
                                )
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotCurrentTimeZone <|
                            Time.customZone 720 []
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "tombstone" ]
                    |> Query.has [ text "01/02/70" ]
        , test "shows passport officer when build plan request gives 401" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok ( 1, theBuild ))
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.PlanAndResourcesFetched 1 <|
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
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ class "not-authorized" ]
        , test "shows passport officer when build prep request gives 401" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                ( 1
                                , { id = 1
                                  , name = "1"
                                  , job =
                                        Just
                                            { teamName = "team"
                                            , pipelineName = "pipeline"
                                            , jobName = "job"
                                            }
                                  , status = Concourse.BuildStatusPending
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
                        (Callback.BuildPrepFetched <|
                            Err <|
                                Http.BadStatus
                                    { status =
                                        { code = 401
                                        , message = "Unauthorized"
                                        }
                                    , headers =
                                        Dict.empty
                                    , url = ""
                                    , body = "not authorized"
                                    }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ class "not-authorized" ]
        , test "events from a different build are discarded" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:1"
                    }
                    |> Tuple.first
                    |> Application.handleCallback
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
                        , data = STModels.StartTask { id = "stepid", source = "" } (Time.millisToPosix 0)
                        }
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data =
                            STModels.Log
                                { id = "stepid"
                                , source = "stdout"
                                }
                                "log message"
                                Nothing
                        }
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/2/events"
                        , data =
                            STModels.Log
                                { id = "stepid"
                                , source = "stdout"
                                }
                                "bad message"
                                Nothing
                        }
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "bad message" ]
        , test "log lines have timestamps in current zone" <|
            \_ ->
                Common.init "/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                ( 1
                                , { id = 1
                                  , name = "1"
                                  , job = Nothing
                                  , status = Concourse.BuildStatusStarted
                                  , duration =
                                        { startedAt =
                                            Just <| Time.millisToPosix 0
                                        , finishedAt = Nothing
                                        }
                                  , reapTime = Nothing
                                  }
                                )
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.PlanAndResourcesFetched 1 <|
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
                        , data =
                            STModels.StartTask
                                { id = "stepid"
                                , source = ""
                                }
                                (Time.millisToPosix 0)
                        }
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data =
                            STModels.Log
                                { id = "stepid"
                                , source = "stdout"
                                }
                                "log message\n"
                                (Just <| Time.millisToPosix 0)
                        }
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.GotCurrentTimeZone <|
                            Time.customZone (5 * 60) []
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.findAll [ class "timestamped-line" ]
                    |> Query.first
                    |> Query.has [ text "05:00:00" ]
        , test "when build is running it scrolls every build event" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" } (Time.millisToPosix 0)
                        }
                    |> Tuple.second
                    |> Expect.equal [ Effects.Scroll Effects.ToBottom ]
        , test "when build is not running it does not scroll on build event" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok ( 1, theBuild ))
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" } (Time.millisToPosix 0)
                        }
                    |> Tuple.second
                    |> Expect.equal []
        , test "when build is running but the user is not scrolled to the bottom it does not scroll on build event" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived (ScrolledToBottom False))
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" } (Time.millisToPosix 0)
                        }
                    |> Tuple.second
                    |> Expect.equal []
        , test "when build is running but the user scrolls back to the bottom it scrolls on build event" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
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
                        , data = STModels.StartTask { id = "stepid", source = "" } (Time.millisToPosix 0)
                        }
                    |> Tuple.second
                    |> Expect.equal [ Effects.Scroll Effects.ToBottom ]
        , test "pressing 'T' twice triggers two builds" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.handleCallback
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
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyDown <|
                                { ctrlKey = False
                                , shiftKey = True
                                , metaKey = False
                                , code = Keyboard.T
                                }
                        )
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyUp <|
                                { ctrlKey = False
                                , shiftKey = False
                                , metaKey = False
                                , code = Keyboard.T
                                }
                        )
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyDown <|
                                { ctrlKey = False
                                , shiftKey = True
                                , metaKey = False
                                , code = Keyboard.T
                                }
                        )
                    |> Tuple.second
                    |> Expect.equal
                        [ Effects.DoTriggerBuild
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = "job"
                            }
                        ]
        , test "pressing 'gg' scrolls to the top" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyDown <|
                                { ctrlKey = False
                                , shiftKey = False
                                , metaKey = False
                                , code = Keyboard.G
                                }
                        )
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyDown <|
                                { ctrlKey = False
                                , shiftKey = False
                                , metaKey = False
                                , code = Keyboard.G
                                }
                        )
                    |> Tuple.second
                    |> Expect.equal [ Effects.Scroll Effects.ToTop ]
        , test "pressing 'G' scrolls to the bottom" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyDown
                                { ctrlKey = False
                                , shiftKey = True
                                , metaKey = False
                                , code = Keyboard.G
                                }
                        )
                    |> Tuple.second
                    |> Expect.equal [ Effects.Scroll Effects.ToBottom ]
        , test "pressing 'g' once does nothing" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyDown <|
                                { ctrlKey = False
                                , shiftKey = False
                                , metaKey = False
                                , code = Keyboard.G
                                }
                        )
                    |> Tuple.second
                    |> Expect.equal []
        , test "pressing '?' shows the keyboard help" <|
            \_ ->
                initFromApplication
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok ( 1, startedBuild ))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyDown <|
                                { ctrlKey = False
                                , shiftKey = True
                                , metaKey = False
                                , code = Keyboard.Slash
                                }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "keyboard-help" ]
                    |> Query.hasNot [ class "hidden" ]
        , test "says 'loading' on page load" <|
            \_ ->
                pageLoadJobBuild
                    |> Common.queryView
                    |> Query.has [ text "loading" ]
        , test "fetches build on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , instanceName = ""
                    , pipelineRunningKeyframes = "pipeline-running"
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> List.member
                        (Effects.FetchJobBuild 1
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = "job"
                            , buildName = "1"
                            }
                        )
                    |> Expect.true "should fetch build"
        , test "gets current timezone on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , instanceName = ""
                    , pipelineRunningKeyframes = "pipeline-running"
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> List.member Effects.GetCurrentTimeZone
                    |> Expect.true "should get timezone"
        , describe "top bar" <|
            [ test "has a top bar" <|
                \_ ->
                    pageLoadJobBuild
                        |> Common.queryView
                        |> Query.has [ id "top-bar-app" ]
            , test "has a concourse icon" <|
                \_ ->
                    pageLoadJobBuild
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style "background-image"
                                "url(/public/images/concourse-logo-white.svg)"
                            ]
            , test "has the breadcrumbs" <|
                \_ ->
                    pageLoadJobBuild
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Expect.all
                            [ Query.has [ id "breadcrumb-pipeline" ]
                            , Query.has [ text "pipeline" ]
                            , Query.has [ id "breadcrumb-job" ]
                            , Query.has [ text "job" ]
                            ]
            , test "has the breadcrumbs after fetching build" <|
                \_ ->
                    pageLoadOneOffBuild
                        |> fetchBuild
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Expect.all
                            [ Query.has [ id "breadcrumb-pipeline" ]
                            , Query.has [ text "pipeline" ]
                            , Query.has [ id "breadcrumb-job" ]
                            , Query.has [ text "job" ]
                            ]
            , test "has a user section" <|
                \_ ->
                    pageLoadJobBuild
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ id "login-component" ]
            ]
        , test "page below top bar has padding to accomodate top bar" <|
            \_ ->
                pageLoadJobBuild
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has [ style "padding-top" "54px" ]
        , describe "after build is fetched" <|
            let
                givenBuildFetched _ =
                    pageLoadJobBuild |> fetchBuild
            in
            [ test "has a header after the build is fetched" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.has [ id "build-header" ]
            , test "page body has padding to accomodate header" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find [ id "build-body" ]
                    >> Query.has [ style "padding-top" "104px" ]
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
            , test "header is 60px tall" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find [ id "build-header" ]
                    >> Query.has [ style "height" "60px" ]
            , test "header lays out horizontally" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find [ id "build-header" ]
                    >> Query.has [ style "display" "flex" ]
            , test "when less than 24h old, shows relative time since build" <|
                \_ ->
                    initFromApplication
                        |> Application.handleCallback (Callback.BuildFetched <| Ok ( 1, theBuild ))
                        |> Tuple.first
                        |> Application.update
                            (Msgs.DeliveryReceived <|
                                ClockTicked
                                    OneSecond
                                    (Time.millisToPosix (2 * 1000))
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "build-header" ]
                        |> Query.has [ text "2s ago" ]
            , test "when at least 24h old, shows absolute time of build" <|
                \_ ->
                    initFromApplication
                        |> Application.handleCallback
                            (Callback.BuildFetched <|
                                Ok ( 1, theBuild )
                            )
                        |> Tuple.first
                        |> Application.update
                            (Msgs.DeliveryReceived <|
                                ClockTicked
                                    OneSecond
                                    (Time.millisToPosix (24 * 60 * 60 * 1000))
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "build-header" ]
                        |> Query.has [ text "Jan 1 1970 12:00:00 AM" ]
            , test "when at least 24h old, absolute time is in current zone" <|
                \_ ->
                    initFromApplication
                        |> Application.handleCallback
                            (Callback.GotCurrentTimeZone <|
                                Time.customZone (5 * 60) []
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.BuildFetched <|
                                Ok ( 1, theBuild )
                            )
                        |> Tuple.first
                        |> Application.update
                            (Msgs.DeliveryReceived <|
                                ClockTicked
                                    OneSecond
                                    (Time.millisToPosix (24 * 60 * 60 * 1000))
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "build-header" ]
                        |> Query.has [ text "Jan 1 1970 05:00:00 AM" ]
            , describe "build banner coloration"
                [ test "pending build has grey banner" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusPending
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#9b9b9b" ]
                , test "started build has yellow banner" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusStarted
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#f1c40f" ]
                , test "succeeded build has green banner" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusSucceeded
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#11c560" ]
                , test "failed build has red banner" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusFailed
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#ed4b35" ]
                , test "errored build has amber banner" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusErrored
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#f5a623" ]
                , test "aborted build has brown banner" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusAborted
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#8b572a" ]
                ]
            , describe "build history tab coloration"
                [ test "pending build has grey tab in build history" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusPending
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> Query.has [ style "background" "#9b9b9b" ]
                , test "started build has animated striped yellow tab in build history" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusStarted
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> isColorWithStripes { thick = "#f1c40f", thin = "#fad43b" }
                , test "succeeded build has green tab in build history" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusSucceeded
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> Query.has [ style "background" "#11c560" ]
                , test "failed build has red tab in build history" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusFailed
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> Query.has [ style "background" "#ed4b35" ]
                , test "errored build has amber tab in build history" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusErrored
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> Query.has [ style "background" "#f5a623" ]
                , test "aborted build has brown tab in build history" <|
                    \_ ->
                        pageLoadJobBuild
                            |> fetchBuildWithStatus Concourse.BuildStatusAborted
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> Query.has [ style "background" "#8b572a" ]
                ]
            , test "header spreads out contents" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find [ id "build-header" ]
                    >> Query.has [ style "justify-content" "space-between" ]
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
                        >> Common.queryView
                        >> Query.find [ id "build-header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                , test "pressing 'L' switches to the next build" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                                    , content =
                                        [ theBuild
                                        , { id = 2
                                          , name = "2"
                                          , job =
                                                Just
                                                    { teamName = "team"
                                                    , pipelineName = "pipeline"
                                                    , jobName = "job"
                                                    }
                                          , status = Concourse.BuildStatusSucceeded
                                          , duration =
                                                { startedAt = Just <| Time.millisToPosix 0
                                                , finishedAt = Just <| Time.millisToPosix 0
                                                }
                                          , reapTime = Nothing
                                          }
                                        ]
                                    }
                                )
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (KeyDown
                                { ctrlKey = False
                                , shiftKey = False
                                , metaKey = False
                                , code = Keyboard.L
                                }
                            )
                        >> Tuple.second
                        >> Expect.equal
                            [ Effects.NavigateTo
                                "/teams/team/pipelines/pipeline/jobs/job/builds/2"
                            ]
                , test "pressing Command-L does nothing" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                                    , content =
                                        [ theBuild
                                        , { id = 2
                                          , name = "2"
                                          , job =
                                                Just
                                                    { teamName = "team"
                                                    , pipelineName = "pipeline"
                                                    , jobName = "job"
                                                    }
                                          , status = Concourse.BuildStatusSucceeded
                                          , duration =
                                                { startedAt = Just <| Time.millisToPosix 0
                                                , finishedAt = Just <| Time.millisToPosix 0
                                                }
                                          , reapTime = Nothing
                                          }
                                        ]
                                    }
                                )
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (KeyDown
                                { ctrlKey = False
                                , shiftKey = False
                                , metaKey = True
                                , code = Keyboard.L
                                }
                            )
                        >> Tuple.second
                        >> Expect.equal []
                , test "pressing Control-L does nothing" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                                    , content =
                                        [ theBuild
                                        , { id = 2
                                          , name = "2"
                                          , job =
                                                Just
                                                    { teamName = "team"
                                                    , pipelineName = "pipeline"
                                                    , jobName = "job"
                                                    }
                                          , status = Concourse.BuildStatusSucceeded
                                          , duration =
                                                { startedAt = Just <| Time.millisToPosix 0
                                                , finishedAt = Just <| Time.millisToPosix 0
                                                }
                                          , reapTime = Nothing
                                          }
                                        ]
                                    }
                                )
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (KeyDown
                                { ctrlKey = True
                                , shiftKey = False
                                , metaKey = False
                                , code = Keyboard.L
                                }
                            )
                        >> Tuple.second
                        >> Expect.equal []
                , test "scrolling builds checks if last build is visible" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                        >> Tuple.first
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.ScrollBuilds
                                    { deltaX = 0, deltaY = 0 }
                            )
                        >> Tuple.second
                        >> List.member (Effects.CheckIsVisible "1")
                        >> Expect.true "should check if last build is visible"
                , test "subscribes to element visibility" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.subscriptions
                        >> List.member Subscription.OnElementVisible
                        >> Expect.true "should be subscribed to visibility"
                , test "scrolling to last build fetches more if possible" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.ElementVisible ( "1", True ))
                        >> Tuple.second
                        >> Expect.equal
                            [ Effects.FetchBuildHistory
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                , jobName = "job"
                                }
                                (Just { direction = Until 1, limit = 100 })
                            ]
                , test "scrolling to last build while fetching fetches no more" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.ElementVisible ( "1", True ))
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.ElementVisible ( "1", True ))
                        >> Tuple.second
                        >> Expect.equal []
                , test "scrolling to absolute last build fetches no more" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.ElementVisible ( "1", True ))
                        >> Tuple.second
                        >> Expect.equal []
                , test "if build is present in history, fetches no more" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                        >> Tuple.first
                        >> Application.handleCallback
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
                        >> List.member
                            (Effects.FetchBuildHistory
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                , jobName = "job"
                                }
                                (Just { direction = Until 2, limit = 100 })
                            )
                        >> Expect.false "should not fetch more builds"
                , test "if build is present in history, checks its visibility" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                        >> List.member (Effects.CheckIsVisible "1")
                        >> Expect.true "should check visibility of current build"
                , test "if build is present and invisible, scrolls to it" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.ElementVisible ( "1", False ))
                        >> Tuple.second
                        >> Expect.equal [ Effects.Scroll <| Effects.ToId "1" ]
                , test "does not scroll to current build more than once" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.ElementVisible ( "1", False ))
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.ElementVisible ( "1", False ))
                        >> Tuple.second
                        >> Expect.equal []
                , test "if build is not present in history, fetches more" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
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
                , test "trigger build button is styled as a box of the color of the build status" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                        >> Query.has
                            [ style "padding" "10px"
                            , style "background-color" brightGreen
                            , style "outline" "none"
                            , style "margin" "0"
                            , style "border-width" "0 0 0 1px"
                            , style "border-color" darkGrey
                            , style "border-style" "solid"
                            ]
                , test "hovered trigger build button is styled as a box of the secondary color of the build status" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.Hover <|
                                    Just Message.Message.TriggerBuildButton
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                        >> Query.has
                            [ style "padding" "10px"
                            , style "background-color" darkGreen
                            , style "outline" "none"
                            , style "margin" "0"
                            , style "border-width" "0 0 0 1px"
                            , style "border-color" darkGrey
                            , style "border-style" "solid"
                            ]
                , test "trigger build button has pointer cursor" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                        >> Query.has [ style "cursor" "pointer" ]
                , test "trigger build button has 'plus' icon" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Common.queryView
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
                        >> Common.queryView
                        >> Query.find
                            [ attribute <|
                                Attr.attribute "aria-label" "Trigger Build"
                            ]
                        >> Query.has [ style "cursor" "default" ]
                , defineHoverBehaviour
                    { name = "disabled trigger build button"
                    , setup =
                        givenHistoryAndDetailsFetched () |> Tuple.first
                    , query =
                        Common.queryView
                            >> Query.find
                                [ attribute <|
                                    Attr.attribute "aria-label" "Trigger Build"
                                ]
                    , updateFunc =
                        \msg ->
                            Application.update msg
                                >> Tuple.first
                    , unhoveredSelector =
                        { description = "grey plus icon"
                        , selector =
                            iconSelector
                                { size = "40px"
                                , image = "ic-add-circle-outline-white.svg"
                                }
                        }
                    , hoveredSelector =
                        { description = "grey plus icon with tooltip"
                        , selector =
                            [ style "position" "relative"
                            , containing
                                [ containing
                                    [ text "manual triggering disabled in job config" ]
                                , style "position" "absolute"
                                , style "right" "100%"
                                , style "top" "15px"
                                , style "width" "300px"
                                , style "color" "#ecf0f1"
                                , style "font-size" "12px"
                                , style "font-family" "Inconsolata,monospace"
                                , style "padding" "10px"
                                , style "text-align" "right"
                                ]
                            , containing <|
                                iconSelector
                                    { size = "40px"
                                    , image = "ic-add-circle-outline-white.svg"
                                    }
                            ]
                        }
                    , mouseEnterMsg =
                        Msgs.Update <|
                            Message.Message.Hover <|
                                Just Message.Message.TriggerBuildButton
                    , mouseLeaveMsg =
                        Msgs.Update <|
                            Message.Message.Hover Nothing
                    }
                ]
            ]
        , describe "given build started and history and details fetched" <|
            let
                givenBuildStarted _ =
                    pageLoadJobBuild
                        |> fetchBuildWithStatus Concourse.BuildStatusStarted
                        |> fetchHistory
                        |> Tuple.first
                        |> fetchJobDetails
            in
            [ test "build action section lays out horizontally" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find [ id "build-header" ]
                    >> Query.children []
                    >> Query.index -1
                    >> Query.has [ style "display" "flex" ]
            , test "abort build button is to the left of the trigger button" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find [ id "build-header" ]
                    >> Query.children []
                    >> Query.index -1
                    >> Query.children []
                    >> Query.first
                    >> Query.has
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
            , test "abort build button is styled as a bright red box" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
                    >> Query.has
                        [ style "padding" "10px"
                        , style "background-color" brightRed
                        , style "outline" "none"
                        , style "margin" "0"
                        , style "border-width" "0 0 0 1px"
                        , style "border-color" darkGrey
                        , style "border-style" "solid"
                        ]
            , test "hovered abort build button is styled as a dark red box" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Application.update
                        (Msgs.Update <|
                            Message.Message.Hover <|
                                Just Message.Message.AbortBuildButton
                        )
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
                    >> Query.has
                        [ style "padding" "10px"
                        , style "background-color" darkRed
                        , style "outline" "none"
                        , style "margin" "0"
                        , style "border-width" "0 0 0 1px"
                        , style "border-color" darkGrey
                        , style "border-style" "solid"
                        ]
            , test "abort build button has pointer cursor" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
                    >> Query.has [ style "cursor" "pointer" ]
            , test "abort build button has 'X' icon" <|
                givenBuildStarted
                    >> Tuple.first
                    >> Common.queryView
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
                        >> Application.handleCallback
                            (Callback.BuildPrepFetched <| Ok ( 1, prep ))
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "prep-status-list" ]
                        >> Expect.all
                            [ Query.children []
                                >> Query.each
                                    (Query.children []
                                        >> Query.first
                                        >> Query.has
                                            [ style "display" "flex"
                                            , style "align-items" "center"
                                            ]
                                    )
                            , Query.has
                                [ style "background-image" icon
                                , style "background-position" "50% 50%"
                                , style "background-repeat" "no-repeat"
                                , style "background-size" "contain"
                                , style "width" "12px"
                                , style "height" "12px"
                                , style "margin-right" "5px"
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
                        >> Application.handleCallback
                            (Callback.BuildPrepFetched <| Ok ( 1, prep ))
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "prep-status-list" ]
                        >> Expect.all
                            [ Query.children []
                                >> Query.each
                                    (Query.children []
                                        >> Query.first
                                        >> Query.has
                                            [ style "display" "flex"
                                            , style "align-items" "center"
                                            ]
                                    )
                            , Query.has
                                [ style "animation"
                                    "container-rotate 1568ms linear infinite"
                                , style "height" "12px"
                                , style "width" "12px"
                                , style "margin" "0 5px 0 0"
                                ]
                            , Query.has [ attribute <| Attr.title "blocking" ]
                            ]
                , test "when paused state is unknown, shows a spinner" <|
                    let
                        prep =
                            { pausedPipeline = BuildPrepStatusUnknown
                            , pausedJob = BuildPrepStatusNotBlocking
                            , maxRunningBuilds = BuildPrepStatusNotBlocking
                            , inputs = Dict.empty
                            , inputsSatisfied = BuildPrepStatusNotBlocking
                            , missingInputReasons = Dict.empty
                            }
                    in
                    givenBuildStarted
                        >> Tuple.first
                        >> Application.handleCallback
                            (Callback.BuildPrepFetched <| Ok ( 1, prep ))
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "prep-status-list" ]
                        >> Expect.all
                            [ Query.children []
                                >> Query.each
                                    (Query.children []
                                        >> Query.first
                                        >> Query.has
                                            [ style "display" "flex"
                                            , style "align-items" "center"
                                            ]
                                    )
                            , Query.has
                                [ style "animation"
                                    "container-rotate 1568ms linear infinite"
                                , style "height" "12px"
                                , style "width" "12px"
                                , style "margin" "0 5px 0 0"
                                ]
                            , Query.has [ attribute <| Attr.title "thinking..." ]
                            ]
                ]
            , describe "build events subscription" <|
                let
                    preBuildPlanReceived _ =
                        pageLoadJobBuild
                            |> fetchStartedBuild
                            |> Tuple.first
                            |> fetchHistory
                            |> Tuple.first
                            |> fetchJobDetails
                            |> Tuple.first
                in
                [ test "after build plan is received, opens event stream" <|
                    preBuildPlanReceived
                        >> Application.handleCallback
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
                        >> Expect.all
                            [ Tuple.second
                                >> Expect.equal
                                    [ Effects.OpenBuildEventStream
                                        { url = "/api/v1/builds/1/events"
                                        , eventTypes = [ "end", "event" ]
                                        }
                                    ]
                            , Tuple.first
                                >> Application.subscriptions
                                >> List.member
                                    (Subscription.FromEventSource
                                        ( "/api/v1/builds/1/events"
                                        , [ "end", "event" ]
                                        )
                                    )
                                >> Expect.true
                                    "why aren't we listening for build events!?"
                            ]
                , test "if build plan request fails, no event stream" <|
                    preBuildPlanReceived
                        >> Application.handleCallback
                            (Callback.PlanAndResourcesFetched 1 <|
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
                        >> Expect.all
                            [ Tuple.second >> Expect.equal []
                            , Tuple.first
                                >> Application.subscriptions
                                >> List.member
                                    (Subscription.FromEventSource
                                        ( "/api/v1/builds/1/events"
                                        , [ "end", "event" ]
                                        )
                                    )
                                >> Expect.false
                                    "should not listen for build events"
                            ]
                ]
            , describe "step header" <|
                let
                    fetchPlanWithGetStep : () -> Application.Model
                    fetchPlanWithGetStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Application.handleCallback
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

                    fetchPlanWithArtifactInputStep : () -> Application.Model
                    fetchPlanWithArtifactInputStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Application.handleCallback
                                (Callback.PlanAndResourcesFetched 307 <|
                                    Ok <|
                                        ( { id = "plan"
                                          , step =
                                                Concourse.BuildStepArtifactInput
                                                    "step"
                                          }
                                        , { inputs = [], outputs = [] }
                                        )
                                )
                            >> Tuple.first

                    fetchPlanWithEnsureArtifactOutputStep : () -> Application.Model
                    fetchPlanWithEnsureArtifactOutputStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Application.handleCallback
                                (Callback.PlanAndResourcesFetched 307 <|
                                    Ok <|
                                        ( { id = "plan"
                                          , step =
                                                Concourse.BuildStepEnsure
                                                    { hook =
                                                        { id = "plan1"
                                                        , step =
                                                            Concourse.BuildStepArtifactOutput
                                                                "step"
                                                        }
                                                    , step =
                                                        { id = "plan2"
                                                        , step =
                                                            Concourse.BuildStepGet
                                                                "step"
                                                                Nothing
                                                        }
                                                    }
                                          }
                                        , { inputs = [], outputs = [] }
                                        )
                                )
                            >> Tuple.first

                    fetchPlanWithTaskStep : () -> Application.Model
                    fetchPlanWithTaskStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Application.handleCallback
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

                    fetchPlanWithPutStep : () -> Application.Model
                    fetchPlanWithPutStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Application.handleCallback
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

                    fetchPlanWithGetStepWithFirstOccurrence :
                        ()
                        -> Application.Model
                    fetchPlanWithGetStepWithFirstOccurrence =
                        let
                            version =
                                Dict.fromList
                                    [ ( "ref", "abc123" ) ]

                            step =
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
                        in
                        givenBuildStarted
                            >> Tuple.first
                            >> Application.handleCallback
                                (Callback.PlanAndResourcesFetched 307 <|
                                    Ok <|
                                        ( { id = "plan", step = step }
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
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.has [ style "display" "flex" ]
                , test "has two children spread apart" <|
                    fetchPlanWithGetStep
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Expect.all
                            [ Query.has
                                [ style "justify-content" "space-between" ]
                            , Query.children [] >> Query.count (Expect.equal 2)
                            ]
                , test "both children lay out horizontally" <|
                    fetchPlanWithGetStep
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.each
                            (Query.has [ style "display" "flex" ])
                , test "resource get step shows downward arrow" <|
                    fetchPlanWithGetStep
                        >> Common.queryView
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-arrow-downward.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "artifact input step shows downward arrow" <|
                    fetchPlanWithArtifactInputStep
                        >> Common.queryView
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-arrow-downward.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "task step shows terminal icon" <|
                    fetchPlanWithTaskStep
                        >> Common.queryView
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-terminal.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "artifact output step shows upward arrow" <|
                    fetchPlanWithEnsureArtifactOutputStep
                        >> Common.queryView
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-arrow-upward.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "put step shows upward arrow" <|
                    fetchPlanWithPutStep
                        >> Common.queryView
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-arrow-upward.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "get step on first occurrence shows yellow downward arrow" <|
                    fetchPlanWithGetStepWithFirstOccurrence
                        >> Common.queryView
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-arrow-downward-yellow.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "hovering over a grey down arrow does nothing" <|
                    fetchPlanWithGetStep
                        >> Common.queryView
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
                            >> Common.queryView
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
                            >> Common.queryView
                            >> Query.findAll
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-arrow-downward-yellow.svg"
                                    }
                                )
                            >> Query.first
                            >> Event.simulate Event.mouseEnter
                            >> Event.expect
                                (Msgs.Update <|
                                    Message.Message.Hover <|
                                        Just <|
                                            Message.Message.FirstOccurrenceIcon
                                                "foo"
                                )
                    , test "no tooltip before 1 second has passed" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.Hover <|
                                        Just <|
                                            Message.Message.FirstOccurrenceIcon
                                                "foo"
                                )
                            >> Tuple.first
                            >> Common.queryView
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
                            >> Application.handleDelivery
                                (ClockTicked OneSecond <|
                                    Time.millisToPosix 0
                                )
                            >> Tuple.first
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.Hover <|
                                        Just <|
                                            Message.Message.FirstOccurrenceIcon
                                                "foo"
                                )
                            >> Tuple.first
                            >> Application.handleDelivery
                                (ClockTicked OneSecond <|
                                    Time.millisToPosix 1
                                )
                            >> Tuple.first
                            >> Common.queryView
                            >> Query.findAll
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-arrow-downward-yellow.svg"
                                    }
                                )
                            >> Query.first
                            >> Query.has
                                [ style "position" "relative"
                                , containing
                                    [ containing [ text "new version" ]
                                    , style "position" "absolute"
                                    , style "left" "0"
                                    , style "bottom" "100%"
                                    , style "background-color" tooltipGreyHex
                                    , style "padding" "5px"
                                    , style "z-index" "100"
                                    , style "width" "6em"
                                    , style "pointer-events" "none"
                                    , style "cursor" "default"
                                    , style "user-select" "none"
                                    , style "-ms-user-select" "none"
                                    , style "-moz-user-select" "none"
                                    , style "-khtml-user-select" "none"
                                    , style "-webkit-user-select" "none"
                                    , style "-webkit-touch-callout" "none"
                                    ]
                                , containing
                                    [ style "width" "0"
                                    , style "height" "0"
                                    , style "left" "50%"
                                    , style "margin-left" "-5px"
                                    , style "border-top" <|
                                        "5px solid "
                                            ++ tooltipGreyHex
                                    , style "border-left"
                                        "5px solid transparent"
                                    , style "border-right"
                                        "5px solid transparent"
                                    , style "position" "absolute"
                                    ]
                                ]
                    , test "mousing off yellow arrow triggers Hover message" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.Hover <|
                                        Just <|
                                            Message.Message.FirstOccurrenceIcon
                                                "foo"
                                )
                            >> Tuple.first
                            >> Common.queryView
                            >> Query.findAll
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-arrow-downward-yellow.svg"
                                    }
                                )
                            >> Query.first
                            >> Event.simulate Event.mouseLeave
                            >> Event.expect
                                (Msgs.Update <| Message.Message.Hover Nothing)
                    , test "unhovering after tooltip appears dismisses" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Application.handleDelivery
                                (ClockTicked OneSecond <|
                                    Time.millisToPosix 0
                                )
                            >> Tuple.first
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.Hover <|
                                        Just <|
                                            Message.Message.FirstOccurrenceIcon
                                                "foo"
                                )
                            >> Tuple.first
                            >> Application.handleDelivery
                                (ClockTicked OneSecond <|
                                    Time.millisToPosix 1
                                )
                            >> Tuple.first
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.Hover Nothing
                                )
                            >> Tuple.first
                            >> Common.queryView
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
                        >> Application.handleDelivery
                            (ClockTicked OneSecond <|
                                Time.millisToPosix 0
                            )
                        >> Tuple.first
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.Hover <|
                                    Just <|
                                        Message.Message.FirstOccurrenceIcon
                                            "foo"
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (ClockTicked OneSecond <|
                                Time.millisToPosix 1
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.findAll [ text "new version" ]
                        >> Query.count (Expect.equal 1)
                , test "artifact input step always has checkmark at the right" <|
                    fetchPlanWithArtifactInputStep
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-success-check.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "artifact output step has pending icon while build runs" <|
                    fetchPlanWithEnsureArtifactOutputStep
                        >> Common.queryView
                        >> Query.findAll [ class "header" ]
                        >> Query.index -1
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-pending.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "artifact output step has green check on finished build" <|
                    fetchPlanWithEnsureArtifactOutputStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url =
                                            "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.BuildStatus
                                                Concourse.BuildStatusFailed
                                                (Time.millisToPosix 0)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.findAll [ class "header" ]
                        >> Query.index -1
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-success-check.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "successful step has a checkmark at the far right" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url =
                                            "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.FinishGet
                                                { source = "stdout"
                                                , id = "plan"
                                                }
                                                0
                                                Dict.empty
                                                []
                                                Nothing
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-success-check.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "get step lists resource version on the right" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.FinishGet
                                                { source = "stdout", id = "plan" }
                                                0
                                                (Dict.fromList [ ( "version", "v3.1.4" ) ])
                                                []
                                                Nothing
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has [ text "v3.1.4" ]
                , test "finished task lists initialization duration in tooltip" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.InitializeTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 10000)
                                      }
                                    , { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.FinishTask
                                                { source = "stdout", id = "plan" }
                                                0
                                                (Time.millisToPosix 30000)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.Hover <|
                                    Just <|
                                        Message.Message.StepState
                                            "plan"
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (ClockTicked OneSecond <|
                                Time.millisToPosix 1
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.findAll [ tag "tr" ]
                        >> Query.index 0
                        >> Query.has [ text "initialization", text "10s" ]
                , test "finished task lists step duration in tooltip" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.InitializeTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 10000)
                                      }
                                    , { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.FinishTask
                                                { source = "stdout", id = "plan" }
                                                0
                                                (Time.millisToPosix 30000)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.Hover <|
                                    Just <|
                                        Message.Message.StepState
                                            "plan"
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (ClockTicked OneSecond <|
                                Time.millisToPosix 1
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.findAll [ tag "tr" ]
                        >> Query.index 1
                        >> Query.has [ text "step", text "20s" ]
                , test "running step has loading spinner at the right" <|
                    fetchPlanWithTaskStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout"
                                                , id = "plan"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            [ style "animation"
                                "container-rotate 1568ms linear infinite"
                            ]
                , test "pending step has dashed circle at the right" <|
                    fetchPlanWithTaskStep
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-pending.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "cancelled step has no-entry circle at the right" <|
                    fetchPlanWithTaskStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.InitializeTask
                                                { source = "stdout"
                                                , id = "plan"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.BuildStatus
                                                Concourse.BuildStatusAborted
                                                (Time.millisToPosix 0)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-interrupted.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "interrupted step has dashed circle with dot at the right" <|
                    fetchPlanWithTaskStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.BuildStatus
                                                Concourse.BuildStatusAborted
                                                (Time.millisToPosix 0)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-cancelled.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "failing step has an X at the far right" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.FinishGet
                                                { source = "stdout", id = "plan" }
                                                1
                                                Dict.empty
                                                []
                                                Nothing
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-failure-times.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "erroring step has orange exclamation triangle at right" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                      , data =
                                            STModels.Error
                                                { source = "stderr", id = "plan" }
                                                "error message"
                                                (Time.millisToPosix 0)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ class "header" ]
                        >> Query.children []
                        >> Query.index -1
                        >> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-exclamation-triangle.svg"
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , describe "erroring build" <|
                    [ test "has orange exclamation triangle at left" <|
                        fetchPlanWithGetStep
                            >> Application.handleDelivery
                                (EventsReceived <|
                                    Ok <|
                                        [ { url = "http://localhost:8080/api/v1/builds/307/events"
                                          , data = STModels.Opened
                                          }
                                        ]
                                )
                            >> Tuple.first
                            >> Application.handleDelivery
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
                            >> Common.queryView
                            >> Query.findAll [ class "header" ]
                            >> Query.first
                            >> Query.children []
                            >> Query.first
                            >> Query.has
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-exclamation-triangle.svg"
                                    }
                                    ++ [ style "background-size" "14px 14px" ]
                                )
                    , test "has passport officer icon" <|
                        let
                            imgUrl =
                                "/public/images/passport-officer-ic.svg"

                            eventsUrl =
                                "http://localhost:8080/api/v1/builds/307/events"
                        in
                        fetchPlanWithGetStep
                            >> Application.handleDelivery
                                (EventsReceived <|
                                    Ok
                                        [ { data = STModels.NetworkError
                                          , url = eventsUrl
                                          }
                                        ]
                                )
                            >> Tuple.first
                            >> Common.queryView
                            >> Query.find [ class "not-authorized" ]
                            >> Query.find [ tag "img" ]
                            >> Query.has [ attribute <| Attr.src imgUrl ]
                    ]
                ]
            , describe "get step with metadata" <|
                let
                    httpURLText =
                        "http://some-url"

                    httpsURLText =
                        "https://some-url"

                    plainText =
                        "plain-text"

                    invalidURLText =
                        "https:// is secure!"

                    metadataView =
                        Application.init
                            flags
                            { protocol = Url.Http
                            , host = ""
                            , port_ = Nothing
                            , path = "/teams/t/pipelines/p/jobs/j/builds/307"
                            , query = Nothing
                            , fragment = Just "Lstepid:1"
                            }
                            |> Tuple.first
                            |> Application.handleCallback
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
                                (Callback.PlanAndResourcesFetched 307 <|
                                    Ok <|
                                        ( { id = "stepid"
                                          , step =
                                                Concourse.BuildStepGet
                                                    "step"
                                                    (Just <| Dict.fromList [ ( "version", "1" ) ])
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
                                                    STModels.FinishGet
                                                        { source = "stdout"
                                                        , id = "stepid"
                                                        }
                                                        1
                                                        (Dict.fromList [ ( "version", "1" ) ])
                                                        [ { name = "http-url"
                                                          , value = httpURLText
                                                          }
                                                        , { name = "https-url"
                                                          , value = httpsURLText
                                                          }
                                                        , { name = "plain-text"
                                                          , value = plainText
                                                          }
                                                        ]
                                                        Nothing
                                              }
                                            ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                in
                [ test "should show hyperlink if metadata starts with 'http://'" <|
                    \_ ->
                        metadataView
                            |> Query.find
                                [ containing [ text httpURLText ]
                                ]
                            |> Query.has
                                [ tag "a"
                                , style "text-decoration-line" "underline"
                                , attribute <| Attr.target "_blank"
                                , attribute <| Attr.href httpURLText
                                ]
                , test "should show hyperlink if metadata starts with 'https://'" <|
                    \_ ->
                        metadataView
                            |> Query.find
                                [ containing [ text httpsURLText ]
                                ]
                            |> Query.has
                                [ tag "a"
                                , style "text-decoration-line" "underline"
                                , attribute <| Attr.target "_blank"
                                , attribute <| Attr.href httpsURLText
                                ]
                , test "should not show hyperlink if metadata is plain text" <|
                    \_ ->
                        metadataView
                            |> Query.find
                                [ containing [ text plainText ]
                                ]
                            |> Query.hasNot
                                [ tag "a"
                                , style "text-decoration-line" "underline"
                                , attribute <| Attr.target "_blank"
                                , attribute <| Attr.href plainText
                                ]
                , test "should not show hyperlink if metadata is malformed URL" <|
                    \_ ->
                        metadataView
                            |> Query.find
                                [ containing [ text invalidURLText ]
                                ]
                            |> Query.hasNot
                                [ tag "a"
                                , style "text-decoration-line" "underline"
                                , attribute <| Attr.target "_blank"
                                , attribute <| Attr.href invalidURLText
                                ]
                ]
            ]
        ]


tooltipGreyHex : String
tooltipGreyHex =
    "#9b9b9b"


darkRed : String
darkRed =
    "#bd3826"


brightRed : String
brightRed =
    "#ed4b35"


darkGreen : String
darkGreen =
    "#419867"


brightGreen : String
brightGreen =
    "#11c560"


darkGrey : String
darkGrey =
    "#3d3c3c"


receiveEvent :
    STModels.BuildEventEnvelope
    -> Application.Model
    -> ( Application.Model, List Effects.Effect )
receiveEvent envelope =
    Application.update (Msgs.DeliveryReceived <| EventsReceived <| Ok [ envelope ])
