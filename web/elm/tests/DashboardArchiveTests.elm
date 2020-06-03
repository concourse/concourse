module DashboardArchiveTests exposing (all)

import Application.Application as Application
import Colors
import Common
import Data
import Message.Callback as Callback
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, containing, style, tag, text)


all : Test
all =
    describe "DashboardArchive"
        [ describe "toggle switch" <|
            let
                toggleSwitch =
                    [ tag "a"
                    , containing [ text "show archived" ]
                    ]

                setup path =
                    Common.init path
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <| Ok [ Data.pipeline "team" 1 ])
                        |> Tuple.first
                        |> Common.queryView
            in
            [ test "exists on the normal view" <|
                \_ ->
                    setup "/"
                        |> Query.has toggleSwitch
            , test "exists on the hd view" <|
                \_ ->
                    setup "/hd"
                        |> Query.has toggleSwitch
            , test "does not exist when there are no pipelines" <|
                \_ ->
                    Common.init "/"
                        |> Common.queryView
                        |> Query.hasNot toggleSwitch
            , test "renders label to the left of the button" <|
                \_ ->
                    setup "/"
                        |> Query.find toggleSwitch
                        |> Query.has [ style "flex-direction" "row-reverse" ]
            , test "has a margin between the button and the label" <|
                \_ ->
                    setup "/"
                        |> Query.find toggleSwitch
                        |> Query.children []
                        |> Query.index 0
                        |> Query.has [ style "margin-left" "10px" ]
            , test "has a margin to the right of the toggle" <|
                \_ ->
                    setup "/"
                        |> Query.find toggleSwitch
                        |> Query.has [ style "margin-right" "10px" ]
            , test "has an offset left border" <|
                \_ ->
                    setup "/"
                        |> Query.find toggleSwitch
                        |> Query.has
                            [ style "border-left" <| "1px solid " ++ Colors.background
                            , style "padding-left" "10px"
                            ]
            ]
        , test "archived pipelines are not rendered" <|
            \_ ->
                Common.init "/"
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 1
                                    |> Data.withName "archived-pipeline"
                                    |> Data.withArchived True
                                ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.hasNot [ class "pipeline-wrapper", containing [ text "archived-pipeline" ] ]
        ]
