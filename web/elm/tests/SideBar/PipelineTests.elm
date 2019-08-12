module SideBar.PipelineTests exposing (all)

import Colors
import Common
import Expect
import HoverState
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
            [ describe "when hovered with tooltip"
                [ test "pipeline name has tooltip" <|
                    \_ ->
                        Pipeline.pipeline
                            { hovered =
                                HoverState.Tooltip
                                    (SideBarPipeline
                                        { teamName = "team"
                                        , pipelineName = "pipeline"
                                        }
                                    )
                                    { left = 0
                                    , top = 0
                                    , arrowSize = 15
                                    , marginTop = -15
                                    }
                            , currentPipeline =
                                Just
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                            }
                            singlePipeline
                            |> .link
                            |> .tooltip
                            |> Expect.equal
                                (Just
                                    { left = 0
                                    , top = 0
                                    , arrowSize = 15
                                    , marginTop = -15
                                    }
                                )
                ]
            , describe "when hovered"
                [ test "pipeline name background is dark with bright border" <|
                    \_ ->
                        pipeline { active = True, hovered = True }
                            |> .link
                            |> .rectangle
                            |> Expect.equal Styles.Dark
                , test "pipeline name has no tooltip" <|
                    \_ ->
                        pipeline { active = True, hovered = True }
                            |> .link
                            |> .tooltip
                            |> Expect.equal Nothing
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline { active = True, hovered = True }
                            |> .icon
                            |> Expect.equal Styles.Bright
                ]
            , describe "when unhovered"
                [ test "pipeline name background is dark with bright border" <|
                    \_ ->
                        pipeline { active = True, hovered = False }
                            |> .link
                            |> .rectangle
                            |> Expect.equal Styles.Dark
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline { active = True, hovered = False }
                            |> .icon
                            |> Expect.equal Styles.Bright
                , test "pipeline name has no tooltip" <|
                    \_ ->
                        pipeline { active = True, hovered = False }
                            |> .link
                            |> .tooltip
                            |> Expect.equal Nothing
                ]
            ]
        , describe "when inactive"
            [ describe "when hovered"
                [ test "pipeline name background is bright" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> .link
                            |> .rectangle
                            |> Expect.equal Styles.Light
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> .icon
                            |> Expect.equal Styles.Bright
                , test "pipeline name has no tooltip" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> .link
                            |> .tooltip
                            |> Expect.equal Nothing
                ]
            , describe "when unhovered"
                [ test "pipeline name is dim" <|
                    \_ ->
                        pipeline { active = False, hovered = False }
                            |> .link
                            |> .opacity
                            |> Expect.equal Styles.Dim
                , test "pipeline name has invisible border" <|
                    \_ ->
                        pipeline { active = False, hovered = False }
                            |> .link
                            |> .rectangle
                            |> Expect.equal Styles.PipelineInvisible
                , test "pipeline icon is dim" <|
                    \_ ->
                        pipeline { active = False, hovered = False }
                            |> .icon
                            |> Expect.equal Styles.Dim
                , test "pipeline name has no tooltip" <|
                    \_ ->
                        pipeline
                            { active = False, hovered = False }
                            |> .link
                            |> .tooltip
                            |> Expect.equal Nothing
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
    { id = 1
    , name = "pipeline"
    , paused = False
    , public = True
    , teamName = "team"
    , groups = []
    }


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
