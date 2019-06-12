module SideBar.TeamTests exposing (all)

import Common
import Expect
import HoverState
import Html exposing (Html)
import Message.Message exposing (DomID(..), Message)
import SideBar.Styles as Styles
import SideBar.Team as Team
import SideBar.Views as Views
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


all : Test
all =
    describe "sidebar team"
        [ describe "when active"
            [ describe "when expanded"
                [ describe "when hovered with tooltip"
                    [ test "team name has tooltip" <|
                        \_ ->
                            Team.team
                                { hovered =
                                    HoverState.Tooltip (SideBarTeam "team")
                                        { x = 0, y = 0 }
                                , pipelines =
                                    [ { id = 0
                                      , name = "pipeline"
                                      , paused = False
                                      , public = True
                                      , teamName = "team"
                                      , groups = []
                                      }
                                    ]
                                , currentPipeline =
                                    Just
                                        { teamName = "team"
                                        , pipelineName = "pipeline"
                                        }
                                }
                                { name = "team"
                                , isExpanded = True
                                }
                                |> .name
                                |> .tooltip
                                |> Expect.equal (Just { x = 0, y = 0 })
                    ]
                , describe "when hovered"
                    [ test "arrow is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> .arrow
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name has no tooltip" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> .name
                                |> .tooltip
                                |> Expect.equal Nothing
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name has light rectangle" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> .name
                                |> .rectangle
                                |> Expect.equal Styles.GreyWithLightBorder
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> .icon
                                |> Expect.equal Styles.Bright
                    ]
                , describe "when unhovered"
                    [ test "arrow is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = False
                                }
                                |> .arrow
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = False
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = False
                                }
                                |> .icon
                                |> Expect.equal Styles.Bright
                    , test "team name has no tooltip" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = False
                                }
                                |> .name
                                |> .tooltip
                                |> Expect.equal Nothing
                    ]
                ]
            , describe "when collapsed"
                [ describe "when hovered"
                    [ test "arrow is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = True
                                }
                                |> .arrow
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = True
                                }
                                |> .icon
                                |> Expect.equal Styles.Bright
                    , test "team name has no tooltip" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = True
                                }
                                |> .name
                                |> .tooltip
                                |> Expect.equal Nothing
                    ]
                , describe "when unhovered"
                    [ test "arrow is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = False
                                }
                                |> .arrow
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = False
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = False
                                }
                                |> .icon
                                |> Expect.equal Styles.Bright
                    , test "team name has no tooltip" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = False
                                }
                                |> .name
                                |> .tooltip
                                |> Expect.equal Nothing
                    ]
                ]
            ]
        , describe "when inactive"
            [ describe "when expanded"
                [ describe "when hovered"
                    [ test "arrow is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = True
                                }
                                |> .arrow
                                |> .opacity
                                |> Expect.equal Styles.GreyedOut
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = True
                                }
                                |> .icon
                                |> Expect.equal Styles.GreyedOut
                    , test "team name has no tooltip" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = True
                                }
                                |> .name
                                |> .tooltip
                                |> Expect.equal Nothing
                    ]
                , describe "when unhovered"
                    [ test "arrow is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = False
                                }
                                |> .arrow
                                |> .opacity
                                |> Expect.equal Styles.GreyedOut
                    , test "team name is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = False
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.GreyedOut
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = False
                                }
                                |> .icon
                                |> Expect.equal Styles.GreyedOut
                    , test "team name has no tooltip" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = False
                                }
                                |> .name
                                |> .tooltip
                                |> Expect.equal Nothing
                    ]
                ]
            , describe "when collapsed"
                [ describe "when hovered"
                    [ test "arrow is dim" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = True
                                }
                                |> .arrow
                                |> .opacity
                                |> Expect.equal Styles.Dim
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = True
                                }
                                |> .icon
                                |> Expect.equal Styles.GreyedOut
                    , test "team name has no tooltip" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = True
                                }
                                |> .name
                                |> .tooltip
                                |> Expect.equal Nothing
                    ]
                , describe "when unhovered"
                    [ test "arrow is dim" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = False
                                }
                                |> .arrow
                                |> .opacity
                                |> Expect.equal Styles.Dim
                    , test "team name is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = False
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.GreyedOut
                    , test "team icon is dim" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = False
                                }
                                |> .icon
                                |> Expect.equal Styles.Dim
                    , test "team name has no tooltip" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = False
                                }
                                |> .name
                                |> .tooltip
                                |> Expect.equal Nothing
                    ]
                ]
            ]
        ]


team : { active : Bool, expanded : Bool, hovered : Bool } -> Views.Team
team { active, expanded, hovered } =
    let
        hoveredDomId =
            if hovered then
                HoverState.Hovered (SideBarTeam "team")

            else
                HoverState.NoHover

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
        , pipelines = pipelines
        , currentPipeline = activePipeline
        }
        { name = "team"
        , isExpanded = expanded
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
