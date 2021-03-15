module SideBar.PipelineTests exposing (instancedPipeline, regularPipeline)

import Assets
import Colors
import Common
import Concourse
import Data
import Expect
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Message.Message exposing (DomID(..), Message, PipelinesSection(..))
import Set
import SideBar.Pipeline as Pipeline
import SideBar.Styles as Styles
import SideBar.Views as Views
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


defaultState =
    { active = False
    , hovered = False
    , favorited = False
    , isFavoritesSection = False
    }


regularPipeline : Test
regularPipeline =
    describe "sidebar pipeline"
        [ describe "when active"
            [ describe "when hovered"
                [ test "pipeline background is dark with bright border" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { defaultState | active = True, hovered = True }
                            |> .background
                            |> Expect.equal Styles.Dark
                , test "pipeline icon is white" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { defaultState | active = True, hovered = True }
                            |> .icon
                            |> Expect.equal (Views.AssetIcon Assets.PipelineIconWhite)
                , describe "when not favorited"
                    [ test "displays a bright unfilled star icon when hovered" <|
                        \_ ->
                            pipeline
                                |> viewPipeline { defaultState | active = True, hovered = True }
                                |> .starIcon
                                |> Expect.equal { filled = False, isBright = True }
                    ]
                , describe "when favorited"
                    [ test "displays a bright filled star icon" <|
                        \_ ->
                            pipeline
                                |> viewPipeline
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
                [ test "pipeline background is dark" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { defaultState | active = True, hovered = False }
                            |> .background
                            |> Expect.equal Styles.Dark
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { defaultState | active = True, hovered = False }
                            |> .icon
                            |> Expect.equal (Views.AssetIcon Assets.PipelineIconLightGrey)
                , describe "when unfavorited"
                    [ test "displays a dim unfilled star icon" <|
                        \_ ->
                            pipeline
                                |> viewPipeline { defaultState | active = True, hovered = False }
                                |> .starIcon
                                |> Expect.equal { filled = False, isBright = True }
                    ]
                , describe "when favorited"
                    [ test "displays a bright filled star icon" <|
                        \_ ->
                            pipeline
                                |> viewPipeline
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
                    pipeline
                        |> viewPipeline { defaultState | active = True }
                        |> .name
                        |> .weight
                        |> Expect.equal Styles.Bold
            ]
        , describe "when inactive"
            [ describe "when hovered"
                [ test "pipeline background is light" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { defaultState | active = False, hovered = True }
                            |> .background
                            |> Expect.equal Styles.Light
                , test "pipeline icon is white" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { defaultState | active = False, hovered = True }
                            |> .icon
                            |> Expect.equal (Views.AssetIcon Assets.PipelineIconWhite)
                ]
            , describe "when unhovered"
                [ test "pipeline name is grey" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { defaultState | active = False, hovered = False }
                            |> .name
                            |> .color
                            |> Expect.equal Styles.Grey
                , test "pipeline has no background" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { defaultState | active = False, hovered = False }
                            |> .background
                            |> Expect.equal Styles.Invisible
                , test "pipeline icon is dim" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { defaultState | active = False, hovered = False }
                            |> .icon
                            |> Expect.equal (Views.AssetIcon Assets.PipelineIconGrey)
                ]
            , test "font weight is default" <|
                \_ ->
                    pipeline
                        |> viewPipeline { defaultState | active = False }
                        |> .name
                        |> .weight
                        |> Expect.equal Styles.Default
            ]
        , describe "when archived"
            [ test "pipeline icon is archived" <|
                \_ ->
                    pipeline
                        |> Data.withArchived True
                        |> viewPipeline defaultState
                        |> .icon
                        |> Expect.equal (Views.AssetIcon Assets.ArchivedPipelineIcon)
            ]
        , describe "when in all pipelines section"
            [ test "domID is for AllPipelines section" <|
                \_ ->
                    pipeline
                        |> viewPipeline { defaultState | isFavoritesSection = False }
                        |> .domID
                        |> Expect.equal
                            (SideBarPipeline AllPipelinesSection pipeline.id)
            ]
        , describe "when in favorites section"
            [ test "domID is for Favorites section" <|
                \_ ->
                    pipeline
                        |> viewPipeline { defaultState | isFavoritesSection = True }
                        |> .domID
                        |> Expect.equal
                            (SideBarPipeline FavoritesSection pipeline.id)
            ]
        ]


instancedPipeline : Test
instancedPipeline =
    describe "sidebar instanced pipeline"
        [ test "icon is a slash" <|
            \_ ->
                pipeline
                    |> viewInstancedPipeline defaultState
                    |> .icon
                    |> Expect.equal (Views.TextIcon "/")
        , describe "when archived"
            [ test "pipeline icon is archived" <|
                \_ ->
                    pipeline
                        |> Data.withArchived True
                        |> viewInstancedPipeline defaultState
                        |> .icon
                        |> Expect.equal (Views.AssetIcon Assets.ArchivedPipelineIcon)
            ]
        , describe "when hovered"
            [ test "background is light" <|
                \_ ->
                    pipeline
                        |> viewPipeline { defaultState | hovered = True }
                        |> .background
                        |> Expect.equal Styles.Light
            ]
        ]


viewPipeline :
    { active : Bool
    , hovered : Bool
    , favorited : Bool
    , isFavoritesSection : Bool
    }
    -> Concourse.Pipeline
    -> Views.Pipeline
viewPipeline { active, hovered, favorited, isFavoritesSection } p =
    let
        hoveredDomId =
            if hovered then
                HoverState.Hovered (SideBarPipeline AllPipelinesSection p.id)

            else
                HoverState.NoHover

        activePipeline =
            if active then
                Just Data.pipelineId

            else
                Nothing

        favoritedPipelines =
            if favorited then
                Set.singleton p.id

            else
                Set.empty
    in
    Pipeline.regularPipeline
        { hovered = hoveredDomId
        , currentPipeline = activePipeline
        , favoritedPipelines = favoritedPipelines
        , isFavoritesSection = isFavoritesSection
        }
        p


viewInstancedPipeline :
    { active : Bool
    , hovered : Bool
    , favorited : Bool
    , isFavoritesSection : Bool
    }
    -> Concourse.Pipeline
    -> Views.Pipeline
viewInstancedPipeline { active, hovered, favorited, isFavoritesSection } p =
    let
        hoveredDomId =
            if hovered then
                HoverState.Hovered (SideBarInstancedPipeline AllPipelinesSection p.id)

            else
                HoverState.NoHover

        activePipeline =
            if active then
                Just Data.pipelineId

            else
                Nothing

        favoritedPipelines =
            if favorited then
                Set.singleton p.id

            else
                Set.empty
    in
    Pipeline.instancedPipeline
        { hovered = hoveredDomId
        , currentPipeline = activePipeline
        , favoritedPipelines = favoritedPipelines
        , isFavoritesSection = isFavoritesSection
        }
        p


pipeline =
    Data.pipeline "team" 0 |> Data.withName "pipeline"


pipelineIcon : Html Message -> Query.Single Message
pipelineIcon =
    Query.fromHtml
        >> Query.children []
        >> Query.index 0


pipelineName : Html Message -> Query.Single Message
pipelineName =
    Query.fromHtml
        >> Query.children []
        >> Query.index 1
