module BuildTests exposing (all)

import Application.Application as Application
import Array
import Assets
import Build.Build as Build
import Build.Models as Models
import Build.StepTree.Models as STModels
import Char
import Colors
import Common exposing (defineHoverBehaviour, isColorWithStripes)
import Concourse exposing (BuildPrepStatus(..))
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Pagination exposing (Direction(..))
import DashboardTests exposing (iconSelector, middleGrey)
import Data
import Dict
import Expect
import Html.Attributes as Attr
import Http
import Json.Encode as Encode
import Keyboard
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message
import Message.ScrollDirection as ScrollDirection
import Message.Subscription as Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as Msgs
import Routes
import StrictEvents exposing (DeltaMode(..))
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
                Data.jobBuildId

            flags =
                { turbulenceImgSrc = ""
                , notFoundImgSrc = ""
                , csrfToken = csrfToken
                , authToken = ""
                , pipelineRunningKeyframes = ""
                }

            fetchPipeline : Application.Model -> ( Application.Model, List Effects.Effect )
            fetchPipeline =
                Application.handleCallback <|
                    Callback.AllPipelinesFetched <|
                        Ok [ Data.pipeline "team" buildId.pipelineId ]

            fetchBuild : BuildStatus -> Application.Model -> ( Application.Model, List Effects.Effect )
            fetchBuild status =
                Application.handleCallback <|
                    Callback.BuildFetched <|
                        Ok (Data.jobBuild status)

            fetchBuildWithStatus :
                BuildStatus
                -> Application.Model
                -> Application.Model
            fetchBuildWithStatus status =
                Application.handleCallback
                    (Callback.BuildFetched <| Ok <| Data.jobBuild status)
                    >> Tuple.first
                    >> Application.handleCallback
                        (Callback.BuildHistoryFetched
                            (Ok
                                { pagination =
                                    { previousPage = Nothing
                                    , nextPage = Nothing
                                    }
                                , content =
                                    [ Data.jobBuild status
                                    ]
                                }
                            )
                        )
                    >> Tuple.first

            fetchJobDetails :
                Application.Model
                -> ( Application.Model, List Effects.Effect )
            fetchJobDetails =
                Application.handleCallback <|
                    Callback.BuildJobDetailsFetched <|
                        Ok
                            (Data.job 0 |> Data.withName "j")

            fetchJobDetailsNoTrigger :
                Application.Model
                -> ( Application.Model, List Effects.Effect )
            fetchJobDetailsNoTrigger =
                Application.handleCallback <|
                    Callback.BuildJobDetailsFetched <|
                        Ok
                            (Data.job 0
                                |> Data.withName "j"
                                |> Data.withDisableManualTrigger True
                            )

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
                            , content = [ Data.jobBuild BuildStatusSucceeded ]
                            }
                        )
                    )

            csrfToken : String
            csrfToken =
                "csrf_token"

            eventsUrl : String
            eventsUrl =
                "http://localhost:8080/api/v1/builds/1/events"
        in
        [ test "converts URL hash to highlighted line in view" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:1"
                    }
                    |> Tuple.first
                    |> fetchBuild BuildStatusStarted
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
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout"
                                                , id = "stepid"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = eventsUrl
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
        , test "scrolls to highlighted line" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:1"
                    }
                    |> Tuple.first
                    |> fetchBuild BuildStatusStarted
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
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout"
                                                , id = "stepid"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = eventsUrl
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
                    |> Tuple.second
                    |> Common.contains (Effects.Scroll (ScrollDirection.ToId "stepid:1") "build-body")
        , test "scrolls to top of highlighted range" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:1:3"
                    }
                    |> Tuple.first
                    |> fetchBuild BuildStatusStarted
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
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout"
                                                , id = "stepid"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = eventsUrl
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
                    |> Tuple.second
                    |> Common.contains (Effects.Scroll (ScrollDirection.ToId "stepid:1") "build-body")
        , test "does not scroll to an invalid range" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:2:1"
                    }
                    |> Tuple.first
                    |> fetchBuild BuildStatusStarted
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
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout"
                                                , id = "stepid"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = eventsUrl
                                      , data =
                                            STModels.Log
                                                { source = "stdout"
                                                , id = "stepid"
                                                }
                                                "log message\n"
                                                Nothing
                                      }
                                    , { url = eventsUrl
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
                    |> Tuple.second
                    |> Common.notContains (Effects.Scroll (ScrollDirection.ToId "stepid:2") "build-body")
        , test "does not re-scroll to an id multiple times" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:1"
                    }
                    |> Tuple.first
                    |> fetchBuild BuildStatusStarted
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
                    |> Application.handleDelivery
                        (EventsReceived <|
                            Ok <|
                                [ { url = eventsUrl
                                  , data =
                                        STModels.StartTask
                                            { source = "stdout"
                                            , id = "stepid"
                                            }
                                            (Time.millisToPosix 0)
                                  }
                                , { url = eventsUrl
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
                    |> Application.handleDelivery
                        (EventsReceived <| Ok [])
                    |> Tuple.second
                    |> Common.notContains (Effects.Scroll (ScrollDirection.ToId "stepid:1") "build-body")
        , test "auto-scroll disallowed before scrolled to highlighted line" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:1"
                    }
                    |> Tuple.first
                    |> fetchBuild BuildStatusStarted
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
                    |> Application.handleDelivery
                        (EventsReceived <|
                            Ok <|
                                [ { url = eventsUrl
                                  , data =
                                        STModels.StartTask
                                            { source = "stdout"
                                            , id = "stepid"
                                            }
                                            (Time.millisToPosix 0)
                                  }
                                , { url = eventsUrl
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
                    |> Application.update
                        (Msgs.Update <|
                            Message.Message.Scrolled
                                { scrollHeight = 20
                                , scrollTop = 10
                                , clientHeight = 10
                                }
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (EventsReceived <| Ok [])
                    |> Tuple.second
                    |> Common.notContains (Effects.Scroll ScrollDirection.ToBottom "build-body")
        , test "auto-scroll disallowed before scrolling away from highlighted line" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:1"
                    }
                    |> Tuple.first
                    |> fetchBuild BuildStatusStarted
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
                    |> Application.handleDelivery
                        (EventsReceived <|
                            Ok <|
                                [ { url = eventsUrl
                                  , data =
                                        STModels.StartTask
                                            { source = "stdout"
                                            , id = "stepid"
                                            }
                                            (Time.millisToPosix 0)
                                  }
                                , { url = eventsUrl
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
                    |> Application.update
                        (Msgs.Update <|
                            Message.Message.Scrolled
                                { scrollHeight = 20
                                , scrollTop = 10
                                , clientHeight = 10
                                }
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (Subscription.ScrolledToId ( "Lstepid:1", "build-body" ))
                    |> Tuple.first
                    |> Application.handleDelivery
                        (EventsReceived <| Ok [])
                    |> Tuple.second
                    |> Common.notContains (Effects.Scroll ScrollDirection.ToBottom "build-body")
        , test "auto-scroll allowed after scrolling away from highlighted line" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:1"
                    }
                    |> Tuple.first
                    |> fetchBuild BuildStatusStarted
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
                    |> Application.handleDelivery
                        (EventsReceived <|
                            Ok <|
                                [ { url = eventsUrl
                                  , data =
                                        STModels.StartTask
                                            { source = "stdout"
                                            , id = "stepid"
                                            }
                                            (Time.millisToPosix 0)
                                  }
                                , { url = eventsUrl
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
                    |> Application.handleDelivery
                        (Subscription.ScrolledToId ( "Lstepid:1", "build-body" ))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.Update <|
                            Message.Message.Scrolled
                                { scrollHeight = 20
                                , scrollTop = 10
                                , clientHeight = 10
                                }
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (EventsReceived <| Ok [])
                    |> Tuple.second
                    |> Common.contains (Effects.Scroll ScrollDirection.ToBottom "build-body")
        , test "subscribes to scrolled to id completion" <|
            \_ ->
                Application.init
                    flags
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Just "Lstepid:1"
                    }
                    |> Tuple.first
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnScrolledToId
        , describe "page title"
            [ test "with a job build" <|
                \_ ->
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> fetchBuild BuildStatusSucceeded
                        |> Tuple.first
                        |> Application.view
                        |> .title
                        |> Expect.equal "j #1 - Concourse"
            , test "with a one-off-build" <|
                \_ ->
                    Common.init "/builds/1"
                        |> Application.handleCallback
                            (Callback.BuildFetched <|
                                Ok
                                    { id = 1
                                    , name = "1"
                                    , teamName = "t"
                                    , job = Nothing
                                    , status = BuildStatusPending
                                    , duration =
                                        { startedAt = Nothing
                                        , finishedAt = Nothing
                                        }
                                    , reapTime = Nothing
                                    }
                            )
                        |> Tuple.first
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
                        , path = "/pipelines/1/jobs/routejob/builds/1"
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
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                (Data.jobBuild BuildStatusSucceeded
                                    |> Data.withDuration
                                        { startedAt = buildTime
                                        , finishedAt = buildTime
                                        }
                                    |> Data.withReapTime buildTime
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
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusSucceeded))
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.PlanAndResourcesFetched 1 <| Data.httpUnauthorized)
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ class "not-authorized" ]
        , test "shows 'build cancelled' in red when aborted build's plan request gives 404" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                (Data.jobBuild BuildStatusAborted
                                    |> Data.withDuration
                                        { startedAt = Nothing
                                        , finishedAt = Just <| Time.millisToPosix 0
                                        }
                                )
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.PlanAndResourcesFetched 1 <| Data.httpNotFound)
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has
                        [ style "background-color" Colors.frame
                        , style "padding" "5px 10px"
                        , style "color" Colors.errorLog
                        , containing [ text "build cancelled" ]
                        ]
        , test "shows passport officer when build prep request gives 401" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusPending))
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.BuildPrepFetched 1 <| Data.httpUnauthorized)
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ class "not-authorized" ]
        , test "focuses build body when build is fetched" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusSucceeded))
                    |> Tuple.second
                    |> Common.contains (Effects.Focus Build.bodyId)
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
                                (Data.jobBuild BuildStatusStarted
                                    |> Data.withJob Nothing
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
        , test "output does not change when the wrong build is fetched" <|
            \_ ->
                Common.init "/pipelines/1/jobs/job/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                (Data.jobBuild BuildStatusStarted
                                    |> Data.withJob Nothing
                                    |> Data.withDuration
                                        { startedAt =
                                            Just <| Time.millisToPosix 0
                                        , finishedAt = Nothing
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
                                            "my-step-name"
                                  }
                                , { inputs = [], outputs = [] }
                                )
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                (Data.jobBuild BuildStatusStarted
                                    |> Data.withId 2
                                    |> Data.withName "2"
                                    |> Data.withJob Nothing
                                    |> Data.withDuration
                                        { startedAt =
                                            Just <| Time.millisToPosix 0
                                        , finishedAt = Nothing
                                        }
                                )
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ text "my-step-name" ]
        , test "build name does not change when the wrong build is fetched" <|
            \_ ->
                Common.init "/teams/main/pipelines/pipeline/jobs/job/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                (Data.jobBuild BuildStatusStarted
                                    |> Data.withJob Nothing
                                    |> Data.withDuration
                                        { startedAt =
                                            Just <| Time.millisToPosix 0
                                        , finishedAt = Nothing
                                        }
                                )
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                (Data.jobBuild BuildStatusStarted
                                    |> Data.withId 2
                                    |> Data.withName "2"
                                    |> Data.withJob Nothing
                                    |> Data.withDuration
                                        { startedAt =
                                            Just <| Time.millisToPosix 0
                                        , finishedAt = Nothing
                                        }
                                )
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "build #2" ]
        , test "build prep does not appear when wrong prep is fetched" <|
            \_ ->
                Common.init "/teams/main/pipelines/pipeline/jobs/job/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                (Data.jobBuild BuildStatusStarted
                                    |> Data.withJob (Just Data.jobId)
                                    |> Data.withDuration
                                        { startedAt =
                                            Just <| Time.millisToPosix 0
                                        , finishedAt = Nothing
                                        }
                                )
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.BuildPrepFetched 2 <|
                            Ok
                                { pausedPipeline = BuildPrepStatusUnknown
                                , pausedJob = BuildPrepStatusUnknown
                                , maxRunningBuilds = BuildPrepStatusUnknown
                                , inputs = Dict.empty
                                , inputsSatisfied = BuildPrepStatusUnknown
                                , missingInputReasons = Dict.empty
                                }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "preparing" ]
        , test "log lines have timestamps in current zone" <|
            \_ ->
                Common.init "/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                (Data.jobBuild BuildStatusStarted
                                    |> Data.withJob Nothing
                                    |> Data.withDuration
                                        { startedAt =
                                            Just <| Time.millisToPosix 0
                                        , finishedAt = Nothing
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
                    |> Application.update
                        (Msgs.Update <|
                            Message.Message.Click <|
                                Message.Message.StepHeader "stepid"
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.findAll [ class "timestamped-line" ]
                    |> Query.first
                    |> Query.has [ text "05:00:00" ]
        , test "when build is running it scrolls every build event" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" } (Time.millisToPosix 0)
                        }
                    |> Tuple.second
                    |> Common.contains (Effects.Scroll ScrollDirection.ToBottom "build-body")
        , test "when build is not running it does not scroll on build event" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusSucceeded))
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" } (Time.millisToPosix 0)
                        }
                    |> Tuple.second
                    |> Expect.equal []
        , test "build body has scroll handler" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusSucceeded))
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "build-body" ]
                    |> Event.simulate
                        (Event.custom "scroll" <|
                            Encode.object
                                [ ( "target"
                                  , Encode.object
                                        [ ( "clientHeight", Encode.int 0 )
                                        , ( "scrollTop", Encode.int 0 )
                                        , ( "scrollHeight", Encode.int 0 )
                                        ]
                                  )
                                ]
                        )
                    |> Event.expect
                        (Msgs.Update <|
                            Message.Message.Scrolled
                                { scrollHeight = 0
                                , scrollTop = 0
                                , clientHeight = 0
                                }
                        )
        , test "when build is running but the user is not scrolled to the bottom it does not scroll on build event" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.Update <|
                            Message.Message.Scrolled
                                { scrollHeight = 2
                                , scrollTop = 0
                                , clientHeight = 1
                                }
                        )
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" } (Time.millisToPosix 0)
                        }
                    |> Tuple.second
                    |> Expect.equal []
        , test "when build is running but the user scrolls back to the bottom it scrolls on build event" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
                    |> Tuple.first
                    |> Application.update
                        (Msgs.Update <|
                            Message.Message.Scrolled
                                { scrollHeight = 2
                                , scrollTop = 0
                                , clientHeight = 1
                                }
                        )
                    |> Tuple.first
                    |> Application.update
                        (Msgs.Update <|
                            Message.Message.Scrolled
                                { scrollHeight = 2
                                , scrollTop = 1
                                , clientHeight = 1
                                }
                        )
                    |> Tuple.first
                    |> receiveEvent
                        { url = "http://localhost:8080/api/v1/builds/1/events"
                        , data = STModels.StartTask { id = "stepid", source = "" } (Time.millisToPosix 0)
                        }
                    |> Tuple.second
                    |> Expect.equal [ Effects.Scroll ScrollDirection.ToBottom "build-body" ]
        , test "pressing 'T' twice triggers two builds" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.BuildJobDetailsFetched <|
                            Ok
                                (Data.job 1
                                    |> Data.withName "j"
                                    |> Data.withPipelineName "p"
                                    |> Data.withTeamName "t"
                                )
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
                    |> Expect.equal [ Effects.DoTriggerBuild Data.shortJobId ]
        , test "pressing 'R' reruns build" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusSucceeded))
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.BuildJobDetailsFetched <|
                            Ok
                                (Data.job 1
                                    |> Data.withName "j"
                                    |> Data.withPipelineName "p"
                                    |> Data.withTeamName "t"
                                )
                        )
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyDown <|
                                { ctrlKey = False
                                , shiftKey = True
                                , metaKey = False
                                , code = Keyboard.R
                                }
                        )
                    |> Tuple.second
                    |> Expect.equal [ Effects.RerunJobBuild Data.jobBuildId ]
        , test "pressing 'R' does nothing if there is a running build" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.BuildJobDetailsFetched <|
                            Ok
                                (Data.job 1
                                    |> Data.withName "j"
                                    |> Data.withPipelineName "p"
                                    |> Data.withTeamName "t"
                                )
                        )
                    |> Tuple.first
                    |> Application.update
                        (Msgs.DeliveryReceived <|
                            KeyDown <|
                                { ctrlKey = False
                                , shiftKey = True
                                , metaKey = False
                                , code = Keyboard.R
                                }
                        )
                    |> Tuple.second
                    |> Expect.equal []
        , test "pressing 'gg' scrolls to the top" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
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
                    |> Expect.equal [ Effects.Scroll ScrollDirection.ToTop "build-body" ]
        , test "pressing 'G' scrolls to the bottom" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
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
                    |> Expect.equal [ Effects.Scroll ScrollDirection.ToBottom "build-body" ]
        , test "pressing 'g' once does nothing" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
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
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Application.handleCallback
                        (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
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
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Common.queryView
                    |> Query.has [ text "loading" ]
        , test "fetches build on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , pipelineRunningKeyframes = "pipeline-running"
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains (Effects.FetchJobBuild Data.jobBuildId)
        , test "does not reload build when highlight is modified" <|
            \_ ->
                let
                    buildParams =
                        { id = Data.jobBuildId
                        , highlight = Routes.HighlightNothing
                        }
                in
                Common.init "/"
                    |> Application.handleDelivery
                        (RouteChanged <|
                            Routes.Build
                                buildParams
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (RouteChanged <|
                            Routes.Build
                                { buildParams
                                    | highlight = Routes.HighlightLine "step" 1
                                }
                        )
                    |> Tuple.second
                    |> Common.notContains
                        (Effects.FetchJobBuild <| buildParams.id)
        , test "does not reload one off build page when highlight is modified" <|
            \_ ->
                let
                    buildParams =
                        { id = 1
                        , highlight = Routes.HighlightNothing
                        }
                in
                Common.init "/"
                    |> Application.handleDelivery
                        (RouteChanged <|
                            Routes.OneOffBuild
                                buildParams
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (RouteChanged <|
                            Routes.OneOffBuild
                                { buildParams
                                    | highlight = Routes.HighlightLine "step" 1
                                }
                        )
                    |> Tuple.second
                    |> Common.notContains
                        (Effects.FetchBuild 0 buildParams.id)
        , test "gets current timezone on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , pipelineRunningKeyframes = "pipeline-running"
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/pipelines/1/jobs/j/builds/1"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains Effects.GetCurrentTimeZone
        , describe "top bar" <|
            [ test "has a top bar" <|
                \_ ->
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> Common.queryView
                        |> Query.has [ id "top-bar-app" ]
            , test "has a concourse icon" <|
                \_ ->
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style "background-image" <|
                                Assets.backgroundImage <|
                                    Just Assets.ConcourseLogoWhite
                            ]
            , test "has the breadcrumbs" <|
                \_ ->
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> fetchPipeline
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Expect.all
                            [ Query.has [ id "breadcrumb-pipeline" ]
                            , Query.has [ text "p" ]
                            , Query.has [ id "breadcrumb-job" ]
                            , Query.has [ text "j" ]
                            ]
            , test "has the breadcrumbs after fetching build" <|
                \_ ->
                    Common.init "/builds/1"
                        |> fetchPipeline
                        |> Tuple.first
                        |> fetchBuild BuildStatusSucceeded
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Expect.all
                            [ Query.has [ id "breadcrumb-pipeline" ]
                            , Query.has [ text "p" ]
                            , Query.has [ id "breadcrumb-job" ]
                            , Query.has [ text "j" ]
                            ]
            , test "has a user section" <|
                \_ ->
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> Common.queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ id "login-component" ]
            ]
        , test "page below top bar has padding to accomodate top bar" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has [ style "padding-top" "54px" ]
        , test "page below top bar fills vertically without scrolling" <|
            \_ ->
                Common.init "/pipelines/1/jobs/j/builds/1"
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has
                        [ style "height" "100%"
                        , style "box-sizing" "border-box"
                        ]
        , describe "after build is fetched" <|
            let
                givenBuildFetched _ =
                    Common.init "/pipelines/1/jobs/j/builds/1" |> fetchBuild BuildStatusSucceeded
            in
            [ test "has a header after the build is fetched" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.has [ id "build-header" ]
            , test "build body scrolls independently of page frame" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find [ id "build-body" ]
                    >> Query.has [ style "overflow-y" "auto" ]
            , test "build body has momentum based scroll enabled" <|
                givenBuildFetched
                    >> Tuple.first
                    >> Common.queryView
                    >> Query.find [ id "build-body" ]
                    >> Query.has [ style "-webkit-overflow-scrolling" "touch" ]
            , test "fetches build history and job details after build is fetched" <|
                givenBuildFetched
                    >> Tuple.second
                    >> Expect.all
                        [ Common.contains (Effects.FetchBuildHistory Data.shortJobId Nothing)
                        , Common.contains (Effects.FetchBuildJobDetails Data.shortJobId)
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
            , test "when build finishes, shows finished timestamp" <|
                \_ ->
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> Application.handleCallback (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
                        |> Tuple.first
                        |> receiveEvent
                            { url = "http://localhost:8080/api/v1/builds/1/events"
                            , data = STModels.BuildStatus BuildStatusSucceeded (Time.millisToPosix 0)
                            }
                        |> Tuple.first
                        |> Application.update
                            (Msgs.DeliveryReceived <|
                                ClockTicked
                                    OneSecond
                                    (Time.millisToPosix (2 * 1000))
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "build-duration" ]
                        |> Query.find [ tag "tr", containing [ text "finished" ] ]
                        |> Query.has [ text "2s ago" ]
            , test "when build finishes succesfully, header background is green" <|
                \_ ->
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> Application.handleCallback (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
                        |> Tuple.first
                        |> receiveEvent
                            { url = "http://localhost:8080/api/v1/builds/1/events"
                            , data = STModels.BuildStatus BuildStatusSucceeded (Time.millisToPosix 0)
                            }
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
                        |> Query.has [ style "background-color" Colors.success ]
            , test "when less than 24h old, shows relative time since build" <|
                \_ ->
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> Application.handleCallback (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusSucceeded))
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
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> Application.handleCallback
                            (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusSucceeded))
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
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> Application.handleCallback
                            (Callback.GotCurrentTimeZone <|
                                Time.customZone (5 * 60) []
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusSucceeded))
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
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusPending
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#9b9b9b" ]
                , test "started build has yellow banner" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusStarted
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#f1c40f" ]
                , test "succeeded build has green banner" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusSucceeded
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#11c560" ]
                , test "failed build has red banner" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusFailed
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#ed4b35" ]
                , test "errored build has amber banner" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusErrored
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#f5a623" ]
                , test "aborted build has brown banner" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusAborted
                            |> Common.queryView
                            |> Query.find [ id "build-header" ]
                            |> Query.has [ style "background" "#8b572a" ]
                ]
            , describe "build history tab coloration"
                [ test "pending build has grey tab in build history" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusPending
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> Query.has [ style "background" "#9b9b9b" ]
                , test "started build has animated striped yellow tab in build history" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusStarted
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> isColorWithStripes { thick = "#f1c40f", thin = "#fad43b" }
                , test "succeeded build has green tab in build history" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusSucceeded
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> Query.has [ style "background" "#11c560" ]
                , test "failed build has red tab in build history" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusFailed
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> Query.has [ style "background" "#ed4b35" ]
                , test "errored build has amber tab in build history" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusErrored
                            |> Common.queryView
                            |> Query.find [ id "builds" ]
                            |> Query.find [ tag "li" ]
                            |> Query.has [ style "background" "#f5a623" ]
                , test "aborted build has brown tab in build history" <|
                    \_ ->
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuildWithStatus BuildStatusAborted
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
                , test """page contents lay out vertically, filling available
                          space without scrolling horizontally""" <|
                    givenHistoryAndDetailsFetched
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "page-below-top-bar" ]
                        >> Query.children []
                        >> Query.index 1
                        >> Query.has
                            [ style "flex-grow" "1"
                            , style "display" "flex"
                            , style "flex-direction" "column"
                            , style "overflow" "hidden"
                            ]
                , test "pressing 'L' updates the URL" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
                            (Callback.BuildHistoryFetched
                                (Ok
                                    { pagination =
                                        { previousPage = Nothing
                                        , nextPage =
                                            Just
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content =
                                        [ Data.jobBuild BuildStatusSucceeded
                                        , Data.jobBuild BuildStatusSucceeded
                                            |> Data.withId 2
                                            |> Data.withName "2"
                                            |> Data.withDuration
                                                { startedAt = Just <| Time.millisToPosix 0
                                                , finishedAt = Just <| Time.millisToPosix 0
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
                                "/pipelines/1/jobs/j/builds/2"
                            ]
                , test "can use keyboard to switch builds after status change event" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
                            (Callback.BuildHistoryFetched
                                (Ok
                                    { pagination =
                                        { previousPage = Nothing
                                        , nextPage =
                                            Just
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content =
                                        [ Data.jobBuild BuildStatusSucceeded
                                        , Data.jobBuild BuildStatusSucceeded
                                            |> Data.withId 2
                                            |> Data.withName "2"
                                            |> Data.withDuration
                                                { startedAt = Just <| Time.millisToPosix 0
                                                , finishedAt = Just <| Time.millisToPosix 0
                                                }
                                        ]
                                    }
                                )
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.EventsReceived <|
                                Ok
                                    [ { data =
                                            STModels.BuildStatus BuildStatusSucceeded <|
                                                Time.millisToPosix 0
                                      , url = "http://localhost:8080/api/v1/builds/1/events"
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.KeyDown
                                { ctrlKey = False
                                , shiftKey = False
                                , metaKey = False
                                , code = Keyboard.L
                                }
                            )
                        >> Tuple.second
                        >> Common.contains
                            (Effects.NavigateTo "/pipelines/1/jobs/j/builds/2")
                , test "switching tabs updates the build name" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.handleCallback
                            (Callback.BuildHistoryFetched
                                (Ok
                                    { pagination =
                                        { previousPage = Nothing
                                        , nextPage =
                                            Just
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content =
                                        [ Data.jobBuild BuildStatusSucceeded
                                        , Data.jobBuild BuildStatusSucceeded
                                            |> Data.withId 2
                                            |> Data.withName "2"
                                            |> Data.withDuration
                                                { startedAt = Just <| Time.millisToPosix 0
                                                , finishedAt = Just <| Time.millisToPosix 0
                                                }
                                        ]
                                    }
                                )
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (RouteChanged <|
                                Routes.Build
                                    { id = Data.jobBuildId |> Data.withBuildName "2"
                                    , highlight = Routes.HighlightNothing
                                    }
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.has [ text " #2" ]
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
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content =
                                        [ Data.jobBuild BuildStatusSucceeded
                                        , Data.jobBuild BuildStatusSucceeded
                                            |> Data.withId 2
                                            |> Data.withName "2"
                                            |> Data.withDuration
                                                { startedAt = Just <| Time.millisToPosix 0
                                                , finishedAt = Just <| Time.millisToPosix 0
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
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content =
                                        [ Data.jobBuild BuildStatusSucceeded
                                        , Data.jobBuild BuildStatusSucceeded
                                            |> Data.withId 2
                                            |> Data.withName "2"
                                            |> Data.withDuration
                                                { startedAt = Just <| Time.millisToPosix 0
                                                , finishedAt = Just <| Time.millisToPosix 0
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
                , describe "scrolling builds"
                    [ test "checks if last build is visible" <|
                        givenBuildFetched
                            >> Tuple.first
                            >> Application.handleCallback
                                (Callback.BuildHistoryFetched
                                    (Ok
                                        { pagination =
                                            { previousPage = Nothing
                                            , nextPage =
                                                Just
                                                    { direction = To 1
                                                    , limit = 100
                                                    }
                                            }
                                        , content = [ Data.jobBuild BuildStatusSucceeded ]
                                        }
                                    )
                                )
                            >> Tuple.first
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.ScrollBuilds
                                        { deltaX = 0, deltaY = 0, deltaMode = DeltaModePixel }
                                )
                            >> Tuple.second
                            >> Common.contains (Effects.CheckIsVisible "1")
                    , test "deltaX is negated" <|
                        givenBuildFetched
                            >> Tuple.first
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.ScrollBuilds
                                        { deltaX = 5, deltaY = 0, deltaMode = DeltaModePixel }
                                )
                            >> Tuple.second
                            >> Common.contains (Effects.Scroll (ScrollDirection.Sideways -5) "builds")
                    , test "deltaY is not negated" <|
                        givenBuildFetched
                            >> Tuple.first
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.ScrollBuilds
                                        { deltaX = 0, deltaY = 5, deltaMode = DeltaModePixel }
                                )
                            >> Tuple.second
                            >> Common.contains (Effects.Scroll (ScrollDirection.Sideways 5) "builds")
                    , test "deltaX is preferred" <|
                        givenBuildFetched
                            >> Tuple.first
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.ScrollBuilds
                                        { deltaX = 5, deltaY = 4, deltaMode = DeltaModePixel }
                                )
                            >> Tuple.second
                            >> Common.contains (Effects.Scroll (ScrollDirection.Sideways -5) "builds")
                    , test "DeltaModeLine" <|
                        givenBuildFetched
                            >> Tuple.first
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.ScrollBuilds
                                        { deltaX = 5, deltaY = 0, deltaMode = DeltaModeLine }
                                )
                            >> Tuple.second
                            >> Common.contains (Effects.Scroll (ScrollDirection.Sideways -100) "builds")
                    , test "DeltaModePage" <|
                        givenBuildFetched
                            >> Tuple.first
                            >> Application.update
                                (Msgs.Update <|
                                    Message.Message.ScrollBuilds
                                        { deltaX = 5, deltaY = 0, deltaMode = DeltaModePage }
                                )
                            >> Tuple.second
                            >> Common.contains (Effects.Scroll (ScrollDirection.Sideways -4000) "builds")
                    ]
                , test "subscribes to element visibility" <|
                    givenBuildFetched
                        >> Tuple.first
                        >> Application.subscriptions
                        >> Common.contains Subscription.OnElementVisible
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
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content = [ Data.jobBuild BuildStatusSucceeded ]
                                    }
                                )
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.ElementVisible ( "1", True ))
                        >> Tuple.second
                        >> Expect.equal
                            [ Effects.FetchBuildHistory Data.shortJobId
                                (Just { direction = To 1, limit = 100 })
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
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content = [ Data.jobBuild BuildStatusSucceeded ]
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
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content = [ Data.jobBuild BuildStatusSucceeded ]
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
                                                { direction = To 2
                                                , limit = 100
                                                }
                                        }
                                    , content =
                                        [ Data.jobBuild BuildStatusSucceeded
                                            |> Data.withId 2
                                            |> Data.withName "2"
                                        ]
                                    }
                                )
                            )
                        >> Tuple.second
                        >> Common.notContains
                            (Effects.FetchBuildHistory Data.shortJobId
                                (Just { direction = To 2, limit = 100 })
                            )
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
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content = [ Data.jobBuild BuildStatusSucceeded ]
                                    }
                                )
                            )
                        >> Tuple.second
                        >> Common.contains (Effects.CheckIsVisible "1")
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
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content = [ Data.jobBuild BuildStatusSucceeded ]
                                    }
                                )
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (Subscription.ElementVisible ( "1", False ))
                        >> Tuple.second
                        >> Expect.equal [ Effects.Scroll (ScrollDirection.ToId "1") "builds" ]
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
                                                { direction = To 1
                                                , limit = 100
                                                }
                                        }
                                    , content = [ Data.jobBuild BuildStatusSucceeded ]
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
                                                { direction = To 2
                                                , limit = 100
                                                }
                                        }
                                    , content =
                                        [ Data.jobBuild BuildStatusSucceeded
                                            |> Data.withId 2
                                            |> Data.withName "2"
                                        ]
                                    }
                                )
                            )
                        >> Tuple.second
                        >> Expect.equal
                            [ Effects.FetchBuildHistory Data.shortJobId
                                (Just { direction = To 2, limit = 100 })
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
                            , style "border-color" darkGreen
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
                            , style "border-color" darkGreen
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
                                , image = Assets.AddCircleIcon |> Assets.CircleOutlineIcon
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
                    , unhoveredSelector =
                        { description = "grey plus icon"
                        , selector =
                            iconSelector
                                { size = "40px"
                                , image = Assets.AddCircleIcon |> Assets.CircleOutlineIcon
                                }
                        }
                    , hoveredSelector =
                        { description = "grey plus icon with tooltip"
                        , selector =
                            [ style "position" "relative"
                            , containing
                                [ containing
                                    [ text "manual triggering disabled in job config" ]
                                ]
                            , containing <|
                                iconSelector
                                    { size = "40px"
                                    , image = Assets.AddCircleIcon |> Assets.CircleOutlineIcon
                                    }
                            ]
                        }
                    , hoverable = Message.Message.TriggerBuildButton
                    }
                ]
            ]
        , describe "given build started and history and details fetched" <|
            let
                givenBuildStarted _ =
                    Common.init "/pipelines/1/jobs/j/builds/1"
                        |> fetchBuildWithStatus BuildStatusStarted
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
                        , style "border-color" darkRed
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
                        , style "border-color" darkRed
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
                            , image = Assets.AbortCircleIcon |> Assets.CircleOutlineIcon
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
                            Assets.backgroundImage <| Just Assets.NotBlockingCheckIcon
                    in
                    givenBuildStarted
                        >> Tuple.first
                        >> Application.handleCallback
                            (Callback.BuildPrepFetched 1 <| Ok prep)
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
                                , style "margin-right" "8px"
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
                            (Callback.BuildPrepFetched 1 <| Ok prep)
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
                                , style "margin" "0 8px 0 0"
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
                            (Callback.BuildPrepFetched 1 <| Ok prep)
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
                                , style "margin" "0 8px 0 0"
                                ]
                            , Query.has [ attribute <| Attr.title "thinking..." ]
                            ]
                ]
            , describe "build events subscription" <|
                let
                    preBuildPlanReceived _ =
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuild BuildStatusStarted
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
                                >> Common.contains
                                    (Effects.OpenBuildEventStream
                                        { url = "/api/v1/builds/1/events"
                                        , eventTypes = [ "end", "event" ]
                                        }
                                    )
                            , Tuple.first
                                >> Application.subscriptions
                                >> Common.contains
                                    (Subscription.FromEventSource
                                        ( "/api/v1/builds/1/events"
                                        , [ "end", "event" ]
                                        )
                                    )
                            ]
                , test "if build plan request fails, no event stream" <|
                    preBuildPlanReceived
                        >> Application.handleCallback
                            (Callback.PlanAndResourcesFetched 1 <| Data.httpUnauthorized)
                        >> Expect.all
                            [ Tuple.second >> Expect.equal []
                            , Tuple.first
                                >> Application.subscriptions
                                >> Common.notContains
                                    (Subscription.FromEventSource
                                        ( "/api/v1/builds/1/events"
                                        , [ "end", "event" ]
                                        )
                                    )
                            ]
                ]
            , describe "sync sticky build log headers" <|
                let
                    setup _ =
                        Common.init "/pipelines/1/jobs/j/builds/1"
                            |> fetchBuild BuildStatusStarted
                            |> Tuple.first
                            |> fetchHistory
                            |> Tuple.first
                            |> fetchJobDetails
                            |> Tuple.first
                in
                [ test "on plan received" <|
                    setup
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
                        >> Tuple.second
                        >> Common.contains Effects.SyncStickyBuildLogHeaders
                , test "on header clicked" <|
                    setup
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.Click <|
                                    Message.Message.StepHeader "plan"
                            )
                        >> Tuple.second
                        >> Common.contains Effects.SyncStickyBuildLogHeaders
                , test "on sub header clicked" <|
                    setup
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.Click <|
                                    Message.Message.StepSubHeader "plan" 0
                            )
                        >> Tuple.second
                        >> Common.contains Effects.SyncStickyBuildLogHeaders
                , test "on window resized" <|
                    setup
                        >> Application.handleDelivery (WindowResized 1 2)
                        >> Tuple.second
                        >> Common.contains Effects.SyncStickyBuildLogHeaders
                ]
            , describe "step header" <|
                let
                    fetchPlanWithGetStep : () -> Application.Model
                    fetchPlanWithGetStep =
                        givenBuildStarted
                            >> Tuple.first
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
                            >> Tuple.first

                    fetchPlanWithArtifactInputStep : () -> Application.Model
                    fetchPlanWithArtifactInputStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Application.handleCallback
                                (Callback.PlanAndResourcesFetched 1 <|
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
                                (Callback.PlanAndResourcesFetched 1 <|
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
                                (Callback.PlanAndResourcesFetched 1 <|
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

                    fetchPlanWithSetPipelineStep : () -> Application.Model
                    fetchPlanWithSetPipelineStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Application.handleCallback
                                (Callback.PlanAndResourcesFetched 1 <|
                                    Ok <|
                                        ( { id = "plan"
                                          , step =
                                                Concourse.BuildStepSetPipeline
                                                    "step"
                                          }
                                        , { inputs = [], outputs = [] }
                                        )
                                )
                            >> Tuple.first

                    fetchPlanWithLoadVarStep : () -> Application.Model
                    fetchPlanWithLoadVarStep =
                        givenBuildStarted
                            >> Tuple.first
                            >> Application.handleCallback
                                (Callback.PlanAndResourcesFetched 307 <|
                                    Ok <|
                                        ( { id = "plan"
                                          , step =
                                                Concourse.BuildStepLoadVar
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
                                (Callback.PlanAndResourcesFetched 1 <|
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
                                (Callback.PlanAndResourcesFetched 1 <|
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
                [ test "step is collapsed by default" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url =
                                            eventsUrl
                                      , data =
                                            STModels.InitializeGet
                                                { source = ""
                                                , id = "plan"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.hasNot [ class "step-body" ]
                , test "step expands on click" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url =
                                            eventsUrl
                                      , data =
                                            STModels.InitializeGet
                                                { source = ""
                                                , id = "plan"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.Click <|
                                    Message.Message.StepHeader "plan"
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.has [ class "step-body" ]
                , test "expanded step collapses on click" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url =
                                            eventsUrl
                                      , data =
                                            STModels.InitializeGet
                                                { source = ""
                                                , id = "plan"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.Click <|
                                    Message.Message.StepHeader "plan"
                            )
                        >> Tuple.first
                        >> Application.update
                            (Msgs.Update <|
                                Message.Message.Click <|
                                    Message.Message.StepHeader "plan"
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.hasNot [ class "step-body" ]
                , test "build step header lays out horizontally" <|
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
                , test "resource get step shows get label" <|
                    fetchPlanWithGetStep
                        >> Common.queryView
                        >> Query.has getStepLabel
                , test "artifact input step shows get label" <|
                    fetchPlanWithArtifactInputStep
                        >> Common.queryView
                        >> Query.has getStepLabel
                , test "task step shows task label" <|
                    fetchPlanWithTaskStep
                        >> Common.queryView
                        >> Query.has taskStepLabel
                , test "set_pipeline step shows set_pipeline label" <|
                    fetchPlanWithSetPipelineStep
                        >> Common.queryView
                        >> Query.has setPipelineStepLabel
                , test "load_var step shows load_var label" <|
                    fetchPlanWithLoadVarStep
                        >> Common.queryView
                        >> Query.has loadVarStepLabel
                , test "artifact output step shows put label" <|
                    fetchPlanWithEnsureArtifactOutputStep
                        >> Common.queryView
                        >> Query.has putStepLabel
                , test "put step shows upward arrow" <|
                    fetchPlanWithPutStep
                        >> Common.queryView
                        >> Query.has putStepLabel
                , test "get step on first occurrence shows yellow downward arrow" <|
                    fetchPlanWithGetStepWithFirstOccurrence
                        >> Common.queryView
                        >> Query.has firstOccurrenceGetStepLabel
                , test "hovering over a normal get step label does nothing" <|
                    fetchPlanWithGetStep
                        >> Common.queryView
                        >> Query.find getStepLabel
                        >> Event.simulate Event.mouseEnter
                        >> Event.toResult
                        >> Expect.err
                , test "hovering over a normal set_pipeline step label does nothing" <|
                    fetchPlanWithSetPipelineStep
                        >> Common.queryView
                        >> Query.find setPipelineStepLabel
                        >> Event.simulate Event.mouseEnter
                        >> Event.toResult
                        >> Expect.err
                , describe "first-occurrence get step label hover behaviour"
                    [ test "first-occurrence get step label has no tooltip" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Common.queryView
                            >> Query.findAll [ text "new version" ]
                            >> Query.count (Expect.equal 0)
                    , test "hovering over yellow label triggers Hover message" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Common.queryView
                            >> Query.findAll firstOccurrenceGetStepLabel
                            >> Query.first
                            >> Event.simulate Event.mouseEnter
                            >> Event.expect
                                (Msgs.Update <|
                                    Message.Message.Hover <|
                                        Just firstOccurrenceLabelID
                                )
                    , test "no tooltip before 1 second has passed" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> hoverFirstOccurrenceLabel
                            >> Common.queryView
                            >> Query.findAll [ text "new version" ]
                            >> Query.count (Expect.equal 0)
                    , test "1 second after hovering, tooltip appears" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> Application.handleDelivery
                                (ClockTicked OneSecond <|
                                    Time.millisToPosix 0
                                )
                            >> Tuple.first
                            >> hoverFirstOccurrenceLabel
                            >> Application.handleDelivery
                                (ClockTicked OneSecond <|
                                    Time.millisToPosix 1
                                )
                            >> Tuple.second
                            >> Common.contains
                                (Effects.GetViewportOf firstOccurrenceLabelID)
                    , test "mousing off yellow label triggers Hover message" <|
                        fetchPlanWithGetStepWithFirstOccurrence
                            >> hoverFirstOccurrenceLabel
                            >> Common.queryView
                            >> Query.findAll firstOccurrenceGetStepLabel
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
                            >> hoverFirstOccurrenceLabel
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
                            >> Query.findAll [ text "new version" ]
                            >> Query.count (Expect.equal 0)
                    ]
                , test "hovering one resource of several produces only a single tooltip" <|
                    fetchPlanWithGetStepWithFirstOccurrence
                        >> Application.handleDelivery
                            (ClockTicked OneSecond <|
                                Time.millisToPosix 0
                            )
                        >> Tuple.first
                        >> hoverFirstOccurrenceLabel
                        >> Application.handleDelivery
                            (ClockTicked OneSecond <|
                                Time.millisToPosix 1
                            )
                        >> Tuple.first
                        >> Application.handleCallback
                            (Callback.GotViewport firstOccurrenceLabelID <|
                                Ok
                                    { scene =
                                        { width = 1
                                        , height = 0
                                        }
                                    , viewport =
                                        { width = 1
                                        , height = 0
                                        , x = 0
                                        , y = 0
                                        }
                                    }
                            )
                        >> Tuple.first
                        >> Application.handleCallback
                            (Callback.GotElement <|
                                Ok
                                    { scene =
                                        { width = 0
                                        , height = 0
                                        }
                                    , viewport =
                                        { width = 0
                                        , height = 0
                                        , x = 0
                                        , y = 0
                                        }
                                    , element =
                                        { x = 0
                                        , y = 0
                                        , width = 1
                                        , height = 1
                                        }
                                    }
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
                                , image = Assets.SuccessCheckIcon
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
                                , image = Assets.PendingIcon
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "artifact output step has green check on finished build" <|
                    fetchPlanWithEnsureArtifactOutputStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url =
                                            eventsUrl
                                      , data =
                                            STModels.BuildStatus
                                                BuildStatusFailed
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
                                , image = Assets.SuccessCheckIcon
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "successful step has a checkmark at the far right" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url =
                                            eventsUrl
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
                                , image = Assets.SuccessCheckIcon
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "get step lists resource version on the right" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
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
                , test "one tick after hovering step state, GetElement fires" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.InitializeTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = eventsUrl
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 10000)
                                      }
                                    , { url = eventsUrl
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
                        >> Tuple.second
                        >> Common.contains
                            (Effects.GetViewportOf <| Message.Message.StepState "plan")
                , test "finished task lists initialization duration in tooltip" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.InitializeTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = eventsUrl
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 10000)
                                      }
                                    , { url = eventsUrl
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
                        >> Application.handleCallback
                            (Callback.GotViewport (Message.Message.StepState "plan") <|
                                Ok
                                    { scene =
                                        { width = 1
                                        , height = 0
                                        }
                                    , viewport =
                                        { width = 1
                                        , height = 0
                                        , x = 0
                                        , y = 0
                                        }
                                    }
                            )
                        >> Tuple.first
                        >> Application.handleCallback
                            (Callback.GotElement <|
                                Ok
                                    { scene =
                                        { width = 0
                                        , height = 0
                                        }
                                    , viewport =
                                        { width = 0
                                        , height = 0
                                        , x = 0
                                        , y = 0
                                        }
                                    , element =
                                        { x = 0
                                        , y = 0
                                        , width = 1
                                        , height = 1
                                        }
                                    }
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
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.InitializeTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = eventsUrl
                                      , data =
                                            STModels.StartTask
                                                { source = "stdout", id = "plan" }
                                                (Time.millisToPosix 10000)
                                      }
                                    , { url = eventsUrl
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
                        >> Application.handleCallback
                            (Callback.GotViewport (Message.Message.StepState "plan") <|
                                Ok
                                    { scene =
                                        { width = 1
                                        , height = 0
                                        }
                                    , viewport =
                                        { width = 1
                                        , height = 0
                                        , x = 0
                                        , y = 0
                                        }
                                    }
                            )
                        >> Tuple.first
                        >> Application.handleCallback
                            (Callback.GotElement <|
                                Ok
                                    { scene =
                                        { width = 0
                                        , height = 0
                                        }
                                    , viewport =
                                        { width = 0
                                        , height = 0
                                        , x = 0
                                        , y = 0
                                        }
                                    , element =
                                        { x = 0
                                        , y = 0
                                        , width = 1
                                        , height = 1
                                        }
                                    }
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
                                    [ { url = eventsUrl
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
                                , image = Assets.PendingIcon
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "cancelled step has no-entry circle at the right" <|
                    fetchPlanWithTaskStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.InitializeTask
                                                { source = "stdout"
                                                , id = "plan"
                                                }
                                                (Time.millisToPosix 0)
                                      }
                                    , { url = eventsUrl
                                      , data =
                                            STModels.BuildStatus
                                                BuildStatusAborted
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
                                , image = Assets.InterruptedIcon
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "interrupted step has dashed circle with dot at the right" <|
                    fetchPlanWithTaskStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.BuildStatus
                                                BuildStatusAborted
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
                                , image = Assets.CancelledIcon
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "failing step has an X at the far right" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
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
                                , image = Assets.FailureTimesIcon
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "erroring step has orange exclamation triangle at right" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
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
                                , image = Assets.ExclamationTriangleIcon
                                }
                                ++ [ style "background-size" "14px 14px" ]
                            )
                , test "successful step has no border" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url =
                                            eventsUrl
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
                        >> Query.has
                            [ style "border" <| "1px solid " ++ Colors.frame ]
                , test "failing step has a red border" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
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
                        >> Query.has
                            [ style "border" <| "1px solid " ++ Colors.failure ]
                , test "started step has a yellow border" <|
                    fetchPlanWithTaskStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
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
                        >> Query.has
                            [ style "border" <| "1px solid " ++ Colors.started ]
                , test "set_pipeline step that changed something has a yellow text" <|
                    fetchPlanWithSetPipelineStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.SetPipelineChanged
                                                { source = "stdout", id = "plan" }
                                                True
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.has changedSetPipelineStepLabel
                , test "set_pipeline step that changed something tooltip appears after 1 second" <|
                    fetchPlanWithSetPipelineStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok <|
                                    [ { url = eventsUrl
                                      , data =
                                            STModels.SetPipelineChanged
                                                { source = "stdout", id = "plan" }
                                                True
                                      }
                                    ]
                            )
                        >> Tuple.first
                        >> Application.handleDelivery
                            (ClockTicked OneSecond <|
                                Time.millisToPosix 0
                            )
                        >> Tuple.first
                        >> hoverSetPipelineChangedLabel
                        >> Application.handleDelivery
                            (ClockTicked OneSecond <|
                                Time.millisToPosix 1
                            )
                        >> Tuple.second
                        >> Common.contains
                            (Effects.GetViewportOf setPipelineChangedLabelID)
                , test "network error on first event shows passport officer" <|
                    let
                        imgUrl =
                            Assets.toString Assets.PassportOfficerIcon
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
                , test """network error after first event fails silently
                          (EventSource browser API will retry connection)""" <|
                    fetchPlanWithGetStep
                        >> Application.handleDelivery
                            (EventsReceived <|
                                Ok
                                    [ { url = eventsUrl
                                      , data = STModels.Opened
                                      }
                                    ]
                            )
                        >> Tuple.first
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
                        >> Query.findAll [ class "not-authorized" ]
                        >> Query.count (Expect.equal 0)
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
                            , path = "/pipelines/1/jobs/j/builds/1"
                            , query = Nothing
                            , fragment = Just "Lstepid:1"
                            }
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.BuildFetched <| Ok (Data.jobBuild BuildStatusStarted))
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.PlanAndResourcesFetched 1 <|
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
                                            [ { url = eventsUrl
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


getStepLabel =
    [ style "color" Colors.pending
    , style "line-height" "28px"
    , style "padding-left" "6px"
    , containing [ text "get:" ]
    ]


firstOccurrenceGetStepLabel =
    [ style "color" Colors.started
    , style "line-height" "28px"
    , style "padding-left" "6px"
    , containing [ text "get:" ]
    ]


putStepLabel =
    [ style "color" Colors.pending
    , style "line-height" "28px"
    , style "padding-left" "6px"
    , containing [ text "put:" ]
    ]


taskStepLabel =
    [ style "color" Colors.pending
    , style "line-height" "28px"
    , style "padding-left" "6px"
    , containing [ text "task:" ]
    ]


setPipelineStepLabel =
    [ style "color" Colors.pending
    , style "line-height" "28px"
    , style "padding-left" "6px"
    , containing [ text "set_pipeline:" ]
    ]


changedSetPipelineStepLabel =
    [ style "color" Colors.started
    , style "line-height" "28px"
    , style "padding-left" "6px"
    , containing [ text "set_pipeline:" ]
    ]


loadVarStepLabel =
    [ style "color" Colors.pending
    , style "line-height" "28px"
    , style "padding-left" "6px"
    , containing [ text "load_var:" ]
    ]


firstOccurrenceLabelID =
    Message.Message.ChangedStepLabel
        "foo"
        "new version"


hoverFirstOccurrenceLabel =
    Application.update
        (Msgs.Update <| Message.Message.Hover <| Just firstOccurrenceLabelID)
        >> Tuple.first

setPipelineChangedLabelID =
    Message.Message.ChangedStepLabel
        "foo"
        "pipeline config changed"


hoverSetPipelineChangedLabel =
    Application.update
        (Msgs.Update <| Message.Message.Hover <| Just setPipelineChangedLabelID)
        >> Tuple.first

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
