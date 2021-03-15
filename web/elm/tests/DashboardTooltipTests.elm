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
import Set
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (text)


all : Test
all =
    describe "tooltip"
        [ test "says 'hide' when an exposed pipeline is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (VisibilityButton AllPipelinesSection 1)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withPublic True ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "hide" ]
        , test "says 'expose' when a hidden pipeline is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (VisibilityButton AllPipelinesSection 1)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withPublic False ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "expose" ]
        , test "says 'favorite' when an unfavorited pipeline is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardFavoritedIcon AllPipelinesSection 1)
                                Data.elementPosition
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "favorite" ]
        , test "says 'unfavorite' when a favorited pipeline is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardFavoritedIcon AllPipelinesSection 1)
                                Data.elementPosition
                        , favoritedPipelines = Set.singleton 1
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "unfavorite" ]
        , test "says 'favorite' when an unfavorited instance group is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (InstanceGroupCardFavoritedIcon AllPipelinesSection { teamName = "team", name = "group" })
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "favorite" ]
        , test "says 'unfavorite' when a favorited instance group is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (InstanceGroupCardFavoritedIcon AllPipelinesSection { teamName = "team", name = "group" })
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 ]
                        , favoritedInstanceGroups = Set.singleton ( "team", "group" )
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "unfavorite" ]
        , test "says 'pause' when an unpaused pipeline is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardPauseToggle AllPipelinesSection 1)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withPaused False ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "pause" ]
        , test "says 'unpause' when a paused pipeline is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardPauseToggle AllPipelinesSection 1)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withPaused True ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "unpause" ]
        , test "says 'disabled' when a pipeline with jobs disabled is hovered" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelineStatusIcon AllPipelinesSection 1)
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
                    { session
                        | hovered =
                            Tooltip
                                (JobPreview AllPipelinesSection 1 "my-job")
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
                in
                Dashboard.tooltip
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
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelinePreview AllPipelinesSection 1)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withInstanceVars Dict.empty ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "{}" ]
        , test "displays pipeline name when hovering over pipeline card" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardName AllPipelinesSection 1)
                                Data.elementPosition
                        , pipelines = Success [ Data.pipeline "team" 1 |> Data.withName "my-pipeline" ]
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "my-pipeline" ]
        , test "displays pipeline name when hovering over pipeline card in HD view" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardNameHD 1)
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
        , test "displays instance var key: value when hovering over pipeline card instance var" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardInstanceVar AllPipelinesSection 1 "foo.bar" "some-value")
                                Data.elementPosition
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "foo.bar: some-value" ]
        , test "displays instance var key: value when hovering over pipeline card instance vars in favorites" <|
            \_ ->
                Dashboard.tooltip
                    { session
                        | hovered =
                            Tooltip
                                (PipelineCardInstanceVars AllPipelinesSection 1 <|
                                    Dict.fromList
                                        [ ( "foo", JsonString "bar" )
                                        , ( "baz", JsonObject [ ( "qux", JsonNumber 123 ) ] )
                                        ]
                                )
                                Data.elementPosition
                    }
                    |> Maybe.map .body
                    |> Maybe.withDefault (Html.text "")
                    |> Query.fromHtml
                    |> Query.has [ text "baz.qux:123, foo:bar" ]
        ]


session =
    Common.init "/" |> .session
