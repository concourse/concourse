module SideBar.PipelineTests exposing (all)

import Assets
import Colors
import Common
import Concourse
import Data
import Expect
import HoverState exposing (TooltipPosition(..))
import Html exposing (Html)
import Message.Message exposing (DomID(..), Message)
import SideBar.Pipeline as Pipeline
import SideBar.Styles as Styles
import SideBar.Views as Views
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


all : Test
all =
    describe "sidebar pipeline"
        [ describe "when active"
            [ describe "when hovered"
                [ test "pipeline name background is dark with bright border" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { active = True, hovered = True }
                            |> .link
                            |> .rectangle
                            |> Expect.equal Styles.Dark
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { active = True, hovered = True }
                            |> .icon
                            |> Expect.equal
                                { asset = Assets.BreadcrumbIcon Assets.PipelineComponent
                                , opacity = Styles.Bright
                                }
                ]
            , describe "when unhovered"
                [ test "pipeline name background is dark with bright border" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { active = True, hovered = False }
                            |> .link
                            |> .rectangle
                            |> Expect.equal Styles.Dark
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { active = True, hovered = False }
                            |> .icon
                            |> Expect.equal
                                { asset = Assets.BreadcrumbIcon Assets.PipelineComponent
                                , opacity = Styles.Bright
                                }
                ]
            ]
        , describe "when inactive"
            [ describe "when hovered"
                [ test "pipeline name background is bright" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { active = False, hovered = True }
                            |> .link
                            |> .rectangle
                            |> Expect.equal Styles.Light
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { active = False, hovered = True }
                            |> .icon
                            |> Expect.equal
                                { asset = Assets.BreadcrumbIcon Assets.PipelineComponent
                                , opacity = Styles.Bright
                                }
                ]
            , describe "when unhovered"
                [ test "pipeline name is dim" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { active = False, hovered = False }
                            |> .link
                            |> .opacity
                            |> Expect.equal Styles.Dim
                , test "pipeline name has invisible border" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { active = False, hovered = False }
                            |> .link
                            |> .rectangle
                            |> Expect.equal Styles.PipelineInvisible
                , test "pipeline icon is dim" <|
                    \_ ->
                        pipeline
                            |> viewPipeline { active = False, hovered = False }
                            |> .icon
                            |> Expect.equal
                                { asset = Assets.BreadcrumbIcon Assets.PipelineComponent
                                , opacity = Styles.Dim
                                }
                ]
            ]
        , describe "when archived"
            [ test "pipeline icon is archived" <|
                \_ ->
                    pipeline
                        |> Data.withArchived True
                        |> viewPipeline { active = True, hovered = True }
                        |> .icon
                        |> .asset
                        |> Expect.equal Assets.ArchivedPipelineIcon
            ]
        ]


viewPipeline : { active : Bool, hovered : Bool } -> Concourse.Pipeline -> Views.Pipeline
viewPipeline { active, hovered } =
    let
        pipelineIdentifier =
            { teamName = "team"
            , pipelineName = "pipeline"
            }

        hoveredDomId =
            if hovered then
                HoverState.Hovered (SideBarPipeline pipelineIdentifier)

            else
                HoverState.NoHover

        activePipeline =
            if active then
                Just pipelineIdentifier

            else
                Nothing
    in
    Pipeline.pipeline
        { hovered = hoveredDomId
        , currentPipeline = activePipeline
        }


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
