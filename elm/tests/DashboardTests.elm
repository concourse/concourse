module DashboardTests exposing (..)

import Test exposing (..)
import Expect exposing (..)
import Concourse
import Concourse.Pipeline
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
    , id = 1
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


pipelines : List Concourse.Pipeline
pipelines =
    [ pipelineBosh
    , pipelineMain
    , pipelineMaintenance
    , pipelineMiami
    , pipelineTest
    ]


all : Test
all =
    describe "Dashboard Suite"
        (List.concat allTests)


allTests : List (List Test)
allTests =
    [ fuzzySearchPipelines
    , fuzzySearchTeams
    , searchTermList
    ]


fuzzySearchPipelines : List Test
fuzzySearchPipelines =
    [ test "returns pipeline names that match the search term" <|
        \_ ->
            Dashboard.filterBy "mai" pipelines
                |> Expect.equal
                    [ pipelineMain
                    , pipelineMaintenance
                    , pipelineMiami
                    ]
    , test "returns pipeline names that contain team in the search term" <|
        \_ ->
            Dashboard.filterBy ":team:" pipelines
                |> Expect.equal
                    [ pipelineMain ]
    , test "returns no pipeline names when does not match the search term" <|
        \_ ->
            Dashboard.filterBy "mar" pipelines
                |> Expect.equal []
    ]


fuzzySearchTeams : List Test
fuzzySearchTeams =
    [ test "returns team names that match the search term" <|
        \_ ->
            Dashboard.filterBy "team:YY" pipelines
                |> Expect.equal [ pipelineBosh, pipelineMain ]
    , test "returns no team names when does not match the search term" <|
        \_ ->
            Dashboard.filterBy "team:YYX" pipelines
                |> Expect.equal []
    ]


searchTermList : List Test
searchTermList =
    [ test "returns pipelines by team names that match the search term" <|
        \_ ->
            let
                queryList =
                    [ "team:YY", "main" ]
            in
                Dashboard.searchTermList queryList pipelines
                    |> Expect.equal [ pipelineMain ]
    ]
