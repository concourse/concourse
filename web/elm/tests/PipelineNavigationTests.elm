module PipelineNavigationTests exposing (all)

import Application.Application as Application
import Common
import Concourse
import Data
import Expect
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as Msgs
import Routes
import Test exposing (Test, describe, test)
import Time


pipelineA : Concourse.Pipeline
pipelineA =
    Data.pipeline "team" 1


pipelineB : Concourse.Pipeline
pipelineB =
    Data.pipeline "team" 2


idA : Concourse.PipelineIdentifier
idA =
    Concourse.toPipelineId pipelineA


idB : Concourse.PipelineIdentifier
idB =
    Concourse.toPipelineId pipelineB


{-| Land on pipeline A, then navigate to pipeline B, without any of A's
requests having resolved yet.
-}
navigatedFromAToB : Application.Model
navigatedFromAToB =
    Common.init "/teams/team/pipelines/pipeline-1"
        |> Application.handleDelivery
            (RouteChanged <| Routes.Pipeline { id = idB, groups = [] })
        |> Tuple.first


navigatedFromAToBInfo : Application.Model
navigatedFromAToBInfo =
    Common.init "/teams/team/pipelines/pipeline-1/info"
        |> Application.handleDelivery
            (RouteChanged <| Routes.PipelineInfo idB)
        |> Tuple.first


title : Application.Model -> String
title =
    Application.view >> .title


all : Test
all =
    describe "navigating between pipelines while requests are in flight"
        [ describe "pipeline page"
            [ test "a late response for the previous pipeline does not replace the current one" <|
                \_ ->
                    navigatedFromAToB
                        |> Application.handleCallback
                            (Callback.PipelineFetched idA (Ok pipelineA))
                        |> Tuple.first
                        |> title
                        |> Expect.equal "pipeline-2 - Concourse"
            , test "a late response for the previous pipeline does not fetch its jobs or resources" <|
                \_ ->
                    navigatedFromAToB
                        |> Application.handleCallback
                            (Callback.PipelineFetched idA (Ok pipelineA))
                        |> Tuple.second
                        |> Expect.all
                            [ Common.notContains (Effects.FetchJobs idA)
                            , Common.notContains (Effects.FetchResources idA)
                            ]
            , test "polling continues for the current pipeline after a late response" <|
                \_ ->
                    navigatedFromAToB
                        |> Application.handleCallback
                            (Callback.PipelineFetched idA (Ok pipelineA))
                        |> Tuple.first
                        |> Application.update
                            (Msgs.DeliveryReceived <|
                                ClockTicked FiveSeconds (Time.millisToPosix 0)
                            )
                        |> Tuple.second
                        |> Expect.all
                            [ Common.contains (Effects.FetchPipeline idB)
                            , Common.notContains (Effects.FetchPipeline idA)
                            ]
            , test "a late 404 for the previous pipeline does not mark the current one as missing" <|
                \_ ->
                    navigatedFromAToB
                        |> Application.handleCallback
                            (Callback.PipelineFetched idA Data.httpNotFound)
                        |> Tuple.first
                        |> title
                        |> Expect.equal "pipeline-2 - Concourse"
            , test "late jobs and resources for the previous pipeline are not rendered" <|
                \_ ->
                    navigatedFromAToB
                        |> Application.handleCallback
                            (Callback.JobsFetched idA (Ok []))
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.ResourcesFetched idA (Ok []))
                        |> Tuple.second
                        |> Common.notContains (Effects.RenderPipeline [] [])
            , test "jobs and resources for the current pipeline are rendered" <|
                \_ ->
                    navigatedFromAToB
                        |> Application.handleCallback
                            (Callback.JobsFetched idB (Ok []))
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.ResourcesFetched idB (Ok []))
                        |> Tuple.second
                        |> Common.contains (Effects.RenderPipeline [] [])
            , test "the current pipeline's response is trusted even if its name is normalised by the API" <|
                -- Routes can carry a URL-encoded name (e.g. "%3C") while the
                -- API returns the decoded one ("<"); the response must still
                -- be accepted and its locator adopted.
                \_ ->
                    let
                        encodedId =
                            { idB | pipelineName = "%3C" }

                        decoded =
                            pipelineB |> Data.withName "<"
                    in
                    Common.init "/teams/team/pipelines/pipeline-1"
                        |> Application.handleDelivery
                            (RouteChanged <| Routes.Pipeline { id = encodedId, groups = [] })
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.PipelineFetched encodedId (Ok decoded))
                        |> Expect.all
                            [ Tuple.first >> title >> Expect.equal "< - Concourse"
                            , Tuple.second
                                >> Common.contains
                                    (Effects.FetchJobs { idB | pipelineName = "<" })
                            ]
            ]
        , describe "pipeline info page"
            [ test "a late response for the previous pipeline does not replace the current one" <|
                \_ ->
                    navigatedFromAToBInfo
                        |> Application.handleCallback
                            (Callback.PipelineFetched idA (Ok pipelineA))
                        |> Tuple.first
                        |> title
                        |> Expect.equal "pipeline-2 - Concourse"
            ]
        ]
