module DashboardTests exposing (all)

import Concourse
import Dashboard
import Dict
import Expect
import Dashboard.Group as Group
import Html.Attributes as Attr
import Html.Styled as HS
import RemoteData
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, class, containing, style, tag, text)


all : Test
all =
    describe "Dashboard" <|
        let
            msg =
                Dashboard.APIDataFetched <|
                    RemoteData.Success
                        ( 0
                        , ( { teams = [ { id = 0, name = "team" } ]
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
                            , version = "0.0.0"
                            }
                          , Nothing
                          )
                        )
        in
            [ test "links to specific builds" <|
                \_ ->
                    Dashboard.init
                        { title = always Cmd.none
                        }
                        { csrfToken = ""
                        , turbulencePath = ""
                        , search = ""
                        , highDensity = False
                        }
                        |> Tuple.first
                        |> Dashboard.update msg
                        |> Tuple.first
                        |> Dashboard.view
                        |> HS.toUnstyled
                        |> Query.fromHtml
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
            , test "shows role pills on team headers" <|
                \_ ->
                    Dashboard.init
                        { title = always Cmd.none
                        }
                        { csrfToken = ""
                        , turbulencePath = ""
                        , search = ""
                        , highDensity = False
                        }
                        |> Tuple.first
                        |> Dashboard.update
                            (Dashboard.APIDataFetched <|
                                RemoteData.Success
                                    ( 0
                                    , ( { teams =
                                            [ { id = 0, name = "owner-team" }
                                            , { id = 1, name = "nonmember-team" }
                                            , { id = 2, name = "viewer-team" }
                                            , { id = 3, name = "member-team" }
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
                                        , resources = []
                                        , version = "0.0.0"
                                        }
                                      , Just
                                            { id = "0"
                                            , userName = "test"
                                            , name = "test"
                                            , email = "test"
                                            , teams =
                                                Dict.fromList
                                                    [ ( "owner-team", [ "owner", "viewer" ] )
                                                    , ( "member-team", [ "member" ] )
                                                    , ( "viewer-team", [ "viewer" ] )
                                                    ]
                                            }
                                      )
                                    )
                            )
                        |> Tuple.first
                        |> Dashboard.view
                        |> HS.toUnstyled
                        |> Query.fromHtml
                        |> Query.findAll [ class "dashboard-team-group" ]
                        |> Expect.all
                            [ Query.index 0
                                >> Expect.all
                                    [ Query.has [ text "owner-team" ]
                                    , Query.has [ text "OWNER" ]
                                    ]
                            , Query.index 1
                                >> Expect.all
                                    [ Query.has [ text "member-team" ]
                                    , Query.has [ text "MEMBER" ]
                                    ]
                            , Query.index 2
                                >> Expect.all
                                    [ Query.has [ text "viewer-team" ]
                                    , Query.has [ text "VIEWER" ]
                                    ]
                            , Query.index 3
                                >> Expect.all
                                    [ Query.has [ text "nonmember-team" ]
                                    , Query.find [ class <| .sectionHeaderClass Group.stickyHeaderConfig ]
                                        >> Query.children []
                                        >> Query.count (Expect.equal 1)
                                    ]
                            ]
            , test "team headers lay out contents horizontally, centering vertically" <|
                \_ ->
                    Dashboard.init
                        { title = always Cmd.none
                        }
                        { csrfToken = ""
                        , turbulencePath = ""
                        , search = ""
                        , highDensity = False
                        }
                        |> Tuple.first
                        |> Dashboard.update msg
                        |> Tuple.first
                        |> Dashboard.view
                        |> HS.toUnstyled
                        |> Query.fromHtml
                        |> Query.findAll [ class <| .sectionHeaderClass Group.stickyHeaderConfig ]
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
                    Dashboard.init
                        { title = always Cmd.none
                        }
                        { csrfToken = ""
                        , turbulencePath = ""
                        , search = ""
                        , highDensity = True
                        }
                        |> Tuple.first
                        |> Dashboard.update
                            (Dashboard.APIDataFetched <|
                                RemoteData.Success
                                    ( 0
                                    , ( { teams = [ { id = 0, name = "team" } ]
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
                                        , resources = []
                                        , version = "0.0.0"
                                        }
                                      , Just
                                            { id = "0"
                                            , userName = "test"
                                            , name = "test"
                                            , email = "test"
                                            , teams =
                                                Dict.fromList
                                                    [ ( "team", [ "owner" ] )
                                                    ]
                                            }
                                      )
                                    )
                            )
                        |> Tuple.first
                        |> Dashboard.view
                        |> HS.toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ class "dashboard-team-name-wrapper" ]
                        |> Query.find [ containing [ text "OWNER" ] ]
                        |> Query.has [ style [ ( "margin-bottom", "1em" ) ] ]
            , test "on non-HD view, the role pill on a group has no margin below" <|
                \_ ->
                    Dashboard.init
                        { title = always Cmd.none
                        }
                        { csrfToken = ""
                        , turbulencePath = ""
                        , search = ""
                        , highDensity = False
                        }
                        |> Tuple.first
                        |> Dashboard.update
                            (Dashboard.APIDataFetched <|
                                RemoteData.Success
                                    ( 0
                                    , ( { teams = [ { id = 0, name = "team" } ]
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
                                            []
                                        , resources = []
                                        , version = "0.0.0"
                                        }
                                      , Just
                                            { id = "0"
                                            , userName = "test"
                                            , name = "test"
                                            , email = "test"
                                            , teams =
                                                Dict.fromList
                                                    [ ( "team", [ "owner" ] )
                                                    ]
                                            }
                                      )
                                    )
                            )
                        |> Tuple.first
                        |> Dashboard.view
                        |> HS.toUnstyled
                        |> Query.fromHtml
                        |> Query.find [ class <| .sectionHeaderClass Group.stickyHeaderConfig ]
                        |> Query.find [ containing [ text "OWNER" ] ]
                        |> Query.has [ style [ ( "margin-bottom", "" ) ] ]
            ]
