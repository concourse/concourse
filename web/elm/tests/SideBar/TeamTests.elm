module SideBar.TeamTests exposing (all)

import Common
import Concourse
import Data
import Dict
import Expect
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Message.Message exposing (DomID(..), Message, PipelinesSection(..))
import Set
import SideBar.Styles as Styles
import SideBar.Team as Team
import SideBar.Views as Views
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


defaultState =
    { active = False
    , expanded = False
    , hovered = False
    , hasFavorited = False
    , section = AllPipelinesSection
    }


all : Test
all =
    describe "sidebar team"
        [ describe "when active"
            [ describe "when expanded"
                [ describe "when hovered"
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , expanded = True
                                    , hovered = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is white" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , expanded = True
                                    , hovered = True
                                }
                                |> .name
                                |> .color
                                |> Expect.equal Styles.White
                    , test "team has a light background" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , expanded = True
                                    , hovered = True
                                }
                                |> .background
                                |> Expect.equal Styles.Light
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
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
                                { defaultState
                                    | active = True
                                    , expanded = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is white" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , expanded = True
                                }
                                |> .name
                                |> .color
                                |> Expect.equal Styles.White
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , expanded = True
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
                                { defaultState
                                    | active = True
                                    , hovered = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is white" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , hovered = True
                                }
                                |> .name
                                |> .color
                                |> Expect.equal Styles.White
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , hovered = True
                                }
                                |> .icon
                                |> Expect.equal Styles.Bright
                    ]
                , describe "when unhovered"
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is white" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                }
                                |> .name
                                |> .color
                                |> Expect.equal Styles.White
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                }
                                |> .icon
                                |> Expect.equal Styles.Bright
                    ]
                ]
            ]
        , describe "when inactive"
            [ describe "when expanded"
                [ describe "when hovered"
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                    , hovered = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is white" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                    , hovered = True
                                }
                                |> .name
                                |> .color
                                |> Expect.equal Styles.White
                    , test "team icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                    , hovered = True
                                }
                                |> .icon
                                |> Expect.equal Styles.Bright
                    ]
                , describe "when unhovered"
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is grey" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                }
                                |> .name
                                |> .color
                                |> Expect.equal Styles.LightGrey
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                }
                                |> .icon
                                |> Expect.equal Styles.GreyedOut
                    ]
                ]
            , describe "when collapsed"
                [ describe "when hovered"
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | hovered = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is white" <|
                        \_ ->
                            team
                                { defaultState
                                    | hovered = True
                                }
                                |> .name
                                |> .color
                                |> Expect.equal Styles.White
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { defaultState
                                    | hovered = True
                                }
                                |> .icon
                                |> Expect.equal Styles.Bright
                    ]
                , describe "when unhovered"
                    [ test "collapse icon is bright" <|
                        \_ ->
                            team defaultState
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team name is grey" <|
                        \_ ->
                            team defaultState
                                |> .name
                                |> .color
                                |> Expect.equal Styles.LightGrey
                    , test "team icon is greyed out" <|
                        \_ ->
                            team defaultState
                                |> .icon
                                |> Expect.equal Styles.GreyedOut
                    ]
                ]
            ]
        , describe "when in all pipelines section"
            [ test "domID is for AllPipelines section" <|
                \_ ->
                    team { defaultState | section = AllPipelinesSection }
                        |> .name
                        |> .domID
                        |> Expect.equal (SideBarTeam AllPipelinesSection "team")
            ]
        , describe "when in favorites section"
            [ test "domID is for Favorites section" <|
                \_ ->
                    team { defaultState | section = FavoritesSection }
                        |> .name
                        |> .domID
                        |> Expect.equal (SideBarTeam FavoritesSection "team")
            ]
        , describe "when in recently viewed section"
            [ test "domID is for RecentlyViewed section" <|
                \_ ->
                    team { defaultState | section = RecentlyViewedSection }
                        |> .name
                        |> .domID
                        |> Expect.equal (SideBarTeam RecentlyViewedSection "team")
            ]
        ]


team :
    { active : Bool
    , expanded : Bool
    , hovered : Bool
    , hasFavorited : Bool
    , section : PipelinesSection
    }
    -> Views.Team
team { active, expanded, hovered, hasFavorited, section } =
    let
        hoveredDomId =
            if hovered then
                HoverState.Hovered (SideBarTeam AllPipelinesSection "team")

            else
                HoverState.NoHover

        pipelines =
            [ Data.pipeline "team" 0 |> Data.withName "pipeline" |> Team.RegularPipeline ]

        pipelineIdentifier =
            { teamName = "team", pipelineName = "pipeline", pipelineInstanceVars = Dict.empty }

        activePipeline =
            if active then
                Just pipelineIdentifier

            else
                Nothing

        favoritedPipelines =
            if hasFavorited then
                Set.singleton 0

            else
                Set.empty
    in
    Team.team
        { hovered = hoveredDomId
        , pipelines = pipelines
        , currentPipeline = activePipeline
        , favoritedPipelines = favoritedPipelines
        , favoritedInstanceGroups = Set.empty
        , section = section
        }
        { name = "team"
        , isExpanded = expanded
        }
