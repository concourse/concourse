module BuildTests exposing (..)

import Test exposing (..)
import Expect exposing (..)
import Build exposing (..)
import Concourse.Pagination
import Concourse
import Concourse.Build
import Dict
import RemoteData
import Navigation
import Routes


all : Test
all =
    describe "Builds"
        [ describe "update" <|
            let
                ( defaultModel, _ ) =
                    Build.init
                        { title = (\_ -> Cmd.none) }
                        { csrfToken = "", hash = "" }
                        (BuildPage 1)

                content =
                    [ { id = 15
                      , name = "7"
                      , job = Just { teamName = "main", pipelineName = "states", jobName = "passing" }
                      , status = Concourse.BuildStatusPending
                      , duration = { startedAt = Nothing, finishedAt = Nothing }
                      , reapTime = Nothing
                      }
                    ]

                pagination =
                    { previousPage = Nothing
                    , nextPage =
                        Just
                            { direction = Concourse.Pagination.Since 18
                            , limit = 100
                            }
                    }

                history =
                    { content = content
                    , pagination = pagination
                    }

                currentBuild =
                    RemoteData.Success
                        { build =
                            { id = 15
                            , name = "7"
                            , job = Just { teamName = "main", pipelineName = "states", jobName = "passing" }
                            , status = Concourse.BuildStatusPending
                            , duration = { startedAt = Nothing, finishedAt = Nothing }
                            , reapTime = Nothing
                            }
                        , prep =
                            Just
                                { pausedPipeline = Concourse.BuildPrepStatusNotBlocking
                                , pausedJob = Concourse.BuildPrepStatusNotBlocking
                                , maxRunningBuilds = Concourse.BuildPrepStatusNotBlocking
                                , inputs = Dict.fromList []
                                , inputsSatisfied = Concourse.BuildPrepStatusNotBlocking
                                , missingInputReasons = Dict.fromList []
                                }
                        , output = Nothing
                        }

                initModel =
                    { defaultModel | currentBuild = currentBuild }
            in
                [ test "BuildHistoryFetched" <|
                    \_ ->
                        let
                            expectedModel =
                                { initModel
                                    | history = history
                                }

                            ( actualModel, _ ) =
                                Build.update (BuildHistoryFetched (Ok history)) initModel
                        in
                            Expect.equal actualModel expectedModel
                , test "BuildTriggered" <|
                    \_ ->
                        let
                            build =
                                { id = 16
                                , name = "8"
                                , job = Just { teamName = "main", pipelineName = "states", jobName = "passing" }
                                , status = Concourse.BuildStatusPending
                                , duration = { startedAt = Nothing, finishedAt = Nothing }
                                , reapTime = Nothing
                                }

                            initialHistory =
                                { content = content
                                , pagination = { previousPage = Nothing, nextPage = Nothing }
                                }

                            newHistory =
                                { initialHistory
                                    | content = build :: content
                                    , pagination = { previousPage = Nothing, nextPage = Nothing }
                                }

                            theModel =
                                { initModel | history = initialHistory }

                            expectedModel =
                                { initModel | history = newHistory }

                            result =
                                Build.update (BuildTriggered (Ok build)) theModel
                        in
                            Expect.equal result ( expectedModel, Navigation.newUrl <| Routes.buildRoute build )
                ]
        ]
