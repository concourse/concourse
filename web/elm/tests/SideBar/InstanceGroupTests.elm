module SideBar.InstanceGroupTests exposing (all)

import Assets
import Colors
import Common
import Concourse exposing (JsonValue(..))
import Data
import Dict
import Expect
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Message.Message exposing (DomID(..), Message, PipelinesSection(..))
import Set
import SideBar.InstanceGroup as InstanceGroup
import SideBar.Styles as Styles
import SideBar.Views as Views
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


defaultState =
    { active = False
    , hovered = False
    , favorited = False
    , section = AllPipelinesSection
    }


all : Test
all =
    describe "sidebar instance group"
        [ describe "when active"
            [ describe "when hovered"
                [ test "background is dark with bright border" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = True, hovered = True }
                            |> .background
                            |> Expect.equal Styles.Dark
                , test "badge is white" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = True, hovered = True }
                            |> .badge
                            |> Expect.equal
                                { count = 3
                                , color = Styles.White
                                }
                ]
            , describe "when not favorited"
                [ test "displays a bright unfilled star icon when hovered" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = True, hovered = True }
                            |> .starIcon
                            |> Expect.equal { filled = False, isBright = True }
                ]
            , describe "when favorited"
                [ test "displays a bright filled star icon" <|
                    \_ ->
                        viewInstanceGroup
                            { defaultState
                                | active = True
                                , hovered = True
                                , favorited = True
                            }
                            |> .starIcon
                            |> Expect.equal { filled = True, isBright = True }
                ]
            ]
        , describe "when unhovered"
            [ test "background is dark" <|
                \_ ->
                    viewInstanceGroup { defaultState | active = True, hovered = False }
                        |> .background
                        |> Expect.equal Styles.Dark
            , test "badge is bright" <|
                \_ ->
                    viewInstanceGroup { defaultState | active = True, hovered = False }
                        |> .badge
                        |> .color
                        |> Expect.equal Styles.LightGrey
            , describe "when unfavorited"
                [ test "displays a dim unfilled star icon" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = True, hovered = False }
                            |> .starIcon
                            |> Expect.equal { filled = False, isBright = True }
                ]
            , describe "when favorited"
                [ test "displays a bright filled star icon" <|
                    \_ ->
                        viewInstanceGroup
                            { defaultState
                                | active = True
                                , hovered = True
                                , favorited = True
                            }
                            |> .starIcon
                            |> Expect.equal { filled = True, isBright = True }
                ]
            ]
        , test "font weight is bold" <|
            \_ ->
                viewInstanceGroup { defaultState | active = True }
                    |> .name
                    |> .weight
                    |> Expect.equal Styles.Bold
        , describe "when inactive"
            [ describe "when hovered"
                [ test "background is light" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = False, hovered = True }
                            |> .background
                            |> Expect.equal Styles.Light
                , test "badge is white" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = False, hovered = True }
                            |> .badge
                            |> .color
                            |> Expect.equal Styles.White
                ]
            , describe "when unhovered"
                [ test "name is dim" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = False, hovered = False }
                            |> .name
                            |> .color
                            |> Expect.equal Styles.Grey
                , test "no background" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = False, hovered = False }
                            |> .background
                            |> Expect.equal Styles.Invisible
                , test "badge is dim" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = False, hovered = False }
                            |> .badge
                            |> .color
                            |> Expect.equal Styles.Grey
                ]
            , test "font weight is default" <|
                \_ ->
                    viewInstanceGroup { defaultState | active = False }
                        |> .name
                        |> .weight
                        |> Expect.equal Styles.Default
            ]
        , describe "when in all pipelines section"
            [ test "domID is for AllPipelines section" <|
                \_ ->
                    viewInstanceGroup { defaultState | section = AllPipelinesSection }
                        |> .domID
                        |> Expect.equal (SideBarInstanceGroup AllPipelinesSection "team" "group")
            ]
        , describe "when in favorites section"
            [ test "domID is for Favorites section" <|
                \_ ->
                    viewInstanceGroup { defaultState | section = FavoritesSection }
                        |> .domID
                        |> Expect.equal (SideBarInstanceGroup FavoritesSection "team" "group")
            ]
        , describe "when in recently viewed section"
            [ test "domID is for RecentlyViewed section" <|
                \_ ->
                    viewInstanceGroup { defaultState | section = RecentlyViewedSection }
                        |> .domID
                        |> Expect.equal (SideBarInstanceGroup RecentlyViewedSection "team" "group")
            ]
        ]


viewInstanceGroup :
    { active : Bool
    , hovered : Bool
    , favorited : Bool
    , section : PipelinesSection
    }
    -> Views.InstanceGroup
viewInstanceGroup { active, hovered, favorited, section } =
    let
        hoveredDomId =
            if hovered then
                HoverState.Hovered (SideBarInstanceGroup AllPipelinesSection "team" "group")

            else
                HoverState.NoHover

        activePipeline =
            if active then
                Just { pipelineName = "group", teamName = "team" }

            else
                Nothing

        favoritedInstanceGroups =
            if favorited then
                Set.singleton ( "team", "group" )

            else
                Set.empty
    in
    InstanceGroup.instanceGroup
        { hovered = hoveredDomId
        , currentPipeline = activePipeline
        , favoritedInstanceGroups = favoritedInstanceGroups
        , section = section
        }
        (pipeline 1)
        [ pipeline 2, pipeline 3 ]


pipeline id =
    Data.pipeline "team" id
        |> Data.withName "group"
        |> Data.withInstanceVars (Dict.fromList [ ( "version", JsonNumber <| toFloat id ) ])
