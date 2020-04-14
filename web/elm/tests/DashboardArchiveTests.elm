module DashboardArchiveTests exposing (all)

import Application.Application as Application
import Common
import Data
import Message.Callback as Callback
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, containing, text)


all : Test
all =
    describe "DashboardArchive"
        [ test "archived pipelines are not rendered" <|
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
