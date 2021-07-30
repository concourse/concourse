module Build.HeaderTests exposing (all)

import Application.Models exposing (Session)
import Build.Header.Header as Header
import Build.Header.Models as Models exposing (CommentBarVisibility(..))
import Build.Header.Views as Views
import Build.StepTree.Models as STModels
import Common
import Concourse exposing (JsonValue(..))
import Concourse.BuildStatus exposing (BuildStatus(..))
import Data
import Dict
import Expect
import HoverState
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
import Test.Html.Query as Query
import Test.Html.Selector
    exposing
        ( containing
        , text
        )
import Time
import UserState


all : Test
all =
    describe "build page header"
        [ describe "title"
            [ describe "job build" <|
                let
                    job =
                        Data.jobId
                            |> Data.withTeamName "some-team"
                            |> Data.withPipelineName "some-pipeline"
                            |> Data.withJobName "some-job"

                    jobBuildModel =
                        { model | name = "123", job = Just job }
                in
                [ test "contains the build name and job name" <|
                    \_ ->
                        Header.header session jobBuildModel
                            |> .leftWidgets
                            |> Common.contains (Views.Title "123" (Just job) Nothing)
                , test "shows job and build name as number" <|
                    \_ ->
                        Header.view session jobBuildModel
                            |> Query.fromHtml
                            |> Query.has
                                [ containing [ text "some-job" ]
                                , containing [ text "#123" ]
                                ]
                ]
            , describe "non-job build" <|
                let
                    nonJobBuild =
                        { model | name = "check", job = Nothing }
                in
                [ test "contains the build name" <|
                    \_ ->
                        Header.header session nonJobBuild
                            |> .leftWidgets
                            |> Common.contains (Views.Title "check" Nothing Nothing)
                , test "shows build name, not as a number" <|
                    \_ ->
                        Header.view session nonJobBuild
                            |> Query.fromHtml
                            |> Expect.all
                                [ Query.has [ containing [ text "check" ] ]
                                , Query.hasNot [ containing [ text "#" ] ]
                                ]
                ]
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
            [ describe "re-run"
                [ test "does not appear on non-running one-off build" <|
                    \_ ->
                        Header.header session
                            { model | status = BuildStatusSucceeded }
                            |> .rightWidgets
                            |> List.filterMap
                                (\w ->
                                    case w of
                                        Views.Button b ->
                                            b

                                        _ ->
                                            Nothing
                                )
                            |> List.map .type_
                            |> Common.notContains Views.Rerun
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
                , test "pipeline lookup considers instance vars" <|
                    \_ ->
                        let
                            instanceVars =
                                Dict.fromList [ ( "foo", JsonString "bar" ) ]
                        in
                        { model | status = BuildStatusSucceeded, job = Just (jobId |> Data.withPipelineInstanceVars instanceVars) }
                            |> Header.header
                                { session
                                    | pipelines =
                                        RemoteData.Success
                                            [ Data.pipeline jobId.teamName 0
                                                |> Data.withName jobId.pipelineName
                                                |> Data.withArchived True
                                            , Data.pipeline jobId.teamName 1
                                                |> Data.withName jobId.pipelineName
                                                |> Data.withInstanceVars instanceVars
                                                |> Data.withArchived False
                                            ]
                                }
                            |> .rightWidgets
                            |> Common.contains
                                (Views.Button <|
                                    Just
                                        { type_ = Views.Rerun
                                        , isClickable = True
                                        , backgroundShade = Views.Light
                                        , backgroundColor = BuildStatusSucceeded
                                        }
                                )
                ]
            , describe "comment"
                [ test "shown when user is authorized to edit comment" <|
                    \_ ->
                        Header.header session
                            { model | job = Just jobId }
                            |> .rightWidgets
                            |> Common.contains
                                (Views.Button <|
                                    Just
                                        { type_ = Views.ToggleComment
                                        , isClickable = True
                                        , backgroundShade = Views.Light
                                        , backgroundColor = model.status
                                        }
                                )
                , test "hidden when user is not authorized to edit comment" <|
                    \_ ->
                        Header.header session
                            { model | job = Just jobId, authorized = False }
                            |> .rightWidgets
                            |> List.filterMap
                                (\w ->
                                    case w of
                                        Views.Button b ->
                                            b

                                        _ ->
                                            Nothing
                                )
                            |> List.map .type_
                            |> Common.notContains Views.ToggleComment
                , test "clicking toggles visiblity of comment" <|
                    \_ ->
                        ( { model | job = Just jobId }
                        , []
                        )
                            |> Header.update (Message.Click Message.ToggleBuildCommentButton)
                            |> Tuple.first
                            |> (\updatedModel ->
                                    case updatedModel.comment of
                                        Visible _ ->
                                            Expect.pass

                                        _ ->
                                            Expect.fail
                                                ("Expected model.comment ("
                                                    ++ Debug.toString updatedModel.comment
                                                    ++ ") to be Visible"
                                                )
                               )
                , test "archived pipeline's only have a toggle comment button" <|
                    \_ ->
                        { model | job = Just jobId, status = BuildStatusSucceeded }
                            |> Header.header
                                { session
                                    | pipelines =
                                        RemoteData.Success
                                            [ Data.pipeline jobId.teamName 0
                                                |> Data.withName jobId.pipelineName
                                                |> Data.withArchived True
                                            ]
                                }
                            |> .rightWidgets
                            |> Expect.equal
                                [ Views.Button <|
                                    Just
                                        { type_ = Views.ToggleComment
                                        , isClickable = True
                                        , backgroundShade = Views.Light
                                        , backgroundColor = BuildStatusSucceeded
                                        }
                                ]
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
                                        , createdBy = Just <| "some-one"
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
                                [ Views.Title "1" (Just jobId) (Just "some-one")
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
    , featureFlags = Data.featureFlags
    , turbulenceImgSrc = ""
    , notFoundImgSrc = ""
    , csrfToken = ""
    , authToken = ""
    , pipelineRunningKeyframes = ""
    , timeZone = Time.utc
    , favoritedPipelines = Set.empty
    , favoritedInstanceGroups = Set.empty
    , route = Routes.Build { id = Data.jobBuildId, highlight = Routes.HighlightNothing, groups = [] }
    }


model : Models.Model {}
model =
    { id = 0
    , name = "0"
    , authorized = True
    , job = Nothing
    , scrolledToCurrentBuild = False
    , history = []
    , duration = { startedAt = Nothing, finishedAt = Nothing }
    , createdBy = Nothing
    , status = BuildStatusPending
    , disableManualTrigger = False
    , now = Nothing
    , fetchingHistory = False
    , nextPage = Nothing
    , hasLoadedYet = False
    , comment = Hidden ""
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
