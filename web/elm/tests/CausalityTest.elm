module CausalityTest exposing (all)

import Application.Application as Application
import Causality.Causality as Causality
import Common exposing (initCustomOpts)
import Concourse
    exposing
        ( Causality
        , CausalityBuild
        , CausalityDirection(..)
        , CausalityJob
        , CausalityResource
        , CausalityResourceVersion
        )
import Concourse.BuildStatus exposing (BuildStatus(..))
import Data exposing (featureFlags)
import Dict
import Expect
import Graph exposing (Edge, Node)
import List.Extra
import Message.Callback as Callback
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector
    exposing
        ( class
        , id
        , text
        )


all : Test
all =
    describe "causality graph" <|
        [ describe "viewing graph" <|
            [ test "shows not found if feature flag is disabled" <|
                \_ ->
                    initEnabled False
                        |> Common.queryView
                        |> Query.find [ class "notfound" ]
                        |> Query.has [ text "404" ]
            , test "shows not found if response is forbidden" <|
                \_ ->
                    init
                        |> Application.handleCallback
                            (Callback.CausalityFetched Data.httpForbidden)
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "notfound" ]
                        |> Query.has [ text "404" ]
            , test "shows error message if too large" <|
                \_ ->
                    init
                        |> Application.handleCallback
                            (Callback.CausalityFetched Data.httpUnproccessableEntity)
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "causality-error" ]
                        |> Query.has [ text "graph too large" ]
            , test "shows error message if there's no causality" <|
                \_ ->
                    init
                        |> Application.handleCallback
                            (Callback.CausalityFetched <|
                                Ok
                                    ( Downstream
                                    , Just { jobs = [], builds = [], resources = [], resourceVersions = [] }
                                    )
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "causality-error" ]
                        |> Query.has [ text "no causality" ]
            ]
        , describe "constructing downstream" <|
            [ test "simple graph with 1 build and output" <|
                \_ ->
                    Causality.constructGraph Downstream simplePipeline
                        |> Expect.equal
                            (Graph.fromNodesAndEdges
                                [ Node 1 <| Causality.Resource "r1" [ Causality.Version 1 someVersion ]
                                , Node -1 <| Causality.Job "j1" [ Causality.Build 1 "1" BuildStatusSucceeded ]
                                ]
                                [ Edge 1 -1 () ]
                            )
            , test "builds that fan out and then fan in" <|
                \_ ->
                    Causality.constructGraph Concourse.Downstream fanOutFanInPipeline
                        |> Expect.equal
                            (Graph.fromNodesAndEdges
                                [ Node 1 <| Causality.Resource "r1" [ Causality.Version 1 someVersion ]
                                , Node 2 <| Causality.Resource "r2" [ Causality.Version 2 someVersion ]
                                , Node 3 <| Causality.Resource "r3" [ Causality.Version 3 someVersion ]
                                , Node -1 <| Causality.Job "j1" [ Causality.Build 1 "1" BuildStatusSucceeded ]
                                , Node -2 <| Causality.Job "j2" [ Causality.Build 2 "1" BuildStatusSucceeded ]
                                , Node -3 <| Causality.Job "j3" [ Causality.Build 3 "1" BuildStatusSucceeded ]
                                ]
                                [ Edge 1 -1 ()
                                , Edge 1 -2 ()
                                , Edge -1 2 ()
                                , Edge -2 3 ()
                                , Edge 2 -3 ()
                                , Edge 3 -3 ()
                                ]
                            )
            , test "a resource and its descendent feeding into the same build" <|
                \_ ->
                    Causality.constructGraph Concourse.Downstream intermediateOutputsPipeline
                        |> Expect.equal
                            (Graph.fromNodesAndEdges
                                [ Node 1 <| Causality.Resource "r1" [ Causality.Version 1 someVersion ]
                                , Node 2 <| Causality.Resource "r2" [ Causality.Version 2 someVersion ]
                                , Node -1 <| Causality.Job "j1" [ Causality.Build 1 "1" BuildStatusSucceeded ]
                                , Node -2 <| Causality.Job "j2" [ Causality.Build 2 "1" BuildStatusSucceeded ]
                                ]
                                [ Edge 1 -1 ()
                                , Edge 1 -2 ()
                                , Edge -1 2 ()
                                , Edge 2 -2 ()
                                ]
                            )
            , test "multiple builds of the same job outputing different resource versions" <|
                \_ ->
                    Causality.constructGraph Concourse.Downstream singleJobMultipleBuildsPipeline
                        |> Expect.equal
                            (Graph.fromNodesAndEdges
                                [ Node 1 <| Causality.Resource "r1" [ Causality.Version 1 someVersion ]
                                , Node 2 <| Causality.Resource "r2" [ Causality.Version 2 someVersion ]
                                , Node -1 <|
                                    Causality.Job "j1"
                                        [ Causality.Build 2 "2" BuildStatusFailed
                                        , Causality.Build 1 "1" BuildStatusSucceeded
                                        ]
                                ]
                                [ Edge 1 -1 ()
                                , Edge -1 2 ()
                                ]
                            )
            ]

        -- basically the same as downstream, but the edges are flipped
        , describe "constructing upstream" <|
            [ test "simple graph with 1 build and input" <|
                \_ ->
                    Causality.constructGraph Upstream simplePipeline
                        |> Expect.equal
                            (Graph.fromNodesAndEdges
                                [ Node 1 <| Causality.Resource "r1" [ Causality.Version 1 someVersion ]
                                , Node -1 <| Causality.Job "j1" [ Causality.Build 1 "1" BuildStatusSucceeded ]
                                ]
                                [ Edge -1 1 () ]
                            )
            ]
        ]


someVersion : Concourse.Version
someVersion =
    Dict.fromList [ ( "v", "1" ) ]



-- figure out list of resources and jobs from passed in versions and builds


causality : List CausalityResourceVersion -> List CausalityBuild -> Causality
causality rvs builds =
    let
        jobs : List CausalityJob
        jobs =
            List.Extra.uniqueBy .jobId builds
                |> List.map (\b -> ( b.jobId, List.filter (\b2 -> b2.jobId == b.jobId) builds ))
                |> List.map (\( id, bs ) -> { id = id, name = "j" ++ String.fromInt id, buildIds = List.map .id bs })

        resources : List CausalityResource
        resources =
            List.Extra.uniqueBy .resourceId rvs
                |> List.map (\rv -> ( rv.resourceId, List.filter (\rv2 -> rv2.resourceId == rv.resourceId) rvs ))
                |> List.map (\( id, rs ) -> { id = id, name = "r" ++ String.fromInt id, resourceVersionIds = List.map .id rs })
    in
    { jobs = jobs
    , builds = builds
    , resources = resources
    , resourceVersions = rvs
    }



-- single resource feeding into single build
-- resource1 [
--   job1 build1
-- ]


simplePipeline : Causality
simplePipeline =
    causality
        [ CausalityResourceVersion 1 someVersion 1 [ 1 ] ]
        [ CausalityBuild 1 "1" 1 BuildStatusSucceeded [] ]



-- r1 fans out into j1 and j2, the outputs of which fans back into j3
-- resource1 [
--   job1 build1 [
--     resource2 [
--       job3 build1 []
--     ]
--   job2 build1 [
--     resource3 [
--       job3 build1 []
--     ]
--   ]
-- ]


fanOutFanInPipeline : Causality
fanOutFanInPipeline =
    causality
        [ CausalityResourceVersion 1 someVersion 1 [ 1, 2 ]
        , CausalityResourceVersion 2 someVersion 2 [ 3 ]
        , CausalityResourceVersion 3 someVersion 3 [ 3 ]
        ]
        [ CausalityBuild 1 "1" 1 BuildStatusSucceeded [ 2 ]
        , CausalityBuild 2 "1" 2 BuildStatusSucceeded [ 3 ]
        , CausalityBuild 3 "1" 3 BuildStatusSucceeded []
        ]



-- b2 uses both r1 and a downstream output of r1 as inputs
-- resource1 [
--   job1 build1 [
--     resource2 [
--       job2 build1 []
--     ]
--   ]
--   job2 build1 []
-- ]


intermediateOutputsPipeline : Causality
intermediateOutputsPipeline =
    causality
        [ CausalityResourceVersion 1 someVersion 1 [ 1, 2 ]
        , CausalityResourceVersion 2 someVersion 2 [ 2 ]
        ]
        [ CausalityBuild 1 "1" 1 BuildStatusSucceeded [ 2 ]
        , CausalityBuild 2 "1" 2 BuildStatusSucceeded []
        ]



-- j1 has 2 builds, one of the builds generated an output while the otherone failed
-- resource1 [
--   job1 build2 []
--   job1 build1 [resource2]
-- ]


singleJobMultipleBuildsPipeline : Causality
singleJobMultipleBuildsPipeline =
    causality
        [ CausalityResourceVersion 1 someVersion 1 [ 1, 2 ]
        , CausalityResourceVersion 2 someVersion 2 []
        ]
        [ CausalityBuild 1 "1" 1 BuildStatusSucceeded [ 2 ]
        , CausalityBuild 2 "2" 1 BuildStatusFailed []
        ]


resourceVersionId : Int
resourceVersionId =
    1


initEnabled : Bool -> Application.Model
initEnabled causalityEnabled =
    Common.initCustom { initCustomOpts | featureFlags = { featureFlags | resource_causality = causalityEnabled } }
        ("/teams/"
            ++ Data.teamName
            ++ "/pipelines/"
            ++ Data.pipelineName
            ++ "/resources/"
            ++ Data.resourceName
            ++ "/causality/"
            ++ String.fromInt resourceVersionId
            ++ "/downstream"
        )


init : Application.Model
init =
    initEnabled True
