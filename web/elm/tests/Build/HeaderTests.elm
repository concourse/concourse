module Build.HeaderTests exposing (all)

import Application.Models exposing (Session)
import Build.Header.Header as Header
import Build.Header.Models as Models
import Build.Header.Views as Views
import Build.Shortcuts as Shortcuts
import Build.StepTree.Models as STModels
import Common
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Data
import Expect
import HoverState
import Keyboard
import List.Extra
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Message
import Message.Subscription as Subscription
import RemoteData
import Routes
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
        , describe "duration"
            [ test "pending build has no duration" <|
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
            ]
        , describe "buttons"
            [ describe "trigger"
                [ test "has tooltip on hover when manual triggering is disabled" <|
                    \_ ->
                        Header.header
                            { session
                                | hovered =
                                    HoverState.Hovered
                                        Message.TriggerBuildButton
                            }
                            { model | disableManualTrigger = False }
                            |> .rightWidgets
                            |> Common.notContains
                                (Views.Button <|
                                    Just
                                        { type_ = Views.Trigger
                                        , isClickable = False
                                        , backgroundShade = Views.Dark
                                        , backgroundColor = model.status
                                        , tooltip = True
                                        }
                                )
                ]
            , describe "re-run"
                [ test "does not appear on non-running one-off build" <|
                    \_ ->
                        Header.header session
                            { model | status = BuildStatusSucceeded }
                            |> .rightWidgets
                            |> Common.notContains
                                (Views.Button <|
                                    Just
                                        { type_ = Views.Rerun
                                        , isClickable = True
                                        , backgroundShade = Views.Light
                                        , backgroundColor = BuildStatusSucceeded
                                        , tooltip = False
                                        }
                                )
                , test "appears on non-running job build" <|
                    \_ ->
                        Header.header session
                            { model | status = BuildStatusSucceeded, job = Just jobId }
                            |> .rightWidgets
                            |> Common.contains
                                (Views.Button <|
                                    Just
                                        { type_ = Views.Rerun
                                        , isClickable = True
                                        , backgroundShade = Views.Light
                                        , backgroundColor = BuildStatusSucceeded
                                        , tooltip = False
                                        }
                                )
                , test "is hoverable with tooltip" <|
                    \_ ->
                        { model | status = BuildStatusSucceeded, job = Just jobId }
                            |> Header.header
                                { session
                                    | hovered =
                                        HoverState.Hovered
                                            Message.RerunBuildButton
                                }
                            |> .rightWidgets
                            |> Common.contains
                                (Views.Button <|
                                    Just
                                        { type_ = Views.Rerun
                                        , isClickable = True
                                        , backgroundShade = Views.Dark
                                        , backgroundColor = BuildStatusSucceeded
                                        , tooltip = True
                                        }
                                )
                , test "clicking sends RerunJobBuild API call" <|
                    \_ ->
                        ( { model | status = BuildStatusSucceeded, job = Just jobId }
                        , []
                        )
                            |> Header.update (Message.Click Message.RerunBuildButton)
                            |> Tuple.second
                            |> Common.contains
                                (Effects.RerunJobBuild
                                    (Data.longJobBuildId |> Data.withBuildName model.name)
                                )
                , test "archived pipeline's have no right widgets" <|
                    \_ ->
                        { model | status = BuildStatusSucceeded, job = Just jobId }
                            |> Header.header
                                { session
                                    | pipelines =
                                        RemoteData.Success
                                            [ Data.pipeline Data.teamName 1
                                                |> Data.withArchived True
                                            ]
                                }
                            |> .rightWidgets
                            |> Expect.equal []
                ]
            ]
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
        , test "re-run build appears in the correct spot" <|
            \_ ->
                ( model, [] )
                    |> Header.handleCallback
                        (Callback.BuildHistoryFetched <|
                            Ok
                                { content =
                                    [ { build | name = "4" }
                                    , { build | id = 1, name = "0" }
                                    ]
                                , pagination =
                                    { previousPage = Nothing
                                    , nextPage = Nothing
                                    }
                                }
                        )
                    |> Header.handleCallback
                        (Callback.BuildTriggered <| Ok { build | id = 2, name = "0.1" })
                    |> Tuple.first
                    |> Header.header session
                    |> .tabs
                    |> List.map .name
                    |> Expect.equal [ "4", "0.1", "0" ]
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
                        (Models.JobBuildPage Data.longJobBuildId)
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
    { expandedTeamsInAllPipelines = Set.empty
    , collapsedTeamsInFavorites = Set.empty
    , pipelines = RemoteData.NotAsked
    , hovered = HoverState.NoHover
    , sideBarState =
        { isOpen = False
        , width = 275
        }
    , draggingSideBar = False
    , screenSize = ScreenSize.Desktop
    , userState = UserState.UserStateLoggedOut
    , clusterName = ""
    , version = ""
    , turbulenceImgSrc = ""
    , notFoundImgSrc = ""
    , csrfToken = ""
    , authToken = ""
    , pipelineRunningKeyframes = ""
    , timeZone = Time.utc
    , favoritedPipelines = Set.empty
    , route = Routes.Build { id = Data.jobBuildId, highlight = Routes.HighlightNothing }
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
    , hasLoadedYet = False
    }


build : Concourse.Build
build =
    Data.jobBuild model.status
        |> Data.withId 0
        |> Data.withName "0"
        |> Data.withJob (Just jobId)
        |> Data.withDuration model.duration


jobId : Concourse.JobIdentifier
jobId =
    Data.jobId
