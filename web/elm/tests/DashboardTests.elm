module DashboardTests exposing
    ( afterSeconds
    , all
    , almostBlack
    , amber
    , apiData
    , blue
    , brown
    , circularJobs
    , darkGrey
    , fadedGreen
    , givenDataAndUser
    , givenDataUnauthenticated
    , green
    , iconSelector
    , job
    , jobWithNameTransitionedAt
    , lightGrey
    , middleGrey
    , orange
    , otherJob
    , red
    , running
    , userWithRoles
    , whenOnDashboard
    , whenOnDashboardViewingAllPipelines
    , white
    )

import Application.Application as Application
import Assets
import Common
    exposing
        ( defineHoverBehaviour
        , isColorWithStripes
        , pipelineRunningKeyframes
        )
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Cli as Cli
import Concourse.PipelineStatus exposing (PipelineStatus(..))
import Data
import Dict
import Expect exposing (Expectation)
import Html.Attributes as Attr
import Http
import Keyboard
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Msgs
import Message.Subscription as Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as ApplicationMsgs
import Routes
import Set
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector
    exposing
        ( Selector
        , attribute
        , class
        , containing
        , id
        , style
        , tag
        , text
        )
import Time
import Url


almostBlack : String
almostBlack =
    "#1e1d1d"


middleGrey : String
middleGrey =
    "#3d3c3c"


lightGrey : String
lightGrey =
    "#9b9b9b"


menuGrey : String
menuGrey =
    "#868585"


green : String
green =
    "#11c560"


blue : String
blue =
    "#3498db"


darkGrey : String
darkGrey =
    "#2a2929"


red : String
red =
    "#ed4b35"


amber : String
amber =
    "#f5a623"


brown : String
brown =
    "#8b572a"


white : String
white =
    "#ffffff"


fadedGreen : String
fadedGreen =
    "#419867"


orange : String
orange =
    "#e67e22"


pipelineRunningKeyframes : String
pipelineRunningKeyframes =
    "pipeline-running"


flags : Application.Flags
flags =
    { turbulenceImgSrc = ""
    , notFoundImgSrc = ""
    , csrfToken = csrfToken
    , authToken = ""
    , pipelineRunningKeyframes = pipelineRunningKeyframes
    }


all : Test
all =
    describe "Dashboard"
        [ test "requests screen size on page load" <|
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
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains Effects.GetScreenSize
        , test "requests cluster info on page load" <|
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
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains Effects.FetchClusterInfo
        , test "requests all resources on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , pipelineRunningKeyframes = ""
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains Effects.FetchAllResources
        , test "requests all jobs on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , pipelineRunningKeyframes = ""
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains Effects.FetchAllJobs
        , test "requests all pipelines on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , pipelineRunningKeyframes = ""
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> Common.contains Effects.FetchAllPipelines
        , test "redirects to login if any data call gives a 401" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <|
                            Data.httpUnauthorized
                        )
                    |> Tuple.second
                    |> Expect.equal [ Effects.RedirectToLogin ]
        , test "retries the request after 1 second if ListAllJobs call gives a 503" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Err <|
                                Http.BadStatus
                                    { url = "http://example.com"
                                    , status =
                                        { code = 503
                                        , message = "service unavailable"
                                        }
                                    , headers = Dict.empty
                                    , body = ""
                                    }
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (ClockTicked OneSecond <|
                            Time.millisToPosix 1000
                        )
                    |> Tuple.second
                    |> Expect.equal [ Effects.FetchAllJobs ]
        , test "only retries the request once per 503 response" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Err <|
                                Http.BadStatus
                                    { url = "http://example.com"
                                    , status =
                                        { code = 503
                                        , message = "service unavailable"
                                        }
                                    , headers = Dict.empty
                                    , body = ""
                                    }
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (ClockTicked OneSecond <|
                            Time.millisToPosix 1000
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (ClockTicked OneSecond <|
                            Time.millisToPosix 1000
                        )
                    |> Tuple.second
                    |> Expect.equal []
        , test "does not show turbulence screen on 503" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Err <|
                                Http.BadStatus
                                    { url = "http://example.com"
                                    , status =
                                        { code = 503
                                        , message = "service unavailable"
                                        }
                                    , headers = Dict.empty
                                    , body = ""
                                    }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "experiencing turbulence" ]
        , test "shows turbulence view if the all teams call gives a bad status error" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ text "experiencing turbulence" ]
        , test "recovers from turbulence view if all teams call succeeds" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <| Ok [])
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "experiencing turbulence" ]
        , test "shows turbulence view if the all resources call gives a bad status error" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllResourcesFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ text "experiencing turbulence" ]
        , test "recovers from turbulence view if all resources call succeeds" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllResourcesFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllResourcesFetched <| Ok [])
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "experiencing turbulence" ]
        , test "shows turbulence view if the all jobs call gives a bad status error" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ text "experiencing turbulence" ]
        , test "recovers from turbulence view if all jobs call succeeds" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <| Ok [])
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "experiencing turbulence" ]
        , test "shows turbulence view if the all pipelines call gives a bad status error" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ text "experiencing turbulence" ]
        , test "recovers from turbulence view if all pipelines call succeeds" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <| Ok [])
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "experiencing turbulence" ]
        , test "does not recover from turbulence view if some endpoints are still errored" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <| Data.httpInternalServerError)
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <| Ok [])
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ text "experiencing turbulence" ]
        , test "title says 'Dashboard - Concourse'" <|
            \_ ->
                Common.init "/"
                    |> Application.view
                    |> .title
                    |> Expect.equal "Dashboard - Concourse"
        , test "renders cluster name at top left" <|
            \_ ->
                Common.init "/"
                    |> givenClusterInfo "0.0.0-dev" "foobar"
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "top-bar-app" ]
                    |> Query.children []
                    |> Query.first
                    |> Query.has
                        [ style "display" "flex"
                        , style "align-items" "center"
                        , containing
                            [ style "font-size" "21px"
                            , style "color" "#ffffff"
                            , style "letter-spacing" "0.1em"
                            , style "margin-left" "10px"
                            , containing [ text "foobar" ]
                            ]
                        ]
        , test "top bar is 54px tall" <|
            \_ ->
                Common.init "/"
                    |> Common.queryView
                    |> Query.find [ id "top-bar-app" ]
                    |> Query.has [ style "height" "54px" ]
        , describe "loading section" <|
            [ test "has a loading section when awaiting API data" <|
                \_ ->
                    Common.init "/"
                        |> Common.queryView
                        |> Query.has [ class "loading" ]
            , test "centers spinner" <|
                \_ ->
                    Common.init "/"
                        |> Common.queryView
                        |> Query.has
                            [ style "display" "flex"
                            , style "justify-content" "center"
                            , style "align-items" "center"
                            , style "width" "100%"
                            , style "height" "100%"
                            ]
            , test "contains a spinner" <|
                \_ ->
                    Common.init "/"
                        |> Common.queryView
                        |> Query.find [ class "loading" ]
                        |> Query.has
                            [ style "animation" "container-rotate 1568ms linear infinite"
                            , style "height" "36px"
                            , style "width" "36px"
                            ]
            ]
        , test "high density view has no vertical scroll" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has
                        [ style "height" "100%"
                        , style "box-sizing" "border-box"
                        ]
        , test "high density body aligns contents vertically" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has
                        [ style "display" "flex"
                        , style "flex-direction" "column"
                        ]
        , test "high density pipelines view fills vertical space" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.find [ class "dashboard" ]
                    |> Query.has [ style "flex-grow" "1" ]
        , test "high density pipelines view has padding" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.find [ class "dashboard" ]
                    |> Query.has [ style "padding" "60px" ]
        , test "high density pipelines view wraps columns" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.find [ class "dashboard" ]
                    |> Query.has
                        [ style "display" "flex"
                        , style "flex-flow" "column wrap"
                        ]
        , test "normal density pipelines view has default layout" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.find [ class "dashboard" ]
                    |> Query.has
                        [ style "display" "initial"
                        , style "padding" "0"
                        ]
        , test "high density view left-aligns contents" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.find [ class "dashboard" ]
                    |> Query.has [ style "align-content" "flex-start" ]
        , test "high density view has no overlapping top bar" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has [ style "padding-top" "54px" ]
        , test "high density view has no overlapping bottom bar" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has [ style "padding-bottom" "50px" ]
        , test "no bottom padding when footer dismisses" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> afterSeconds 6
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.hasNot [ style "padding-bottom" "50px" ]
        , test "logging out causes pipeline list to reload" <|
            let
                showsLoadingState : Application.Model -> Expectation
                showsLoadingState =
                    Common.queryView
                        >> Query.findAll [ class "dashboard-team-group" ]
                        >> Query.count (Expect.equal 0)
            in
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [ ( "team", [ "owner" ] ) ])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Application.update
                        (ApplicationMsgs.Update <|
                            Msgs.Click Msgs.LogoutButton
                        )
                    |> Tuple.first
                    |> showsLoadingState
        , test "pipeline cards continue to show when teams refresh" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> givenDataUnauthenticated []
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ class "card", containing [ text "pipeline" ] ]
        , test "high-density pipeline cards continue to show when teams refresh" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "a-pipeline" ]
                        )
                    |> Tuple.first
                    |> givenDataUnauthenticated []
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ class "card", containing [ text "a-pipeline" ] ]
        , test "links to specific builds" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <|
                            Ok
                                [ Data.job 1
                                    |> Data.withTeamName "team"
                                    |> Data.withPipelineName "pipeline"
                                    |> Data.withFinishedBuild
                                        (Data.jobBuild BuildStatusSucceeded
                                            |> Data.withTeamName "team"
                                            |> Data.withJob (Just Data.jobId)
                                            |> Just
                                        )
                                ]
                        )
                    |> Tuple.first
                    |> givenDataUnauthenticated []
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 1 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find
                        [ class "dashboard-team-group"
                        , attribute <| Attr.attribute "data-team-name" "team"
                        ]
                    |> Query.find
                        [ attribute <| Attr.attribute "data-tooltip" "job" ]
                    |> Query.find
                        [ tag "a" ]
                    |> Query.has
                        [ attribute <|
                            Attr.href "/pipelines/1/jobs/job/builds/1"
                        ]
        , test "HD view redirects to no pipelines view when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <|
                            Ok <|
                                apiData [ ( "team", [] ) ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok []
                        )
                    |> Expect.all
                        [ Tuple.second
                            >> Common.contains (Effects.ModifyUrl "/")
                        , Tuple.first
                            >> Common.queryView
                            >> Query.has [ text "welcome to concourse!" ]
                        ]
        , test "HD view does not redirect when there are pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <|
                            Ok <|
                                apiData [ ( "team", [] ) ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Expect.all
                        [ Tuple.second
                            >> Expect.notEqual [ Effects.ModifyUrl "/" ]
                        , Tuple.first
                            >> Common.queryView
                            >> Query.hasNot [ text "welcome to concourse!" ]
                        ]
        , test "no search bar when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <|
                            Ok <|
                                apiData [ ( "team", [] ) ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ tag "input" ]
        , test "typing '?' in search bar does not toggle help" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <|
                            Ok <|
                                apiData [ ( "team", [] ) ]
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Application.update (ApplicationMsgs.Update Msgs.FocusMsg)
                    |> Tuple.first
                    |> Application.handleDelivery
                        (KeyDown
                            { ctrlKey = False
                            , shiftKey = True
                            , metaKey = False
                            , code = Keyboard.Slash
                            }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ id "keyboard-help" ]
        , test "bottom bar appears when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <|
                            Ok <|
                                apiData [ ( "team", [] ) ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ id "dashboard-info" ]
        , test "bottom bar has no legend when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <|
                            Ok <|
                                apiData [ ( "team", [] ) ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ id "legend" ]
        , test "concourse info is right-justified when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <|
                            Ok <|
                                apiData [ ( "team", [] ) ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "dashboard-info" ]
                    |> Query.has [ style "justify-content" "flex-end" ]
        , test "pressing '?' does nothing when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.AllTeamsFetched <|
                            Ok <|
                                apiData [ ( "team", [] ) ]
                        )
                    |> Tuple.first
                    |> Application.handleDelivery
                        (KeyDown
                            { ctrlKey = False
                            , shiftKey = True
                            , metaKey = False
                            , code = Keyboard.Slash
                            }
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ id "dashboard-info" ]
        , test "on HD view, team names have increased letter spacing" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ class "dashboard-team-name-wrapper" ]
                    |> Query.has [ style "letter-spacing" ".2em" ]
        , describe "team pills"
            [ test
                ("shows team name with no pill when unauthenticated "
                    ++ "and team has an exposed pipeline"
                )
              <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> teamHeaderHasNoPill "team"
            , test "shows OWNER pill on team header for team on which user has owner role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> teamHeaderHasPill "team" "OWNER"
            , test "shows MEMBER pill on team header for team on which user has member role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "member" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> teamHeaderHasPill "team" "MEMBER"
            , test "shows PIPELINE_OPERATOR pill on team header for team on which user has member role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "pipeline-operator" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> teamHeaderHasPill "team" "PIPELINE_OPERATOR"
            , test "shows VIEWER pill on team header for team on which user has viewer role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "viewer" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> teamHeaderHasPill "team" "VIEWER"
            , test "shows no pill on team header for team on which user has no role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> teamHeaderHasNoPill "team"
            , test
                ("shows pill for most-privileged role on team header for team "
                    ++ "on which user has multiple roles"
                )
              <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "viewer", "member" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> teamHeaderHasPill "team" "MEMBER"
            , test "sorts teams according to user role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData
                                [ ( "owner-team", [] )
                                , ( "nonmember-team", [] )
                                , ( "viewer-team", [] )
                                , ( "member-team", [] )
                                ]
                            )
                            (userWithRoles
                                [ ( "owner-team", [ "owner" ] )
                                , ( "member-team", [ "member" ] )
                                , ( "viewer-team", [ "viewer" ] )
                                , ( "nonmember-team", [] )
                                ]
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "owner-team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.findAll teamHeaderSelector
                        |> Expect.all
                            [ Query.count (Expect.equal 4)
                            , Query.index 0 >> Query.has [ text "owner-team" ]
                            , Query.index 1 >> Query.has [ text "member-team" ]
                            , Query.index 2 >> Query.has [ text "viewer-team" ]
                            , Query.index 3 >> Query.has [ text "nonmember-team" ]
                            ]
            , test "team headers lay out contents horizontally, centering vertically" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.findAll teamHeaderSelector
                        |> Query.each
                            (Query.has
                                [ style "display" "flex"
                                , style "align-items" "center"
                                ]
                            )
            , test "team headers have a bottom margin of 25px" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.findAll teamHeaderSelector
                        |> Query.each
                            (Query.has [ style "margin-bottom" "25px" ])
            , test "on HD view, there is space between the list of pipelines and the role pill" <|
                \_ ->
                    whenOnDashboard { highDensity = True }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "dashboard-team-name-wrapper" ]
                        |> Query.find [ containing [ text "OWNER" ] ]
                        |> Query.has [ style "margin-bottom" "1em" ]
            , test "on non-HD view, the role pill on a group has no margin below" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find teamHeaderSelector
                        |> Query.find [ containing [ text "OWNER" ] ]
                        |> Query.has [ style "margin-bottom" "" ]
            , test "has momentum based scrolling" <|
                \_ ->
                    whenOnDashboard { highDensity = True }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "page-below-top-bar" ]
                        |> Query.find [ class "dashboard" ]
                        |> Query.has [ style "-webkit-overflow-scrolling" "touch" ]
            ]
        , describe "bottom bar"
            [ test "appears by default" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.has [ id "dashboard-info" ]
            , test "is 50px tall, almost black, fixed to the bottom of the viewport and covers entire width" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.has
                            [ style "line-height" "35px"
                            , style "padding" "7.5px 30px"
                            , style "position" "fixed"
                            , style "bottom" "0"
                            , style "background-color" almostBlack
                            , style "width" "100%"
                            , style "box-sizing" "border-box"
                            ]
            , test "lays out contents horizontally, maximizing space between children" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.has
                            [ style "display" "flex"
                            , style "justify-content" "space-between"
                            ]
            , test "has a z-index of 2" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.has [ style "z-index" "2" ]
            , test "two children are legend and concourse-info" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.children []
                        |> Expect.all
                            [ Query.count (Expect.equal 2)
                            , Query.index 0 >> Query.has [ id "legend" ]
                            , Query.index 1 >> Query.has [ id "concourse-info" ]
                            ]
            , test "lays out children on two lines when view width is below 1230px" <|
                \_ ->
                    Common.init "/"
                        |> givenDataUnauthenticated
                            (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Application.update
                            (ApplicationMsgs.DeliveryReceived <|
                                WindowResized 1229 300
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.has [ style "flex-direction" "column" ]
            , describe "legend"
                [ test "lays out contents horizontally" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.has [ style "display" "flex" ]
                , test "shows pipeline statuses" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Expect.all
                                [ Query.count (Expect.equal 9)
                                , Query.index 0
                                    >> Query.children []
                                    >> Expect.all
                                        [ Query.count (Expect.equal 3)
                                        , Query.index 0
                                            >> Query.has
                                                (iconSelector
                                                    { size = "20px"
                                                    , image = PipelineStatusPending True |> Assets.PipelineStatusIcon
                                                    }
                                                )
                                        , Query.index 1
                                            >> Query.has [ style "width" "10px" ]
                                        , Query.index 2 >> Query.has [ text "pending" ]
                                        ]
                                , Query.index 1
                                    >> Query.children []
                                    >> Expect.all
                                        [ Query.count (Expect.equal 3)
                                        , Query.index 0
                                            >> Query.has
                                                (iconSelector
                                                    { size = "20px"
                                                    , image = PipelineStatusPaused |> Assets.PipelineStatusIcon
                                                    }
                                                )
                                        , Query.index 1
                                            >> Query.has [ style "width" "10px" ]
                                        , Query.index 2 >> Query.has [ text "paused" ]
                                        ]
                                ]
                , test "the legend separator is grey" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index -2
                            |> Query.has [ style "color" menuGrey ]
                , test "the legend separator centers contents vertically" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index -2
                            |> Query.has [ style "display" "flex", style "align-items" "center" ]
                , test "the legend separator is gone when the window width is below 812px" <|
                    \_ ->
                        Common.init "/"
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Application.update
                                (ApplicationMsgs.DeliveryReceived <|
                                    WindowResized 800 300
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Expect.all
                                [ Query.hasNot [ text "|" ]
                                , Query.children [] >> Query.count (Expect.equal 8)
                                ]
                , test "legend items wrap when window width is below 812px" <|
                    \_ ->
                        Common.init "/"
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Application.update
                                (ApplicationMsgs.DeliveryReceived <|
                                    WindowResized 800 300
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.has [ style "flex-wrap" "wrap" ]
                , test "legend items lay out contents horizontally, centered vertically in grey caps" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index 0
                            |> Query.has
                                [ style "text-transform" "uppercase"
                                , style "display" "flex"
                                , style "align-items" "center"
                                , style "color" menuGrey
                                ]
                , test "legend items have 20px space between them" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.each
                                (Query.has [ style "margin-right" "20px" ])
                , test "third legend item shows running indicator" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index 2
                            |> Expect.all
                                [ Query.has
                                    [ style "text-transform" "uppercase"
                                    , style "display" "flex"
                                    ]
                                , Query.children []
                                    >> Expect.all
                                        [ Query.count (Expect.equal 3)
                                        , Query.index 0
                                            >> Query.has
                                                (iconSelector
                                                    { size = "20px"
                                                    , image = Assets.RunningLegend
                                                    }
                                                )
                                        , Query.index 1
                                            >> Query.has
                                                [ style "width" "10px" ]
                                        , Query.index 2 >> Query.has [ text "running" ]
                                        ]
                                ]
                ]
            , describe "HD toggle" <|
                let
                    findHDToggle =
                        Query.find [ id "legend" ]
                            >> Query.children []
                            >> Query.index -1

                    hdToggle =
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> findHDToggle
                in
                [ describe "on non-hd view"
                    [ test "lays out contents horizontally" <|
                        \_ ->
                            hdToggle
                                |> Query.has [ style "display" "flex" ]
                    , test "centers contents vertically" <|
                        \_ ->
                            hdToggle
                                |> Query.has [ style "align-items" "center" ]
                    , test "has a margin of 10px between the button and the label" <|
                        \_ ->
                            hdToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style "margin-right" "10px" ]
                    , test "displays the label using a grey color" <|
                        \_ ->
                            hdToggle
                                |> Query.has [ style "color" menuGrey ]
                    , test "label text is all caps" <|
                        \_ ->
                            hdToggle
                                |> Query.has
                                    [ style "text-transform" "uppercase" ]
                    , test "displays label on the right" <|
                        \_ ->
                            hdToggle
                                |> Query.has [ style "flex-direction" "row" ]
                    , test "links to HD view" <|
                        \_ ->
                            hdToggle
                                |> Query.has [ attribute <| Attr.href "/hd" ]
                    , test "displays the off state" <|
                        \_ ->
                            hdToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style "background-image" <|
                                        Assets.backgroundImage <|
                                            Just (Assets.ToggleSwitch False)
                                    , style "background-size" "contain"
                                    , style "height" "20px"
                                    , style "width" "35px"
                                    ]
                    , test "will not shrink on resizing" <|
                        \_ ->
                            hdToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has [ style "flex-shrink" "0" ]
                    ]
                , describe "on HD view"
                    [ test "displays the on state" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (apiData [ ( "team", [] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> findHDToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style "background-image" <|
                                        Assets.backgroundImage <|
                                            Just (Assets.ToggleSwitch True)
                                    , style "background-size" "contain"
                                    , style "height" "20px"
                                    , style "width" "35px"
                                    ]
                    , test "links to normal dashboard view" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (apiData [ ( "team", [] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> findHDToggle
                                |> Query.has [ attribute <| Attr.href "/" ]
                    , test "will not shrink on resizing" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (apiData [ ( "team", [] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> findHDToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style "flex-shrink" "0" ]
                    ]
                ]
            , describe "when there are favorited pipelines" <|
                let
                    setup params =
                        whenOnDashboard params
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Application.handleDelivery
                                (Subscription.FavoritedPipelinesReceived <|
                                    Ok <|
                                        Set.singleton 0
                                )
                in
                [ test "displays favorite pipelines header" <|
                    \_ ->
                        setup { highDensity = False }
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.has [ text "favorite pipelines" ]
                , test "does not display header when on the HD view" <|
                    \_ ->
                        setup { highDensity = True }
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.hasNot [ text "favorite pipelines" ]
                ]
            , describe "when there are no favorited pipelines"
                [ test "does not display header" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.hasNot [ text "favorite pipelines" ]
                ]
            , describe "all pipelines header"
                [ test "displayed when there are pipelines" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.has [ text "all pipelines" ]
                , test "displayed when there are no pipelines" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok []
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.has [ text "all pipelines" ]
                , test "not displayed when there are no teams" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok []
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.hasNot [ text "all pipelines" ]
                ]
            , describe "info section" <|
                let
                    info =
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "concourse-info" ]
                in
                [ test "lays out contents horizontally" <|
                    \_ ->
                        info
                            |> Query.has [ style "display" "flex" ]
                , test "displays info in a grey color" <|
                    \_ ->
                        info
                            |> Query.has [ style "color" menuGrey ]
                , test "displays text slightly larger" <|
                    \_ ->
                        info
                            |> Query.has [ style "font-size" "1.25em" ]
                , test "each info item is spaced out by 30px" <|
                    \_ ->
                        info
                            |> Query.children []
                            |> Query.each
                                (Query.has [ style "margin-right" "30px" ])
                , test "each info item centers contents vertically" <|
                    \_ ->
                        info
                            |> Query.children []
                            |> Query.each
                                (Query.has
                                    [ style "align-items" "center"
                                    , style "display" "flex"
                                    ]
                                )
                , test "items in CLI section are 10 px apart" <|
                    \_ ->
                        info
                            |> Query.children []
                            |> Query.index -1
                            |> Query.children []
                            |> Query.each
                                (Query.has [ style "margin-right" "10px" ])
                , describe "cli download icons" <|
                    let
                        cliIcons =
                            info
                                |> Query.children []
                                |> Query.index -1
                                |> Query.children [ tag "a" ]
                    in
                    [ test "icons are grey" <|
                        \_ ->
                            cliIcons
                                |> Query.each
                                    (Query.has [ style "opacity" "0.5" ])
                    , test "have 'download' attribute" <|
                        \_ ->
                            cliIcons
                                |> Query.each
                                    (Query.has
                                        [ attribute <| Attr.download "" ]
                                    )
                    , test "icons have descriptive ARIA labels" <|
                        \_ ->
                            cliIcons
                                |> Expect.all
                                    [ Query.count (Expect.equal 3)
                                    , Query.index 0
                                        >> Query.has
                                            [ attribute <|
                                                Attr.attribute
                                                    "aria-label"
                                                    "Download OS X CLI"
                                            ]
                                    , Query.index 1
                                        >> Query.has
                                            [ attribute <|
                                                Attr.attribute
                                                    "aria-label"
                                                    "Download Windows CLI"
                                            ]
                                    , Query.index 2
                                        >> Query.has
                                            [ attribute <|
                                                Attr.attribute
                                                    "aria-label"
                                                    "Download Linux CLI"
                                            ]
                                    ]
                    , defineHoverBehaviour
                        { name = "os x cli icon"
                        , setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (apiData [ ( "team", [] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                        , query = Common.queryView >> Query.find [ id "cli-osx" ]
                        , unhoveredSelector =
                            { description = "grey apple icon"
                            , selector =
                                [ style "opacity" "0.5"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = Assets.CliIcon Cli.OSX
                                        , size = "20px"
                                        }
                            }
                        , hoverable =
                            Msgs.FooterCliIcon Cli.OSX
                        , hoveredSelector =
                            { description = "white apple icon"
                            , selector =
                                [ style "opacity" "1"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = Assets.CliIcon Cli.OSX
                                        , size = "20px"
                                        }
                            }
                        }
                    , defineHoverBehaviour
                        { name = "windows cli icon"
                        , setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (apiData [ ( "team", [] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                        , query =
                            Common.queryView
                                >> Query.find [ id "cli-windows" ]
                        , unhoveredSelector =
                            { description = "grey windows icon"
                            , selector =
                                [ style "opacity" "0.5"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = Assets.CliIcon Cli.Windows
                                        , size = "20px"
                                        }
                            }
                        , hoverable =
                            Msgs.FooterCliIcon Cli.Windows
                        , hoveredSelector =
                            { description = "white windows icon"
                            , selector =
                                [ style "opacity" "1"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = Assets.CliIcon Cli.Windows
                                        , size = "20px"
                                        }
                            }
                        }
                    , defineHoverBehaviour
                        { name = "linux cli icon"
                        , setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (apiData
                                        [ ( "team", [] ) ]
                                    )
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                        , query =
                            Common.queryView
                                >> Query.find [ id "cli-linux" ]
                        , unhoveredSelector =
                            { description = "grey linux icon"
                            , selector =
                                [ style "opacity" "0.5"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = Assets.CliIcon Cli.Linux
                                        , size = "20px"
                                        }
                            }
                        , hoverable =
                            Msgs.FooterCliIcon Cli.Linux
                        , hoveredSelector =
                            { description = "white linux icon"
                            , selector =
                                [ style "opacity" "1"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = Assets.CliIcon Cli.Linux
                                        , size = "20px"
                                        }
                            }
                        }
                    ]
                , test "shows concourse version" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenClusterInfo "1.2.3" "cluster-name"
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ id "concourse-info" ]
                            |> Query.has [ text "v1.2.3" ]
                ]
            , test "hides after 6 seconds" <|
                \_ ->
                    Common.init "/"
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> afterSeconds 6
                        |> Common.queryView
                        |> Query.hasNot [ id "dashboard-info" ]
            , test "reappears on mouse action" <|
                \_ ->
                    Common.init "/"
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> afterSeconds 6
                        |> Application.update
                            (ApplicationMsgs.DeliveryReceived <| Moused { x = 0, y = 0 })
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.has [ id "dashboard-info" ]
            , test "is replaced by keyboard help when pressing '?'" <|
                \_ ->
                    Common.init "/"
                        |> givenDataUnauthenticated
                            (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Application.update
                            (ApplicationMsgs.DeliveryReceived <|
                                KeyDown
                                    { ctrlKey = False
                                    , shiftKey = True
                                    , metaKey = False
                                    , code = Keyboard.Slash
                                    }
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Expect.all
                            [ Query.hasNot [ id "dashboard-info" ]
                            , Query.has [ id "keyboard-help" ]
                            ]
            ]
        , test "subscribes to one and five second timers" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.subscriptions
                    |> Expect.all
                        [ Common.contains (Subscription.OnClockTick OneSecond)
                        , Common.contains (Subscription.OnClockTick FiveSeconds)
                        ]
        , test "subscribes to keyups" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.subscriptions
                    |> Common.contains Subscription.OnKeyUp
        , test "auto refreshes jobs on five-second tick after previous request finishes" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <| Ok [])
                    |> Tuple.first
                    |> Application.handleDelivery
                        (ClockTicked FiveSeconds <|
                            Time.millisToPosix 0
                        )
                    |> Tuple.second
                    |> Common.contains Effects.FetchAllJobs
        , test "stops polling jobs if the endpoint is disabled" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <| Data.httpNotImplemented)
                    |> Tuple.first
                    |> Application.handleDelivery
                        (ClockTicked FiveSeconds <|
                            Time.millisToPosix 0
                        )
                    |> Tuple.second
                    |> Common.notContains Effects.FetchAllJobs
        , test "auto refreshes jobs on next five-second tick after dropping" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <| Ok [])
                    |> Tuple.first
                    |> Application.update
                        (ApplicationMsgs.Update <| Msgs.DragStart "team" "pipeline")
                    |> Tuple.first
                    |> Application.handleDelivery
                        (ClockTicked FiveSeconds <|
                            Time.millisToPosix 0
                        )
                    |> Tuple.first
                    |> Application.update
                        (ApplicationMsgs.Update <| Msgs.DragEnd)
                    |> Tuple.first
                    |> Application.handleDelivery
                        (ClockTicked FiveSeconds <|
                            Time.millisToPosix 0
                        )
                    |> Tuple.second
                    |> Common.contains Effects.FetchAllJobs
        , test "don't fetch any jobs until the first request finishes" <|
            \_ ->
                Common.init "/"
                    |> Application.handleDelivery
                        (ClockTicked FiveSeconds <| Time.millisToPosix 0)
                    |> Tuple.second
                    |> Common.notContains Effects.FetchAllJobs
        , test "don't fetch all jobs until the last request finishes" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllJobsFetched <| Ok [])
                    |> Tuple.first
                    |> Application.handleDelivery
                        (ClockTicked FiveSeconds <| Time.millisToPosix 0)
                    |> Tuple.first
                    |> Application.handleDelivery
                        (ClockTicked FiveSeconds <| Time.millisToPosix 0)
                    |> Tuple.second
                    |> Common.notContains Effects.FetchAllJobs
        , test "navigate to non-hd view on logged out when in non-hd view" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.LoggedOut <| Ok ())
                    |> Tuple.second
                    |> Common.contains (Effects.NavigateTo "/")
        , test "navigate to hd view on logged out when in hd view" <|
            \_ ->
                Common.init "/hd"
                    |> Application.handleCallback
                        (Callback.LoggedOut <| Ok ())
                    |> Tuple.second
                    |> Common.contains (Effects.NavigateTo "/hd")
        , test "fetch all teams on logged out" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.LoggedOut <| Ok ())
                    |> Tuple.second
                    |> Common.contains Effects.FetchAllTeams
        , test "fetch all resources on logged out" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.LoggedOut <| Ok ())
                    |> Tuple.second
                    |> Common.contains Effects.FetchAllResources
        , test "fetch all jobs on logged out" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.LoggedOut <| Ok ())
                    |> Tuple.second
                    |> Common.contains Effects.FetchAllJobs
        , test "fetch all pipelines on logged out" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.LoggedOut <| Ok ())
                    |> Tuple.second
                    |> Common.contains Effects.FetchAllPipelines
        ]


afterSeconds : Int -> Application.Model -> Application.Model
afterSeconds n =
    List.repeat n
        (Application.update
            (ApplicationMsgs.DeliveryReceived <|
                ClockTicked OneSecond <|
                    Time.millisToPosix 1000
            )
            >> Tuple.first
        )
        |> List.foldr (>>) identity


csrfToken : String
csrfToken =
    "csrf_token"


iconSelector : { size : String, image : Assets.Asset } -> List Selector
iconSelector { size, image } =
    [ style "background-image" <| Assets.backgroundImage <| Just image
    , style "background-position" "50% 50%"
    , style "background-repeat" "no-repeat"
    , style "width" size
    , style "height" size
    ]


whenOnDashboard : { highDensity : Bool } -> Application.Model
whenOnDashboard { highDensity } =
    Common.init
        (if highDensity then
            "/hd"

         else
            "/"
        )
        |> Application.handleCallback
            (Callback.GotViewport Msgs.Dashboard <|
                Ok <|
                    { scene =
                        { width = 600
                        , height = 600
                        }
                    , viewport =
                        { width = 600
                        , height = 600
                        , x = 0
                        , y = 0
                        }
                    }
            )
        |> Tuple.first


whenOnDashboardViewingAllPipelines : { highDensity : Bool } -> Application.Model
whenOnDashboardViewingAllPipelines { highDensity } =
    whenOnDashboard { highDensity = highDensity }
        |> Application.handleDelivery
            (RouteChanged <|
                Routes.Dashboard
                    { searchType =
                        if highDensity then
                            Routes.HighDensity

                        else
                            Routes.Normal ""
                    , dashboardView = Routes.ViewAllPipelines
                    }
            )
        |> Tuple.first


givenDataAndUser :
    List Concourse.Team
    -> Concourse.User
    -> Application.Model
    -> ( Application.Model, List Effects.Effect )
givenDataAndUser data user =
    Application.handleCallback
        (Callback.AllTeamsFetched <| Ok data)
        >> Tuple.first
        >> Application.handleCallback (Callback.UserFetched <| Ok user)


userWithRoles : List ( String, List String ) -> Concourse.User
userWithRoles roles =
    { id = "0"
    , userName = "test"
    , name = "test"
    , email = "test"
    , isAdmin = False
    , teams =
        Dict.fromList roles
    }


givenDataUnauthenticated :
    List Concourse.Team
    -> Application.Model
    -> ( Application.Model, List Effects.Effect )
givenDataUnauthenticated data =
    Application.handleCallback
        (Callback.AllTeamsFetched <| Ok data)
        >> Tuple.first
        >> Application.handleCallback
            (Callback.UserFetched <| Data.httpUnauthorized)


givenClusterInfo :
    String
    -> String
    -> Application.Model
    -> ( Application.Model, List Effects.Effect )
givenClusterInfo version clusterName =
    Application.handleCallback
        (Callback.ClusterInfoFetched <|
            Ok { version = version, clusterName = clusterName }
        )


apiData : List ( String, List String ) -> List Concourse.Team
apiData pipelines =
    pipelines |> List.map Tuple.first |> List.indexedMap Concourse.Team


running : Concourse.Job -> Concourse.Job
running =
    Data.withNextBuild (Data.jobBuild BuildStatusStarted |> Just)


otherJob : BuildStatus -> Concourse.Job
otherJob =
    jobWithNameTransitionedAt "other-job" <| Just <| Time.millisToPosix 0


job : BuildStatus -> Concourse.Job
job =
    jobWithNameTransitionedAt "job" <| Just <| Time.millisToPosix 0


jobWithNameTransitionedAt : String -> Maybe Time.Posix -> BuildStatus -> Concourse.Job
jobWithNameTransitionedAt jobName transitionedAt status =
    { name = jobName
    , pipelineId = 1
    , pipelineName = "pipeline"
    , teamName = "team"
    , nextBuild = Nothing
    , finishedBuild =
        Just
            { id = 0
            , teamName = "team"
            , name = "0"
            , job = Just Data.jobId
            , status = status
            , duration =
                { startedAt = Nothing
                , finishedAt = Nothing
                }
            , reapTime = Nothing
            }
    , transitionBuild =
        transitionedAt
            |> Maybe.map
                (\t ->
                    { id = 1
                    , teamName = "team"
                    , name = "1"
                    , job = Just Data.jobId
                    , status = status
                    , duration =
                        { startedAt = Nothing
                        , finishedAt = Just <| t
                        }
                    , reapTime = Nothing
                    }
                )
    , paused = False
    , disableManualTrigger = False
    , inputs = []
    , outputs = []
    , groups = []
    }


circularJobs : List Concourse.Job
circularJobs =
    [ { name = "jobA"
      , pipelineId = 1
      , pipelineName = "pipeline"
      , teamName = "team"
      , nextBuild = Nothing
      , finishedBuild =
            Just
                { id = 0
                , name = "0"
                , teamName = "team"
                , job = Just (Data.jobId |> Data.withJobName "jobA")
                , status = BuildStatusSucceeded
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Nothing
                    }
                , reapTime = Nothing
                }
      , transitionBuild =
            Just
                { id = 1
                , name = "1"
                , teamName = "team"
                , job = Just (Data.jobId |> Data.withJobName "jobA")
                , status = BuildStatusSucceeded
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Just <| Time.millisToPosix 0
                    }
                , reapTime = Nothing
                }
      , paused = False
      , disableManualTrigger = False
      , inputs =
            [ { name = "inA"
              , resource = "res0"
              , passed = [ "jobB" ]
              , trigger = True
              }
            ]
      , outputs = []
      , groups = []
      }
    , { name = "jobB"
      , pipelineId = 1
      , pipelineName = "pipeline"
      , teamName = "team"
      , nextBuild = Nothing
      , finishedBuild =
            Just
                { id = 0
                , name = "0"
                , teamName = "team"
                , job = Just (Data.jobId |> Data.withJobName "jobB")
                , status = BuildStatusSucceeded
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Nothing
                    }
                , reapTime = Nothing
                }
      , transitionBuild =
            Just
                { id = 1
                , name = "1"
                , teamName = "team"
                , job = Just (Data.jobId |> Data.withJobName "jobB")
                , status = BuildStatusSucceeded
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Just <| Time.millisToPosix 0
                    }
                , reapTime = Nothing
                }
      , paused = False
      , disableManualTrigger = False
      , inputs =
            [ { name = "inB"
              , resource = "res0"
              , passed = [ "jobA" ]
              , trigger = True
              }
            ]
      , outputs = []
      , groups = []
      }
    ]


teamHeaderSelector : List Selector
teamHeaderSelector =
    [ class <| .sectionHeaderClass Effects.stickyHeaderConfig ]


teamHeaderHasNoPill :
    String
    -> Query.Single ApplicationMsgs.TopLevelMessage
    -> Expectation
teamHeaderHasNoPill teamName =
    Query.find (teamHeaderSelector ++ [ containing [ text teamName ] ])
        >> Query.children []
        >> Query.count (Expect.equal 1)


teamHeaderHasPill :
    String
    -> String
    -> Query.Single ApplicationMsgs.TopLevelMessage
    -> Expectation
teamHeaderHasPill teamName pillText =
    Query.find (teamHeaderSelector ++ [ containing [ text teamName ] ])
        >> Query.children []
        >> Expect.all
            [ Query.count (Expect.equal 2)
            , Query.index 1 >> Query.has [ text pillText ]
            ]
