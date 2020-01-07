module Build.HeaderTests exposing (all)

import Application.Models exposing (Session)
import Build.Header.Header as Header
import Build.Header.Models as Models
import Build.Header.Views as Views
import Build.StepTree.Models as STModels
import Common
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Expect
import HoverState
import Keyboard
import List.Extra
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Subscription as Subscription
import RemoteData
import ScreenSize
import Set
import Test exposing (Test, describe, test)
import Time
import UserState


all : Test
all =
    describe "build page header"
        [ describe "title"
            [ test "is 'build' on a one-off build page" <|
                \_ ->
                    Header.header session model
                        |> .leftWidgets
                        |> Common.contains (Views.Title "0" Nothing)
            ]
        , test "pending build has no duration" <|
            \_ ->
                Header.header session
                    { model
                        | duration =
                            { startedAt = Nothing
                            , finishedAt = Nothing
                            }
                    }
                    |> .leftWidgets
                    |> Common.contains
                        (Views.Duration <| Views.Pending)
        , test "running build has running time" <|
            \_ ->
                Header.header session
                    { model
                        | duration =
                            { startedAt = Just <| Time.millisToPosix 0
                            , finishedAt = Nothing
                            }
                    }
                    |> .leftWidgets
                    |> Common.contains
                        (Views.Duration <|
                            Views.Running <|
                                Views.Absolute "Jan 1 1970 12:00:00 AM" Nothing
                        )
        , test "cancelled build has cancelled duration" <|
            \_ ->
                Header.header session
                    { model
                        | duration =
                            { startedAt = Nothing
                            , finishedAt = Just <| Time.millisToPosix 0
                            }
                    }
                    |> .leftWidgets
                    |> Common.contains
                        (Views.Duration <|
                            Views.Cancelled <|
                                Views.Absolute "Jan 1 1970 12:00:00 AM" Nothing
                        )
        , test "finished build has duration" <|
            \_ ->
                Header.header session
                    { model
                        | duration =
                            { startedAt = Just <| Time.millisToPosix 0
                            , finishedAt = Just <| Time.millisToPosix 1000
                            }
                    }
                    |> .leftWidgets
                    |> Common.contains
                        (Views.Duration <|
                            Views.Finished
                                { started =
                                    Views.Absolute "Jan 1 1970 12:00:00 AM" Nothing
                                , finished =
                                    Views.Absolute "Jan 1 1970 12:00:01 AM" Nothing
                                , duration = Views.JustSeconds 1
                                }
                        )
        , test "stops fetching history once current build appears" <|
            \_ ->
                ( model, [] )
                    |> Header.handleCallback
                        (Callback.BuildFetched <| Ok build)
                    |> Header.handleCallback
                        (Callback.BuildHistoryFetched <|
                            Ok
                                { content = [ build ]
                                , pagination =
                                    { previousPage = Nothing
                                    , nextPage = Nothing
                                    }
                                }
                        )
                    |> Tuple.second
                    |> Common.notContains (Effects.FetchBuildHistory jobId Nothing)
        , test "updates duration on finishing status event" <|
            \_ ->
                ( model, [] )
                    |> Header.handleCallback
                        (Callback.BuildFetched <|
                            Ok
                                { build
                                    | status = BuildStatusStarted
                                    , duration =
                                        { startedAt =
                                            Just <| Time.millisToPosix 0
                                        , finishedAt = Nothing
                                        }
                                }
                        )
                    |> Header.handleDelivery
                        (Subscription.EventsReceived <|
                            Ok
                                [ { data =
                                        STModels.BuildStatus BuildStatusSucceeded <|
                                            Time.millisToPosix 1000
                                  , url = "http://localhost:8080/api/v1/builds/0/events"
                                  }
                                ]
                        )
                    |> Tuple.first
                    |> Header.header session
                    |> .leftWidgets
                    |> Common.contains
                        (Views.Duration <|
                            Views.Finished
                                { started =
                                    Views.Absolute "Jan 1 1970 12:00:00 AM" Nothing
                                , finished =
                                    Views.Absolute "Jan 1 1970 12:00:01 AM" Nothing
                                , duration = Views.JustSeconds 1
                                }
                        )
        , test "status event from wrong build is discarded" <|
            \_ ->
                ( model, [] )
                    |> Header.handleCallback
                        (Callback.BuildFetched <| Ok build)
                    |> Header.handleDelivery
                        (Subscription.EventsReceived <|
                            Ok
                                [ { data = STModels.BuildStatus BuildStatusStarted <| Time.millisToPosix 0
                                  , url = "http://localhost:8080/api/v1/builds/1/events"
                                  }
                                ]
                        )
                    |> Tuple.first
                    |> Header.header session
                    |> .backgroundColor
                    |> Expect.equal BuildStatusPending
        , test "can use keyboard to switch builds after status change event" <|
            \_ ->
                ( model, [] )
                    |> Header.handleCallback
                        (Callback.BuildFetched <| Ok build)
                    |> Header.handleCallback
                        (Callback.BuildHistoryFetched <|
                            Ok
                                { content =
                                    [ build
                                    , { build | id = 1, name = "1" }
                                    ]
                                , pagination =
                                    { previousPage = Nothing
                                    , nextPage = Nothing
                                    }
                                }
                        )
                    |> Header.handleDelivery
                        (Subscription.EventsReceived <|
                            Ok
                                [ { data =
                                        STModels.BuildStatus BuildStatusSucceeded <|
                                            Time.millisToPosix 0
                                  , url = "http://localhost:8080/api/v1/builds/0/events"
                                  }
                                ]
                        )
                    |> Header.handleDelivery
                        (Subscription.KeyDown
                            { ctrlKey = False
                            , shiftKey = False
                            , metaKey = False
                            , code = Keyboard.L
                            }
                        )
                    |> Tuple.second
                    |> Common.contains
                        (Effects.NavigateTo "/teams/team/pipelines/pipeline/jobs/job/builds/1")
        , test "updates when route changes" <|
            \_ ->
                ( model, [] )
                    |> Header.handleCallback
                        (Callback.BuildFetched <| Ok build)
                    |> Header.handleCallback
                        (Callback.BuildHistoryFetched <|
                            Ok
                                { content =
                                    [ build
                                    , { build
                                        | id = 1
                                        , name = "1"
                                        , status = BuildStatusStarted
                                        , duration =
                                            { startedAt =
                                                Just <|
                                                    Time.millisToPosix 0
                                            , finishedAt = Nothing
                                            }
                                      }
                                    ]
                                , pagination =
                                    { previousPage = Nothing
                                    , nextPage = Nothing
                                    }
                                }
                        )
                    |> Header.changeToBuild
                        (Models.JobBuildPage
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = "job"
                            , buildName = "1"
                            }
                        )
                    |> Tuple.first
                    |> Header.header session
                    |> Expect.all
                        [ .backgroundColor
                            >> Expect.equal BuildStatusStarted
                        , .leftWidgets
                            >> Expect.equal
                                [ Views.Title "1" (Just jobId)
                                , Views.Duration <|
                                    Views.Running <|
                                        Views.Absolute
                                            "Jan 1 1970 12:00:00 AM"
                                            Nothing
                                ]
                        , .tabs
                            >> List.Extra.last
                            >> Maybe.map .isCurrent
                            >> Expect.equal (Just True)
                        ]
        ]


session : Session
session =
    { expandedTeams = Set.empty
    , pipelines = RemoteData.NotAsked
    , hovered = HoverState.NoHover
    , isSideBarOpen = False
    , screenSize = ScreenSize.Desktop
    , userState = UserState.UserStateLoggedOut
    , clusterName = ""
    , turbulenceImgSrc = ""
    , notFoundImgSrc = ""
    , csrfToken = ""
    , authToken = ""
    , pipelineRunningKeyframes = ""
    , timeZone = Time.utc
    }


model : Models.Model {}
model =
    { id = 0
    , name = "0"
    , job = Nothing
    , scrolledToCurrentBuild = False
    , history = []
    , duration = { startedAt = Nothing, finishedAt = Nothing }
    , status = BuildStatusPending
    , disableManualTrigger = False
    , now = Nothing
    , fetchingHistory = False
    , nextPage = Nothing
    , previousTriggerBuildByKey = False -- TODO WTF variable name
    , hasLoadedYet = False
    }


jobId =
    { teamName = "team"
    , pipelineName = "pipeline"
    , jobName = "job"
    }


build =
    { id = 0
    , name = "0"
    , job = Just jobId
    , status = model.status
    , duration = model.duration
    , reapTime = Nothing
    }
