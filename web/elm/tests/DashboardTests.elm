module DashboardTests exposing (all)

import Concourse
import Dashboard
import Dict
import Expect exposing (Expectation)
import Dashboard.Group as Group
import Html.Attributes as Attr
import Html.Styled as HS
import RemoteData
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, class, containing, style, tag, text, Selector)


all : Test
all =
    describe "Dashboard"
        [ test "links to specific builds" <|
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
        , test
            ("on HD view, the role pill on a group has margin below, to create spacing "
                ++ "between the list of pipelines and the role pill"
            )
          <|
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


whenOnDashboard : { highDensity : Bool } -> Dashboard.Model
whenOnDashboard { highDensity } =
    Dashboard.init
        { title = always Cmd.none
        }
        { csrfToken = ""
        , turbulencePath = ""
        , search = ""
        , highDensity = highDensity
        }
        |> Tuple.first


queryView : Dashboard.Model -> Query.Single Dashboard.Msg
queryView =
    Dashboard.view
        >> HS.toUnstyled
        >> Query.fromHtml


givenDataAndUser : Group.APIData -> Concourse.User -> Dashboard.Model -> Dashboard.Model
givenDataAndUser data user =
    Dashboard.update
        (Dashboard.APIDataFetched <|
            RemoteData.Success ( 0, ( data, Just user ) )
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


givenDataUnauthenticated : Group.APIData -> Dashboard.Model -> Dashboard.Model
givenDataUnauthenticated data =
    Dashboard.update
        (Dashboard.APIDataFetched <|
            RemoteData.Success ( 0, ( data, Nothing ) )
        )
        >> Tuple.first


givenPipelineWithJob : Group.APIData
givenPipelineWithJob =
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
    }


oneTeamOnePipeline : String -> Group.APIData
oneTeamOnePipeline teamName =
    apiData [ ( teamName, [ "pipeline" ] ) ]


apiData : List ( String, List String ) -> Group.APIData
apiData pipelines =
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
    }


teamHeaderSelector : List Selector
teamHeaderSelector =
    [ class <| .sectionHeaderClass Group.stickyHeaderConfig ]


teamHeaderHasNoPill : String -> Query.Single Dashboard.Msg -> Expectation
teamHeaderHasNoPill teamName =
    Query.find (teamHeaderSelector ++ [ containing [ text teamName ] ])
        >> Query.children []
        >> Query.count (Expect.equal 1)


teamHeaderHasPill : String -> String -> Query.Single Dashboard.Msg -> Expectation
teamHeaderHasPill teamName pillText =
    Query.find (teamHeaderSelector ++ [ containing [ text teamName ] ])
        >> Query.children []
        >> Expect.all
            [ Query.count (Expect.equal 2)
            , Query.index 1 >> Query.has [ text pillText ]
            ]
