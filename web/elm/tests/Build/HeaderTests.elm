module Build.HeaderTests exposing (all)

import Application.Models exposing (Session)
import Build.Header.Header as Header
import Build.Header.Models as Models
import Build.Header.Views as Views
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Expect
import HoverState
import Message.Callback as Callback
import Message.Effects as Effects
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
                        |> List.member (Views.Title "0" Nothing)
                        |> Expect.equal True
            ]
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
                    |> List.member
                        (Views.Duration <|
                            Views.Cancelled <|
                                Views.Absolute "Jan 1 1970 12:00:00 AM" Nothing
                        )
                    |> Expect.equal True
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
                    |> List.member
                        (Views.Duration <|
                            Views.Finished
                                { started =
                                    Views.Absolute "Jan 1 1970 12:00:00 AM" Nothing
                                , finished =
                                    Views.Absolute "Jan 1 1970 12:00:01 AM" Nothing
                                , duration = Views.JustSeconds 1
                                }
                        )
                    |> Expect.equal True
        , test "stops fetching history once current build appears" <|
            \_ ->
                let
                    jobId =
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "job"
                        }

                    build =
                        { id = 0
                        , name = "4"
                        , job = Just jobId
                        , status = model.status
                        , duration = model.duration
                        , reapTime = Nothing
                        }
                in
                ( model, [] )
                    |> Header.handleCallback
                        (Callback.BuildFetched <| Ok ( 0, build ))
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
                    |> List.member (Effects.FetchBuildHistory jobId Nothing)
                    |> Expect.equal False
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
    , browsingIndex = 0
    }
