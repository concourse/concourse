module SideBar.TeamTests exposing (all)

import Common
import Data
import Expect
import HoverState exposing (TooltipPosition(..))
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
                [ describe "when hovered"
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Bright
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
                    , test "team has a light background" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = True
                                }
                                |> .background
                                |> Expect.equal Styles.Light
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
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = True
                                , hovered = False
                                }
                                |> .collapseIcon
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
                    ]
                ]
            , describe "when collapsed"
                [ describe "when hovered"
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = True
                                }
                                |> .collapseIcon
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
                    ]
                , describe "when unhovered"
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team
                                { active = True
                                , expanded = False
                                , hovered = False
                                }
                                |> .collapseIcon
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
                    ]
                ]
            ]
        , describe "when inactive"
            [ describe "when expanded"
                [ describe "when hovered"
                    [ test "collapse icon is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = True
                                }
                                |> .collapseIcon
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
                    ]
                , describe "when unhovered"
                    [ test "collapse icon is greyed out" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = True
                                , hovered = False
                                }
                                |> .collapseIcon
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
                    ]
                ]
            , describe "when collapsed"
                [ describe "when hovered"
                    [ test "collapse icon is dim" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = True
                                }
                                |> .collapseIcon
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
                    ]
                , describe "when unhovered"
                    [ test "collapse icon is dim" <|
                        \_ ->
                            team
                                { active = False
                                , expanded = False
                                , hovered = False
                                }
                                |> .collapseIcon
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
                    ]
                ]
            ]

        --  , describe "when favorited"
        --      [ test "shows up in a favorite section" <|
        --          \_ ->
        --                  givenPipelineFavorited
        --                  |>  Query.find [ id "side-bar" ]
        --                  |> Query.has [ text "favorites" ]
        --      ]
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
            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]

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
