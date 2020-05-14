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


defaultState =
    { active = False
    , expanded = False
    , hovered = False
    , hasFavorited = False
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
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , expanded = True
                                    , hovered = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
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
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , expanded = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
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
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                    , hovered = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
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
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | active = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
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
                    [ test "collapse icon is greyed out" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                    , hovered = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.GreyedOut
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                    , hovered = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                    , hovered = True
                                }
                                |> .icon
                                |> Expect.equal Styles.GreyedOut
                    ]
                , describe "when unhovered"
                    [ test "collapse icon is greyed out" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.GreyedOut
                    , test "team name is greyed out" <|
                        \_ ->
                            team
                                { defaultState
                                    | expanded = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.GreyedOut
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
                    [ test "collapse icon is dim" <|
                        \_ ->
                            team
                                { defaultState
                                    | hovered = True
                                }
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Dim
                    , test "team name is bright" <|
                        \_ ->
                            team
                                { defaultState
                                    | hovered = True
                                }
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.Bright
                    , test "team icon is greyed out" <|
                        \_ ->
                            team
                                { defaultState
                                    | hovered = True
                                }
                                |> .icon
                                |> Expect.equal Styles.GreyedOut
                    ]
                , describe "when unhovered"
                    [ test "collapse icon is dim" <|
                        \_ ->
                            team defaultState
                                |> .collapseIcon
                                |> .opacity
                                |> Expect.equal Styles.Dim
                    , test "team name is greyed out" <|
                        \_ ->
                            team defaultState
                                |> .name
                                |> .opacity
                                |> Expect.equal Styles.GreyedOut
                    , test "team icon is dim" <|
                        \_ ->
                            team defaultState
                                |> .icon
                                |> Expect.equal Styles.Dim
                    ]
                ]
            ]
        ]


team : { active : Bool, expanded : Bool, hovered : Bool, hasFavorited : Bool } -> Views.Team
team { active, expanded, hovered, hasFavorited } =
    let
        hoveredDomId =
            if hovered then
                HoverState.Hovered (SideBarTeam "team")

            else
                HoverState.NoHover

        pipelines =
            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]

        pipelineIdentifier =
            { teamName = "team", pipelineName = "pipeline" }

        activePipeline =
            if active then
                Just pipelineIdentifier

            else
                Nothing

        favoritedPipelines =
            if hasFavorited then
                [ pipelineIdentifier ]

            else
                []
    in
    Team.team
        { hovered = hoveredDomId
        , pipelines = pipelines
        , currentPipeline = activePipeline
        , favoritedPipelines = favoritedPipelines
        }
        { name = "team"
        , isExpanded = expanded
        }
