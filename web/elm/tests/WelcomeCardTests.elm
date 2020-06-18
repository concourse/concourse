module WelcomeCardTests exposing (all, hasWelcomeCard)

import Application.Application as Application
import Assets
import Common exposing (defineHoverBehaviour)
import Concourse
import Concourse.Cli as Cli
import DashboardTests exposing (apiData, darkGrey, givenDataAndUser, givenDataUnauthenticated, iconSelector, userWithRoles, whenOnDashboard)
import Data
import Expect
import Html.Attributes as Attr
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Msgs
import Message.TopLevelMessage as ApplicationMsgs
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, containing, id, style, tag, text)
import Views.Styles


all : Test
all =
    describe "welcome card"
        [ describe "when unauthenticated with no teams" <|
            hasWelcomeCard
                (\_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [])
                        |> Tuple.first
                        |> givenPipelines []
                )
                ++ [ test "page body is empty" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated (apiData [])
                                |> Tuple.first
                                |> givenPipelines []
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ id "page-below-top-bar" ]
                                |> Query.children []
                                |> Query.first
                                |> Query.children []
                                |> Query.count (Expect.equal 0)
                   ]
        , describe "when unauthenticated with a team but no pipelines" <|
            hasWelcomeCard
                (\_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> givenPipelines []
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
                        |> Tuple.first
                        |> givenPipelines []
                )
        , test "no login instruction when logged in" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataAndUser
                        (apiData [ ( "team", [] ) ])
                        (userWithRoles [])
                    |> Tuple.first
                    |> givenPipelines []
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ id "login-instruction" ]
        , test "has login instruction when unauthenticated" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                    |> Tuple.first
                    |> givenPipelines []
                    |> Tuple.first
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
        , test "does not appear when there are visible pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenPipelines
                        [ Data.pipeline "team" 0 ]
                    |> Tuple.first
                    |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "welcome to concourse" ]
        , test "does not appear when pipelines have not yet been fetched" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ text "welcome to concourse" ]
        ]


hasWelcomeCard : (() -> ( Application.Model, List Effects.Effect )) -> List Test
hasWelcomeCard setup =
    let
        subject : () -> Query.Single ApplicationMsgs.TopLevelMessage
        subject =
            setup
                >> Tuple.first
                >> Common.queryView
                >> Query.find [ id "welcome-card" ]
    in
    [ test "exists" <|
        setup
            >> Tuple.first
            >> Common.queryView
            >> Query.has [ id "welcome-card" ]
    , test "with light text" <|
        subject
            >> Query.has [ style "font-weight" Views.Styles.fontWeightLight ]
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
                    , setup = setup () |> Tuple.first
                    , query =
                        Common.queryView
                            >> Query.find [ id "top-cli-osx" ]
                    , unhoveredSelector =
                        { description = "grey apple icon"
                        , selector =
                            [ style "opacity" "0.5"
                            , style "margin" "5px"
                            ]
                                ++ iconSelector
                                    { size = "32px"
                                    , image = Assets.CliIcon Cli.OSX
                                    }
                        }
                    , hoverable =
                        Msgs.WelcomeCardCliIcon Cli.OSX
                    , hoveredSelector =
                        { description = "white apple icon"
                        , selector =
                            [ style "opacity" "1"
                            , style "margin" "5px"
                            ]
                                ++ iconSelector
                                    { size = "32px"
                                    , image = Assets.CliIcon Cli.OSX
                                    }
                        }
                    }
                , defineHoverBehaviour
                    { name = "windows cli icon"
                    , setup = setup () |> Tuple.first
                    , query =
                        Common.queryView
                            >> Query.find
                                [ id "top-cli-windows" ]
                    , unhoveredSelector =
                        { description = "grey windows icon"
                        , selector =
                            [ style "opacity" "0.5"
                            , style "margin" "5px"
                            ]
                                ++ iconSelector
                                    { size = "32px"
                                    , image = Assets.CliIcon Cli.Windows
                                    }
                        }
                    , hoverable =
                        Msgs.WelcomeCardCliIcon Cli.Windows
                    , hoveredSelector =
                        { description = "white windows icon"
                        , selector =
                            [ style "opacity" "1"
                            , style "margin" "5px"
                            ]
                                ++ iconSelector
                                    { size = "32px"
                                    , image = Assets.CliIcon Cli.Windows
                                    }
                        }
                    }
                , defineHoverBehaviour
                    { name = "linux cli icon"
                    , setup = setup () |> Tuple.first
                    , query =
                        Common.queryView
                            >> Query.find
                                [ id "top-cli-linux" ]
                    , unhoveredSelector =
                        { description = "grey linux icon"
                        , selector =
                            [ style "opacity" "0.5"
                            , style "margin" "5px"
                            ]
                                ++ iconSelector
                                    { size = "32px"
                                    , image = Assets.CliIcon Cli.Linux
                                    }
                        }
                    , hoverable =
                        Msgs.WelcomeCardCliIcon Cli.Linux
                    , hoveredSelector =
                        { description = "white linux icon"
                        , selector =
                            [ style "opacity" "1"
                            , style "margin" "5px"
                            ]
                                ++ iconSelector
                                    { size = "32px"
                                    , image = Assets.CliIcon Cli.Linux
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


givenPipelines : List Concourse.Pipeline -> Application.Model -> ( Application.Model, List Effects.Effect )
givenPipelines pipelines model =
    model
        |> Application.handleCallback (Callback.AllPipelinesFetched <| Ok pipelines)
