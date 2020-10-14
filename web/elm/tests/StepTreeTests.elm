module StepTreeTests exposing
    ( all
    , initAggregate
    , initAggregateNested
    , initEnsure
    , initGet
    , initInParallel
    , initInParallelNested
    , initOnFailure
    , initOnSuccess
    , initPut
    , initTask
    , initTimeout
    , initTry
    )

import Ansi.Log
import Array
import Build.StepTree.Models as Models
import Build.StepTree.StepTree as StepTree
import Concourse exposing (BuildStep(..), HookedPlan, JsonValue(..))
import Dict exposing (Dict)
import Expect exposing (..)
import Routes
import Test exposing (..)


all : Test
all =
    describe "StepTree"
        [ initTask
        , initSetPipeline
        , initLoadVar
        , initCheck
        , initGet
        , initPut
        , initAggregate
        , initAggregateNested
        , initAcross
        , initAcrossNested
        , initAcrossWithDo
        , initInParallel
        , initInParallelNested
        , initOnSuccess
        , initOnFailure
        , initEnsure
        , initTry
        , initTimeout
        ]


someStep : Routes.StepID -> Models.StepName -> Models.StepState -> Models.Step
someStep =
    someVersionedStep Nothing


someVersionedStep : Maybe Models.Version -> Routes.StepID -> Models.StepName -> Models.StepState -> Models.Step
someVersionedStep version id name state =
    { id = id
    , name = name
    , state = state
    , log = cookedLog
    , error = Nothing
    , expanded = False
    , version = version
    , metadata = []
    , changed = False
    , timestamps = Dict.empty
    , initialize = Nothing
    , start = Nothing
    , finish = Nothing
    , tabFocus = Models.Auto
    , expandedHeaders = Dict.empty
    , imageCheck = Nothing
    , imageGet = Nothing
    }


someExpandedStep : Routes.StepID -> Models.StepName -> Models.StepState -> Models.Step
someExpandedStep id name state =
    someStep id name state |> (\s -> { s | expanded = True })


emptyResources : Concourse.BuildResources
emptyResources =
    { inputs = [], outputs = [] }


initTask : Test
initTask =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepTask "some-name"
                }
    in
    describe "init with Task"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.Task "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps
                    [ ( "some-id", someStep "some-id" "some-name" Models.StepStatePending ) ]
                    steps
        ]


initSetPipeline : Test
initSetPipeline =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepSetPipeline "some-name"
                }
    in
    describe "init with SetPipeline"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.SetPipeline "some-id") tree
        , test "the steps" <|
            \_ ->
                assertSteps [ ( "some-id", someStep "some-id" "some-name" Models.StepStatePending ) ] steps
        ]


initLoadVar : Test
initLoadVar =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepLoadVar "some-name"
                }
    in
    describe "init with LoadVar"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.LoadVar "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps [ ( "some-id", someStep "some-id" "some-name" Models.StepStatePending ) ] steps
        ]


initCheck : Test
initCheck =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepCheck "some-name"
                }
    in
    describe "init with Check"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.Check "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps [ ( "some-id", someStep "some-id" "some-name" Models.StepStatePending ) ] steps
        ]


initGet : Test
initGet =
    let
        version =
            Dict.fromList [ ( "some", "version" ) ]

        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepGet "some-name" (Just version)
                }
    in
    describe "init with Get"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.Get "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps [ ( "some-id", someVersionedStep (Just version) "some-id" "some-name" Models.StepStatePending ) ] steps
        ]


initPut : Test
initPut =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepPut "some-name"
                }
    in
    describe "init with Put"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.Put "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps [ ( "some-id", someStep "some-id" "some-name" Models.StepStatePending ) ] steps
        ]


initAggregate : Test
initAggregate =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "aggregate-id"
                , step =
                    BuildStepAggregate
                        << Array.fromList
                    <|
                        [ { id = "task-a-id", step = BuildStepTask "task-a" }
                        , { id = "task-b-id", step = BuildStepTask "task-b" }
                        ]
                }
    in
    describe "init with Aggregate"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Aggregate
                        << Array.fromList
                     <|
                        [ Models.Task "task-a-id"
                        , Models.Task "task-b-id"
                        ]
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someStep "task-b-id" "task-b" Models.StepStatePending )
                    ]
                    steps
        ]


initAggregateNested : Test
initAggregateNested =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "aggregate-id"
                , step =
                    BuildStepAggregate
                        << Array.fromList
                    <|
                        [ { id = "task-a-id", step = BuildStepTask "task-a" }
                        , { id = "task-b-id", step = BuildStepTask "task-b" }
                        , { id = "nested-aggregate-id"
                          , step =
                                BuildStepAggregate
                                    << Array.fromList
                                <|
                                    [ { id = "task-c-id", step = BuildStepTask "task-c" }
                                    , { id = "task-d-id", step = BuildStepTask "task-d" }
                                    ]
                          }
                        ]
                }
    in
    describe "init with Aggregate nested"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Aggregate
                        << Array.fromList
                     <|
                        [ Models.Task "task-a-id"
                        , Models.Task "task-b-id"
                        , Models.Aggregate
                            << Array.fromList
                          <|
                            [ Models.Task "task-c-id"
                            , Models.Task "task-d-id"
                            ]
                        ]
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someStep "task-b-id" "task-b" Models.StepStatePending )
                    , ( "task-c-id", someStep "task-c-id" "task-c" Models.StepStatePending )
                    , ( "task-d-id", someStep "task-d-id" "task-d" Models.StepStatePending )
                    ]
                    steps
        ]


initAcross : Test
initAcross =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "across-id"
                , step =
                    BuildStepAcross
                        { vars = [ "var" ]
                        , steps =
                            [ ( [ JsonString "v1" ]
                              , { id = "task-a-id", step = BuildStepTask "task-a" }
                              )
                            , ( [ JsonString "v2" ]
                              , { id = "task-b-id", step = BuildStepTask "task-b" }
                              )
                            ]
                        }
                }
    in
    describe "init with Across"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Across "across-id"
                        [ "var" ]
                        [ [ JsonString "v1" ], [ JsonString "v2" ] ]
                        << Array.fromList
                     <|
                        [ Models.Task "task-a-id"
                        , Models.Task "task-b-id"
                        ]
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "across-id", someStep "across-id" "var" Models.StepStatePending )
                    , ( "task-a-id", someExpandedStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someExpandedStep "task-b-id" "task-b" Models.StepStatePending )
                    ]
                    steps
        ]


initAcrossNested : Test
initAcrossNested =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "across-id"
                , step =
                    BuildStepAcross
                        { vars = [ "var1" ]
                        , steps =
                            [ ( [ JsonString "a1" ]
                              , { id = "nested-across-id"
                                , step =
                                    BuildStepAcross
                                        { vars = [ "var2" ]
                                        , steps =
                                            [ ( [ JsonString "b1" ]
                                              , { id = "task-a-id", step = BuildStepTask "task-a" }
                                              )
                                            , ( [ JsonString "b2" ]
                                              , { id = "task-b-id", step = BuildStepTask "task-b" }
                                              )
                                            ]
                                        }
                                }
                              )
                            ]
                        }
                }
    in
    describe "init with nested Across"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Across "across-id"
                        [ "var1" ]
                        [ [ JsonString "a1" ] ]
                        << Array.fromList
                     <|
                        [ Models.Across "nested-across-id"
                            [ "var2" ]
                            [ [ JsonString "b1" ], [ JsonString "b2" ] ]
                            << Array.fromList
                          <|
                            [ Models.Task "task-a-id"
                            , Models.Task "task-b-id"
                            ]
                        ]
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "across-id", someStep "across-id" "var1" Models.StepStatePending )
                    , ( "nested-across-id", someExpandedStep "nested-across-id" "var2" Models.StepStatePending )
                    , ( "task-a-id", someExpandedStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someExpandedStep "task-b-id" "task-b" Models.StepStatePending )
                    ]
                    steps
        ]


initAcrossWithDo : Test
initAcrossWithDo =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "across-id"
                , step =
                    BuildStepAcross
                        { vars = [ "var" ]
                        , steps =
                            [ ( [ JsonString "v1" ]
                              , { id = "do-id"
                                , step =
                                    BuildStepDo <|
                                        Array.fromList
                                            [ { id = "task-a-id", step = BuildStepTask "task-a" }
                                            , { id = "task-b-id", step = BuildStepTask "task-b" }
                                            ]
                                }
                              )
                            ]
                        }
                }
    in
    describe "init Across with Do substep"
        [ test "does not expand substeps" <|
            \_ ->
                Expect.equal
                    (Models.Across "across-id"
                        [ "var" ]
                        [ [ JsonString "v1" ] ]
                        << Array.fromList
                     <|
                        [ Models.Do <|
                            Array.fromList
                                [ Models.Task "task-a-id"
                                , Models.Task "task-b-id"
                                ]
                        ]
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "across-id", someStep "across-id" "var" Models.StepStatePending )
                    , ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someStep "task-b-id" "task-b" Models.StepStatePending )
                    ]
                    steps
        ]


initInParallel : Test
initInParallel =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "parallel-id"
                , step =
                    BuildStepInParallel
                        << Array.fromList
                    <|
                        [ { id = "task-a-id", step = BuildStepTask "task-a" }
                        , { id = "task-b-id", step = BuildStepTask "task-b" }
                        ]
                }
    in
    describe "init with Parallel"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.InParallel
                        << Array.fromList
                     <|
                        [ Models.Task "task-a-id"
                        , Models.Task "task-b-id"
                        ]
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someStep "task-b-id" "task-b" Models.StepStatePending )
                    ]
                    steps
        ]


initInParallelNested : Test
initInParallelNested =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "parallel-id"
                , step =
                    BuildStepInParallel
                        << Array.fromList
                    <|
                        [ { id = "task-a-id", step = BuildStepTask "task-a" }
                        , { id = "task-b-id", step = BuildStepTask "task-b" }
                        , { id = "nested-parallel-id"
                          , step =
                                BuildStepInParallel
                                    << Array.fromList
                                <|
                                    [ { id = "task-c-id", step = BuildStepTask "task-c" }
                                    , { id = "task-d-id", step = BuildStepTask "task-d" }
                                    ]
                          }
                        ]
                }
    in
    describe "init with Parallel nested"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.InParallel
                        << Array.fromList
                     <|
                        [ Models.Task "task-a-id"
                        , Models.Task "task-b-id"
                        , Models.InParallel
                            << Array.fromList
                          <|
                            [ Models.Task "task-c-id"
                            , Models.Task "task-d-id"
                            ]
                        ]
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someStep "task-b-id" "task-b" Models.StepStatePending )
                    , ( "task-c-id", someStep "task-c-id" "task-c" Models.StepStatePending )
                    , ( "task-d-id", someStep "task-d-id" "task-d" Models.StepStatePending )
                    ]
                    steps
        ]


initOnSuccess : Test
initOnSuccess =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepOnSuccess <|
                        HookedPlan
                            { id = "task-a-id", step = BuildStepTask "task-a" }
                            { id = "task-b-id", step = BuildStepTask "task-b" }
                }
    in
    describe "init with OnSuccess"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.OnSuccess <|
                        Models.HookedStep
                            (Models.Task "task-a-id")
                            (Models.Task "task-b-id")
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someStep "task-b-id" "task-b" Models.StepStatePending )
                    ]
                    steps
        ]


initOnFailure : Test
initOnFailure =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepOnFailure <|
                        HookedPlan
                            { id = "task-a-id", step = BuildStepTask "task-a" }
                            { id = "task-b-id", step = BuildStepTask "task-b" }
                }
    in
    describe "init with OnFailure"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.OnFailure <|
                        Models.HookedStep
                            (Models.Task "task-a-id")
                            (Models.Task "task-b-id")
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someStep "task-b-id" "task-b" Models.StepStatePending )
                    ]
                    steps
        ]


initEnsure : Test
initEnsure =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepEnsure <|
                        HookedPlan
                            { id = "task-a-id", step = BuildStepTask "task-a" }
                            { id = "task-b-id", step = BuildStepTask "task-b" }
                }
    in
    describe "init with Ensure"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Ensure <|
                        Models.HookedStep
                            (Models.Task "task-a-id")
                            (Models.Task "task-b-id")
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    , ( "task-b-id", someStep "task-b-id" "task-b" Models.StepStatePending )
                    ]
                    steps
        ]


initTry : Test
initTry =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepTry { id = "task-a-id", step = BuildStepTask "task-a" }
                }
    in
    describe "init with Try"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Try <|
                        Models.Task "task-a-id"
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    ]
                    steps
        ]


initTimeout : Test
initTimeout =
    let
        { tree, steps } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepTimeout { id = "task-a-id", step = BuildStepTask "task-a" }
                }
    in
    describe "init with Timeout"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Timeout <|
                        Models.Task "task-a-id"
                    )
                    tree
        , test "the steps" <|
            \_ ->
                assertSteps
                    [ ( "task-a-id", someStep "task-a-id" "task-a" Models.StepStatePending )
                    ]
                    steps
        ]


assertSteps : List ( Routes.StepID, Models.Step ) -> Dict Routes.StepID Models.Step -> Expectation
assertSteps expected actual =
    Expect.equalDicts (Dict.fromList expected) actual


cookedLog : Ansi.Log.Model
cookedLog =
    Ansi.Log.init Ansi.Log.Cooked
