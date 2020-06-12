module SideBar.ViewOptionTests exposing (all)

import Assets
import Colors
import Common
import Concourse
import Data
import Expect
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Message.Message exposing (DomID(..), Message)
import RemoteData
import SideBar.Styles as Styles
import SideBar.ViewOption
import SideBar.ViewOptionType exposing (ViewOption(..))
import SideBar.Views as Views
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


all : Test
all =
    describe "sidebar view option"
        [ describe "when active"
            [ describe "when hovered"
                [ test "view option background is dark" <|
                    \_ ->
                        viewOption { active = True, hovered = True, v = ViewNonArchivedPipelines }
                            |> .background
                            |> Expect.equal Styles.Dark
                , test "view option icon is bright" <|
                    \_ ->
                        viewOption { active = True, hovered = True, v = ViewNonArchivedPipelines }
                            |> .icon
                            |> .opacity
                            |> Expect.equal Styles.Bright
                ]
            , describe "when unhovered"
                [ test "view option background is dark" <|
                    \_ ->
                        viewOption { active = True, hovered = False, v = ViewNonArchivedPipelines }
                            |> .background
                            |> Expect.equal Styles.Dark
                , test "view option icon is bright" <|
                    \_ ->
                        viewOption { active = True, hovered = False, v = ViewNonArchivedPipelines }
                            |> .icon
                            |> .opacity
                            |> Expect.equal Styles.Bright
                ]
            ]
        , describe "when inactive"
            [ describe "when hovered"
                [ test "view option background is light" <|
                    \_ ->
                        viewOption { active = False, hovered = True, v = ViewNonArchivedPipelines }
                            |> .background
                            |> Expect.equal Styles.Light
                , test "view option icon is bright" <|
                    \_ ->
                        viewOption { active = False, hovered = True, v = ViewNonArchivedPipelines }
                            |> .icon
                            |> .opacity
                            |> Expect.equal Styles.Bright
                ]
            , describe "when unhovered"
                [ test "view option name is dim" <|
                    \_ ->
                        viewOption { active = False, hovered = False, v = ViewNonArchivedPipelines }
                            |> .name
                            |> .opacity
                            |> Expect.equal Styles.Dim
                , test "view option has no background" <|
                    \_ ->
                        viewOption { active = False, hovered = False, v = ViewNonArchivedPipelines }
                            |> .background
                            |> Expect.equal Styles.Invisible
                , test "view option icon is dim" <|
                    \_ ->
                        viewOption { active = False, hovered = False, v = ViewNonArchivedPipelines }
                            |> .icon
                            |> .opacity
                            |> Expect.equal Styles.Dim
                ]
            ]
        , describe "view option names"
            [ test "ViewNonArchivedPipelines" <|
                \_ ->
                    viewOption { active = False, hovered = False, v = ViewNonArchivedPipelines }
                        |> .name
                        |> .text
                        |> Expect.equal "Active/Paused"
            , test "ViewArchivedPipelines" <|
                \_ ->
                    viewOption { active = False, hovered = False, v = ViewArchivedPipelines }
                        |> .name
                        |> .text
                        |> Expect.equal "Archived"
            ]
        , describe "view option icons"
            [ test "ViewNonArchivedPipelines" <|
                \_ ->
                    viewOption { active = False, hovered = False, v = ViewNonArchivedPipelines }
                        |> .icon
                        |> .asset
                        |> Expect.equal (Assets.BreadcrumbIcon Assets.PipelineComponent)
            , test "ViewArchivedPipelines" <|
                \_ ->
                    viewOption { active = False, hovered = False, v = ViewArchivedPipelines }
                        |> .icon
                        |> .asset
                        |> Expect.equal Assets.ArchivedPipelineIcon
            ]
        , describe "filtering"
            [ test "ViewNonArchivedPipelines" <|
                \_ ->
                    viewOptionWithPipelines
                        [ Data.pipeline "team" 0
                        , Data.pipeline "team" 1
                        , Data.pipeline "team" 2
                        , Data.pipeline "team" 3 |> Data.withArchived True
                        , Data.pipeline "other-team" 4
                        , Data.pipeline "other-team" 5 |> Data.withArchived True
                        ]
                        { active = False
                        , hovered = False
                        , v = ViewNonArchivedPipelines
                        }
                        |> .numPipelines
                        |> Expect.equal 4
            , test "ViewArchivedPipelines" <|
                \_ ->
                    viewOptionWithPipelines
                        [ Data.pipeline "team" 0
                        , Data.pipeline "team" 1
                        , Data.pipeline "team" 2
                        , Data.pipeline "team" 3 |> Data.withArchived True
                        , Data.pipeline "other-team" 4
                        , Data.pipeline "other-team" 5 |> Data.withArchived True
                        ]
                        { active = False
                        , hovered = False
                        , v = ViewArchivedPipelines
                        }
                        |> .numPipelines
                        |> Expect.equal 2
            ]
        ]


viewOptionWithPipelines :
    List Concourse.Pipeline
    -> { active : Bool, hovered : Bool, v : ViewOption }
    -> Views.ViewOption
viewOptionWithPipelines pipelines { active, hovered, v } =
    let
        hoveredDomId =
            if hovered then
                HoverState.Hovered (SideBarViewOption v)

            else
                HoverState.NoHover

        activeViewOption =
            if active then
                v

            else if v == ViewNonArchivedPipelines then
                ViewArchivedPipelines

            else
                ViewNonArchivedPipelines
    in
    SideBar.ViewOption.viewOption
        { hovered = hoveredDomId
        , viewOption = activeViewOption
        , pipelines = RemoteData.Success pipelines
        }
        v


viewOption : { active : Bool, hovered : Bool, v : ViewOption } -> Views.ViewOption
viewOption =
    viewOptionWithPipelines []
