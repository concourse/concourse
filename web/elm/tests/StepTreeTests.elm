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
import Dict
import Expect exposing (..)
import Routes
import Test exposing (..)


all : Test
all =
    describe "StepTree"
        [ initTask
        , initSetPipeline
        , initLoadVar
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
        { tree, foci } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepTask "some-name"
                }
    in
    describe "init with Task"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Task (someStep "some-id" "some-name" Models.StepStatePending))
                    tree
        , test "using the focus" <|
            \_ ->
                assertFocus "some-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Task (someStep "some-id" "some-name" Models.StepStateSucceeded))
        ]


initSetPipeline : Test
initSetPipeline =
    let
        { tree, foci } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepSetPipeline "some-name"
                }
    in
    describe "init with SetPipeline"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.SetPipeline (someStep "some-id" "some-name" Models.StepStatePending))
                    tree
        , test "using the focus" <|
            \_ ->
                assertFocus "some-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.SetPipeline (someStep "some-id" "some-name" Models.StepStateSucceeded))
        ]


initLoadVar : Test
initLoadVar =
    let
        { tree, foci } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepLoadVar "some-name"
                }
    in
    describe "init with LoadVar"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.LoadVar (someStep "some-id" "some-name" Models.StepStatePending))
                    tree
        , test "using the focus" <|
            \_ ->
                assertFocus "some-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.LoadVar (someStep "some-id" "some-name" Models.StepStateSucceeded))
        ]


initGet : Test
initGet =
    let
        version =
            Dict.fromList [ ( "some", "version" ) ]

        { tree, foci } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepGet "some-name" (Just version)
                }
    in
    describe "init with Get"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Get (someVersionedStep (Just version) "some-id" "some-name" Models.StepStatePending))
                    tree
        , test "using the focus" <|
            \_ ->
                assertFocus "some-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Get (someVersionedStep (Just version) "some-id" "some-name" Models.StepStateSucceeded))
        ]


initPut : Test
initPut =
    let
        { tree, foci } =
            StepTree.init Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = BuildStepPut "some-name"
                }
    in
    describe "init with Put"
        [ test "the tree" <|
            \_ ->
                Expect.equal
                    (Models.Put (someStep "some-id" "some-name" Models.StepStatePending))
                    tree
        , test "using the focus" <|
            \_ ->
                assertFocus "some-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Put (someStep "some-id" "some-name" Models.StepStateSucceeded))
        ]


initAggregate : Test
initAggregate =
    let
        { tree, foci } =
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
                        [ Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending)
                        , Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending)
                        ]
                    )
                    tree
        , test "using the focus" <|
            \_ ->
                assertFocus "task-a-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Aggregate
                        << Array.fromList
                     <|
                        [ Models.Task (someStep "task-a-id" "task-a" Models.StepStateSucceeded)
                        , Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending)
                        ]
                    )
        ]


initAggregateNested : Test
initAggregateNested =
    let
        { tree, foci } =
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
                        [ Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending)
                        , Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending)
                        , Models.Aggregate
                            << Array.fromList
                          <|
                            [ Models.Task (someStep "task-c-id" "task-c" Models.StepStatePending)
                            , Models.Task (someStep "task-d-id" "task-d" Models.StepStatePending)
                            ]
                        ]
                    )
                    tree
        , test "using the focuses for nested elements" <|
            \_ ->
                assertFocus "task-c-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Aggregate
                        << Array.fromList
                     <|
                        [ Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending)
                        , Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending)
                        , Models.Aggregate
                            << Array.fromList
                          <|
                            [ Models.Task (someStep "task-c-id" "task-c" Models.StepStateSucceeded)
                            , Models.Task (someStep "task-d-id" "task-d" Models.StepStatePending)
                            ]
                        ]
                    )
        ]


initAcross : Test
initAcross =
    let
        { tree, foci } =
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
                    (Models.Across [ "var" ]
                        [ [ JsonString "v1" ], [ JsonString "v2" ] ]
                        [ False, False ]
                        (someStep "across-id" "var" Models.StepStatePending)
                        << Array.fromList
                     <|
                        [ Models.Task (someExpandedStep "task-a-id" "task-a" Models.StepStatePending)
                        , Models.Task (someExpandedStep "task-b-id" "task-b" Models.StepStatePending)
                        ]
                    )
                    tree
        , test "using the focus on root step" <|
            \_ ->
                assertFocus "across-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Across [ "var" ]
                        [ [ JsonString "v1" ], [ JsonString "v2" ] ]
                        [ False, False ]
                        (someStep "across-id" "var" Models.StepStateSucceeded)
                        << Array.fromList
                     <|
                        [ Models.Task (someExpandedStep "task-a-id" "task-a" Models.StepStatePending)
                        , Models.Task (someExpandedStep "task-b-id" "task-b" Models.StepStatePending)
                        ]
                    )
        , test "using the focus on child step" <|
            \_ ->
                assertFocus "task-a-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Across [ "var" ]
                        [ [ JsonString "v1" ], [ JsonString "v2" ] ]
                        [ False, False ]
                        (someStep "across-id" "var" Models.StepStatePending)
                        << Array.fromList
                     <|
                        [ Models.Task (someExpandedStep "task-a-id" "task-a" Models.StepStateSucceeded)
                        , Models.Task (someExpandedStep "task-b-id" "task-b" Models.StepStatePending)
                        ]
                    )
        ]


initAcrossNested : Test
initAcrossNested =
    let
        { tree, foci } =
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
                    (Models.Across [ "var1" ]
                        [ [ JsonString "a1" ] ]
                        [ False ]
                        (someStep "across-id" "var1" Models.StepStatePending)
                        << Array.fromList
                     <|
                        [ Models.Across [ "var2" ]
                            [ [ JsonString "b1" ], [ JsonString "b2" ] ]
                            [ False, False ]
                            (someExpandedStep "nested-across-id" "var2" Models.StepStatePending)
                            << Array.fromList
                          <|
                            [ Models.Task (someExpandedStep "task-a-id" "task-a" Models.StepStatePending)
                            , Models.Task (someExpandedStep "task-b-id" "task-b" Models.StepStatePending)
                            ]
                        ]
                    )
                    tree
        , test "using the focuses for nested elements" <|
            \_ ->
                assertFocus "task-a-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Across [ "var1" ]
                        [ [ JsonString "a1" ] ]
                        [ False ]
                        (someStep "across-id" "var1" Models.StepStatePending)
                        << Array.fromList
                     <|
                        [ Models.Across [ "var2" ]
                            [ [ JsonString "b1" ], [ JsonString "b2" ] ]
                            [ False, False ]
                            (someExpandedStep "nested-across-id" "var2" Models.StepStatePending)
                            << Array.fromList
                          <|
                            [ Models.Task (someExpandedStep "task-a-id" "task-a" Models.StepStateSucceeded)
                            , Models.Task (someExpandedStep "task-b-id" "task-b" Models.StepStatePending)
                            ]
                        ]
                    )
        ]


initAcrossWithDo : Test
initAcrossWithDo =
    let
        { tree, foci } =
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
                    (Models.Across [ "var" ]
                        [ [ JsonString "v1" ] ]
                        [ False ]
                        (someStep "across-id" "var" Models.StepStatePending)
                        << Array.fromList
                     <|
                        [ Models.Do <|
                            Array.fromList
                                [ Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending)
                                , Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending)
                                ]
                        ]
                    )
                    tree
        ]


initInParallel : Test
initInParallel =
    let
        { tree, foci } =
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
                        [ Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending)
                        , Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending)
                        ]
                    )
                    tree
        , test "using the focus" <|
            \_ ->
                assertFocus "task-a-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.InParallel
                        << Array.fromList
                     <|
                        [ Models.Task (someStep "task-a-id" "task-a" Models.StepStateSucceeded)
                        , Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending)
                        ]
                    )
        ]


initInParallelNested : Test
initInParallelNested =
    let
        { tree, foci } =
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
                        [ Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending)
                        , Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending)
                        , Models.InParallel
                            << Array.fromList
                          <|
                            [ Models.Task (someStep "task-c-id" "task-c" Models.StepStatePending)
                            , Models.Task (someStep "task-d-id" "task-d" Models.StepStatePending)
                            ]
                        ]
                    )
                    tree
        , test "using the focuses for nested elements" <|
            \_ ->
                assertFocus "task-c-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.InParallel
                        << Array.fromList
                     <|
                        [ Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending)
                        , Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending)
                        , Models.InParallel
                            << Array.fromList
                          <|
                            [ Models.Task (someStep "task-c-id" "task-c" Models.StepStateSucceeded)
                            , Models.Task (someStep "task-d-id" "task-d" Models.StepStatePending)
                            ]
                        ]
                    )
        ]


initOnSuccess : Test
initOnSuccess =
    let
        { tree, foci } =
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
                            (Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending))
                            (Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending))
                    )
                    tree
        , test "updating a step via the focus" <|
            \_ ->
                assertFocus "task-a-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.OnSuccess <|
                        Models.HookedStep
                            (Models.Task (someStep "task-a-id" "task-a" Models.StepStateSucceeded))
                            (Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending))
                    )
        , test "updating a hook via the focus" <|
            \_ ->
                assertFocus "task-b-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.OnSuccess <|
                        Models.HookedStep
                            (Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending))
                            (Models.Task (someStep "task-b-id" "task-b" Models.StepStateSucceeded))
                    )
        ]


initOnFailure : Test
initOnFailure =
    let
        { tree, foci } =
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
                            (Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending))
                            (Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending))
                    )
                    tree
        , test "updating a step via the focus" <|
            \_ ->
                assertFocus "task-a-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.OnFailure <|
                        Models.HookedStep
                            (Models.Task (someStep "task-a-id" "task-a" Models.StepStateSucceeded))
                            (Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending))
                    )
        , test "updating a hook via the focus" <|
            \_ ->
                assertFocus "task-b-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.OnFailure <|
                        Models.HookedStep
                            (Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending))
                            (Models.Task (someStep "task-b-id" "task-b" Models.StepStateSucceeded))
                    )
        ]


initEnsure : Test
initEnsure =
    let
        { tree, foci } =
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
                            (Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending))
                            (Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending))
                    )
                    tree
        , test "updating a step via the focus" <|
            \_ ->
                assertFocus "task-a-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Ensure <|
                        Models.HookedStep
                            (Models.Task (someStep "task-a-id" "task-a" Models.StepStateSucceeded))
                            (Models.Task (someStep "task-b-id" "task-b" Models.StepStatePending))
                    )
        , test "updating a hook via the focus" <|
            \_ ->
                assertFocus "task-b-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Ensure <|
                        Models.HookedStep
                            (Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending))
                            (Models.Task (someStep "task-b-id" "task-b" Models.StepStateSucceeded))
                    )
        ]


initTry : Test
initTry =
    let
        { tree, foci } =
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
                        Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending)
                    )
                    tree
        , test "updating a step via the focus" <|
            \_ ->
                assertFocus "task-a-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Try <|
                        Models.Task (someStep "task-a-id" "task-a" Models.StepStateSucceeded)
                    )
        ]


initTimeout : Test
initTimeout =
    let
        { tree, foci } =
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
                        Models.Task (someStep "task-a-id" "task-a" Models.StepStatePending)
                    )
                    tree
        , test "updating a step via the focus" <|
            \_ ->
                assertFocus "task-a-id"
                    foci
                    tree
                    (\s -> { s | state = Models.StepStateSucceeded })
                    (Models.Timeout <|
                        Models.Task (someStep "task-a-id" "task-a" Models.StepStateSucceeded)
                    )
        ]


assertFocus :
    Routes.StepID
    -> Dict.Dict Routes.StepID Models.StepFocus
    -> Models.StepTree
    -> (Models.Step -> Models.Step)
    -> Models.StepTree
    -> Expectation
assertFocus id foci tree update expected =
    case Dict.get id foci of
        Nothing ->
            Expect.true "failed" False

        Just focus ->
            Expect.equal
                expected
                (focus (Models.map update) tree)


cookedLog : Ansi.Log.Model
cookedLog =
    Ansi.Log.init Ansi.Log.Cooked
