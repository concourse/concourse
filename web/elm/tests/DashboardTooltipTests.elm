module DashboardTooltipTests exposing (all)

import Application.Models exposing (Session)
import Common
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
        ]


session =
    Common.init "/" |> .session
