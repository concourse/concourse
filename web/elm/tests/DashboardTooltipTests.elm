module DashboardTooltipTests exposing (all)

import Application.Models exposing (Session)
import Common
import Concourse exposing (JsonValue(..))
import Dashboard.Dashboard as Dashboard
import Data
import Dict
import Expect
import HoverState exposing (HoverState(..))
import Html
import Message.Message exposing (DomID(..), PipelinesSection(..))
import RemoteData exposing (RemoteData(..))
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (text)


all : Test
all =
    describe "tooltip"
        [ test "says 'hide' when an exposed pipeline is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { pipelines =
                        Just <|
                            Dict.fromList
                                [ ( "team", [ Data.dashboardPipeline 1 True ] ) ]
                    }
                    { session
                        | hovered =
                            Tooltip
                                (VisibilityButton AllPipelinesSection Data.pipelineId)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "hide" ]
        , test "says 'expose' when a hidden pipeline is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { pipelines =
                        Just <|
                            Dict.fromList
                                [ ( "team", [ Data.dashboardPipeline 1 False ] ) ]
                    }
                    { session
                        | hovered =
                            Tooltip
                                (VisibilityButton AllPipelinesSection Data.pipelineId)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "expose" ]
        , test "says 'disabled' when a pipeline with jobs disabled is hovered" <|
            \_ ->
                let
                    p =
                        Data.dashboardPipeline 1 True
                in
                Dashboard.tooltip
                    { pipelines =
                        Just <|
                            Dict.fromList
                                [ ( "team", [ { p | jobsDisabled = True } ] ) ]
                    }
                    { session
                        | hovered =
                            Tooltip
                                (PipelineStatusIcon AllPipelinesSection Data.pipelineId)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "disabled" ]
        , test "displays job name when hovering over job preview" <|
            \_ ->
                Dashboard.tooltip
                    { pipelines = Nothing }
                    { session
                        | hovered =
                            Tooltip
                                (JobPreview AllPipelinesSection (Data.jobId |> Data.withJobName "my-job"))
                                Data.elementPosition
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "my-job" ]
        , test "displays hyphen-separated instance vars vals when hovering over pipeline preview" <|
            \_ ->
                let
                    instanceVars =
                        Dict.fromList
                            [ ( "a", JsonString "foo" )
                            , ( "b", JsonString "bar" )
                            , ( "c", JsonString "baz" )
                            ]

                    p =
                        Data.dashboardPipeline 1 True
                            |> Data.withInstanceVars instanceVars
                in
                Dashboard.tooltip
                    { pipelines = Just <| Dict.fromList [ ( "team", [ p ] ) ] }
                    { session
                        | hovered =
                            Tooltip
                                (PipelinePreview AllPipelinesSection 1)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withInstanceVars instanceVars ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "foo-bar-baz" ]
        , test "displays empty set when pipeline has no instance vars" <|
            \_ ->
                let
                    instanceVars =
                        Dict.empty

                    p =
                        Data.dashboardPipeline 1 True
                            |> Data.withInstanceVars instanceVars
                in
                Dashboard.tooltip
                    { pipelines = Just <| Dict.fromList [ ( "team", [ p ] ) ] }
                    { session
                        | hovered =
                            Tooltip
                                (PipelinePreview AllPipelinesSection 1)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withInstanceVars instanceVars ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "{}" ]
        , test "displays pipeline name when hovering over pipeline card" <|
            \_ ->
                let
                    p =
                        Data.dashboardPipeline 1 True
                            |> Data.withName "my-pipeline"
                in
                Dashboard.tooltip
                    { pipelines =
                        Just <|
                            Dict.fromList
                                [ ( "team", [ p ] ) ]
                    }
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardName AllPipelinesSection Data.pipelineId)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withName "my-pipeline" ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "my-pipeline" ]
        , test "displays pipeline name when hovering over pipeline card in HD view" <|
            \_ ->
                let
                    p =
                        Data.dashboardPipeline 1 True
                            |> Data.withName "my-pipeline"
                in
                Dashboard.tooltip
                    { pipelines =
                        Just <|
                            Dict.fromList
                                [ ( "team", [ p ] ) ]
                    }
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardNameHD Data.pipelineId)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withName "my-pipeline" ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "my-pipeline" ]
        , test "displays group name when hovering over instance group card" <|
            \_ ->
                Dashboard.tooltip
                    { pipelines = Nothing }
                    { session
                        | hovered =
                            Tooltip
                                (InstanceGroupCardName AllPipelinesSection "my-team" "my-group")
                                Data.elementPosition
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "my-group" ]
        , test "displays group name when hovering over instance group card in HD view" <|
            \_ ->
                Dashboard.tooltip
                    { pipelines = Nothing }
                    { session
                        | hovered =
                            Tooltip
                                (InstanceGroupCardNameHD "my-team" "my-group")
                                Data.elementPosition
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "my-group" ]
        , test "displays instance var value when hovering over pipeline card instance var" <|
            \_ ->
                Dashboard.tooltip
                    { pipelines = Nothing }
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardInstanceVar AllPipelinesSection 1 "foo.bar" "some-value")
                                Data.elementPosition
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "some-value" ]
        ]


session =
    Common.init "/" |> .session
