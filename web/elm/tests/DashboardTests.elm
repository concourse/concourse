module DashboardTests
    exposing
        ( all
        , apiData
        , darkGrey
        , defineHoverBehaviour
        , givenDataAndUser
        , givenDataUnauthenticated
        , iconSelector
        , white
        )

import Concourse
import Concourse.Cli as Cli
import Dashboard
import Dashboard.APIData as APIData
import Dashboard.Msgs as Msgs
import Date exposing (Date)
import Dict
import Expect exposing (Expectation)
import Dashboard.Group as Group
import Html.Attributes as Attr
import Html.Styled as HS
import Concourse.PipelineStatus as PipelineStatus
import RemoteData
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, class, containing, id, style, tag, text, Selector)
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
    "rgba(17, 197, 96, 0.2)"


pipelineRunningKeyframes : String
pipelineRunningKeyframes =
    "pipeline-running"


all : Test
all =
    describe "Dashboard"
        [ test "logging out causes pipeline list to reload" <|
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
                        [ attribute <| Attr.href "/teams/team/pipelines/pipeline/jobs/job/builds/1" ]
        , test "shows team name with no pill when unauthenticated and team has an exposed pipeline" <|
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
        , describe "pipeline cards" <|
            let
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
                [ describe "colored banner" <|
                    let
                        findBanner =
                            Query.find [ class "dashboard-pipeline", containing [ text "pipeline" ] ]
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
                                    , ( "animation"
                                      , pipelineRunningKeyframes ++ " 3s linear infinite"
                                      )
                                    , ( "background-size", "35px" )
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
                , describe "on HD view"
                    [ test "card lays out contents horizontally" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> queryView
                                |> Query.find
                                    [ class "dashboard-pipeline"
                                    , containing [ text "pipeline" ]
                                    ]
                                |> Query.has [ style [ ( "display", "flex" ) ] ]
                    , test "card is 60px tall" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> queryView
                                |> Query.find
                                    [ class "dashboard-pipeline"
                                    , containing [ text "pipeline" ]
                                    ]
                                |> Query.has [ style [ ( "height", "60px" ) ] ]
                    , test "card is 200px wide" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> queryView
                                |> Query.find
                                    [ class "dashboard-pipeline"
                                    , containing [ text "pipeline" ]
                                    ]
                                |> Query.has [ style [ ( "width", "200px" ) ] ]
                    , test "card is positioned relatively to anchor resource error triangle" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> queryView
                                |> Query.find
                                    [ class "dashboard-pipeline"
                                    , containing [ text "pipeline" ]
                                    ]
                                |> Query.has [ style [ ( "position", "relative" ) ] ]
                    , test "cards are spaced 4px apart vertically and 60px apart horizontally" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (oneTeamOnePipeline "team")
                                |> queryView
                                |> Query.find
                                    [ class "dashboard-pipeline"
                                    , containing [ text "pipeline" ]
                                    ]
                                |> Query.has [ style [ ( "margin", "0 60px 4px 0" ) ] ]
                    ]
                , describe "body"
                    [ describe "on HD view"
                        [ test "is faded green when pipeline is suceeding" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusSucceeded
                                        False
                                    |> Query.find
                                        [ class "dashboard-pipeline"
                                        , containing [ text "pipeline" ]
                                        ]
                                    |> Query.children []
                                    |> Query.index 1
                                    |> Query.has
                                        [ style
                                            [ ( "background-color", fadedGreen )
                                            ]
                                        ]
                        , test "is red when pipeline is failing" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusFailed
                                        False
                                    |> Query.find
                                        [ class "dashboard-pipeline"
                                        , containing [ text "pipeline" ]
                                        ]
                                    |> Query.children []
                                    |> Query.index 1
                                    |> Query.has
                                        [ style
                                            [ ( "background-color", red )
                                            ]
                                        ]
                        , test "is amber when pipeline is erroring" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> pipelineWithStatus
                                        Concourse.BuildStatusErrored
                                        False
                                    |> Query.find
                                        [ class "dashboard-pipeline"
                                        , containing [ text "pipeline" ]
                                        ]
                                    |> Query.children []
                                    |> Query.index 1
                                    |> Query.has
                                        [ style
                                            [ ( "background-color", amber )
                                            ]
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
                                |> Query.find [ class "dashboard-pipeline-footer" ]
                                |> Query.has [ style styles ]
                    in
                        [ test "there is a middle grey line dividing the footer from the rest of the card" <|
                            \_ ->
                                hasStyle [ ( "border-top", "2px solid " ++ middleGrey ) ]
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
                                    |> Query.find [ class "dashboard-pipeline-footer" ]
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
                                    |> Query.find [ class "dashboard-pipeline-footer" ]
                                    |> Query.children []
                                    |> Query.each (Query.has [ style [ ( "display", "flex" ) ] ])
                        , describe "left-hand section" <|
                            let
                                findStatusIcon =
                                    Query.find [ class "dashboard-pipeline-footer" ]
                                        >> Query.children []
                                        >> Query.first
                                        >> Query.children []
                                        >> Query.first

                                findStatusText =
                                    Query.find [ class "dashboard-pipeline-footer" ]
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
                                                            , image = "ic_pause_blue.svg"
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
                                                        , image = "ic_pending_grey.svg"
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
                                                        , image = "ic_running_green.svg"
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
                                                        , image = "ic_failing_red.svg"
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
                                                    , image = "ic_aborted_brown.svg"
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
                                                    , image = "ic_error_orange.svg"
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
                                        |> Query.find [ class "dashboard-pipeline-footer" ]
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
                                        |> Query.find [ class "dashboard-pipeline-footer" ]
                                        |> Query.children []
                                        |> Query.index -1
                                        |> Query.children []
                                        |> Query.index -1
                                        |> Query.has
                                            (iconSelector
                                                { size = "20px"
                                                , image = "baseline-visibility_off-24px.svg"
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
                                        |> Query.find [ class "dashboard-pipeline-footer" ]
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
                                        |> Query.find [ class "dashboard-pipeline-footer" ]
                                        |> Query.children []
                                        |> Query.index -1
                                        |> Query.children []
                                        |> Query.index 0
                                        |> Query.has
                                            (iconSelector
                                                { size = "20px"
                                                , image = "ic_pause_white.svg"
                                                }
                                            )
                            , test "pause button has pointer cursor" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> givenDataAndUser
                                            (oneTeamOnePipeline "team")
                                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                                        |> queryView
                                        |> Query.find [ class "dashboard-pipeline-footer" ]
                                        |> Query.find
                                            (iconSelector
                                                { size = "20px"
                                                , image = "ic_pause_white.svg"
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
                                        |> Query.find [ class "dashboard-pipeline-footer" ]
                                        |> Query.find
                                            (iconSelector
                                                { size = "20px"
                                                , image = "ic_pause_white.svg"
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
                                        >> Query.find [ class "dashboard-pipeline-footer" ]
                                        >> Query.children []
                                        >> Query.index -1
                                        >> Query.children []
                                        >> Query.index 0
                                , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                                , unhoveredSelector =
                                    { description = "a faded 20px square pause button with pointer cursor"
                                    , selector =
                                        (iconSelector
                                            { size = "20px"
                                            , image = "ic_pause_white.svg"
                                            }
                                            ++ [ style
                                                    [ ( "cursor", "pointer" )
                                                    , ( "opacity", "0.5" )
                                                    ]
                                               ]
                                        )
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
                                        (iconSelector
                                            { size = "20px"
                                            , image = "ic_pause_white.svg"
                                            }
                                            ++ [ style
                                                    [ ( "cursor", "pointer" )
                                                    , ( "opacity", "1" )
                                                    ]
                                               ]
                                        )
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
                                        >> Query.find [ class "dashboard-pipeline-footer" ]
                                        >> Query.children []
                                        >> Query.index -1
                                        >> Query.children []
                                        >> Query.index 0
                                , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                                , unhoveredSelector =
                                    { description = "a transparent 20px square play button with pointer cursor"
                                    , selector =
                                        (iconSelector
                                            { size = "20px"
                                            , image = "ic_play_white.svg"
                                            }
                                            ++ [ style
                                                    [ ( "cursor", "pointer" )
                                                    , ( "opacity", "0.5" )
                                                    ]
                                               ]
                                        )
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
                                        (iconSelector
                                            { size = "20px"
                                            , image = "ic_play_white.svg"
                                            }
                                            ++ [ style
                                                    [ ( "cursor", "pointer" )
                                                    , ( "opacity", "1" )
                                                    ]
                                               ]
                                        )
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
                                                    , image = "ic_pending_grey.svg"
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
                                                    , image = "ic_pause_blue.svg"
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
                                                    , image = "ic_running_legend.svg"
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
                                            [ ( "background-image", "url(public/images/ic_hd_off.svg)" )
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
                                    |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
                                    |> queryView
                                    |> findHDToggle
                                    |> Query.children []
                                    |> Query.index 0
                                    |> Query.has
                                        [ style
                                            [ ( "background-image", "url(public/images/ic_hd_on.svg)" )
                                            , ( "background-size", "contain" )
                                            , ( "height", "20px" )
                                            , ( "width", "35px" )
                                            ]
                                        ]
                        , test "will not shrink on resizing" <|
                            \_ ->
                                whenOnDashboard { highDensity = True }
                                    |> givenDataUnauthenticated (apiData [ ( "team", [ "pipeline" ] ) ])
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
                            [ test "font size is slightly larger" <|
                                \_ ->
                                    cliIcons
                                        |> Query.each
                                            (Query.has [ style [ ( "font-size", "1.2em" ) ] ])
                            , test "icons are grey" <|
                                \_ ->
                                    cliIcons
                                        |> Query.each
                                            (Query.has [ style [ ( "color", menuGrey ) ] ])
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
                                        [ style [ ( "color", menuGrey ) ]
                                        , containing [ tag "i", class "fa-apple" ]
                                        ]
                                    }
                                , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                                , mouseEnterMsg = Msgs.CliHover <| Just Cli.OSX
                                , mouseLeaveMsg = Msgs.CliHover Nothing
                                , hoveredSelector =
                                    { description = "white apple icon"
                                    , selector =
                                        [ style [ ( "color", white ) ]
                                        , containing [ tag "i", class "fa-apple" ]
                                        ]
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
                                        [ style [ ( "color", menuGrey ) ]
                                        , containing [ tag "i", class "fa-windows" ]
                                        ]
                                    }
                                , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                                , mouseEnterMsg = Msgs.CliHover <| Just Cli.Windows
                                , mouseLeaveMsg = Msgs.CliHover Nothing
                                , hoveredSelector =
                                    { description = "white windows icon"
                                    , selector =
                                        [ style [ ( "color", white ) ]
                                        , containing [ tag "i", class "fa-windows" ]
                                        ]
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
                                        [ style [ ( "color", menuGrey ) ]
                                        , containing [ tag "i", class "fa-linux" ]
                                        ]
                                    }
                                , updateFunc = \msg -> Dashboard.update msg >> Tuple.first
                                , mouseEnterMsg = Msgs.CliHover <| Just Cli.Linux
                                , mouseLeaveMsg = Msgs.CliHover Nothing
                                , hoveredSelector =
                                    { description = "white linux icon"
                                    , selector =
                                        [ style [ ( "color", white ) ]
                                        , containing [ tag "i", class "fa-linux" ]
                                        ]
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
        [ ( "background-image", "url(public/images/" ++ image ++ ")" )
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
