module DashboardTests exposing
    ( all
    , almostBlack
    , apiData
    , darkGrey
    , defineHoverBehaviour
    , givenDataAndUser
    , givenDataUnauthenticated
    , iconSelector
    , isColorWithStripes
    , middleGrey
    , white
    )

import Application.Application as Application
import Common
import Concourse
import Concourse.Cli as Cli
import Dashboard.Dashboard as Dashboard
import Dict
import Expect exposing (Expectation)
import Html.Attributes as Attr
import Http
import Keyboard
import List.Extra
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Msgs
import Message.Subscription as Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as ApplicationMsgs
import Routes
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
import UserState


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
    , instanceName = ""
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
                    , instanceName = ""
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
                    |> List.member Effects.GetScreenSize
                    |> Expect.true "should request screen size"
        , test "title says 'Dashboard - Concourse'" <|
            \_ ->
                Common.init "/"
                    |> Application.view
                    |> .title
                    |> Expect.equal "Dashboard - Concourse"
        , test "renders instance name at top left" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = ""
                    , csrfToken = csrfToken
                    , authToken = ""
                    , instanceName = "foobar"
                    , pipelineRunningKeyframes = ""
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path = "/"
                    , query = Nothing
                    , fragment = Nothing
                    }
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
        , describe "welcome card" <|
            let
                hasWelcomeCard : (() -> Application.Model) -> List Test
                hasWelcomeCard setup =
                    let
                        subject : () -> Query.Single ApplicationMsgs.TopLevelMessage
                        subject =
                            setup
                                >> Common.queryView
                                >> Query.find [ id "welcome-card" ]
                    in
                    [ test "exists" <|
                        setup
                            >> Common.queryView
                            >> Query.has [ id "welcome-card" ]
                    , test "with correct antialiasing in WebKit" <|
                        subject
                            >> Query.has
                                [ style "-webkit-font-smoothing" "antialiased" ]
                    , test "with non-bold text" <|
                        subject
                            >> Query.has [ style "font-weight" "400" ]
                    , test "title says 'welcome to concourse!'" <|
                        subject
                            >> Query.children []
                            >> Query.first
                            >> Query.has [ text "welcome to concourse!" ]
                    , test "welcome message has large font" <|
                        subject
                            >> Query.children []
                            >> Query.first
                            >> Query.has [ style "font-size" "32px" ]
                    , test "has dark grey background" <|
                        subject
                            >> Query.has
                                [ style "background-color" darkGrey ]
                    , test "is inset from the page" <|
                        subject
                            >> Query.has [ style "margin" "25px" ]
                    , test "has padding around its contents" <|
                        subject
                            >> Query.has [ style "padding" "40px" ]
                    , describe "body" <|
                        let
                            body : () -> Query.Single ApplicationMsgs.TopLevelMessage
                            body =
                                subject >> Query.children [] >> Query.index 1
                        in
                        [ test "has set-pipeline instruction" <|
                            let
                                instruction =
                                    "then, use `fly set-pipeline` to set "
                                        ++ "up your new pipeline"
                            in
                            body
                                >> Query.has [ text instruction ]
                        , test "has 16px font" <|
                            body
                                >> Query.has
                                    [ style "font-size" "16px" ]
                        , describe "CLI download section" <|
                            let
                                downloadSection =
                                    body >> Query.children [] >> Query.index 0
                            in
                            [ test
                                ("lays out contents horizontally, "
                                    ++ "centers vertically"
                                )
                              <|
                                downloadSection
                                    >> Query.has
                                        [ style "display" "flex"
                                        , style "align-items" "center"
                                        ]
                            , test "says 'first, download the CLI tools:'" <|
                                let
                                    instruction =
                                        "first, download the CLI tools:"
                                in
                                downloadSection
                                    >> Query.children []
                                    >> Query.index 0
                                    >> Query.has [ text instruction ]
                            , test
                                ("there is space between the label and "
                                    ++ "the icons"
                                )
                              <|
                                downloadSection
                                    >> Query.children []
                                    >> Query.index 0
                                    >> Query.has
                                        [ style "margin-right" "10px" ]
                            , describe "cli download icons" <|
                                let
                                    cliIcons =
                                        downloadSection
                                            >> Query.children [ tag "a" ]
                                in
                                [ test "have 'download' attribute" <|
                                    cliIcons
                                        >> Query.each
                                            (Query.has
                                                [ attribute <| Attr.download "" ]
                                            )
                                , test "icons have descriptive ARIA labels" <|
                                    cliIcons
                                        >> Expect.all
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
                                    , setup = setup ()
                                    , query =
                                        Common.queryView
                                            >> Query.find [ id "top-cli-osx" ]
                                    , updateFunc =
                                        \msg ->
                                            Application.update msg
                                                >> Tuple.first
                                    , unhoveredSelector =
                                        { description = "grey apple icon"
                                        , selector =
                                            [ style "opacity" "0.5"
                                            , style "margin" "5px"
                                            ]
                                                ++ iconSelector
                                                    { size = "32px"
                                                    , image = "apple-logo.svg"
                                                    }
                                        }
                                    , mouseEnterMsg =
                                        ApplicationMsgs.Update <|
                                            Msgs.Hover <|
                                                Maybe.map Msgs.WelcomeCardCliIcon <|
                                                    List.Extra.getAt 0 Cli.clis
                                    , mouseLeaveMsg =
                                        ApplicationMsgs.Update <|
                                            Msgs.Hover Nothing
                                    , hoveredSelector =
                                        { description = "white apple icon"
                                        , selector =
                                            [ style "opacity" "1"
                                            , style "margin" "5px"
                                            ]
                                                ++ iconSelector
                                                    { size = "32px"
                                                    , image = "apple-logo.svg"
                                                    }
                                        }
                                    }
                                , defineHoverBehaviour
                                    { name = "windows cli icon"
                                    , setup = setup ()
                                    , query =
                                        Common.queryView
                                            >> Query.find
                                                [ id "top-cli-windows" ]
                                    , updateFunc =
                                        \msg ->
                                            Application.update msg
                                                >> Tuple.first
                                    , unhoveredSelector =
                                        { description = "grey windows icon"
                                        , selector =
                                            [ style "opacity" "0.5"
                                            , style "margin" "5px"
                                            ]
                                                ++ iconSelector
                                                    { size = "32px"
                                                    , image = "windows-logo.svg"
                                                    }
                                        }
                                    , mouseEnterMsg =
                                        ApplicationMsgs.Update <|
                                            Msgs.Hover <|
                                                Maybe.map Msgs.WelcomeCardCliIcon <|
                                                    List.Extra.getAt 1 Cli.clis
                                    , mouseLeaveMsg =
                                        ApplicationMsgs.Update <|
                                            Msgs.Hover Nothing
                                    , hoveredSelector =
                                        { description = "white windows icon"
                                        , selector =
                                            [ style "opacity" "1"
                                            , style "margin" "5px"
                                            ]
                                                ++ iconSelector
                                                    { size = "32px"
                                                    , image = "windows-logo.svg"
                                                    }
                                        }
                                    }
                                , defineHoverBehaviour
                                    { name = "linux cli icon"
                                    , setup = setup ()
                                    , query =
                                        Common.queryView
                                            >> Query.find
                                                [ id "top-cli-linux" ]
                                    , updateFunc =
                                        \msg ->
                                            Application.update msg
                                                >> Tuple.first
                                    , unhoveredSelector =
                                        { description = "grey linux icon"
                                        , selector =
                                            [ style "opacity" "0.5"
                                            , style "margin" "5px"
                                            ]
                                                ++ iconSelector
                                                    { size = "32px"
                                                    , image = "linux-logo.svg"
                                                    }
                                        }
                                    , mouseEnterMsg =
                                        ApplicationMsgs.Update <|
                                            Msgs.Hover <|
                                                Maybe.map Msgs.WelcomeCardCliIcon <|
                                                    List.Extra.getAt 2 Cli.clis
                                    , mouseLeaveMsg =
                                        ApplicationMsgs.Update <|
                                            Msgs.Hover Nothing
                                    , hoveredSelector =
                                        { description = "white linux icon"
                                        , selector =
                                            [ style "opacity" "1"
                                            , style "margin" "5px"
                                            ]
                                                ++ iconSelector
                                                    { size = "32px"
                                                    , image = "linux-logo.svg"
                                                    }
                                        }
                                    }
                                ]
                            ]
                        ]
                    , describe "ascii art" <|
                        let
                            art : () -> Query.Single ApplicationMsgs.TopLevelMessage
                            art =
                                subject >> Query.children [] >> Query.index 2
                        in
                        [ test "not selectable for all browsers" <|
                            art
                                >> Query.has
                                    [ style "user-select" "none"
                                    , style "-ms-user-select" "none"
                                    , style "-moz-user-select" "none"
                                    , style "-khtml-user-select" "none"
                                    , style "-webkit-user-select" "none"
                                    , style "-webkit-touch-callout" "none"
                                    ]
                        , test "cursor is set to default" <|
                            art
                                >> Query.has [ style "cursor" "default" ]
                        ]
                    ]
            in
            [ describe "when unauthenticated with no teams" <|
                hasWelcomeCard
                    (\_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [])
                    )
                    ++ [ test "page body is empty" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> givenDataUnauthenticated (apiData [])
                                    |> Common.queryView
                                    |> Query.find [ id "page-below-top-bar" ]
                                    |> Query.children []
                                    |> Query.first
                                    |> Query.children []
                                    |> Query.count (Expect.equal 1)
                       ]
            , describe "when unauthenticated with a team but no pipelines" <|
                hasWelcomeCard
                    (\_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                    )
            , describe
                ("when logged in with teams but no pipelines, "
                    ++ "shows no pipelines card"
                )
              <|
                hasWelcomeCard
                    (\_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataAndUser
                                (apiData [ ( "team", [] ) ])
                                (userWithRoles [])
                    )
            , test "no login instruction when logged in" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [])
                        |> Common.queryView
                        |> Query.hasNot [ id "login-instruction" ]
            , test "has login instruction when unauthenticated" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Common.queryView
                        |> Query.find [ id "welcome-card" ]
                        |> Query.find [ id "login-instruction" ]
                        |> Expect.all
                            [ Query.has [ text "login" ]
                            , Query.find [ tag "a", containing [ text "here" ] ]
                                >> Expect.all
                                    [ Query.has
                                        [ attribute <| Attr.href "/login" ]
                                    , Query.has
                                        [ style "text-decoration" "underline" ]
                                    ]
                            , Query.has [ style "line-height" "42px" ]
                            ]
            ]
        , test "high density view has no vertical scroll" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [ "pipeline" ] ) ])
                        (userWithRoles [])
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
                        (apiData [ ( "team", [ "pipeline" ] ) ])
                        (userWithRoles [])
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
                        (apiData [ ( "team", [ "pipeline" ] ) ])
                        (userWithRoles [])
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.children []
                    |> Query.first
                    |> Query.has [ style "flex-grow" "1" ]
        , test "high density pipelines view has padding" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [ "pipeline" ] ) ])
                        (userWithRoles [])
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.children []
                    |> Query.first
                    |> Query.has [ style "padding" "60px" ]
        , test "high density pipelines view wraps columns" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> givenDataAndUser
                        (apiData [ ( "team", [ "pipeline" ] ) ])
                        (userWithRoles [])
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.children []
                    |> Query.first
                    |> Query.has
                        [ style "display" "flex"
                        , style "flex-flow" "column wrap"
                        ]
        , test "normal density pipelines view has default layout" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataAndUser
                        (apiData [ ( "team", [ "pipeline" ] ) ])
                        (userWithRoles [])
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.children []
                    |> Query.first
                    |> Query.has
                        [ style "display" "initial"
                        , style "padding" "0"
                        ]
        , test "high density view left-aligns contents" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataAndUser
                        (apiData [ ( "team", [ "pipeline" ] ) ])
                        (userWithRoles [])
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.children []
                    |> Query.first
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
                        (apiData [ ( "team", [ "pipeline" ] ) ])
                        (userWithRoles [])
                    |> Common.queryView
                    |> Query.find [ id "page-below-top-bar" ]
                    |> Query.has [ style "padding-bottom" "50px" ]
        , test "top bar has bold font" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Common.queryView
                    |> Query.find [ id "top-bar-app" ]
                    |> Query.has [ style "font-weight" "700" ]
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
                        (oneTeamOnePipelineNonPublic "team")
                        (userWithRoles [ ( "team", [ "owner" ] ) ])
                    |> Application.update (ApplicationMsgs.Update Msgs.LogOut)
                    |> Tuple.first
                    |> showsLoadingState
        , test "links to specific builds" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataUnauthenticated givenPipelineWithJob
                    |> Common.queryView
                    |> Query.find
                        [ class "dashboard-team-group"
                        , attribute <| Attr.attribute "data-team-name" "team"
                        ]
                    |> Query.find
                        [ class "node"
                        , attribute <| Attr.attribute "data-tooltip" "job"
                        ]
                    |> Query.find
                        [ tag "a" ]
                    |> Query.has
                        [ attribute <|
                            Attr.href "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                        ]
        , test "HD view redirects to normal view when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> Application.handleCallback
                        (Callback.APIDataFetched <|
                            Ok
                                ( Time.millisToPosix 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Expect.all
                        [ Tuple.second
                            >> Expect.equal [ Effects.ModifyUrl "/" ]
                        , Tuple.first
                            >> Application.handleCallback
                                (Callback.APIDataFetched <|
                                    Ok
                                        ( Time.millisToPosix 0
                                        , apiData
                                            [ ( "team", [ "pipeline" ] ) ]
                                            Nothing
                                        )
                                )
                            >> Tuple.first
                            >> Common.queryView
                            >> Query.find [ class "card-footer" ]
                            >> Query.children []
                            >> Query.first
                            >> Query.children []
                            >> Query.index -1
                            >> Query.has [ text "pending" ]
                        ]
        , test "HD view redirects to no pipelines view when pipelines disappear" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> Application.handleCallback
                        (Callback.APIDataFetched <|
                            Ok
                                ( Time.millisToPosix 0
                                , apiData [ ( "team", [ "pipeline" ] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.APIDataFetched <|
                            Ok
                                ( Time.millisToPosix 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Expect.all
                        [ Tuple.second
                            >> Expect.equal [ Effects.ModifyUrl "/" ]
                        , Tuple.first
                            >> Common.queryView
                            >> Query.has [ text "welcome to concourse!" ]
                        ]
        , test "no search bar when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.APIDataFetched <|
                            Ok
                                ( Time.millisToPosix 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ tag "input" ]
        , test "typing '?' in search bar does not toggle help" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.APIDataFetched <|
                            Ok
                                ( Time.millisToPosix 0
                                , apiData
                                    [ ( "team", [ "pipeline" ] ) ]
                                    Nothing
                                )
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
                        (Callback.APIDataFetched <|
                            Ok
                                ( Time.millisToPosix 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.has [ id "dashboard-info" ]
        , test "bottom bar has no legend when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.APIDataFetched <|
                            Ok
                                ( Time.millisToPosix 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ id "legend" ]
        , test "concourse info is right-justified when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.APIDataFetched <|
                            Ok
                                ( Time.millisToPosix 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find [ id "dashboard-info" ]
                    |> Query.has [ style "justify-content" "flex-end" ]
        , test "pressing '?' does nothing when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.handleCallback
                        (Callback.APIDataFetched <|
                            Ok
                                ( Time.millisToPosix 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
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
                        (apiData [ ( "team", [ "pipeline" ] ) ])
                        (userWithRoles [])
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
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> Common.queryView
                        |> teamHeaderHasNoPill "team"
            , test "shows OWNER pill on team header for team on which user has owner role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> Common.queryView
                        |> teamHeaderHasPill "team" "OWNER"
            , test "shows MEMBER pill on team header for team on which user has member role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "member" ] ) ])
                        |> Common.queryView
                        |> teamHeaderHasPill "team" "MEMBER"
            , test "shows VIEWER pill on team header for team on which user has viewer role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "viewer" ] ) ])
                        |> Common.queryView
                        |> teamHeaderHasPill "team" "VIEWER"
            , test "shows no pill on team header for team on which user has no role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [])
                        |> Common.queryView
                        |> teamHeaderHasNoPill "team"
            , test "shows pill for first role on team header for team on which user has multiple roles" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "member", "viewer" ] ) ])
                        |> Common.queryView
                        |> teamHeaderHasPill "team" "MEMBER"
            , test "sorts teams according to user role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData
                                [ ( "owner-team", [ "pipeline" ] )
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
                        |> givenDataUnauthenticated (oneTeamOnePipeline "team")
                        |> Common.queryView
                        |> Query.findAll teamHeaderSelector
                        |> Query.each
                            (Query.has
                                [ style "display" "flex"
                                , style "align-items" "center"
                                ]
                            )
            , test "on HD view, there is space between the list of pipelines and the role pill" <|
                \_ ->
                    whenOnDashboard { highDensity = True }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> Common.queryView
                        |> Query.find [ class "dashboard-team-name-wrapper" ]
                        |> Query.find [ containing [ text "OWNER" ] ]
                        |> Query.has [ style "margin-bottom" "1em" ]
            , test "on non-HD view, the role pill on a group has no margin below" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> Common.queryView
                        |> Query.find teamHeaderSelector
                        |> Query.find [ containing [ text "OWNER" ] ]
                        |> Query.has [ style "margin-bottom" "" ]
            ]
        , describe "pipeline cards" <|
            let
                findHeader :
                    Query.Single ApplicationMsgs.TopLevelMessage
                    -> Query.Single ApplicationMsgs.TopLevelMessage
                findHeader =
                    Query.find [ class "card-header" ]

                findBody :
                    Query.Single ApplicationMsgs.TopLevelMessage
                    -> Query.Single ApplicationMsgs.TopLevelMessage
                findBody =
                    Query.find [ class "card-body" ]

                pipelineWithStatus :
                    Concourse.BuildStatus
                    -> Bool
                    -> Application.Model
                    -> Query.Single ApplicationMsgs.TopLevelMessage
                pipelineWithStatus status isRunning =
                    let
                        jobFunc =
                            if isRunning then
                                job >> running

                            else
                                job
                    in
                    givenDataUnauthenticated
                        (\u ->
                            { teams =
                                [ { id = 0, name = "team" } ]
                            , pipelines =
                                [ onePipeline "team" ]
                            , jobs =
                                [ jobFunc status
                                ]
                            , resources = []
                            , version = ""
                            , user = u
                            }
                        )
                        >> Common.queryView
            in
            [ describe "when team has no visible pipelines" <|
                let
                    noPipelinesCard : () -> Query.Single ApplicationMsgs.TopLevelMessage
                    noPipelinesCard _ =
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData
                                    [ ( "some-team", [] )
                                    , ( "other-team", [ "pipeline" ] )
                                    ]
                                )
                            |> Common.queryView
                            |> Query.find
                                [ class "dashboard-team-group"
                                , attribute <|
                                    Attr.attribute "data-team-name"
                                        "some-team"
                                ]
                            |> Query.find [ class "card" ]
                in
                [ describe "header" <|
                    let
                        header : () -> Query.Single ApplicationMsgs.TopLevelMessage
                        header =
                            noPipelinesCard
                                >> findHeader
                    in
                    [ test "says 'no pipeline set' in smooth white font" <|
                        header
                            >> Expect.all
                                [ Query.has [ text "no pipeline set" ]
                                , Query.has
                                    [ style "color" white
                                    , style "-webkit-font-smoothing" "antialiased"
                                    ]
                                ]
                    , test "has dark grey background and 12.5px padding" <|
                        header
                            >> Query.has
                                [ style "background-color" darkGrey
                                , style "padding" "12.5px"
                                ]
                    , test "text is larger and wider spaced" <|
                        header
                            >> Query.has
                                [ style "font-size" "1.5em"
                                , style "letter-spacing" "0.1em"
                                ]
                    , test "text is centered" <|
                        header
                            >> Query.has [ style "text-align" "center" ]
                    ]
                , describe "body" <|
                    let
                        body : () -> Query.Single ApplicationMsgs.TopLevelMessage
                        body =
                            noPipelinesCard
                                >> Query.find [ class "card-body" ]
                    in
                    [ test "has 200px width, 120px height, 20px 36px padding" <|
                        body
                            >> Query.has
                                [ style "width" "200px"
                                , style "height" "120px"
                                , style "padding" "20px 36px"
                                ]
                    , test "has dark grey background" <|
                        body
                            >> Query.has [ style "background-color" darkGrey ]
                    , test "has 2px margins above and below" <|
                        body
                            >> Query.has [ style "margin" "2px 0" ]
                    , test "has lighter grey placeholder box that fills" <|
                        body
                            >> Expect.all
                                [ Query.has [ style "display" "flex" ]
                                , Query.children []
                                    >> Query.first
                                    >> Query.has
                                        [ style "background-color" middleGrey
                                        , style "flex-grow" "1"
                                        ]
                                ]
                    ]
                , test "footer is dark grey and 47 pixels tall" <|
                    noPipelinesCard
                        >> Query.find [ class "card-footer" ]
                        >> Query.has
                            [ style "background-color" darkGrey
                            , style "height" "47px"
                            ]
                ]
            , test "has 'move' cursor" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated
                            (oneTeamOnePipeline "team")
                        |> Common.queryView
                        |> Query.find
                            [ class "card"
                            , containing [ text "pipeline" ]
                            ]
                        |> Query.has [ style "cursor" "move" ]
            , test "has 25px margins" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated
                            (oneTeamOnePipeline "team")
                        |> Common.queryView
                        |> Query.find
                            [ class "card"
                            , containing [ text "pipeline" ]
                            ]
                        |> Query.has [ style "margin" "25px" ]
            , describe "header" <|
                let
                    header : () -> Query.Single ApplicationMsgs.TopLevelMessage
                    header _ =
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (oneTeamOnePipeline "team")
                            |> Common.queryView
                            |> Query.find
                                [ class "card"
                                , containing [ text "pipeline" ]
                                ]
                            |> findHeader
                in
                [ test "has dark grey background" <|
                    header
                        >> Query.has [ style "background-color" darkGrey ]
                , test "has larger, spaced-out smooth white text" <|
                    header
                        >> Query.has
                            [ style "font-size" "1.5em"
                            , style "letter-spacing" "0.1em"
                            , style "color" white
                            , style "-webkit-font-smoothing" "antialiased"
                            ]
                , test "has 12.5px padding" <|
                    header
                        >> Query.has [ style "padding" "12.5px" ]
                , test "text does not overflow or wrap" <|
                    header
                        >> Query.children []
                        >> Query.first
                        >> Query.has
                            [ style "width" "245px"
                            , style "white-space" "nowrap"
                            , style "overflow" "hidden"
                            , style "text-overflow" "ellipsis"
                            ]
                ]
            , describe "colored banner" <|
                let
                    findBanner =
                        Query.find
                            [ class "card"
                            , containing [ text "pipeline" ]
                            ]
                            >> Query.children []
                            >> Query.first

                    isSolid : String -> Query.Single ApplicationMsgs.TopLevelMessage -> Expectation
                    isSolid color =
                        Query.has [ style "background-color" color ]
                in
                [ describe "non-HD view"
                    [ test "is 7px tall" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> Common.queryView
                                |> findBanner
                                |> Query.has [ style "height" "7px" ]
                    , test "is blue when pipeline is paused" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (\u ->
                                        { teams =
                                            [ { id = 0, name = "team" } ]
                                        , pipelines =
                                            [ onePipelinePaused "team" ]
                                        , jobs = []
                                        , resources = []
                                        , version = ""
                                        , user = u
                                        }
                                    )
                                |> Common.queryView
                                |> findBanner
                                |> isSolid blue
                    , test "is green when pipeline is succeeding" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusSucceeded
                                    False
                                |> findBanner
                                |> isSolid green
                    , test "is green with black stripes when pipeline is succeeding and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusSucceeded
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = green, thick = darkGrey }
                    , test "is grey when pipeline is pending" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> Common.queryView
                                |> findBanner
                                |> isSolid lightGrey
                    , test "is grey with black stripes when pipeline is pending and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusStarted
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = lightGrey, thick = darkGrey }
                    , test "is red when pipeline is failing" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusFailed
                                    False
                                |> findBanner
                                |> isSolid red
                    , test "is red with black stripes when pipeline is failing and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusFailed
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = red, thick = darkGrey }
                    , test "is amber when pipeline is erroring" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusErrored
                                    False
                                |> findBanner
                                |> isSolid amber
                    , test "is amber with black stripes when pipeline is erroring and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusErrored
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = amber, thick = darkGrey }
                    , test "is brown when pipeline is aborted" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusAborted
                                    False
                                |> findBanner
                                |> isSolid brown
                    , test "is brown with black stripes when pipeline is aborted and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusAborted
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = brown, thick = darkGrey }
                    , describe "status priorities" <|
                        let
                            givenTwoJobs :
                                Concourse.BuildStatus
                                -> Concourse.BuildStatus
                                -> Query.Single ApplicationMsgs.TopLevelMessage
                            givenTwoJobs firstStatus secondStatus =
                                whenOnDashboard { highDensity = False }
                                    |> givenDataUnauthenticated
                                        (\u ->
                                            { teams =
                                                [ { id = 0, name = "team" } ]
                                            , pipelines =
                                                [ onePipeline "team" ]
                                            , jobs =
                                                [ job firstStatus
                                                , otherJob secondStatus
                                                ]
                                            , resources = []
                                            , version = ""
                                            , user = u
                                            }
                                        )
                                    |> Common.queryView
                        in
                        [ test "failed is more important than errored" <|
                            \_ ->
                                givenTwoJobs
                                    Concourse.BuildStatusFailed
                                    Concourse.BuildStatusErrored
                                    |> findBanner
                                    |> isSolid red
                        , test "errored is more important than aborted" <|
                            \_ ->
                                givenTwoJobs
                                    Concourse.BuildStatusErrored
                                    Concourse.BuildStatusAborted
                                    |> findBanner
                                    |> isSolid amber
                        , test "aborted is more important than succeeding" <|
                            \_ ->
                                givenTwoJobs
                                    Concourse.BuildStatusAborted
                                    Concourse.BuildStatusSucceeded
                                    |> findBanner
                                    |> isSolid brown
                        , test "succeeding is more important than pending" <|
                            \_ ->
                                givenTwoJobs
                                    Concourse.BuildStatusSucceeded
                                    Concourse.BuildStatusPending
                                    |> findBanner
                                    |> isSolid green
                        ]
                    , test "does not crash with a circular pipeline" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (\u ->
                                        { teams = [ { id = 0, name = "team" } ]
                                        , pipelines = [ onePipeline "team" ]
                                        , jobs = circularJobs
                                        , resources = []
                                        , version = ""
                                        , user = u
                                        }
                                    )
                                |> Common.queryView
                                |> findBanner
                                |> isSolid green
                    , describe "HD view"
                        [ test "is 8px wide" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> givenDataUnauthenticated
                                        (oneTeamOnePipeline "team")
                                    |> Common.queryView
                                    |> findBanner
                                    |> Query.has [ style "width" "8px" ]
                        , test "is blue when pipeline is paused" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> givenDataUnauthenticated
                                        (\u ->
                                            { teams =
                                                [ { id = 0, name = "team" } ]
                                            , pipelines =
                                                [ onePipelinePaused "team" ]
                                            , jobs = []
                                            , resources = []
                                            , version = ""
                                            , user = u
                                            }
                                        )
                                    |> Common.queryView
                                    |> findBanner
                                    |> isSolid blue
                        , test "is green when pipeline is succeeding" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusSucceeded
                                        False
                                    |> findBanner
                                    |> isSolid green
                        , test "is green with black stripes when pipeline is succeeding and running" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusSucceeded
                                        True
                                    |> findBanner
                                    |> isColorWithStripes { thin = green, thick = darkGrey }
                        , test "is grey when pipeline is pending" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> givenDataUnauthenticated
                                        (oneTeamOnePipeline "team")
                                    |> Common.queryView
                                    |> findBanner
                                    |> isSolid lightGrey
                        , test "is grey with black stripes when pipeline is pending and running" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusStarted
                                        True
                                    |> findBanner
                                    |> isColorWithStripes { thin = lightGrey, thick = darkGrey }
                        , test "is red when pipeline is failing" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusFailed
                                        False
                                    |> findBanner
                                    |> isSolid red
                        , test "is red with black stripes when pipeline is failing and running" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusFailed
                                        True
                                    |> findBanner
                                    |> isColorWithStripes { thin = red, thick = darkGrey }
                        , test "is amber when pipeline is erroring" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusErrored
                                        False
                                    |> findBanner
                                    |> isSolid amber
                        , test "is amber with black stripes when pipeline is erroring and running" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusErrored
                                        True
                                    |> findBanner
                                    |> isColorWithStripes { thin = amber, thick = darkGrey }
                        , test "is brown when pipeline is aborted" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusAborted
                                        False
                                    |> findBanner
                                    |> isSolid brown
                        , test "is brown with black stripes when pipeline is aborted and running" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusAborted
                                        True
                                    |> findBanner
                                    |> isColorWithStripes { thin = brown, thick = darkGrey }
                        , describe "status priorities" <|
                            let
                                givenTwoJobs :
                                    Concourse.BuildStatus
                                    -> Concourse.BuildStatus
                                    -> Query.Single ApplicationMsgs.TopLevelMessage
                                givenTwoJobs firstStatus secondStatus =
                                    whenOnDashboard { highDensity = False }
                                        |> givenDataUnauthenticated
                                            (\u ->
                                                { teams =
                                                    [ { id = 0, name = "team" } ]
                                                , pipelines =
                                                    [ onePipeline "team" ]
                                                , jobs =
                                                    [ job firstStatus
                                                    , otherJob secondStatus
                                                    ]
                                                , resources = []
                                                , version = ""
                                                , user = u
                                                }
                                            )
                                        |> Common.queryView
                            in
                            [ test "failed is more important than errored" <|
                                \_ ->
                                    givenTwoJobs
                                        Concourse.BuildStatusFailed
                                        Concourse.BuildStatusErrored
                                        |> findBanner
                                        |> isSolid red
                            , test "errored is more important than aborted" <|
                                \_ ->
                                    givenTwoJobs
                                        Concourse.BuildStatusErrored
                                        Concourse.BuildStatusAborted
                                        |> findBanner
                                        |> isSolid amber
                            , test "aborted is more important than succeeding" <|
                                \_ ->
                                    givenTwoJobs
                                        Concourse.BuildStatusAborted
                                        Concourse.BuildStatusSucceeded
                                        |> findBanner
                                        |> isSolid brown
                            , test "succeeding is more important than pending" <|
                                \_ ->
                                    givenTwoJobs
                                        Concourse.BuildStatusSucceeded
                                        Concourse.BuildStatusPending
                                        |> findBanner
                                        |> isSolid green
                            ]
                        ]
                    ]
                ]
            , describe "on HD view" <|
                let
                    setup : () -> Query.Single ApplicationMsgs.TopLevelMessage
                    setup _ =
                        whenOnDashboard { highDensity = True }
                            |> givenDataUnauthenticated
                                (oneTeamOnePipeline "team")
                            |> Common.queryView

                    noPipelines : () -> Query.Single ApplicationMsgs.TopLevelMessage
                    noPipelines _ =
                        whenOnDashboard { highDensity = True }
                            |> givenDataUnauthenticated
                                (apiData
                                    [ ( "some-team", [] )
                                    , ( "other-team", [ "pipeline" ] )
                                    ]
                                )
                            |> Common.queryView

                    card : Query.Single ApplicationMsgs.TopLevelMessage -> Query.Single ApplicationMsgs.TopLevelMessage
                    card =
                        Query.find
                            [ class "card"
                            , containing [ text "pipeline" ]
                            ]

                    cardText : Query.Single ApplicationMsgs.TopLevelMessage -> Query.Single ApplicationMsgs.TopLevelMessage
                    cardText =
                        card
                            >> Query.children []
                            >> Query.index 1

                    noPipelinesCard =
                        Query.find
                            [ class "card"
                            , containing [ text "no pipeline" ]
                            ]
                in
                [ test "no pipelines card has 14px font and 1px spacing" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has
                            [ style "font-size" "14px"
                            , style "letter-spacing" "1px"
                            ]
                , test "no pipelines card text is vertically centered" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has
                            [ style "display" "flex"
                            , style "align-items" "center"
                            ]
                , test "no pipelines card is 60px tall" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has [ style "height" "60px" ]
                , test "no pipelines card has 60px right margin" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has [ style "margin-right" "60px" ]
                , test "no pipelines card text has 10px padding" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.children []
                        >> Query.first
                        >> Query.has [ style "padding" "10px" ]
                , test "no pipelines card is 200px wide" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has [ style "width" "200px" ]
                , test "no pipelines card has dark grey background" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has [ style "background-color" darkGrey ]
                , test "card has larger tighter font" <|
                    setup
                        >> card
                        >> Query.has
                            [ style "font-size" "19px"
                            , style "letter-spacing" "1px"
                            ]
                , test "card text does not overflow or wrap" <|
                    setup
                        >> cardText
                        >> Query.has
                            [ style "width" "180px"
                            , style "white-space" "nowrap"
                            , style "overflow" "hidden"
                            , style "text-overflow" "ellipsis"
                            ]
                , test "card text is vertically centered" <|
                    setup
                        >> cardText
                        >> Query.has
                            [ style "align-self" "center" ]
                , test "card text has 10px padding" <|
                    setup
                        >> cardText
                        >> Query.has
                            [ style "padding" "10px" ]
                , test "card lays out contents horizontally" <|
                    setup
                        >> card
                        >> Query.has
                            [ style "display" "flex" ]
                , test "card is 60px tall" <|
                    setup
                        >> card
                        >> Query.has [ style "height" "60px" ]
                , test "card is 200px wide" <|
                    setup
                        >> card
                        >> Query.has [ style "width" "200px" ]
                , test "no triangle when there is no resource error" <|
                    setup
                        >> card
                        >> Query.children []
                        >> Query.count (Expect.equal 2)
                , describe "resource error triangle" <|
                    let
                        givenResourceError : () -> Query.Single ApplicationMsgs.TopLevelMessage
                        givenResourceError _ =
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (\user ->
                                        { teams =
                                            [ { id = 0
                                              , name = "team"
                                              }
                                            ]
                                        , pipelines =
                                            [ { id = 0
                                              , name = "pipeline"
                                              , paused = False
                                              , public = True
                                              , teamName = "team"
                                              , groups = []
                                              }
                                            ]
                                        , jobs = []
                                        , resources =
                                            [ { teamName = "team"
                                              , pipelineName = "pipeline"
                                              , name = "resource"
                                              , failingToCheck = True
                                              , checkError = ""
                                              , checkSetupError = ""
                                              , lastChecked = Nothing
                                              , pinnedVersion = Nothing
                                              , pinnedInConfig = False
                                              , pinComment = Nothing
                                              , icon = Nothing
                                              }
                                            ]
                                        , version = ""
                                        , user = user
                                        }
                                    )
                                |> Common.queryView

                        resourceErrorTriangle =
                            Query.children []
                                >> Query.index -1
                    in
                    [ test "exists" <|
                        givenResourceError
                            >> card
                            >> Query.children []
                            >> Query.count (Expect.equal 3)
                    , test "is at the top right of card" <|
                        givenResourceError
                            >> card
                            >> Expect.all
                                [ Query.has [ style "position" "relative" ]
                                , resourceErrorTriangle
                                    >> Query.has
                                        [ style "position" "absolute"
                                        , style "top" "0"
                                        , style "right" "0"
                                        ]
                                ]
                    , test "is an orange 'top right' triangle" <|
                        givenResourceError
                            >> card
                            >> resourceErrorTriangle
                            >> Query.has
                                [ style "width" "0"
                                , style "height" "0"
                                , style "border-top" <| "30px solid " ++ orange
                                , style "border-left" "30px solid transparent"
                                ]
                    ]
                , test
                    ("cards are spaced 4px apart vertically and "
                        ++ "60px apart horizontally"
                    )
                  <|
                    setup
                        >> card
                        >> Query.has
                            [ style "margin" "0 60px 4px 0" ]
                , test "card is faded green when pipeline is suceeding" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> pipelineWithStatus
                                Concourse.BuildStatusSucceeded
                                False
                            |> card
                            |> Query.has [ style "background-color" fadedGreen ]
                , test "card is red when pipeline is failing" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> pipelineWithStatus
                                Concourse.BuildStatusFailed
                                False
                            |> card
                            |> Query.has [ style "background-color" red ]
                , test "card is amber when pipeline is erroring" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> pipelineWithStatus
                                Concourse.BuildStatusErrored
                                False
                            |> card
                            |> Query.has [ style "background-color" amber ]
                ]
            , describe "body"
                [ test "has dark grey background" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (oneTeamOnePipeline "team")
                            |> Common.queryView
                            |> Query.find
                                [ class "card"
                                , containing [ text "pipeline" ]
                                ]
                            |> findBody
                            |> Query.has [ style "background-color" darkGrey ]
                , test "has 2x margin above and below" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (oneTeamOnePipeline "team")
                            |> Common.queryView
                            |> Query.find
                                [ class "card"
                                , containing [ text "pipeline" ]
                                ]
                            |> findBody
                            |> Query.has [ style "margin" "2px 0" ]
                ]
            , describe "footer" <|
                let
                    hasStyle : String -> String -> Expectation
                    hasStyle property value =
                        whenOnDashboard { highDensity = False }
                            |> givenDataAndUser
                                (oneTeamOnePipeline "team")
                                (userWithRoles [ ( "team", [ "owner" ] ) ])
                            |> Common.queryView
                            |> Query.find [ class "card-footer" ]
                            |> Query.has [ style property value ]
                in
                [ test "has dark grey background" <|
                    \_ ->
                        hasStyle "background-color" darkGrey
                , test "has medium padding" <|
                    \_ ->
                        hasStyle "padding" "13.5px"
                , test "lays out contents horizontally" <|
                    \_ ->
                        hasStyle "display" "flex"
                , test "is divided into a left and right section, spread apart" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataAndUser
                                (oneTeamOnePipeline "team")
                                (userWithRoles [ ( "team", [ "owner" ] ) ])
                            |> Common.queryView
                            |> Query.find [ class "card-footer" ]
                            |> Expect.all
                                [ Query.children []
                                    >> Query.count (Expect.equal 2)
                                , Query.has
                                    [ style "justify-content" "space-between" ]
                                ]
                , test "both sections lay out contents horizontally" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataAndUser
                                (oneTeamOnePipeline "team")
                                (userWithRoles [ ( "team", [ "owner" ] ) ])
                            |> Common.queryView
                            |> Query.find [ class "card-footer" ]
                            |> Query.children []
                            |> Query.each (Query.has [ style "display" "flex" ])
                , describe "left-hand section" <|
                    let
                        findStatusIcon =
                            Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.first

                        findStatusText =
                            Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index -1
                    in
                    [ describe "when pipeline is paused" <|
                        let
                            setup =
                                whenOnDashboard { highDensity = False }
                                    |> givenDataUnauthenticated
                                        (\u ->
                                            { teams =
                                                [ { id = 0, name = "team" } ]
                                            , pipelines =
                                                [ onePipelinePaused "team" ]
                                            , jobs = []
                                            , resources = []
                                            , version = ""
                                            , user = u
                                            }
                                        )
                                    |> Common.queryView
                        in
                        [ test "status icon is blue pause" <|
                            \_ ->
                                setup
                                    |> findStatusIcon
                                    |> Query.has
                                        (iconSelector
                                            { size = "20px"
                                            , image = "ic-pause-blue.svg"
                                            }
                                            ++ [ style "background-size" "contain" ]
                                        )
                        , test "status text is blue" <|
                            \_ ->
                                setup
                                    |> findStatusText
                                    |> Query.has [ style "color" blue ]
                        , test "status text is larger and spaced more widely" <|
                            \_ ->
                                setup
                                    |> findStatusText
                                    |> Query.has
                                        [ style "font-size" "18px"
                                        , style "line-height" "20px"
                                        , style "letter-spacing" "0.05em"
                                        ]
                        , test "status text is offset to the right of the icon" <|
                            \_ ->
                                setup
                                    |> findStatusText
                                    |> Query.has [ style "margin-left" "8px" ]
                        , test "status text says 'paused'" <|
                            \_ ->
                                setup
                                    |> findStatusText
                                    |> Query.has [ text "paused" ]
                        ]
                    , describe "when pipeline is pending" <|
                        [ test "status icon is grey" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusPending
                                        False
                                    |> findStatusIcon
                                    |> Query.has
                                        (iconSelector
                                            { size = "20px"
                                            , image = "ic-pending-grey.svg"
                                            }
                                            ++ [ style "background-size" "contain" ]
                                        )
                        , test "status text is grey" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusPending
                                        False
                                    |> findStatusText
                                    |> Query.has [ style "color" lightGrey ]
                        , test "status text says 'pending'" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusPending
                                        False
                                    |> findStatusText
                                    |> Query.has
                                        [ text "pending" ]
                        , test "when running, status text says 'pending'" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusPending
                                        True
                                    |> findStatusText
                                    |> Query.has
                                        [ text "running" ]
                        ]
                    , describe "when pipeline is succeeding"
                        [ test "status icon is a green check" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusSucceeded
                                        False
                                    |> findStatusIcon
                                    |> Query.has
                                        (iconSelector
                                            { size = "20px"
                                            , image = "ic-running-green.svg"
                                            }
                                            ++ [ style "background-size" "contain" ]
                                        )
                        , test "status text is green" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusSucceeded
                                        False
                                    |> findStatusText
                                    |> Query.has [ style "color" green ]
                        , test "when running, status text says 'running'" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusSucceeded
                                        True
                                    |> findStatusText
                                    |> Query.has
                                        [ text "running" ]
                        , test "when not running, status text shows age" <|
                            \_ ->
                                Common.init "/"
                                    |> givenDataUnauthenticatedFromApplication
                                        (\u ->
                                            { teams =
                                                [ { id = 0, name = "team" } ]
                                            , pipelines =
                                                [ onePipeline "team" ]
                                            , jobs =
                                                [ jobWithNameTransitionedAt
                                                    "job"
                                                    (Just <| Time.millisToPosix 0)
                                                    Concourse.BuildStatusSucceeded
                                                ]
                                            , resources = []
                                            , version = ""
                                            , user = u
                                            }
                                        )
                                    |> afterSeconds 1
                                    |> Common.queryView
                                    |> findStatusText
                                    |> Query.has
                                        [ text "1s" ]
                        ]
                    , describe "when pipeline is failing"
                        [ test "status icon is a red !" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusFailed
                                        False
                                    |> findStatusIcon
                                    |> Query.has
                                        (iconSelector
                                            { size = "20px"
                                            , image = "ic-failing-red.svg"
                                            }
                                            ++ [ style "background-size" "contain" ]
                                        )
                        , test "status text is red" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusFailed
                                        False
                                    |> findStatusText
                                    |> Query.has [ style "color" red ]
                        ]
                    , test "when pipeline is aborted, status icon is a brown x" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusAborted
                                    False
                                |> findStatusIcon
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = "ic-aborted-brown.svg"
                                        }
                                        ++ [ style "background-size" "contain" ]
                                    )
                    , test "when pipeline is errored, status icon is an amber triangle" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusErrored
                                    False
                                |> findStatusIcon
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = "ic-error-orange.svg"
                                        }
                                        ++ [ style "background-size" "contain" ]
                                    )
                    ]
                , describe "right-hand section"
                    [ test
                        ("there is a 20px square open eye icon on the far right for a public pipeline"
                            ++ " with image resized to fit"
                        )
                      <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.children []
                                |> Query.index -1
                                |> Query.children []
                                |> Query.index -1
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = "baseline-visibility-24px.svg"
                                        }
                                        ++ [ style "background-size" "contain" ]
                                    )
                    , test
                        ("there is a 20px square slashed-out eye icon with on the far right for a"
                            ++ " non-public pipeline with image resized to fit"
                        )
                      <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipelineNonPublic "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.children []
                                |> Query.index -1
                                |> Query.children []
                                |> Query.index -1
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = "baseline-visibility-off-24px.svg"
                                        }
                                        ++ [ style "background-size" "contain" ]
                                    )
                    , test "there is medium spacing between the eye and the play/pause button" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.children []
                                |> Query.index -1
                                |> Query.children []
                                |> Expect.all
                                    [ Query.count (Expect.equal 3)
                                    , Query.index 1 >> Query.has [ style "width" "13.5px" ]
                                    ]
                    , test "the right section has a 20px square pause button on the left" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.children []
                                |> Query.index -1
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = "ic-pause-white.svg"
                                        }
                                    )
                    , test "pause button has pointer cursor" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.find
                                    (iconSelector
                                        { size = "20px"
                                        , image = "ic-pause-white.svg"
                                        }
                                    )
                                |> Query.has [ style "cursor" "pointer" ]
                    , test "pause button is transparent" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.find
                                    (iconSelector
                                        { size = "20px"
                                        , image = "ic-pause-white.svg"
                                        }
                                    )
                                |> Query.has [ style "opacity" "0.5" ]
                    , defineHoverBehaviour
                        { name = "pause button"
                        , setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                        , query =
                            Common.queryView
                                >> Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.index -1
                                >> Query.children []
                                >> Query.index 0
                        , updateFunc = \msg -> Application.update msg >> Tuple.first
                        , unhoveredSelector =
                            { description = "a faded 20px square pause button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = "ic-pause-white.svg"
                                    }
                                    ++ [ style "cursor" "pointer"
                                       , style "opacity" "0.5"
                                       ]
                            }
                        , mouseEnterMsg =
                            ApplicationMsgs.Update <|
                                Msgs.Hover <|
                                    Just <|
                                        Msgs.PipelineButton
                                            { pipelineName = "pipeline"
                                            , teamName = "team"
                                            }
                        , mouseLeaveMsg =
                            ApplicationMsgs.Update <|
                                Msgs.Hover Nothing
                        , hoveredSelector =
                            { description = "a bright 20px square pause button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = "ic-pause-white.svg"
                                    }
                                    ++ [ style "cursor" "pointer"
                                       , style "opacity" "1"
                                       ]
                            }
                        }
                    , defineHoverBehaviour
                        { name = "play button"
                        , setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipelinePaused "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                        , query =
                            Common.queryView
                                >> Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.index -1
                                >> Query.children []
                                >> Query.index 0
                        , updateFunc = \msg -> Application.update msg >> Tuple.first
                        , unhoveredSelector =
                            { description = "a transparent 20px square play button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = "ic-play-white.svg"
                                    }
                                    ++ [ style "cursor" "pointer"
                                       , style "opacity" "0.5"
                                       ]
                            }
                        , mouseEnterMsg =
                            ApplicationMsgs.Update <|
                                Msgs.Hover <|
                                    Just <|
                                        Msgs.PipelineButton
                                            { pipelineName = "pipeline"
                                            , teamName = "team"
                                            }
                        , mouseLeaveMsg =
                            ApplicationMsgs.Update <| Msgs.Hover Nothing
                        , hoveredSelector =
                            { description = "an opaque 20px square play button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = "ic-play-white.svg"
                                    }
                                    ++ [ style "cursor" "pointer"
                                       , style "opacity" "1"
                                       ]
                            }
                        }
                    , test "clicking pause button sends TogglePipeline msg" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.find
                                    (iconSelector
                                        { size = "20px"
                                        , image = "ic-pause-white.svg"
                                        }
                                    )
                                |> Event.simulate Event.click
                                |> Event.expect
                                    (ApplicationMsgs.Update <|
                                        Msgs.TogglePipelinePaused
                                            { pipelineName = "pipeline"
                                            , teamName = "team"
                                            }
                                            False
                                    )
                    , test "pause button turns into spinner on click" <|
                        \_ ->
                            let
                                animation =
                                    "container-rotate 1568ms linear infinite"
                            in
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.TogglePipelinePaused
                                            { pipelineName = "pipeline"
                                            , teamName = "team"
                                            }
                                            False
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.has [ style "animation" animation ]
                    , test "clicking pause button sends toggle api call" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.TogglePipelinePaused
                                            { pipelineName = "pipeline"
                                            , teamName = "team"
                                            }
                                            False
                                    )
                                |> Tuple.second
                                |> Expect.equal
                                    [ Effects.SendTogglePipelineRequest
                                        { pipelineName = "pipeline"
                                        , teamName = "team"
                                        }
                                        False
                                    ]
                    , test "dashboard data is refetched after ok toggle call" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.TogglePipelinePaused
                                            { pipelineName = "pipeline"
                                            , teamName = "team"
                                            }
                                            False
                                    )
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.PipelineToggled
                                        { pipelineName = "pipeline"
                                        , teamName = "team"
                                        }
                                        (Ok ())
                                    )
                                |> Tuple.second
                                |> Expect.equal [ Effects.FetchData ]
                    , test "401 toggle call redirects to login" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.TogglePipelinePaused
                                            { pipelineName = "pipeline"
                                            , teamName = "team"
                                            }
                                            False
                                    )
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.PipelineToggled
                                        { pipelineName = "pipeline"
                                        , teamName = "team"
                                        }
                                        (Err <|
                                            Http.BadStatus
                                                { url = "http://example.com"
                                                , status =
                                                    { code = 401
                                                    , message = ""
                                                    }
                                                , headers = Dict.empty
                                                , body = ""
                                                }
                                        )
                                    )
                                |> Tuple.second
                                |> Expect.equal [ Effects.RedirectToLogin ]
                    ]
                ]
            ]
        , describe "bottom bar"
            [ test "appears by default" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> Common.queryView
                        |> Query.has [ id "dashboard-info" ]
            , test "is 50px tall, almost black, fixed to the bottom of the viewport and covers entire width" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
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
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> Common.queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.has
                            [ style "display" "flex"
                            , style "justify-content" "space-between"
                            ]
            , test "two children are legend and concourse-info" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
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
                        |> givenDataUnauthenticatedFromApplication
                            (apiData [ ( "team", [ "pipeline" ] ) ])
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
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.has [ style "display" "flex" ]
                , test "shows pipeline statuses" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
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
                                                    , image = "ic-pending-grey.svg"
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
                                                    , image = "ic-pause-blue.svg"
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
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index -2
                            |> Query.has [ style "color" menuGrey ]
                , test "the legend separator centers contents vertically" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index -2
                            |> Query.has [ style "display" "flex", style "align-items" "center" ]
                , test "the legend separator is gone when the window width is below 812px" <|
                    \_ ->
                        Common.init "/"
                            |> givenDataUnauthenticatedFromApplication
                                (apiData [ ( "team", [ "pipeline" ] ) ])
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
                            |> givenDataUnauthenticatedFromApplication
                                (apiData [ ( "team", [ "pipeline" ] ) ])
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
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
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
                                (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> Common.queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.each
                                (Query.has [ style "margin-right" "20px" ])
                , test "third legend item shows running indicator" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [ "pipeline" ] ) ])
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
                                                    , image = "ic-running-legend.svg"
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
                                (apiData [ ( "team", [ "pipeline" ] ) ])
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
                                    [ style "background-image"
                                        "url(/public/images/ic-hd-off.svg)"
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
                                    (apiData [ ( "team", [ "pipeline" ] ) ])
                                |> Common.queryView
                                |> findHDToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style "background-image"
                                        "url(/public/images/ic-hd-on.svg)"
                                    , style "background-size" "contain"
                                    , style "height" "20px"
                                    , style "width" "35px"
                                    ]
                    , test "links to normal dashboard view" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (apiData [ ( "team", [ "pipeline" ] ) ])
                                |> Common.queryView
                                |> findHDToggle
                                |> Query.has [ attribute <| Attr.href "/" ]
                    , test "will not shrink on resizing" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (apiData
                                        [ ( "team", [ "pipeline" ] ) ]
                                    )
                                |> Common.queryView
                                |> findHDToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style "flex-shrink" "0" ]
                    ]
                ]
            , describe "info section" <|
                let
                    info =
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
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
                                    (apiData
                                        [ ( "team", [ "pipeline" ] ) ]
                                    )
                        , query =
                            Common.queryView >> Query.find [ id "cli-osx" ]
                        , unhoveredSelector =
                            { description = "grey apple icon"
                            , selector =
                                [ style "opacity" "0.5"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = "apple-logo.svg"
                                        , size = "20px"
                                        }
                            }
                        , updateFunc =
                            \msg ->
                                Application.update msg
                                    >> Tuple.first
                        , mouseEnterMsg =
                            ApplicationMsgs.Update <|
                                Msgs.Hover <|
                                    Maybe.map Msgs.FooterCliIcon <|
                                        List.Extra.getAt 0 Cli.clis
                        , mouseLeaveMsg =
                            ApplicationMsgs.Update <|
                                Msgs.Hover Nothing
                        , hoveredSelector =
                            { description = "white apple icon"
                            , selector =
                                [ style "opacity" "1"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = "apple-logo.svg"
                                        , size = "20px"
                                        }
                            }
                        }
                    , defineHoverBehaviour
                        { name = "windows cli icon"
                        , setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (apiData
                                        [ ( "team", [ "pipeline" ] ) ]
                                    )
                        , query =
                            Common.queryView >> Query.find [ id "cli-windows" ]
                        , unhoveredSelector =
                            { description = "grey windows icon"
                            , selector =
                                [ style "opacity" "0.5"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = "windows-logo.svg"
                                        , size = "20px"
                                        }
                            }
                        , updateFunc =
                            \msg -> Application.update msg >> Tuple.first
                        , mouseEnterMsg =
                            ApplicationMsgs.Update <|
                                Msgs.Hover <|
                                    Maybe.map Msgs.FooterCliIcon <|
                                        List.Extra.getAt 1 Cli.clis
                        , mouseLeaveMsg =
                            ApplicationMsgs.Update <|
                                Msgs.Hover Nothing
                        , hoveredSelector =
                            { description = "white windows icon"
                            , selector =
                                [ style "opacity" "1"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = "windows-logo.svg"
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
                                        [ ( "team", [ "pipeline" ] ) ]
                                    )
                        , query =
                            Common.queryView >> Query.find [ id "cli-linux" ]
                        , unhoveredSelector =
                            { description = "grey linux icon"
                            , selector =
                                [ style "opacity" "0.5"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = "linux-logo.svg"
                                        , size = "20px"
                                        }
                            }
                        , updateFunc =
                            \msg -> Application.update msg >> Tuple.first
                        , mouseEnterMsg =
                            ApplicationMsgs.Update <|
                                Msgs.Hover <|
                                    Maybe.map Msgs.FooterCliIcon <|
                                        List.Extra.getAt 2 Cli.clis
                        , mouseLeaveMsg =
                            ApplicationMsgs.Update <|
                                Msgs.Hover Nothing
                        , hoveredSelector =
                            { description = "white linux icon"
                            , selector =
                                [ style "opacity" "1"
                                , style "background-size" "contain"
                                ]
                                    ++ iconSelector
                                        { image = "linux-logo.svg"
                                        , size = "20px"
                                        }
                            }
                        }
                    ]
                , test "shows concourse version" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (\u ->
                                    { teams =
                                        [ { id = 0, name = "team" } ]
                                    , pipelines =
                                        [ onePipeline "team" ]
                                    , jobs = []
                                    , resources = []
                                    , version = "1.2.3"
                                    , user = u
                                    }
                                )
                            |> Common.queryView
                            |> Query.find [ id "concourse-info" ]
                            |> Query.has [ text "v1.2.3" ]
                ]
            , test "hides after 6 seconds" <|
                \_ ->
                    Common.init "/"
                        |> givenDataUnauthenticatedFromApplication (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> afterSeconds 6
                        |> Common.queryView
                        |> Query.hasNot [ id "dashboard-info" ]
            , test "reappears on mouse action" <|
                \_ ->
                    Common.init "/"
                        |> givenDataUnauthenticatedFromApplication (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> afterSeconds 6
                        |> Application.update
                            (ApplicationMsgs.DeliveryReceived Moused)
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.has [ id "dashboard-info" ]
            , test "is replaced by keyboard help when pressing '?'" <|
                \_ ->
                    Common.init "/"
                        |> givenDataUnauthenticatedFromApplication
                            (apiData [ ( "team", [ "pipeline" ] ) ])
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
                        [ List.member (Subscription.OnClockTick OneSecond)
                            >> Expect.true "doesn't have one second timer"
                        , List.member (Subscription.OnClockTick FiveSeconds)
                            >> Expect.true "doesn't have five second timer"
                        ]
        , test "subscribes to keyups" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Application.subscriptions
                    |> List.member Subscription.OnKeyUp
                    |> Expect.true "doesn't subscribe to keyups?"
        , test "auto refreshes data every five seconds" <|
            \_ ->
                Common.init "/"
                    |> Application.update
                        (ApplicationMsgs.DeliveryReceived <|
                            ClockTicked FiveSeconds <|
                                Time.millisToPosix 0
                        )
                    |> Tuple.second
                    |> Expect.equal [ Effects.FetchData ]
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


defineHoverBehaviour :
    { name : String
    , setup : model
    , query : model -> Query.Single msg
    , unhoveredSelector : { description : String, selector : List Selector }
    , mouseEnterMsg : msg
    , mouseLeaveMsg : msg
    , updateFunc : msg -> model -> model
    , hoveredSelector : { description : String, selector : List Selector }
    }
    -> Test
defineHoverBehaviour { name, setup, query, unhoveredSelector, mouseEnterMsg, mouseLeaveMsg, updateFunc, hoveredSelector } =
    describe (name ++ " hover behaviour")
        [ test (name ++ " is " ++ unhoveredSelector.description) <|
            \_ ->
                setup
                    |> query
                    |> Query.has unhoveredSelector.selector
        , test ("mousing over " ++ name ++ " triggers " ++ Debug.toString mouseEnterMsg ++ " msg") <|
            \_ ->
                setup
                    |> query
                    |> Event.simulate Event.mouseEnter
                    |> Event.expect mouseEnterMsg
        , test
            (Debug.toString mouseEnterMsg
                ++ " msg causes "
                ++ name
                ++ " to become "
                ++ hoveredSelector.description
            )
          <|
            \_ ->
                setup
                    |> updateFunc mouseEnterMsg
                    |> query
                    |> Query.has hoveredSelector.selector
        , test ("mousing off " ++ name ++ " triggers " ++ Debug.toString mouseLeaveMsg ++ " msg") <|
            \_ ->
                setup
                    |> updateFunc mouseEnterMsg
                    |> query
                    |> Event.simulate Event.mouseLeave
                    |> Event.expect mouseLeaveMsg
        , test
            (Debug.toString mouseLeaveMsg
                ++ " msg causes "
                ++ name
                ++ " to become "
                ++ unhoveredSelector.description
            )
          <|
            \_ ->
                setup
                    |> updateFunc mouseEnterMsg
                    |> updateFunc mouseLeaveMsg
                    |> query
                    |> Query.has unhoveredSelector.selector
        ]


iconSelector : { size : String, image : String } -> List Selector
iconSelector { size, image } =
    [ style "background-image" <| "url(/public/images/" ++ image ++ ")"
    , style "background-position" "50% 50%"
    , style "background-repeat" "no-repeat"
    , style "width" size
    , style "height" size
    ]


whenOnDashboard : { highDensity : Bool } -> Application.Model
whenOnDashboard { highDensity } =
    Common.init <|
        if highDensity then
            "/hd"

        else
            "/"


givenDataAndUser :
    (Maybe Concourse.User -> Concourse.APIData)
    -> Concourse.User
    -> Application.Model
    -> Application.Model
givenDataAndUser data user =
    Application.handleCallback
        (Callback.APIDataFetched <|
            Ok ( Time.millisToPosix 0, data <| Just user )
        )
        >> Tuple.first


userWithRoles : List ( String, List String ) -> Concourse.User
userWithRoles roles =
    { id = "0"
    , userName = "test"
    , name = "test"
    , email = "test"
    , teams =
        Dict.fromList roles
    }


givenDataUnauthenticatedFromApplication :
    (Maybe Concourse.User -> Concourse.APIData)
    -> Application.Model
    -> Application.Model
givenDataUnauthenticatedFromApplication data =
    Application.handleCallback
        (Callback.APIDataFetched <| Ok ( Time.millisToPosix 0, data Nothing ))
        >> Tuple.first


givenDataUnauthenticated :
    (Maybe Concourse.User -> Concourse.APIData)
    -> Application.Model
    -> Application.Model
givenDataUnauthenticated data =
    Application.handleCallback
        (Callback.APIDataFetched <|
            Ok ( Time.millisToPosix 0, data Nothing )
        )
        >> Tuple.first


givenPipelineWithJob : Maybe Concourse.User -> Concourse.APIData
givenPipelineWithJob user =
    { teams = []
    , pipelines =
        [ { id = 0
          , name = "pipeline"
          , paused = False
          , public = True
          , teamName = "team"
          , groups = []
          }
        ]
    , jobs =
        [ { pipeline =
                { teamName = "team"
                , pipelineName = "pipeline"
                }
          , name = "job"
          , pipelineName = "pipeline"
          , teamName = "team"
          , nextBuild = Nothing
          , finishedBuild =
                Just
                    { id = 0
                    , name = "1"
                    , job = Just { teamName = "team", pipelineName = "pipeline", jobName = "job" }
                    , status = Concourse.BuildStatusSucceeded
                    , duration = { startedAt = Nothing, finishedAt = Nothing }
                    , reapTime = Nothing
                    }
          , transitionBuild = Nothing
          , paused = False
          , disableManualTrigger = False
          , inputs = []
          , outputs = []
          , groups = []
          }
        ]
    , resources = []
    , version = ""
    , user = user
    }


oneTeamOnePipelinePaused : String -> Maybe Concourse.User -> Concourse.APIData
oneTeamOnePipelinePaused teamName user =
    { teams = [ { id = 0, name = teamName } ]
    , pipelines =
        [ { id = 0
          , name = "pipeline"
          , paused = True
          , public = True
          , teamName = teamName
          , groups = []
          }
        ]
    , jobs = []
    , resources = []
    , version = ""
    , user = user
    }


oneTeamOnePipelineNonPublic : String -> Maybe Concourse.User -> Concourse.APIData
oneTeamOnePipelineNonPublic teamName user =
    { teams = [ { id = 0, name = teamName } ]
    , pipelines =
        [ { id = 0
          , name = "pipeline"
          , paused = False
          , public = False
          , teamName = teamName
          , groups = []
          }
        ]
    , jobs = []
    , resources = []
    , version = ""
    , user = user
    }


oneTeamOnePipeline : String -> Maybe Concourse.User -> Concourse.APIData
oneTeamOnePipeline teamName =
    apiData [ ( teamName, [ "pipeline" ] ) ]


onePipeline : String -> Concourse.Pipeline
onePipeline teamName =
    { id = 0
    , name = "pipeline"
    , paused = False
    , public = True
    , teamName = teamName
    , groups = []
    }


onePipelinePaused : String -> Concourse.Pipeline
onePipelinePaused teamName =
    { id = 0
    , name = "pipeline"
    , paused = True
    , public = True
    , teamName = teamName
    , groups = []
    }


apiData : List ( String, List String ) -> Maybe Concourse.User -> Concourse.APIData
apiData pipelines user =
    { teams = pipelines |> List.map Tuple.first |> List.indexedMap Concourse.Team
    , pipelines =
        pipelines
            |> List.concatMap
                (\( teamName, ps ) ->
                    ps
                        |> List.indexedMap
                            (\i p ->
                                { id = i
                                , name = p
                                , paused = False
                                , public = True
                                , teamName = teamName
                                , groups = []
                                }
                            )
                )
    , jobs = []
    , resources = []
    , version = ""
    , user = user
    }


running : Concourse.Job -> Concourse.Job
running j =
    { j
        | nextBuild =
            Just
                { id = 1
                , name = "1"
                , job =
                    Just
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "job"
                        }
                , status = Concourse.BuildStatusStarted
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Nothing
                    }
                , reapTime = Nothing
                }
    }


otherJob : Concourse.BuildStatus -> Concourse.Job
otherJob =
    jobWithNameTransitionedAt "other-job" <| Just <| Time.millisToPosix 0


job : Concourse.BuildStatus -> Concourse.Job
job =
    jobWithNameTransitionedAt "job" <| Just <| Time.millisToPosix 0


jobWithNameTransitionedAt : String -> Maybe Time.Posix -> Concourse.BuildStatus -> Concourse.Job
jobWithNameTransitionedAt jobName transitionedAt status =
    { pipeline =
        { teamName = "team"
        , pipelineName = "pipeline"
        }
    , name = jobName
    , pipelineName = "pipeline"
    , teamName = "team"
    , nextBuild = Nothing
    , finishedBuild =
        Just
            { id = 0
            , name = "0"
            , job =
                Just
                    { teamName = "team"
                    , pipelineName = "pipeline"
                    , jobName = jobName
                    }
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
                    , name = "1"
                    , job =
                        Just
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = jobName
                            }
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
    [ { pipeline =
            { teamName = "team"
            , pipelineName = "pipeline"
            }
      , name = "jobA"
      , pipelineName = "pipeline"
      , teamName = "team"
      , nextBuild = Nothing
      , finishedBuild =
            Just
                { id = 0
                , name = "0"
                , job =
                    Just
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "jobA"
                        }
                , status = Concourse.BuildStatusSucceeded
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
                , job =
                    Just
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "jobA"
                        }
                , status = Concourse.BuildStatusSucceeded
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
    , { pipeline =
            { teamName = "team"
            , pipelineName = "pipeline"
            }
      , name = "jobB"
      , pipelineName = "pipeline"
      , teamName = "team"
      , nextBuild = Nothing
      , finishedBuild =
            Just
                { id = 0
                , name = "0"
                , job =
                    Just
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "jobB"
                        }
                , status = Concourse.BuildStatusSucceeded
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
                , job =
                    Just
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "jobB"
                        }
                , status = Concourse.BuildStatusSucceeded
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


isColorWithStripes : { thick : String, thin : String } -> Query.Single msg -> Expectation
isColorWithStripes { thick, thin } =
    Query.has
        [ style "background-image" <|
            "repeating-linear-gradient(-115deg,"
                ++ thick
                ++ " 0,"
                ++ thick
                ++ " 10px,"
                ++ thin
                ++ " 0,"
                ++ thin
                ++ " 16px)"
        , style "background-size" "106px 114px"
        , style "animation" <|
            pipelineRunningKeyframes
                ++ " 3s linear infinite"
        ]
