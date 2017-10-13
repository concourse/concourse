module DashboardTests exposing (..)

import Test exposing (..)
import Expect exposing (..)
import Concourse
import Concourse.Pipeline
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
    , url = "http://google.com"
    }


pipelineMain =
    { groups = listGroups
    , id = 123
    , name = "main:team:pipeline"
    , paused = False
    , public = True
    , teamName = "YYZ"
    , url = "http://google.com"
    }


pipelineMaintenance =
    { groups = listGroups
    , id = 2
    , name = "maintenance"
    , paused = False
    , public = True
    , teamName = "SFO"
    , url = "http://google.com"
    }


pipelineMiami =
    { groups = listGroups
    , id = 3
    , name = "miami"
    , paused = False
    , public = True
    , teamName = "DWF"
    , url = "http://google.com"
    }


pipelineTest =
    { groups = listGroups
    , id = 3
    , name = "test"
    , paused = False
    , public = True
    , teamName = "DWF"
    , url = "http://google.com"
    }


pipelineSucceeded =
    { groups = listGroups
    , id = 10
    , name = "test-succeeded"
    , paused = False
    , public = True
    , teamName = "DWF"
    , url = "http://google.com"
    }


pipelineErrored =
    { groups = listGroups
    , id = 20
    , name = "test-errored"
    , paused = False
    , public = True
    , teamName = "DWF"
    , url = "http://google.com"
    }


pipelineAborted =
    { groups = listGroups
    , id = 30
    , name = "test-aborted"
    , paused = False
    , public = True
    , teamName = "DWF"
    , url = "http://google.com"
    }


pipelinePaused =
    { groups = listGroups
    , id = 40
    , name = "test-paused"
    , paused = False
    , public = True
    , teamName = "DWF"
    , url = "http://google.com"
    }


pipelineFailed =
    { groups = listGroups
    , id = 50
    , name = "test-failed"
    , paused = False
    , public = True
    , teamName = "DWF"
    , url = "http://google.com"
    }


pipelinePending =
    { groups = listGroups
    , id = 60
    , name = "test-pending"
    , paused = False
    , public = True
    , teamName = "DWF"
    , url = "http://google.com"
    }


pipelineStarted =
    { groups = listGroups
    , id = 60
    , name = "test-started"
    , paused = False
    , public = True
    , teamName = "DWF"
    , url = "http://google.com"
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
    , { pipeline = pipelineStarted, status = "started" }
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
    , test "returns pipeline that match the search status started" <|
        \_ ->
            Dashboard.filterBy "status:started" statusPipelines
                |> Expect.equal [ pipelineStarted ]
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
                    , url = "http://example.com"
                    }

                jobMock : List Concourse.Job
                jobMock =
                    [ { disableManualTrigger = False
                      , finishedBuild = Just buildMock
                      , groups = []
                      , inputs = []
                      , name = "failing"
                      , nextBuild = Just buildMock
                      , outputs = []
                      , paused = False
                      , pipeline = { teamName = "YYZ", pipelineName = "main:team:pipeline" }
                      , transitionBuild = Just buildMock
                      , url = "http://example.com"
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
                    , hideFooterCounter = 0
                    , fetchedPipelines = []
                    , query = ""
                    }

                queryStatusTeamPipeline =
                    [ "status:fail", "team:SF", "main" ]
            in
                Dashboard.searchTermList modelMock queryStatusTeamPipeline pipelines
                    |> Expect.equal [ pipelineMaintenance ]
    ]
