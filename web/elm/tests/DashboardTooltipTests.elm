module DashboardTooltipTests exposing (all)

import Dashboard.Dashboard as Dashboard
import Data
import Dict
import Expect
import HoverState exposing (HoverState(..))
import Html
import Message.Message exposing (DomID(..))
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
                                [ ( "team", [ Data.dashboardPipeline 0 True ] ) ]
                    }
                    { hovered =
                        Tooltip
                            (VisibilityButton
                                { teamName = Data.teamName
                                , pipelineName = Data.pipelineName
                                }
                            )
                            Data.elementPosition
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
                                [ ( "team", [ Data.dashboardPipeline 0 False ] ) ]
                    }
                    { hovered =
                        Tooltip
                            (VisibilityButton
                                { teamName = Data.teamName
                                , pipelineName = Data.pipelineName
                                }
                            )
                            Data.elementPosition
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "expose" ]
        , test "says 'disabled' when a pipeline with jobs disabled is hovered" <|
            \_ ->
                let
                    p =
                        Data.dashboardPipeline 0 True
                in
                Dashboard.tooltip
                    { pipelines =
                        Just <|
                            Dict.fromList
                                [ ( "team", [ { p | jobsDisabled = True } ] ) ]
                    }
                    { hovered =
                        Tooltip
                            (PipelineStatusIcon
                                { teamName = Data.teamName
                                , pipelineName = Data.pipelineName
                                }
                            )
                            Data.elementPosition
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "disabled" ]
        ]
