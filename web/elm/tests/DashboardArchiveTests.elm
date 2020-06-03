module DashboardArchiveTests exposing (all)

import Application.Application as Application
import Common
import Data
import Message.Callback as Callback
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, containing, tag, text)


all : Test
all =
    describe "DashboardArchive"
        [ describe "toggle switch" <|
            let
                toggleSwitch =
                    [ tag "a"
                    , containing [ text "show archived" ]
                    ]
            in
            [ test "there is a 'show archived' toggle on the normal view" <|
                \_ ->
                    Common.init "/"
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <| Ok [Data.pipeline "team" 1])
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.has toggleSwitch
            , test "there is a 'show archived' toggle on the hd view" <|
                \_ ->
                    Common.init "/hd"
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <| Ok [Data.pipeline "team" 1])
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.has toggleSwitch
            , test "there is no toggle when there are no pipelines" <|
                \_ ->
                    Common.init "/"
                        |> Common.queryView
                        |> Query.hasNot toggleSwitch
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
