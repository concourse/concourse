module Build.HeaderTests exposing (all)

import Application.Models exposing (Session)
import Build.Header.Header as Header
import Build.Header.Models as Models
import Build.Header.Views as Views
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Expect
import HoverState
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
                    Header.header session model build
                        |> .leftWidgets
                        |> List.member (Views.Title build.name Nothing)
                        |> Expect.equal True
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


build : Concourse.Build
build =
    { id = 0
    , name = "0"
    , job = Nothing
    , status = BuildStatusPending
    , duration =
        { startedAt = Nothing
        , finishedAt = Nothing
        }
    , reapTime = Nothing
    }


model : Models.Model {}
model =
    { page = Models.OneOffBuildPage 0
    , scrolledToCurrentBuild = False
    , history = []
    , build = RemoteData.Success build -- TODO remove duplication
    , disableManualTrigger = False
    , now = Nothing
    , fetchingHistory = False
    , nextPage = Nothing
    , previousTriggerBuildByKey = False -- TODO WTF variable name
    , browsingIndex = 0
    }
