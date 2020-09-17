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
    , isFavoritesSection = False
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
                , test "badge is bright" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = True, hovered = True }
                            |> .badge
                            |> Expect.equal
                                { count = 3
                                , opacity = Styles.Bright
                                }
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
                        |> .opacity
                        |> Expect.equal Styles.Bright
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
                , test "badge is bright" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = False, hovered = True }
                            |> .badge
                            |> .opacity
                            |> Expect.equal Styles.Bright
                ]
            , describe "when unhovered"
                [ test "name is dim" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = False, hovered = False }
                            |> .name
                            |> .opacity
                            |> Expect.equal Styles.Dim
                , test "no background" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = False, hovered = False }
                            |> .background
                            |> Expect.equal Styles.Invisible
                , test "badge is dim" <|
                    \_ ->
                        viewInstanceGroup { defaultState | active = False, hovered = False }
                            |> .badge
                            |> .opacity
                            |> Expect.equal Styles.Dim
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
                    viewInstanceGroup { defaultState | isFavoritesSection = False }
                        |> .domID
                        |> Expect.equal (SideBarInstanceGroup AllPipelinesSection "team" "group")
            ]
        , describe "when in favorites section"
            [ test "domID is for Favorites section" <|
                \_ ->
                    viewInstanceGroup { defaultState | isFavoritesSection = True }
                        |> .domID
                        |> Expect.equal (SideBarInstanceGroup FavoritesSection "team" "group")
            ]
        ]


viewInstanceGroup :
    { active : Bool
    , hovered : Bool
    , isFavoritesSection : Bool
    }
    -> Views.InstanceGroup
viewInstanceGroup { active, hovered, isFavoritesSection } =
    let
        hoveredDomId =
            if hovered then
                HoverState.Hovered (SideBarInstanceGroup AllPipelinesSection "team" "group")

            else
                HoverState.NoHover

        activePipeline =
            if active then
                Just { name = "group", teamName = "team" }

            else
                Nothing
    in
    InstanceGroup.instanceGroup
        { hovered = hoveredDomId
        , currentPipeline = activePipeline
        , isFavoritesSection = isFavoritesSection
        }
        (pipeline 1)
        [ pipeline 2, pipeline 3 ]


pipeline id =
    Data.pipeline "team" id
        |> Data.withName "group"
        |> Data.withInstanceVars (Dict.fromList [ ( "version", JsonNumber <| toFloat id ) ])
