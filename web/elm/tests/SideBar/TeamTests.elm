module SideBar.TeamTests exposing (all)

import Common
import Html exposing (Html)
import Message.Message exposing (DomID(..), Message)
import SideBar.Team as Team
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


all : Test
all =
    describe "sidebar team"
        [ describe "when active"
            [ describe "when expanded"
                [ describe "when hovered"
                    [ test "arrow is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> arrow
                                |> Query.has [ style "opacity" "1" ]
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> teamName
                                |> Query.has [ style "opacity" "1" ]
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> teamIcon
                                |> Query.has [ style "opacity" "1" ]
                    ]
                , describe "when unhovered"
                    [ test "arrow is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = False
                                }
                                |> arrow
                                |> Query.has [ style "opacity" "1" ]
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = False
                                }
                                |> teamName
                                |> Query.has [ style "opacity" "1" ]
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = False
                                }
                                |> teamIcon
                                |> Query.has [ style "opacity" "1" ]
                    ]
                ]
            , describe "when collapsed"
                [ describe "when hovered"
                    [ test "arrow is greyed out" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = True
                                }
                                |> arrow
                                |> Query.has [ style "opacity" "0.5" ]
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = True
                                }
                                |> teamName
                                |> Query.has [ style "opacity" "1" ]
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = True
                                }
                                |> teamIcon
                                |> Query.has [ style "opacity" "1" ]
                    ]
                , describe "when unhovered"
                    [ test "arrow is dim" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = False
                                }
                                |> arrow
                                |> Query.has [ style "opacity" "0.2" ]
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = False
                                }
                                |> teamName
                                |> Query.has [ style "opacity" "1" ]
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = False
                                }
                                |> teamIcon
                                |> Query.has [ style "opacity" "1" ]
                    ]
                ]
            ]
        , describe "when inactive"
            [ describe "when expanded"
                [ describe "when hovered"
                    [ test "arrow is bright" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = True
                                }
                                |> arrow
                                |> Query.has [ style "opacity" "1" ]
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = True
                                }
                                |> teamName
                                |> Query.has [ style "opacity" "1" ]
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = True
                                }
                                |> teamIcon
                                |> Query.has [ style "opacity" "0.5" ]
                    ]
                , describe "when unhovered"
                    [ test "arrow is bright" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = False
                                }
                                |> arrow
                                |> Query.has [ style "opacity" "1" ]
                    , test "team name is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = False
                                }
                                |> teamName
                                |> Query.has [ style "opacity" "0.5" ]
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = False
                                }
                                |> teamIcon
                                |> Query.has [ style "opacity" "0.5" ]
                    ]
                ]
            , describe "when collapsed"
                [ describe "when hovered"
                    [ test "arrow is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = True
                                }
                                |> arrow
                                |> Query.has [ style "opacity" "0.5" ]
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = True
                                }
                                |> teamName
                                |> Query.has [ style "opacity" "1" ]
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = True
                                }
                                |> teamIcon
                                |> Query.has [ style "opacity" "0.5" ]
                    ]
                , describe "when unhovered"
                    [ test "arrow is dim" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = False
                                }
                                |> arrow
                                |> Query.has [ style "opacity" "0.2" ]
                    , test "team name is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = False
                                }
                                |> teamName
                                |> Query.has [ style "opacity" "0.5" ]
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = False
                                }
                                |> teamIcon
                                |> Query.has [ style "opacity" "0.2" ]
                    ]
                ]
            ]
        ]


team : { active : Bool, expanded : Bool, hovered : Bool } -> Html Message
team { active, expanded, hovered } =
    let
        hoveredDomId =
            if hovered then
                Just (SideBarTeam "team")

            else
                Nothing

        pipelines =
            [ { id = 1
              , name = "pipeline"
              , paused = False
              , public = True
              , teamName = "team"
              , groups = []
              }
            ]

        activePipeline =
            if active then
                Just { teamName = "team", pipelineName = "pipeline" }

            else
                Nothing
    in
    Team.team
        { hovered = hoveredDomId
        , isExpanded = expanded
        , teamName = "team"
        , pipelines = pipelines
        , currentPipeline = activePipeline
        }


teamIcon : Html Message -> Query.Single Message
teamIcon =
    Query.fromHtml
        >> Query.children []
        >> Query.first
        >> Query.children []
        >> Query.index 0


arrow : Html Message -> Query.Single Message
arrow =
    Query.fromHtml
        >> Query.children []
        >> Query.first
        >> Query.children []
        >> Query.index 1


teamName : Html Message -> Query.Single Message
teamName =
    Query.fromHtml
        >> Query.children []
        >> Query.first
        >> Query.children []
        >> Query.index 2
