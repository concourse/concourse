module DashboardTests exposing (..)

import Test exposing (..)
import Expect exposing (..)
import Concourse
import Concourse.Build
import Concourse.BuildStatus
import Concourse.Pipeline
import Date
import Dict exposing (..)
import RemoteData exposing (..)
import Dashboard


listGroups =
    [ { name = "aa", jobs = [ "bb", "cc" ], resources = [ "all", "eh" ] } ]


pipelineBosh =
    { groups = listGroups
    , id = 3
    , name = "bosh"
    , paused = False
    , public = True
    , teamName = "YYZ"
    }


pipelineMain =
    { groups = listGroups
    , id = 123
    , name = "main:team:pipeline"
    , paused = False
    , public = True
    , teamName = "YYZ"
    }


pipelineMaintenance =
    { groups = listGroups
    , id = 2
    , name = "maintenance"
    , paused = False
    , public = True
    , teamName = "SFO"
    }


pipelineMiami =
    { groups = listGroups
    , id = 3
    , name = "miami"
    , paused = False
    , public = True
    , teamName = "DWF"
    }


pipelineTest =
    { groups = listGroups
    , id = 3
    , name = "test"
    , paused = False
    , public = True
    , teamName = "DWF"
    }


pipelineSucceeded =
    { groups = listGroups
    , id = 10
    , name = "test-succeeded"
    , paused = False
    , public = True
    , teamName = "DWF"
    }


pipelineErrored =
    { groups = listGroups
    , id = 20
    , name = "test-errored"
    , paused = False
    , public = True
    , teamName = "DWF"
    }


pipelineAborted =
    { groups = listGroups
    , id = 30
    , name = "test-aborted"
    , paused = False
    , public = True
    , teamName = "DWF"
    }


pipelinePaused =
    { groups = listGroups
    , id = 40
    , name = "test-paused"
    , paused = False
    , public = True
    , teamName = "DWF"
    }


pipelineFailed =
    { groups = listGroups
    , id = 50
    , name = "test-failed"
    , paused = False
    , public = True
    , teamName = "DWF"
    }


pipelinePending =
    { groups = listGroups
    , id = 60
    , name = "test-pending"
    , paused = False
    , public = True
    , teamName = "DWF"
    }


pipelineRunning =
    { groups = listGroups
    , id = 60
    , name = "test-running"
    , paused = False
    , public = True
    , teamName = "DWF"
    }


pipelines : List Concourse.Pipeline
pipelines =
    [ pipelineBosh
    , pipelineMain
    , pipelineMaintenance
    , pipelineMiami
    , pipelineTest
    ]


statusPipelines : List Dashboard.StatusPipeline
statusPipelines =
    [ { pipeline = pipelineBosh, status = "" }
    , { pipeline = pipelineMain, status = "" }
    , { pipeline = pipelineMaintenance, status = "" }
    , { pipeline = pipelineMiami, status = "" }
    , { pipeline = pipelineTest, status = "" }
    , { pipeline = pipelineSucceeded, status = "succeeded" }
    , { pipeline = pipelineErrored, status = "errored" }
    , { pipeline = pipelineAborted, status = "aborted" }
    , { pipeline = pipelinePaused, status = "paused" }
    , { pipeline = pipelineFailed, status = "failed" }
    , { pipeline = pipelinePending, status = "pending" }
    , { pipeline = pipelineRunning, status = "running" }
    ]


all : Test
all =
    describe "Dashboard Suite"
        (List.concat allTests)


allTests : List (List Test)
allTests =
    [ fuzzySearchPipelines
    , fuzzySearchTeams
    , fuzzySearchStatusPipeline
    , searchTermList
    , [ pipelineStatus ]
    ]


fuzzySearchPipelines : List Test
fuzzySearchPipelines =
    [ test "returns pipeline names that match the search term" <|
        \_ ->
            Dashboard.filterBy "mai" statusPipelines
                |> Expect.equal
                    [ pipelineMain
                    , pipelineMaintenance
                    , pipelineMiami
                    ]
    , test "returns pipeline names that contain team in the search term" <|
        \_ ->
            Dashboard.filterBy ":team:" statusPipelines
                |> Expect.equal
                    [ pipelineMain ]
    , test "returns no pipeline names when does not match the search term" <|
        \_ ->
            Dashboard.filterBy "mar" statusPipelines
                |> Expect.equal []
    ]


fuzzySearchTeams : List Test
fuzzySearchTeams =
    [ test "returns team names that match the search term" <|
        \_ ->
            Dashboard.filterBy "team:YY" statusPipelines
                |> Expect.equal [ pipelineBosh, pipelineMain ]
    , test "returns no team names when does not match the search term" <|
        \_ ->
            Dashboard.filterBy "team:YYX" statusPipelines
                |> Expect.equal []
    ]


fuzzySearchStatusPipeline : List Test
fuzzySearchStatusPipeline =
    [ test "returns pipeline that match the search status succeeded" <|
        \_ ->
            Dashboard.filterBy "status:succeeded" statusPipelines
                |> Expect.equal [ pipelineSucceeded ]
    , test "returns pipeline that match the search status errored" <|
        \_ ->
            Dashboard.filterBy "status:errored" statusPipelines
                |> Expect.equal [ pipelineErrored ]
    , test "returns pipeline that match the search status aborted" <|
        \_ ->
            Dashboard.filterBy "status:aborted" statusPipelines
                |> Expect.equal [ pipelineAborted ]
    , test "returns pipeline that match the search status paused" <|
        \_ ->
            Dashboard.filterBy "status:paused" statusPipelines
                |> Expect.equal [ pipelinePaused ]
    , test "returns pipeline that match the search status failed" <|
        \_ ->
            Dashboard.filterBy "status:failed" statusPipelines
                |> Expect.equal [ pipelineFailed ]
    , test "returns pipeline that match the search status pending" <|
        \_ ->
            Dashboard.filterBy "status:pending" statusPipelines
                |> Expect.equal [ pipelinePending ]
    , test "returns pipeline that match the search status running" <|
        \_ ->
            Dashboard.filterBy "status:running" statusPipelines
                |> Expect.equal [ pipelineRunning ]
    , test "returns no pipeline if status does not match the search term" <|
        \_ ->
            Dashboard.filterBy "status:failure" statusPipelines
                |> Expect.equal []
    ]


searchTermList : List Test
searchTermList =
    [ test "returns pipelines by status and team names that match the search term" <|
        \_ ->
            let
                jobIndentifierMock : Concourse.JobIdentifier
                jobIndentifierMock =
                    { jobName = "failing", pipelineName = "main:team:pipeline", teamName = "YYZ" }

                buildMock : Concourse.Build
                buildMock =
                    { duration = { finishedAt = Nothing, startedAt = Nothing }
                    , id = 1
                    , job = Just jobIndentifierMock
                    , name = "failing"
                    , reapTime = Nothing
                    , status = Concourse.BuildStatusFailed
                    }

                jobMock : List Concourse.Job
                jobMock =
                    [ { disableManualTrigger = False
                      , finishedBuild = Just buildMock
                      , groups = []
                      , inputs = []
                      , name = "failing"
                      , pipelineName = "main:team:pipeline"
                      , teamName = "YYZ"
                      , nextBuild = Nothing
                      , outputs = []
                      , paused = False
                      , pipeline = { teamName = "YYZ", pipelineName = "main:team:pipeline" }
                      , transitionBuild = Just buildMock
                      }
                    ]

                modelMock : Dashboard.Model
                modelMock =
                    { topBar = { user = RemoteData.NotAsked, query = "" }
                    , pipelines = RemoteData.NotAsked
                    , jobs = Dict.fromList [ ( 2, RemoteData.Success jobMock ) ]
                    , now = Nothing
                    , turbulenceImgSrc = ""
                    , concourseVersion = ""
                    , hideFooter = False
                    , showHelp = False
                    , hideFooterCounter = 0
                    , fetchedPipelines = []
                    }

                queryStatusTeamPipeline =
                    [ "status:fail", "team:SF", "main" ]
            in
                Dashboard.searchTermList modelMock queryStatusTeamPipeline pipelines
                    |> Expect.equal [ pipelineMaintenance ]
    ]


pipelineStatus : Test
pipelineStatus =
    let
        someJobInfo =
            { jobName = "some-job"
            , pipelineName = "some-pipeline"
            , teamName = "some-team"
            }

        someBuild : Concourse.Build
        someBuild =
            { id = 1
            , name = "build-succeeded"
            , job = Just someJobInfo
            , status = Concourse.BuildStatusSucceeded
            , duration =
                { startedAt = Just (Date.fromTime 0)
                , finishedAt = Just (Date.fromTime 0)
                }
            , reapTime = Just (Date.fromTime 0)
            }

        someBuildAborted : Concourse.Build
        someBuildAborted =
            { id = 111
            , name = "build-aborted"
            , job = Just someJobInfo
            , status = Concourse.BuildStatusAborted
            , duration =
                { startedAt = Just (Date.fromTime 0)
                , finishedAt = Just (Date.fromTime 0)
                }
            , reapTime = Just (Date.fromTime 0)
            }

        someBuildErrored : Concourse.Build
        someBuildErrored =
            { id = 222
            , name = "build-errored"
            , job = Just someJobInfo
            , status = Concourse.BuildStatusErrored
            , duration =
                { startedAt = Just (Date.fromTime 0)
                , finishedAt = Just (Date.fromTime 0)
                }
            , reapTime = Just (Date.fromTime 0)
            }

        someBuildFailed : Concourse.Build
        someBuildFailed =
            { id = 333
            , name = "build-failed"
            , job = Just someJobInfo
            , status = Concourse.BuildStatusFailed
            , duration =
                { startedAt = Just (Date.fromTime 0)
                , finishedAt = Just (Date.fromTime 0)
                }
            , reapTime = Just (Date.fromTime 0)
            }

        someBuildPending : Concourse.Build
        someBuildPending =
            { id = 444
            , name = "build-pending"
            , job = Just someJobInfo
            , status = Concourse.BuildStatusPending
            , duration =
                { startedAt = Just (Date.fromTime 0)
                , finishedAt = Just (Date.fromTime 0)
                }
            , reapTime = Just (Date.fromTime 0)
            }

        someBuildPaused : Concourse.Build
        someBuildPaused =
            { id = 555
            , name = "build-paused"
            , job = Just someJobInfo
            , status = Concourse.BuildStatusPending
            , duration =
                { startedAt = Just (Date.fromTime 0)
                , finishedAt = Just (Date.fromTime 0)
                }
            , reapTime = Just (Date.fromTime 0)
            }

        someBuildRunning : Concourse.Build
        someBuildRunning =
            { id = 666
            , name = "build-started"
            , job = Just someJobInfo
            , status = Concourse.BuildStatusStarted
            , duration =
                { startedAt = Just (Date.fromTime 0)
                , finishedAt = Just (Date.fromTime 0)
                }
            , reapTime = Just (Date.fromTime 0)
            }

        someBuildSucceeded : Concourse.Build
        someBuildSucceeded =
            { id = 777
            , name = "build-succeeded"
            , job = Just someJobInfo
            , status = Concourse.BuildStatusSucceeded
            , duration =
                { startedAt = Just (Date.fromTime 0)
                , finishedAt = Just (Date.fromTime 0)
                }
            , reapTime = Just (Date.fromTime 0)
            }

        abortedJob =
            { nextBuild = Nothing
            , finishedBuild = Just someBuildAborted
            , paused = False
            }

        erroredJob =
            { nextBuild = Nothing
            , finishedBuild = Just someBuildErrored
            , paused = False
            }

        failedJob =
            { nextBuild = Nothing
            , finishedBuild = Just someBuildFailed
            , paused = False
            }

        pausedJob =
            { nextBuild = Nothing
            , finishedBuild = Just someBuildPaused
            , paused = True
            }

        pendingJob =
            { nextBuild = Just someBuildPending
            , finishedBuild = Nothing
            , paused = False
            }

        runningJob =
            { nextBuild = Just someBuildRunning
            , finishedBuild = Nothing
            , paused = False
            }

        succeededJob =
            { nextBuild = Nothing
            , finishedBuild = Just someBuildSucceeded
            , paused = False
            }

        pipeline paused jobs =
            { pipeline = { paused = paused }
            , jobs = (RemoteData.Success jobs)
            }
    in
        describe "many statuses of a pipeline"
            [ test "returns status failed" <|
                \_ ->
                    Dashboard.pipelineStatus (pipeline False [ failedJob, erroredJob, abortedJob, succeededJob ])
                        |> Expect.equal Concourse.PipelineStatusFailed
            , test "returns status errored" <|
                \_ ->
                    Dashboard.pipelineStatus (pipeline False [ erroredJob, abortedJob, succeededJob ])
                        |> Expect.equal Concourse.PipelineStatusErrored
            , test "returns status aborted" <|
                \_ ->
                    Dashboard.pipelineStatus (pipeline False [ abortedJob, succeededJob ])
                        |> Expect.equal Concourse.PipelineStatusAborted
            , test "returns status paused" <|
                \_ ->
                    Dashboard.pipelineStatus (pipeline True [ pausedJob ])
                        |> Expect.equal Concourse.PipelineStatusPaused
            , test "returns status pending" <|
                \_ ->
                    Dashboard.pipelineStatus (pipeline False [ pendingJob ])
                        |> Expect.equal Concourse.PipelineStatusPending
            , test "returns status running for aborted and running jobs" <|
                \_ ->
                    Dashboard.pipelineStatus (pipeline False [ runningJob, abortedJob ])
                        |> Expect.equal Concourse.PipelineStatusRunning
            , test "returns status succeeded" <|
                \_ ->
                    Dashboard.pipelineStatus (pipeline False [ succeededJob ])
                        |> Expect.equal Concourse.PipelineStatusSucceeded
            , test "returns last status as failed" <|
                \_ ->
                    Dashboard.lastPipelineStatus (pipeline False [ failedJob, erroredJob, abortedJob, succeededJob ])
                        |> Expect.equal Concourse.PipelineStatusFailed
            , test "returns last status as errored" <|
                \_ ->
                    Dashboard.lastPipelineStatus (pipeline False [ erroredJob, abortedJob, succeededJob ])
                        |> Expect.equal Concourse.PipelineStatusErrored
            , test "returns last status as aborted" <|
                \_ ->
                    Dashboard.lastPipelineStatus (pipeline False [ abortedJob, succeededJob ])
                        |> Expect.equal Concourse.PipelineStatusAborted
            , test "returns last status as succeeded" <|
                \_ ->
                    Dashboard.lastPipelineStatus (pipeline False [ succeededJob ])
                        |> Expect.equal Concourse.PipelineStatusSucceeded
            ]
