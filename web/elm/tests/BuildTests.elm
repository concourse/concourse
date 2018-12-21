module BuildTests exposing (all)

import Build
import Build.Effects as Effects
import Build.Msgs as Msgs
import Concourse
import Expect
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (id, style, text)


all : Test
all =
    describe "build page" <|
        let
            pageLoad =
                Build.init
                    { title = always Cmd.none
                    }
                    { csrfToken = ""
                    , hash = ""
                    }
                    (Build.JobBuildPage
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "job"
                        , buildName = "1"
                        }
                    )

            fetchBuild =
                Build.update
                    (Msgs.BuildFetched 1
                        (Ok
                            { id = 1
                            , name = "1"
                            , job =
                                Just
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    , jobName = "job"
                                    }
                            , status = Concourse.BuildStatusPending
                            , duration =
                                { startedAt = Nothing
                                , finishedAt = Nothing
                                }
                            , reapTime = Nothing
                            }
                        )
                    )
        in
        [ test "says loading on page load" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.has [ text "loading" ]
        , test "fetches build on page load" <|
            \_ ->
                pageLoad
                    |> Tuple.second
                    |> Expect.equal
                        [ Effects.FetchJobBuild 1
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = "job"
                            , buildName = "1"
                            }
                        , Effects.GetCurrentTime
                        ]
        , test "has a header after the build is fetched" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.has [ id "build-header" ]
        , test "header lays out horizontally" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find [ id "build-header" ]
                    |> Query.has
                        [ style [ ( "display", "flex" ) ] ]
        , test "header spreads out contents" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find [ id "build-header" ]
                    |> Query.has
                        [ style [ ( "justify-content", "space-between" ) ] ]
        ]
