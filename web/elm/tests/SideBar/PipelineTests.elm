module SideBar.PipelineTests exposing (all)

import Colors
import Common
import Html exposing (Html)
import Message.Message exposing (DomID(..), Message)
import SideBar.Pipeline as Pipeline
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


all : Test
all =
    describe "sidebar pipeline"
        [ describe "when active"
            [ describe "when hovered"
                [ test "pipeline name background is dark" <|
                    \_ ->
                        pipeline { active = True, hovered = False }
                            |> pipelineName
                            |> Query.has
                                [ style "background-color" "#272727" ]
                , test "pipeline name border color is bright" <|
                    \_ ->
                        pipeline { active = True, hovered = False }
                            |> pipelineName
                            |> Query.has
                                [ style "border" <|
                                    "1px solid "
                                        ++ Colors.groupBorderSelected
                                ]
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> pipelineIcon
                            |> Query.has [ style "opacity" "1" ]
                ]
            , describe "when unhovered"
                [ test "pipeline name background is dark" <|
                    \_ ->
                        pipeline { active = True, hovered = False }
                            |> pipelineName
                            |> Query.has
                                [ style "background-color" "#272727" ]
                , test "pipeline name border color is bright" <|
                    \_ ->
                        pipeline { active = True, hovered = False }
                            |> pipelineName
                            |> Query.has
                                [ style "border" <|
                                    "1px solid "
                                        ++ Colors.groupBorderSelected
                                ]
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> pipelineIcon
                            |> Query.has [ style "opacity" "1" ]
                ]
            ]
        , describe "when inactive"
            [ describe "when hovered"
                [ test "pipeline name background is bright" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> pipelineName
                            |> Query.has
                                [ style "background-color" "#3A3A3A" ]
                , test "pipeline name border color is bright" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> pipelineName
                            |> Query.has
                                [ style "border" "1px solid #525151" ]
                , test "pipeline icon is bright" <|
                    \_ ->
                        pipeline { active = False, hovered = True }
                            |> pipelineIcon
                            |> Query.has [ style "opacity" "1" ]
                ]
            , describe "when unhovered"
                [ test "pipeline name background is greyed out" <|
                    \_ ->
                        pipeline { active = False, hovered = False }
                            |> pipelineName
                            |> Query.has [ style "opacity" "0.5" ]
                , test "pipeline name has invisible border" <|
                    \_ ->
                        pipeline { active = False, hovered = False }
                            |> pipelineName
                            |> Query.has [ style "border" <| "1px solid " ++ Colors.sideBar ]
                , test "pipeline icon is dim" <|
                    \_ ->
                        pipeline { active = False, hovered = False }
                            |> pipelineIcon
                            |> Query.has [ style "opacity" "0.2" ]
                ]
            ]
        ]


pipeline : { active : Bool, hovered : Bool } -> Html Message
pipeline { active, hovered } =
    let
        singlePipeline =
            { id = 1
            , name = "pipeline"
            , paused = False
            , public = True
            , teamName = "team"
            , groups = []
            }

        pipelineIdentifier =
            { teamName = "team"
            , pipelineName = "pipeline"
            }

        hoveredDomId =
            if hovered then
                Just (SideBarPipeline pipelineIdentifier)

            else
                Nothing

        activePipeline =
            if active then
                Just pipelineIdentifier

            else
                Nothing
    in
    Pipeline.pipeline
        { hovered = hoveredDomId
        , teamName = "team"
        , currentPipeline = activePipeline
        }
        singlePipeline


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
