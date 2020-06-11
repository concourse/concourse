module SideBar.PipelineTests exposing (all)

import Colors
import Common
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
                [ test "pipeline background is dark with bright border" <|
                    \_ ->
                        pipeline { active = True, hovered = True }
                            |> .background
                            |> Expect.equal Styles.Dark
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline { active = True, hovered = True }
                            |> .icon
                            |> Expect.equal Styles.Bright
                ]
            , describe "when unhovered"
                [ test "pipeline background is dark" <|
                    \_ ->
                        pipeline { active = True, hovered = False }
                            |> .background
                            |> Expect.equal Styles.Dark
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline { active = True, hovered = False }
                            |> .icon
                            |> Expect.equal Styles.Bright
                ]
            ]
        , describe "when inactive"
            [ describe "when hovered"
                [ test "pipeline background is light" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> .background
                            |> Expect.equal Styles.Light
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> .icon
                            |> Expect.equal Styles.Bright
                ]
            , describe "when unhovered"
                [ test "pipeline name is dim" <|
                    \_ ->
                        pipeline { active = False, hovered = False }
                            |> .name
                            |> .opacity
                            |> Expect.equal Styles.Dim
                , test "pipeline has no background" <|
                    \_ ->
                        pipeline { active = False, hovered = False }
                            |> .background
                            |> Expect.equal Styles.Invisible
                , test "pipeline icon is dim" <|
                    \_ ->
                        pipeline { active = False, hovered = False }
                            |> .icon
                            |> Expect.equal Styles.Dim
                ]
            ]
        ]


pipeline : { active : Bool, hovered : Bool } -> Views.Pipeline
pipeline { active, hovered } =
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
        singlePipeline


singlePipeline =
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
