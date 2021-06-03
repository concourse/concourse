module CausalityTest exposing (all)

import Causality.Causality as Causality
import Concourse
    exposing
        ( CausalityBuild(..)
        , CausalityDirection(..)
        , CausalityResourceVersion
        )
import Concourse.BuildStatus
import Dict
import Expect
import Graph exposing (Edge, Node)
import Test exposing (..)


all : Test
all =
    describe "causality graph" <|
        [ describe "constructing downstream" <|
            [ test "simple graph with 1 build and output" <|
                \_ ->
                    Causality.constructGraph Downstream simplePipeline
                        |> Expect.equal
                            (Graph.fromNodesAndEdges
                                [ Node 1 <| Causality.Resource "resource" [ Causality.Version 1 someVersion ]
                                , Node -1 <| Causality.Job "job" [ Causality.Build 1 "1" Concourse.BuildStatus.BuildStatusSucceeded ]
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
                                , Node -1 <| Causality.Job "j1" [ Causality.Build 1 "1" Concourse.BuildStatus.BuildStatusSucceeded ]
                                , Node -2 <| Causality.Job "j2" [ Causality.Build 2 "1" Concourse.BuildStatus.BuildStatusSucceeded ]
                                , Node -3 <| Causality.Job "j3" [ Causality.Build 3 "1" Concourse.BuildStatus.BuildStatusSucceeded ]
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
                                , Node -1 <| Causality.Job "j1" [ Causality.Build 1 "1" Concourse.BuildStatus.BuildStatusSucceeded ]
                                , Node -2 <| Causality.Job "j2" [ Causality.Build 2 "1" Concourse.BuildStatus.BuildStatusSucceeded ]
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
                                , Node -1 <| Causality.Job "j1" [ Causality.Build 1 "1" Concourse.BuildStatus.BuildStatusSucceeded ]
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
                                [ Node 1 <| Causality.Resource "resource" [ Causality.Version 1 someVersion ]
                                , Node -1 <| Causality.Job "job" [ Causality.Build 1 "1" Concourse.BuildStatus.BuildStatusSucceeded ]
                                ]
                                [ Edge -1 1 () ]
                            )
            ]
        ]


someVersion : Concourse.Version
someVersion =
    Dict.fromList [ ( "v", "1" ) ]


resourceVersion : Int -> String -> List CausalityBuild -> CausalityResourceVersion
resourceVersion id name builds =
    { resourceId = id
    , versionId = id
    , resourceName = name
    , version = someVersion
    , builds = builds
    }


build : Int -> String -> List CausalityResourceVersion -> CausalityBuild
build id name rvs =
    CausalityBuildVariant
        { id = id
        , name = "1"
        , jobId = id
        , jobName = name
        , resourceVersions = rvs
        , status = Concourse.BuildStatus.BuildStatusSucceeded
        }



-- single resource feeding into single build


simplePipeline : CausalityResourceVersion
simplePipeline =
    resourceVersion 1 "resource" <|
        [ build 1 "job" []
        ]



--    r1 fans out into j1 and j2, the outputs of which fans back into j3


fanOutFanInPipeline : CausalityResourceVersion
fanOutFanInPipeline =
    resourceVersion 1 "r1" <|
        [ build 1 "j1" <|
            [ resourceVersion 2 "r2" <|
                [ build 3 "j3" [] ]
            ]
        , build 2 "j2" <|
            [ resourceVersion 3 "r3" <|
                [ build 3 "j3" [] ]
            ]
        ]



-- b2 uses both r1 and a downstream output of r1 as inputs


intermediateOutputsPipeline : CausalityResourceVersion
intermediateOutputsPipeline =
    resourceVersion 1 "r1" <|
        [ build 1 "j1" <|
            [ resourceVersion 2 "r2" <|
                [ build 2 "j2" [] ]
            ]
        , build 2 "j2" []
        ]


singleJobMultipleBuildsPipeline : CausalityResourceVersion
singleJobMultipleBuildsPipeline =
    resourceVersion 1 "r1" <|
        [ build 1 "j1" <|
            [ resourceVersion 2 "r2" []
            ]
        , build 1 "j1" []
        ]
