module DashboardTests exposing
    ( all
    , apiData
    , darkGrey
    , defineHoverBehaviour
    , givenDataAndUser
    , givenDataUnauthenticated
    , iconSelector
    , middleGrey
    , white
    )

import Char
import Concourse
import Concourse.Cli as Cli
import Concourse.PipelineStatus as PipelineStatus
import Dashboard
import Dashboard.APIData as APIData
import Dashboard.Group as Group
import Dashboard.Msgs as Msgs
import Date exposing (Date)
import Dict
import Expect exposing (Expectation)
import Html.Attributes as Attr
import Html.Styled as HS
import RemoteData
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
import Time exposing (Time)


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
    "#4a90e2"


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
    "#fff"


fadedGreen : String
fadedGreen =
    "#284834"


orange : String
orange =
    "#e67e22"


pipelineRunningKeyframes : String
pipelineRunningKeyframes =
    "pipeline-running"


all : Test
all =
    describe "Dashboard"
        [ describe "welcome card" <|
            let
                hasWelcomeCard : (() -> Dashboard.Model) -> List Test
                hasWelcomeCard setup =
                    let
                        subject : () -> Query.Single Msgs.Msg
                        subject =
                            setup
                                >> queryView
                                >> Query.find [ id "welcome-card" ]
                    in
                    [ test "exists" <|
                        setup
                            >> queryView
                            >> Query.has [ id "welcome-card" ]
                    , test "with correct antialiasing in WebKit" <|
                        subject
                            >> Query.has
                                [ style
                                    [ ( "-webkit-font-smoothing"
                                      , "antialiased"
                                      )
                                    ]
                                ]
                    , test "title says 'welcome to concourse!'" <|
                        subject
                            >> Query.children []
                            >> Query.first
                            >> Query.has [ text "welcome to concourse!" ]
                    , test "welcome message has large font" <|
                        subject
                            >> Query.children []
                            >> Query.first
                            >> Query.has [ style [ ( "font-size", "32px" ) ] ]
                    , test "has dark grey background" <|
                        subject
                            >> Query.has
                                [ style
                                    [ ( "background-color"
                                      , darkGrey
                                      )
                                    ]
                                ]
                    , test "is inset from the page" <|
                        subject
                            >> Query.has [ style [ ( "margin", "25px" ) ] ]
                    , test "has padding around its contents" <|
                        subject
                            >> Query.has [ style [ ( "padding", "40px" ) ] ]
                    , describe "body" <|
                        let
                            body : () -> Query.Single Msgs.Msg
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
                                    [ style [ ( "font-size", "16px" ) ] ]
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
                                        [ style
                                            [ ( "display", "flex" )
                                            , ( "align-items", "center" )
                                            ]
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
                                        [ style
                                            [ ( "margin-right"
                                              , "10px"
                                              )
                                            ]
                                        ]
                            , describe "cli download icons" <|
                                let
                                    cliIcons =
                                        downloadSection
                                            >> Query.children [ tag "a" ]
                                in
                                [ test "icons have descriptive ARIA labels" <|
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
                                    , query = queryView >> Query.find [ id "top-cli-osx" ]
                                    , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                                    , unhoveredSelector =
                                        { description = "grey apple icon"
                                        , selector =
                                            [ style
                                                [ ( "opacity", "0.5" )
                                                , ( "margin", "5px" )
                                                ]
                                            ]
                                                ++ iconSelector { size = "32px", image = "apple-logo.svg" }
                                        }
                                    , mouseEnterMsg = Msgs.TopCliHover <| Just Cli.OSX
                                    , mouseLeaveMsg = Msgs.TopCliHover Nothing
                                    , hoveredSelector =
                                        { description = "white apple icon"
                                        , selector =
                                            [ style
                                                [ ( "opacity", "1" )
                                                , ( "margin", "5px" )
                                                ]
                                            ]
                                                ++ iconSelector { size = "32px", image = "apple-logo.svg" }
                                        }
                                    }
                                , defineHoverBehaviour
                                    { name = "windows cli icon"
                                    , setup = setup ()
                                    , query = queryView >> Query.find [ id "top-cli-windows" ]
                                    , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                                    , unhoveredSelector =
                                        { description = "grey windows icon"
                                        , selector =
                                            [ style
                                                [ ( "opacity", "0.5" )
                                                , ( "margin", "5px" )
                                                ]
                                            ]
                                                ++ iconSelector { size = "32px", image = "windows-logo.svg" }
                                        }
                                    , mouseEnterMsg = Msgs.TopCliHover <| Just Cli.Windows
                                    , mouseLeaveMsg = Msgs.TopCliHover Nothing
                                    , hoveredSelector =
                                        { description = "white windows icon"
                                        , selector =
                                            [ style
                                                [ ( "opacity", "1" )
                                                , ( "margin", "5px" )
                                                ]
                                            ]
                                                ++ iconSelector { size = "32px", image = "windows-logo.svg" }
                                        }
                                    }
                                , defineHoverBehaviour
                                    { name = "linux cli icon"
                                    , setup = setup ()
                                    , query = queryView >> Query.find [ id "top-cli-linux" ]
                                    , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                                    , unhoveredSelector =
                                        { description = "grey linux icon"
                                        , selector =
                                            [ style
                                                [ ( "opacity", "0.5" )
                                                , ( "margin", "5px" )
                                                ]
                                            ]
                                                ++ iconSelector { size = "32px", image = "linux-logo.svg" }
                                        }
                                    , mouseEnterMsg = Msgs.TopCliHover <| Just Cli.Linux
                                    , mouseLeaveMsg = Msgs.TopCliHover Nothing
                                    , hoveredSelector =
                                        { description = "white linux icon"
                                        , selector =
                                            [ style
                                                [ ( "opacity", "1" )
                                                , ( "margin", "5px" )
                                                ]
                                            ]
                                                ++ iconSelector { size = "32px", image = "linux-logo.svg" }
                                        }
                                    }
                                ]
                            ]
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
                                    |> queryView
                                    |> Query.find [ class "dashboard-content" ]
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
                        |> queryView
                        |> Query.hasNot [ id "login-instruction" ]
            , test "has login instruction when unauthenticated" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                        |> queryView
                        |> Query.find [ id "welcome-card" ]
                        |> Query.find [ id "login-instruction" ]
                        |> Expect.all
                            [ Query.has [ text "login" ]
                            , Query.find [ tag "a", containing [ text "here" ] ]
                                >> Expect.all
                                    [ Query.has
                                        [ attribute <| Attr.href "/login" ]
                                    , Query.has
                                        [ style
                                            [ ( "text-decoration"
                                              , "underline"
                                              )
                                            ]
                                        ]
                                    ]
                            , Query.has
                                [ style [ ( "line-height", "42px" ) ] ]
                            ]
            ]
        , test "logging out causes pipeline list to reload" <|
            let
                showsLoadingState : Dashboard.Model -> Expectation
                showsLoadingState =
                    queryView
                        >> Query.findAll [ class "dashboard-team-group" ]
                        >> Query.count (Expect.equal 0)
            in
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataAndUser
                        (oneTeamOnePipelineNonPublic "team")
                        (userWithRoles [ ( "team", [ "owner" ] ) ])
                    |> Dashboard.update Msgs.LogOut
                    |> Tuple.first
                    |> showsLoadingState
        , test "links to specific builds" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataUnauthenticated givenPipelineWithJob
                    |> queryView
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
                    |> Dashboard.update
                        (Msgs.APIDataFetched <|
                            RemoteData.Success
                                ( 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Expect.all
                        [ Tuple.second
                            >> Expect.equal [ Dashboard.ModifyUrl "/" ]
                        , Tuple.first
                            >> Dashboard.update
                                (Msgs.APIDataFetched <|
                                    RemoteData.Success
                                        ( 0
                                        , apiData
                                            [ ( "team", [ "pipeline" ] ) ]
                                            Nothing
                                        )
                                )
                            >> Tuple.first
                            >> queryView
                            >> Expect.all
                                [ Query.find
                                    [ class "card-footer" ]
                                    >> Query.children []
                                    >> Query.first
                                    >> Query.children []
                                    >> Query.index -1
                                    >> Query.has [ text "pending" ]
                                , Query.has [ tag "input" ]
                                ]
                        ]
        , test "HD view redirects to no pipelines view when pipelines disappear" <|
            \_ ->
                whenOnDashboard { highDensity = True }
                    |> Dashboard.update
                        (Msgs.APIDataFetched <|
                            RemoteData.Success
                                ( 0
                                , apiData [ ( "team", [ "pipeline" ] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> Dashboard.update
                        (Msgs.APIDataFetched <|
                            RemoteData.Success
                                ( 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Expect.all
                        [ Tuple.second
                            >> Expect.equal [ Dashboard.ModifyUrl "/" ]
                        , Tuple.first
                            >> queryView
                            >> Query.has [ text "welcome to concourse!" ]
                        ]
        , test "no search bar when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Dashboard.update
                        (Msgs.APIDataFetched <|
                            RemoteData.Success
                                ( 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> queryView
                    |> Query.hasNot [ tag "input" ]
        , test "bottom bar appears when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Dashboard.update
                        (Msgs.APIDataFetched <|
                            RemoteData.Success
                                ( 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> queryView
                    |> Query.has [ id "dashboard-info" ]
        , test "bottom bar has no legend when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Dashboard.update
                        (Msgs.APIDataFetched <|
                            RemoteData.Success
                                ( 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> queryView
                    |> Query.hasNot [ id "legend" ]
        , test "concourse info is right-justified when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Dashboard.update
                        (Msgs.APIDataFetched <|
                            RemoteData.Success
                                ( 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> queryView
                    |> Query.find [ id "dashboard-info" ]
                    |> Query.has [ style [ ( "justify-content", "flex-end" ) ] ]
        , test "pressing '?' does nothing when there are no pipelines" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> Dashboard.update
                        (Msgs.APIDataFetched <|
                            RemoteData.Success
                                ( 0
                                , apiData [ ( "team", [] ) ] Nothing
                                )
                        )
                    |> Tuple.first
                    |> Dashboard.update (Msgs.KeyPressed (Char.toCode '?'))
                    |> Tuple.first
                    |> queryView
                    |> Query.has [ id "dashboard-info" ]
        , describe "team pills"
            [ test
                ("shows team name with no pill when unauthenticated "
                    ++ "and team has an exposed pipeline"
                )
              <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> queryView
                        |> teamHeaderHasNoPill "team"
            , test "shows OWNER pill on team header for team on which user has owner role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> queryView
                        |> teamHeaderHasPill "team" "OWNER"
            , test "shows MEMBER pill on team header for team on which user has member role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "member" ] ) ])
                        |> queryView
                        |> teamHeaderHasPill "team" "MEMBER"
            , test "shows VIEWER pill on team header for team on which user has viewer role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "viewer" ] ) ])
                        |> queryView
                        |> teamHeaderHasPill "team" "VIEWER"
            , test "shows no pill on team header for team on which user has no role" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [])
                        |> queryView
                        |> teamHeaderHasNoPill "team"
            , test "shows pill for first role on team header for team on which user has multiple roles" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "member", "viewer" ] ) ])
                        |> queryView
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
                        |> queryView
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
                        |> queryView
                        |> Query.findAll teamHeaderSelector
                        |> Query.each
                            (Query.has
                                [ style
                                    [ ( "display", "flex" )
                                    , ( "align-items", "center" )
                                    ]
                                ]
                            )
            , test "on HD view, there is space between the list of pipelines and the role pill" <|
                \_ ->
                    whenOnDashboard { highDensity = True }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> queryView
                        |> Query.find [ class "dashboard-team-name-wrapper" ]
                        |> Query.find [ containing [ text "OWNER" ] ]
                        |> Query.has [ style [ ( "margin-bottom", "1em" ) ] ]
            , test "on non-HD view, the role pill on a group has no margin below" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (oneTeamOnePipeline "team")
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> queryView
                        |> Query.find teamHeaderSelector
                        |> Query.find [ containing [ text "OWNER" ] ]
                        |> Query.has [ style [ ( "margin-bottom", "" ) ] ]
            ]
        , describe "pipeline cards" <|
            let
                findHeader : Query.Single Msgs.Msg -> Query.Single Msgs.Msg
                findHeader =
                    Query.find [ class "card-header" ]

                findBody : Query.Single Msgs.Msg -> Query.Single Msgs.Msg
                findBody =
                    Query.find [ class "card-body" ]

                pipelineWithStatus :
                    Concourse.BuildStatus
                    -> Bool
                    -> Dashboard.Model
                    -> Query.Single Msgs.Msg
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
                        >> queryView
            in
            [ describe "when team has no visible pipelines" <|
                let
                    noPipelinesCard : () -> Query.Single Msgs.Msg
                    noPipelinesCard _ =
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData
                                    [ ( "some-team", [] )
                                    , ( "other-team", [ "pipeline" ] )
                                    ]
                                )
                            |> queryView
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
                        header : () -> Query.Single Msgs.Msg
                        header =
                            noPipelinesCard
                                >> findHeader
                    in
                    [ test "says 'no pipeline set' in smooth white font" <|
                        header
                            >> Expect.all
                                [ Query.has [ text "no pipeline set" ]
                                , Query.has
                                    [ style
                                        [ ( "color", white )
                                        , ( "-webkit-font-smoothing"
                                          , "antialiased"
                                          )
                                        ]
                                    ]
                                ]
                    , test "has dark grey background and 12.5px padding" <|
                        header
                            >> Query.has
                                [ style
                                    [ ( "background-color", darkGrey )
                                    , ( "padding", "12.5px" )
                                    ]
                                ]
                    , test "text is larger and wider spaced" <|
                        header
                            >> Query.has
                                [ style
                                    [ ( "font-size", "1.5em" )
                                    , ( "letter-spacing", "0.1em" )
                                    ]
                                ]
                    , test "text is centered" <|
                        header
                            >> Query.has
                                [ style
                                    [ ( "text-align", "center" )
                                    ]
                                ]
                    ]
                , describe "body" <|
                    let
                        body : () -> Query.Single Msgs.Msg
                        body =
                            noPipelinesCard
                                >> Query.find [ class "card-body" ]
                    in
                    [ test "has 200px width, 120px height, 20px 36px padding" <|
                        body
                            >> Query.has
                                [ style
                                    [ ( "width", "200px" )
                                    , ( "height", "120px" )
                                    , ( "padding", "20px 36px" )
                                    ]
                                ]
                    , test "has dark grey background" <|
                        body
                            >> Query.has
                                [ style
                                    [ ( "background-color", darkGrey )
                                    ]
                                ]
                    , test "has 2px margins above and below" <|
                        body
                            >> Query.has
                                [ style
                                    [ ( "margin", "2px 0" )
                                    ]
                                ]
                    , test "has lighter grey placeholder box that fills" <|
                        body
                            >> Expect.all
                                [ Query.has
                                    [ style [ ( "display", "flex" ) ] ]
                                , Query.children []
                                    >> Query.first
                                    >> Query.has
                                        [ style
                                            [ ( "background-color"
                                              , middleGrey
                                              )
                                            , ( "flex-grow", "1" )
                                            ]
                                        ]
                                ]
                    ]
                , test "footer is dark grey and 47 pixels tall" <|
                    noPipelinesCard
                        >> Query.find [ class "card-footer" ]
                        >> Query.has
                            [ style
                                [ ( "background-color", darkGrey )
                                , ( "height", "47px" )
                                ]
                            ]
                ]
            , test "has 'move' cursor" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated
                            (oneTeamOnePipeline "team")
                        |> queryView
                        |> Query.find
                            [ class "card"
                            , containing [ text "pipeline" ]
                            ]
                        |> Query.has [ style [ ( "cursor", "move" ) ] ]
            , test "has 25px margins" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated
                            (oneTeamOnePipeline "team")
                        |> queryView
                        |> Query.find
                            [ class "card"
                            , containing [ text "pipeline" ]
                            ]
                        |> Query.has [ style [ ( "margin", "25px" ) ] ]
            , describe "header" <|
                let
                    header : () -> Query.Single Msgs.Msg
                    header _ =
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (oneTeamOnePipeline "team")
                            |> queryView
                            |> Query.find
                                [ class "card"
                                , containing [ text "pipeline" ]
                                ]
                            |> findHeader
                in
                [ test "has dark grey background" <|
                    header
                        >> Query.has
                            [ style
                                [ ( "background-color", darkGrey ) ]
                            ]
                , test "has larger, spaced-out smooth white text" <|
                    header
                        >> Query.has
                            [ style
                                [ ( "font-size", "1.5em" )
                                , ( "letter-spacing", "0.1em" )
                                , ( "color", white )
                                , ( "-webkit-font-smoothing", "antialiased" )
                                ]
                            ]
                , test "has 12.5px padding" <|
                    header
                        >> Query.has
                            [ style
                                [ ( "padding", "12.5px" ) ]
                            ]
                , test "text does not overflow or wrap" <|
                    header
                        >> Query.children []
                        >> Query.first
                        >> Query.has
                            [ style
                                [ ( "width", "245px" )
                                , ( "white-space", "nowrap" )
                                , ( "overflow", "hidden" )
                                , ( "text-overflow", "ellipsis" )
                                ]
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

                    isSolid : String -> Query.Single Msgs.Msg -> Expectation
                    isSolid color =
                        Query.has
                            [ style
                                [ ( "background-color", color ) ]
                            ]

                    isColorWithStripes : String -> String -> Query.Single Msgs.Msg -> Expectation
                    isColorWithStripes color stripeColor =
                        Query.has
                            [ style
                                [ ( "background-image"
                                  , "repeating-linear-gradient(-115deg,"
                                        ++ stripeColor
                                        ++ " 0,"
                                        ++ stripeColor
                                        ++ " 10px,"
                                        ++ color
                                        ++ " 0,"
                                        ++ color
                                        ++ " 16px)"
                                  )
                                , ( "background-size", "106px 114px" )
                                , ( "animation"
                                  , pipelineRunningKeyframes ++ " 3s linear infinite"
                                  )
                                ]
                            ]

                    isColorWithStripesHd : String -> String -> Query.Single Msgs.Msg -> Expectation
                    isColorWithStripesHd color stripeColor =
                        Query.has
                            [ style
                                [ ( "background-image"
                                  , "repeating-linear-gradient(-115deg,"
                                        ++ stripeColor
                                        ++ " 0,"
                                        ++ stripeColor
                                        ++ " 10px,"
                                        ++ color
                                        ++ " 0,"
                                        ++ color
                                        ++ " 16px)"
                                  )
                                , ( "background-size", "106px 114px" )
                                , ( "animation"
                                  , pipelineRunningKeyframes ++ " 3s linear infinite"
                                  )
                                ]
                            ]
                in
                [ describe "non-HD view"
                    [ test "is 7px tall" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> queryView
                                |> findBanner
                                |> Query.has [ style [ ( "height", "7px" ) ] ]
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
                                |> queryView
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
                                |> isColorWithStripes green darkGrey
                    , test "is grey when pipeline is pending" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> queryView
                                |> findBanner
                                |> isSolid lightGrey
                    , test "is grey with black stripes when pipeline is pending and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    Concourse.BuildStatusStarted
                                    True
                                |> findBanner
                                |> isColorWithStripes lightGrey darkGrey
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
                                |> isColorWithStripes red darkGrey
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
                                |> isColorWithStripes amber darkGrey
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
                                |> isColorWithStripes brown darkGrey
                    , describe "status priorities" <|
                        let
                            givenTwoJobs :
                                Concourse.BuildStatus
                                -> Concourse.BuildStatus
                                -> Query.Single Msgs.Msg
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
                                    |> queryView
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
                    , describe "HD view"
                        [ test "is 8px wide" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> givenDataUnauthenticated
                                        (oneTeamOnePipeline "team")
                                    |> queryView
                                    |> findBanner
                                    |> Query.has [ style [ ( "width", "8px" ) ] ]
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
                                    |> queryView
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
                                    |> isColorWithStripesHd green darkGrey
                        , test "is grey when pipeline is pending" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> givenDataUnauthenticated
                                        (oneTeamOnePipeline "team")
                                    |> queryView
                                    |> findBanner
                                    |> isSolid lightGrey
                        , test "is grey with black stripes when pipeline is pending and running" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusStarted
                                        True
                                    |> findBanner
                                    |> isColorWithStripesHd lightGrey darkGrey
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
                                    |> isColorWithStripesHd red darkGrey
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
                                    |> isColorWithStripesHd amber darkGrey
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
                                    |> isColorWithStripesHd brown darkGrey
                        , describe "status priorities" <|
                            let
                                givenTwoJobs :
                                    Concourse.BuildStatus
                                    -> Concourse.BuildStatus
                                    -> Query.Single Msgs.Msg
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
                                        |> queryView
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
                    setup : () -> Query.Single Msgs.Msg
                    setup _ =
                        whenOnDashboard { highDensity = True }
                            |> givenDataUnauthenticated
                                (oneTeamOnePipeline "team")
                            |> queryView

                    noPipelines : () -> Query.Single Msgs.Msg
                    noPipelines _ =
                        whenOnDashboard { highDensity = True }
                            |> givenDataUnauthenticated
                                (apiData
                                    [ ( "some-team", [] )
                                    , ( "other-team", [ "pipeline" ] )
                                    ]
                                )
                            |> queryView

                    card : Query.Single Msgs.Msg -> Query.Single Msgs.Msg
                    card =
                        Query.find
                            [ class "card"
                            , containing [ text "pipeline" ]
                            ]

                    cardText : Query.Single Msgs.Msg -> Query.Single Msgs.Msg
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
                            [ style
                                [ ( "font-size", "14px" )
                                , ( "letter-spacing", "1px" )
                                ]
                            ]
                , test "no pipelines card text is vertically centered" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "align-items", "center" )
                                ]
                            ]
                , test "no pipelines card is 60px tall" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has [ style [ ( "height", "60px" ) ] ]
                , test "no pipelines card has 60px right margin" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has [ style [ ( "margin-right", "60px" ) ] ]
                , test "no pipelines card text has 10px padding" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.children []
                        >> Query.index 1
                        >> Query.has [ style [ ( "padding", "10px" ) ] ]
                , test "no pipelines card is 200px wide" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has [ style [ ( "width", "200px" ) ] ]
                , test "no pipelines card has dark grey background" <|
                    noPipelines
                        >> noPipelinesCard
                        >> Query.has
                            [ style
                                [ ( "background-color", darkGrey ) ]
                            ]
                , test "card has larger tighter font" <|
                    setup
                        >> card
                        >> Query.has
                            [ style
                                [ ( "font-size", "19px" )
                                , ( "letter-spacing", "1px" )
                                ]
                            ]
                , test "card text does not overflow or wrap" <|
                    setup
                        >> cardText
                        >> Query.has
                            [ style
                                [ ( "width", "180px" )
                                , ( "white-space", "nowrap" )
                                , ( "overflow", "hidden" )
                                , ( "text-overflow", "ellipsis" )
                                ]
                            ]
                , test "card text is vertically centered" <|
                    setup
                        >> cardText
                        >> Query.has
                            [ style [ ( "align-self", "center" ) ] ]
                , test "card text has 10px padding" <|
                    setup
                        >> cardText
                        >> Query.has
                            [ style [ ( "padding", "10px" ) ] ]
                , test "card lays out contents horizontally" <|
                    setup
                        >> card
                        >> Query.has
                            [ style [ ( "display", "flex" ) ] ]
                , test "card is 60px tall" <|
                    setup
                        >> card
                        >> Query.has [ style [ ( "height", "60px" ) ] ]
                , test "card is 200px wide" <|
                    setup
                        >> card
                        >> Query.has [ style [ ( "width", "200px" ) ] ]
                , test "no triangle when there is no resource error" <|
                    setup
                        >> card
                        >> Query.children []
                        >> Query.count (Expect.equal 2)
                , describe "resource error triangle" <|
                    let
                        givenResourceError : () -> Query.Single Msgs.Msg
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
                                              }
                                            ]
                                        , version = ""
                                        , user = user
                                        }
                                    )
                                |> queryView

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
                                [ Query.has
                                    [ style
                                        [ ( "position"
                                          , "relative"
                                          )
                                        ]
                                    ]
                                , resourceErrorTriangle
                                    >> Query.has
                                        [ style
                                            [ ( "position"
                                              , "absolute"
                                              )
                                            , ( "top", "0" )
                                            , ( "right", "0" )
                                            ]
                                        ]
                                ]
                    , test "is an orange 'top right' triangle" <|
                        givenResourceError
                            >> card
                            >> resourceErrorTriangle
                            >> Query.has
                                [ style
                                    [ ( "width", "0" )
                                    , ( "height", "0" )
                                    , ( "border-top"
                                      , "30px solid " ++ orange
                                      )
                                    , ( "border-left"
                                      , "30px solid transparent"
                                      )
                                    ]
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
                            [ style [ ( "margin", "0 60px 4px 0" ) ] ]
                , test "card is faded green when pipeline is suceeding" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> pipelineWithStatus
                                Concourse.BuildStatusSucceeded
                                False
                            |> card
                            |> Query.has
                                [ style
                                    [ ( "background-color", fadedGreen )
                                    ]
                                ]
                , test "card is red when pipeline is failing" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> pipelineWithStatus
                                Concourse.BuildStatusFailed
                                False
                            |> card
                            |> Query.has
                                [ style
                                    [ ( "background-color", red )
                                    ]
                                ]
                , test "card is amber when pipeline is erroring" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> pipelineWithStatus
                                Concourse.BuildStatusErrored
                                False
                            |> card
                            |> Query.has
                                [ style
                                    [ ( "background-color", amber )
                                    ]
                                ]
                ]
            , describe "body"
                [ test "has dark grey background" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (oneTeamOnePipeline "team")
                            |> queryView
                            |> Query.find
                                [ class "card"
                                , containing [ text "pipeline" ]
                                ]
                            |> findBody
                            |> Query.has
                                [ style
                                    [ ( "background-color", darkGrey )
                                    ]
                                ]
                , test "has 2x margin above and below" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (oneTeamOnePipeline "team")
                            |> queryView
                            |> Query.find
                                [ class "card"
                                , containing [ text "pipeline" ]
                                ]
                            |> findBody
                            |> Query.has
                                [ style
                                    [ ( "margin", "2px 0" )
                                    ]
                                ]
                ]
            , describe "footer" <|
                let
                    hasStyle : List ( String, String ) -> Expectation
                    hasStyle styles =
                        whenOnDashboard { highDensity = False }
                            |> givenDataAndUser
                                (oneTeamOnePipeline "team")
                                (userWithRoles [ ( "team", [ "owner" ] ) ])
                            |> queryView
                            |> Query.find [ class "card-footer" ]
                            |> Query.has [ style styles ]
                in
                [ test "has dark grey background" <|
                    \_ ->
                        hasStyle [ ( "background-color", darkGrey ) ]
                , test "has medium padding" <|
                    \_ ->
                        hasStyle [ ( "padding", "13.5px" ) ]
                , test "lays out contents horizontally" <|
                    \_ ->
                        hasStyle [ ( "display", "flex" ) ]
                , test "is divided into a left and right section, spread apart" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataAndUser
                                (oneTeamOnePipeline "team")
                                (userWithRoles [ ( "team", [ "owner" ] ) ])
                            |> queryView
                            |> Query.find [ class "card-footer" ]
                            |> Expect.all
                                [ Query.children []
                                    >> Query.count (Expect.equal 2)
                                , Query.has
                                    [ style [ ( "justify-content", "space-between" ) ] ]
                                ]
                , test "both sections lay out contents horizontally" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataAndUser
                                (oneTeamOnePipeline "team")
                                (userWithRoles [ ( "team", [ "owner" ] ) ])
                            |> queryView
                            |> Query.find [ class "card-footer" ]
                            |> Query.children []
                            |> Query.each (Query.has [ style [ ( "display", "flex" ) ] ])
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
                                    |> queryView
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
                                            ++ [ style
                                                    [ ( "background-size", "contain" ) ]
                                               ]
                                        )
                        , test "status text is blue" <|
                            \_ ->
                                setup
                                    |> findStatusText
                                    |> Query.has
                                        [ style [ ( "color", blue ) ] ]
                        , test "status text is larger and spaced more widely" <|
                            \_ ->
                                setup
                                    |> findStatusText
                                    |> Query.has
                                        [ style
                                            [ ( "font-size", "18px" )
                                            , ( "line-height", "20px" )
                                            , ( "letter-spacing", "0.05em" )
                                            ]
                                        ]
                        , test "status text is offset to the right of the icon" <|
                            \_ ->
                                setup
                                    |> findStatusText
                                    |> Query.has
                                        [ style
                                            [ ( "margin-left", "8px" )
                                            ]
                                        ]
                        , test "status text says 'paused'" <|
                            \_ ->
                                setup
                                    |> findStatusText
                                    |> Query.has
                                        [ text "paused" ]
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
                                            ++ [ style [ ( "background-size", "contain" ) ] ]
                                        )
                        , test "status text is grey" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusPending
                                        False
                                    |> findStatusText
                                    |> Query.has
                                        [ style [ ( "color", lightGrey ) ] ]
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
                                            ++ [ style [ ( "background-size", "contain" ) ] ]
                                        )
                        , test "status text is green" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusSucceeded
                                        False
                                    |> findStatusText
                                    |> Query.has
                                        [ style [ ( "color", green ) ] ]
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
                                whenOnDashboard { highDensity = False }
                                    |> givenDataUnauthenticated
                                        (\u ->
                                            { teams =
                                                [ { id = 0, name = "team" } ]
                                            , pipelines =
                                                [ onePipeline "team" ]
                                            , jobs =
                                                [ jobWithNameTransitionedAt
                                                    "job"
                                                    (Just 0)
                                                    Concourse.BuildStatusSucceeded
                                                ]
                                            , resources = []
                                            , version = ""
                                            , user = u
                                            }
                                        )
                                    |> Dashboard.update (Msgs.ClockTick 1000)
                                    |> Tuple.first
                                    |> queryView
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
                                            ++ [ style [ ( "background-size", "contain" ) ] ]
                                        )
                        , test "status text is red" <|
                            \_ ->
                                whenOnDashboard { highDensity = False }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusFailed
                                        False
                                    |> findStatusText
                                    |> Query.has
                                        [ style [ ( "color", red ) ] ]
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
                                        ++ [ style [ ( "background-size", "contain" ) ] ]
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
                                        ++ [ style [ ( "background-size", "contain" ) ] ]
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
                                |> queryView
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
                                        ++ [ style [ ( "background-size", "contain" ) ] ]
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
                                |> queryView
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
                                        ++ [ style [ ( "background-size", "contain" ) ] ]
                                    )
                    , test "there is medium spacing between the eye and the play/pause button" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.children []
                                |> Query.index -1
                                |> Query.children []
                                |> Expect.all
                                    [ Query.count (Expect.equal 3)
                                    , Query.index 1 >> Query.has [ style [ ( "width", "13.5px" ) ] ]
                                    ]
                    , test "the right section has a 20px square pause button on the left" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> queryView
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
                                |> queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.find
                                    (iconSelector
                                        { size = "20px"
                                        , image = "ic-pause-white.svg"
                                        }
                                    )
                                |> Query.has [ style [ ( "cursor", "pointer" ) ] ]
                    , test "pause button is transparent" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.find
                                    (iconSelector
                                        { size = "20px"
                                        , image = "ic-pause-white.svg"
                                        }
                                    )
                                |> Query.has [ style [ ( "opacity", "0.5" ) ] ]
                    , defineHoverBehaviour
                        { name = "pause button"
                        , setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (oneTeamOnePipeline "team")
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                        , query =
                            Dashboard.view
                                >> HS.toUnstyled
                                >> Query.fromHtml
                                >> Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.index -1
                                >> Query.children []
                                >> Query.index 0
                        , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                        , unhoveredSelector =
                            { description = "a faded 20px square pause button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = "ic-pause-white.svg"
                                    }
                                    ++ [ style
                                            [ ( "cursor", "pointer" )
                                            , ( "opacity", "0.5" )
                                            ]
                                       ]
                            }
                        , mouseEnterMsg =
                            Msgs.PipelineButtonHover <|
                                Just
                                    { id = 0
                                    , name = "pipeline"
                                    , teamName = "team"
                                    , public = True
                                    , jobs = []
                                    , resourceError = False
                                    , status = PipelineStatus.PipelineStatusPending False
                                    }
                        , mouseLeaveMsg = Msgs.PipelineButtonHover Nothing
                        , hoveredSelector =
                            { description = "a bright 20px square pause button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = "ic-pause-white.svg"
                                    }
                                    ++ [ style
                                            [ ( "cursor", "pointer" )
                                            , ( "opacity", "1" )
                                            ]
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
                            Dashboard.view
                                >> HS.toUnstyled
                                >> Query.fromHtml
                                >> Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.index -1
                                >> Query.children []
                                >> Query.index 0
                        , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                        , unhoveredSelector =
                            { description = "a transparent 20px square play button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = "ic-play-white.svg"
                                    }
                                    ++ [ style
                                            [ ( "cursor", "pointer" )
                                            , ( "opacity", "0.5" )
                                            ]
                                       ]
                            }
                        , mouseEnterMsg =
                            Msgs.PipelineButtonHover <|
                                Just
                                    { id = 0
                                    , name = "pipeline"
                                    , teamName = "team"
                                    , public = True
                                    , jobs = []
                                    , resourceError = False
                                    , status = PipelineStatus.PipelineStatusPaused
                                    }
                        , mouseLeaveMsg = Msgs.PipelineButtonHover Nothing
                        , hoveredSelector =
                            { description = "an opaque 20px square play button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = "ic-play-white.svg"
                                    }
                                    ++ [ style
                                            [ ( "cursor", "pointer" )
                                            , ( "opacity", "1" )
                                            ]
                                       ]
                            }
                        }
                    ]
                ]
            ]
        , describe "bottom bar"
            [ test "appears by default" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> queryView
                        |> Query.has [ id "dashboard-info" ]
            , test "is 50px tall, almost black, fixed to the bottom of the viewport and covers entire width" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.has
                            [ style
                                [ ( "line-height", "35px" )
                                , ( "padding", "7.5px 30px" )
                                , ( "position", "fixed" )
                                , ( "bottom", "0" )
                                , ( "background-color", almostBlack )
                                , ( "width", "100%" )
                                , ( "box-sizing", "border-box" )
                                ]
                            ]
            , test "lays out contents horizontally, maximizing space between children" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "justify-content", "space-between" )
                                ]
                            ]
            , test "two children are legend and concourse-info" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.children []
                        |> Expect.all
                            [ Query.count (Expect.equal 2)
                            , Query.index 0 >> Query.has [ id "legend" ]
                            , Query.index 1 >> Query.has [ id "concourse-info" ]
                            ]
            , test "lays out children on two lines when view width is below 1230px" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> Dashboard.update (Msgs.ScreenResized { width = 1229, height = 300 })
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "dashboard-info" ]
                        |> Query.has
                            [ style
                                [ ( "flex-direction", "column" )
                                ]
                            ]
            , describe "legend"
                [ test "lays out contents horizontally" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> queryView
                            |> Query.find [ id "legend" ]
                            |> Query.has
                                [ style
                                    [ ( "display", "flex" )
                                    ]
                                ]
                , test "shows pipeline statuses" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> queryView
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
                                            >> Query.has
                                                [ style
                                                    [ ( "width", "10px" )
                                                    ]
                                                ]
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
                                            >> Query.has
                                                [ style
                                                    [ ( "width", "10px" )
                                                    ]
                                                ]
                                        , Query.index 2 >> Query.has [ text "paused" ]
                                        ]
                                ]
                , test "the legend separator is grey" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index -2
                            |> Query.has [ style [ ( "color", menuGrey ) ] ]
                , test "the legend separator centers contents vertically" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index -2
                            |> Query.has [ style [ ( "display", "flex" ), ( "align-items", "center" ) ] ]
                , test "the legend separator is gone when the window width is below 812px" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> Dashboard.update (Msgs.ScreenResized { width = 800, height = 300 })
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "legend" ]
                            |> Expect.all
                                [ Query.hasNot [ text "|" ]
                                , Query.children [] >> Query.count (Expect.equal 8)
                                ]
                , test "legend items wrap when window width is below 812px" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> Dashboard.update (Msgs.ScreenResized { width = 800, height = 300 })
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "legend" ]
                            |> Query.has
                                [ style [ ( "flex-wrap", "wrap" ) ]
                                ]
                , test "legend items lay out contents horizontally, centered vertically in grey caps" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index 0
                            |> Query.has
                                [ style
                                    [ ( "text-transform", "uppercase" )
                                    , ( "display", "flex" )
                                    , ( "align-items", "center" )
                                    , ( "color", menuGrey )
                                    ]
                                ]
                , test "legend items have 20px space between them" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.each
                                (Query.has
                                    [ style
                                        [ ( "margin-right", "20px" )
                                        ]
                                    ]
                                )
                , test "third legend item shows running indicator" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> queryView
                            |> Query.find [ id "legend" ]
                            |> Query.children []
                            |> Query.index 2
                            |> Expect.all
                                [ Query.has
                                    [ style
                                        [ ( "text-transform", "uppercase" )
                                        , ( "display", "flex" )
                                        ]
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
                                                [ style
                                                    [ ( "width", "10px" )
                                                    ]
                                                ]
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
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> queryView
                            |> findHDToggle
                in
                [ describe "on non-hd view"
                    [ test "lays out contents horizontally" <|
                        \_ ->
                            hdToggle
                                |> Query.has
                                    [ style
                                        [ ( "display", "flex" ) ]
                                    ]
                    , test "centers contents vertically" <|
                        \_ ->
                            hdToggle
                                |> Query.has
                                    [ style
                                        [ ( "align-items", "center" ) ]
                                    ]
                    , test "has a margin of 10px between the button and the label" <|
                        \_ ->
                            hdToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style [ ( "margin-right", "10px" ) ] ]
                    , test "displays the label using a grey color" <|
                        \_ ->
                            hdToggle
                                |> Query.has
                                    [ style
                                        [ ( "color", menuGrey ) ]
                                    ]
                    , test "label text is all caps" <|
                        \_ ->
                            hdToggle
                                |> Query.has
                                    [ style
                                        [ ( "text-transform", "uppercase" ) ]
                                    ]
                    , test "displays the off state" <|
                        \_ ->
                            hdToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style
                                        [ ( "background-image"
                                          , "url(/public/images/ic-hd-off.svg)"
                                          )
                                        , ( "background-size", "contain" )
                                        , ( "height", "20px" )
                                        , ( "width", "35px" )
                                        ]
                                    ]
                    , test "will not shrink on resizing" <|
                        \_ ->
                            hdToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style [ ( "flex-shrink", "0" ) ] ]
                    ]
                , describe "on HD view"
                    [ test "displays the on state" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (apiData
                                        [ ( "team", [ "pipeline" ] ) ]
                                    )
                                |> queryView
                                |> findHDToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style
                                        [ ( "background-image"
                                          , "url(/public/images/ic-hd-on.svg)"
                                          )
                                        , ( "background-size", "contain" )
                                        , ( "height", "20px" )
                                        , ( "width", "35px" )
                                        ]
                                    ]
                    , test "will not shrink on resizing" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (apiData
                                        [ ( "team", [ "pipeline" ] ) ]
                                    )
                                |> queryView
                                |> findHDToggle
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    [ style [ ( "flex-shrink", "0" ) ] ]
                    ]
                ]
            , describe "info section" <|
                let
                    info =
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                            |> queryView
                            |> Query.find [ id "concourse-info" ]
                in
                [ test "lays out contents horizontally" <|
                    \_ ->
                        info
                            |> Query.has
                                [ style
                                    [ ( "display", "flex" )
                                    ]
                                ]
                , test "displays info in a grey color" <|
                    \_ ->
                        info
                            |> Query.has [ style [ ( "color", menuGrey ) ] ]
                , test "displays text slightly larger" <|
                    \_ ->
                        info
                            |> Query.has [ style [ ( "font-size", "1.25em" ) ] ]
                , test "each info item is spaced out by 30px" <|
                    \_ ->
                        info
                            |> Query.children []
                            |> Query.each
                                (Query.has [ style [ ( "margin-right", "30px" ) ] ])
                , test "each info item centers contents vertically" <|
                    \_ ->
                        info
                            |> Query.children []
                            |> Query.each
                                (Query.has
                                    [ style
                                        [ ( "align-items", "center" )
                                        , ( "display", "flex" )
                                        ]
                                    ]
                                )
                , test "items in CLI section are 10 px apart" <|
                    \_ ->
                        info
                            |> Query.children []
                            |> Query.index -1
                            |> Query.children []
                            |> Query.each
                                (Query.has [ style [ ( "margin-right", "10px" ) ] ])
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
                                    (Query.has [ style [ ( "opacity", "0.5" ) ] ])
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
                        , query = queryView >> Query.find [ id "cli-osx" ]
                        , unhoveredSelector =
                            { description = "grey apple icon"
                            , selector =
                                [ style
                                    [ ( "opacity", "0.5" )
                                    , ( "background-size", "contain" )
                                    ]
                                ]
                                    ++ iconSelector
                                        { image = "apple-logo.svg"
                                        , size = "20px"
                                        }
                            }
                        , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                        , mouseEnterMsg = Msgs.CliHover <| Just Cli.OSX
                        , mouseLeaveMsg = Msgs.CliHover Nothing
                        , hoveredSelector =
                            { description = "white apple icon"
                            , selector =
                                [ style
                                    [ ( "opacity", "1" )
                                    , ( "background-size", "contain" )
                                    ]
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
                        , query = queryView >> Query.find [ id "cli-windows" ]
                        , unhoveredSelector =
                            { description = "grey windows icon"
                            , selector =
                                [ style
                                    [ ( "opacity", "0.5" )
                                    , ( "background-size", "contain" )
                                    ]
                                ]
                                    ++ iconSelector
                                        { image = "windows-logo.svg"
                                        , size = "20px"
                                        }
                            }
                        , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                        , mouseEnterMsg = Msgs.CliHover <| Just Cli.Windows
                        , mouseLeaveMsg = Msgs.CliHover Nothing
                        , hoveredSelector =
                            { description = "white windows icon"
                            , selector =
                                [ style
                                    [ ( "opacity", "1" )
                                    , ( "background-size", "contain" )
                                    ]
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
                        , query = queryView >> Query.find [ id "cli-linux" ]
                        , unhoveredSelector =
                            { description = "grey linux icon"
                            , selector =
                                [ style
                                    [ ( "opacity", "0.5" )
                                    , ( "background-size", "contain" )
                                    ]
                                ]
                                    ++ iconSelector
                                        { image = "linux-logo.svg"
                                        , size = "20px"
                                        }
                            }
                        , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                        , mouseEnterMsg = Msgs.CliHover <| Just Cli.Linux
                        , mouseLeaveMsg = Msgs.CliHover Nothing
                        , hoveredSelector =
                            { description = "white linux icon"
                            , selector =
                                [ style
                                    [ ( "opacity", "1" )
                                    , ( "background-size", "contain" )
                                    ]
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
                            |> queryView
                            |> Query.find [ id "concourse-info" ]
                            |> Query.has [ text "v1.2.3" ]
                ]
            , test "hides after 6 seconds" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                        |> Dashboard.update (Msgs.ClockTick 1000)
                        |> Tuple.first
                        |> Dashboard.update (Msgs.ClockTick 1000)
                        |> Tuple.first
                        |> Dashboard.update (Msgs.ClockTick 1000)
                        |> Tuple.first
                        |> Dashboard.update (Msgs.ClockTick 1000)
                        |> Tuple.first
                        |> Dashboard.update (Msgs.ClockTick 1000)
                        |> Tuple.first
                        |> Dashboard.update (Msgs.ClockTick 1000)
                        |> Tuple.first
                        |> queryView
                        |> Query.hasNot [ id "dashboard-info" ]
            ]
        ]


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
        , test ("mousing over " ++ name ++ " triggers " ++ toString mouseEnterMsg ++ " msg") <|
            \_ ->
                setup
                    |> query
                    |> Event.simulate Event.mouseEnter
                    |> Event.expect mouseEnterMsg
        , test
            (toString mouseEnterMsg
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
        , test ("mousing off " ++ name ++ " triggers " ++ toString mouseLeaveMsg ++ " msg") <|
            \_ ->
                setup
                    |> updateFunc mouseEnterMsg
                    |> query
                    |> Event.simulate Event.mouseLeave
                    |> Event.expect mouseLeaveMsg
        , test
            (toString mouseLeaveMsg
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
    [ style
        [ ( "background-image", "url(/public/images/" ++ image ++ ")" )
        , ( "background-position", "50% 50%" )
        , ( "background-repeat", "no-repeat" )
        , ( "width", size )
        , ( "height", size )
        ]
    ]


whenOnDashboard : { highDensity : Bool } -> Dashboard.Model
whenOnDashboard { highDensity } =
    Dashboard.init
        { title = always Cmd.none
        }
        { csrfToken = ""
        , turbulencePath = ""
        , search = ""
        , highDensity = highDensity
        , pipelineRunningKeyframes = pipelineRunningKeyframes
        }
        |> Tuple.first


queryView : Dashboard.Model -> Query.Single Msgs.Msg
queryView =
    Dashboard.view
        >> HS.toUnstyled
        >> Query.fromHtml


givenDataAndUser : (Maybe Concourse.User -> APIData.APIData) -> Concourse.User -> Dashboard.Model -> Dashboard.Model
givenDataAndUser data user =
    Dashboard.update
        (Msgs.APIDataFetched <|
            RemoteData.Success ( 0, data <| Just user )
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


givenDataUnauthenticated : (Maybe Concourse.User -> APIData.APIData) -> Dashboard.Model -> Dashboard.Model
givenDataUnauthenticated data =
    Dashboard.update
        (Msgs.APIDataFetched <|
            RemoteData.Success ( 0, data Nothing )
        )
        >> Tuple.first


givenPipelineWithJob : Maybe Concourse.User -> APIData.APIData
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


oneTeamOnePipelinePaused : String -> Maybe Concourse.User -> APIData.APIData
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


oneTeamOnePipelineNonPublic : String -> Maybe Concourse.User -> APIData.APIData
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


oneTeamOnePipeline : String -> Maybe Concourse.User -> APIData.APIData
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


apiData : List ( String, List String ) -> Maybe Concourse.User -> APIData.APIData
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
running job =
    { job
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
    jobWithNameTransitionedAt "other-job" <| Just 0


job : Concourse.BuildStatus -> Concourse.Job
job =
    jobWithNameTransitionedAt "job" <| Just 0


jobWithNameTransitionedAt : String -> Maybe Time -> Concourse.BuildStatus -> Concourse.Job
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
                        , finishedAt = Just <| Date.fromTime t
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


teamHeaderSelector : List Selector
teamHeaderSelector =
    [ class <| .sectionHeaderClass Group.stickyHeaderConfig ]


teamHeaderHasNoPill : String -> Query.Single Msgs.Msg -> Expectation
teamHeaderHasNoPill teamName =
    Query.find (teamHeaderSelector ++ [ containing [ text teamName ] ])
        >> Query.children []
        >> Query.count (Expect.equal 1)


teamHeaderHasPill : String -> String -> Query.Single Msgs.Msg -> Expectation
teamHeaderHasPill teamName pillText =
    Query.find (teamHeaderSelector ++ [ containing [ text teamName ] ])
        >> Query.children []
        >> Expect.all
            [ Query.count (Expect.equal 2)
            , Query.index 1 >> Query.has [ text pillText ]
            ]
