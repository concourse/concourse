module StepTreeTests exposing (..)

import Array
import Dict
import Test exposing (..)
import Expect exposing (..)
import Focus
import Regex
import String
import Ansi.Log
import Concourse exposing (BuildStep(..), HookedPlan)
import StepTree


all : Test
all =
    describe "StepTree"
        [ initTask
        , initGet
        , initPut
        , initDependentGet
        , initAggregate
        , initAggregateNested
        , initOnSuccess
        , initOnFailure
        , initEnsure
        , initTry
        , initTimeout
        ]


someStep : StepTree.StepID -> StepTree.StepName -> StepTree.StepState -> StepTree.Step
someStep =
    someVersionedStep Nothing


someVersionedStep : Maybe StepTree.Version -> StepTree.StepID -> StepTree.StepName -> StepTree.StepState -> StepTree.Step
someVersionedStep version id name state =
    { id = id
    , name = name
    , state = state
    , log = cookedLog
    , error = Nothing
    , expanded = Nothing
    , version = version
    , metadata = []
    , firstOccurrence = False
    }


emptyResources =
    { inputs = [], outputs = [] }


initTask : Test
initTask =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
                { id = "some-id"
                , step = BuildStepTask "some-name"
                }
    in
        describe "init with Task"
            [ test "the tree" <|
                \_ ->
                    Expect.equal
                        (StepTree.Task (someStep "some-id" "some-name" StepTree.StepStatePending))
                        tree
            , test "using the focus" <|
                \_ ->
                    assertFocus "some-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.Task (someStep "some-id" "some-name" StepTree.StepStateSucceeded))
            ]


initGet : Test
initGet =
    let
        version =
            Dict.fromList [ ( "some", "version" ) ]

        { tree, foci, finished } =
            StepTree.init emptyResources
                { id = "some-id"
                , step = BuildStepGet "some-name" (Just version)
                }
    in
        describe "init with Get"
            [ test "the tree" <|
                \_ ->
                    Expect.equal
                        (StepTree.Get (someVersionedStep (Just version) "some-id" "some-name" StepTree.StepStatePending))
                        tree
            , test "using the focus" <|
                \_ ->
                    assertFocus "some-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.Get (someVersionedStep (Just version) "some-id" "some-name" StepTree.StepStateSucceeded))
            ]


initPut : Test
initPut =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
                { id = "some-id"
                , step = BuildStepPut "some-name"
                }
    in
        describe "init with Put"
            [ test "the tree" <|
                \_ ->
                    Expect.equal
                        (StepTree.Put (someStep "some-id" "some-name" StepTree.StepStatePending))
                        tree
            , test "using the focus" <|
                \_ ->
                    assertFocus "some-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.Put (someStep "some-id" "some-name" StepTree.StepStateSucceeded))
            ]


initDependentGet : Test
initDependentGet =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
                { id = "some-id"
                , step = BuildStepDependentGet "some-name"
                }
    in
        describe "init with DependentGet"
            [ test "the tree" <|
                \_ ->
                    Expect.equal
                        (StepTree.DependentGet (someStep "some-id" "some-name" StepTree.StepStatePending))
                        tree
            , test "using the focus" <|
                \_ ->
                    assertFocus "some-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.DependentGet (someStep "some-id" "some-name" StepTree.StepStateSucceeded))
            ]


initAggregate : Test
initAggregate =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
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
                        (StepTree.Aggregate
                            << Array.fromList
                         <|
                            [ StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending)
                            , StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending)
                            ]
                        )
                        tree
            , test "using the focus" <|
                \_ ->
                    assertFocus "task-a-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.Aggregate
                            << Array.fromList
                         <|
                            [ StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStateSucceeded)
                            , StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending)
                            ]
                        )
            ]


initAggregateNested : Test
initAggregateNested =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
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
                        (StepTree.Aggregate
                            << Array.fromList
                         <|
                            [ StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending)
                            , StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending)
                            , StepTree.Aggregate
                                << Array.fromList
                              <|
                                [ StepTree.Task (someStep "task-c-id" "task-c" StepTree.StepStatePending)
                                , StepTree.Task (someStep "task-d-id" "task-d" StepTree.StepStatePending)
                                ]
                            ]
                        )
                        tree
            , test "using the focuses for nested elements" <|
                \_ ->
                    assertFocus "task-c-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.Aggregate
                            << Array.fromList
                         <|
                            [ StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending)
                            , StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending)
                            , StepTree.Aggregate
                                << Array.fromList
                              <|
                                [ StepTree.Task (someStep "task-c-id" "task-c" StepTree.StepStateSucceeded)
                                , StepTree.Task (someStep "task-d-id" "task-d" StepTree.StepStatePending)
                                ]
                            ]
                        )
            ]


initOnSuccess : Test
initOnSuccess =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
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
                        (StepTree.OnSuccess <|
                            StepTree.HookedStep
                                (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending))
                                (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending))
                        )
                        tree
            , test "updating a step via the focus" <|
                \_ ->
                    assertFocus "task-a-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.OnSuccess <|
                            StepTree.HookedStep
                                (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStateSucceeded))
                                (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending))
                        )
            , test "updating a hook via the focus" <|
                \_ ->
                    assertFocus "task-b-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.OnSuccess <|
                            StepTree.HookedStep
                                (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending))
                                (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStateSucceeded))
                        )
            ]


initOnFailure : Test
initOnFailure =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
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
                        (StepTree.OnFailure <|
                            StepTree.HookedStep
                                (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending))
                                (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending))
                        )
                        tree
            , test "updating a step via the focus" <|
                \_ ->
                    assertFocus "task-a-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.OnFailure <|
                            StepTree.HookedStep
                                (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStateSucceeded))
                                (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending))
                        )
            , test "updating a hook via the focus" <|
                \_ ->
                    assertFocus "task-b-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.OnFailure <|
                            StepTree.HookedStep
                                (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending))
                                (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStateSucceeded))
                        )
            ]


initEnsure : Test
initEnsure =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
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
                        (StepTree.Ensure <|
                            StepTree.HookedStep
                                (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending))
                                (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending))
                        )
                        tree
            , test "updating a step via the focus" <|
                \_ ->
                    assertFocus "task-a-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.Ensure <|
                            StepTree.HookedStep
                                (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStateSucceeded))
                                (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending))
                        )
            , test "updating a hook via the focus" <|
                \_ ->
                    assertFocus "task-b-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.Ensure <|
                            StepTree.HookedStep
                                (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending))
                                (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStateSucceeded))
                        )
            ]


initTry : Test
initTry =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepTry { id = "task-a-id", step = BuildStepTask "task-a" }
                }
    in
        describe "init with Try"
            [ test "the tree" <|
                \_ ->
                    Expect.equal
                        (StepTree.Try <|
                            StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending)
                        )
                        tree
            , test "updating a step via the focus" <|
                \_ ->
                    assertFocus "task-a-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.Try <|
                            StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStateSucceeded)
                        )
            ]


initTimeout : Test
initTimeout =
    let
        { tree, foci, finished } =
            StepTree.init emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepTimeout { id = "task-a-id", step = BuildStepTask "task-a" }
                }
    in
        describe "init with Timeout"
            [ test "the tree" <|
                \_ ->
                    Expect.equal
                        (StepTree.Timeout <|
                            StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending)
                        )
                        tree
            , test "updating a step via the focus" <|
                \_ ->
                    assertFocus "task-a-id"
                        foci
                        tree
                        (\s -> { s | state = StepTree.StepStateSucceeded })
                        (StepTree.Timeout <|
                            StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStateSucceeded)
                        )
            ]


updateStep : (StepTree.Step -> StepTree.Step) -> StepTree.StepTree -> StepTree.StepTree
updateStep f tree =
    case tree of
        StepTree.Task step ->
            StepTree.Task (f step)

        StepTree.Get step ->
            StepTree.Get (f step)

        StepTree.Put step ->
            StepTree.Put (f step)

        StepTree.DependentGet step ->
            StepTree.DependentGet (f step)

        _ ->
            tree


assertFocus : StepTree.StepID -> Dict.Dict StepTree.StepID StepTree.StepFocus -> StepTree.StepTree -> (StepTree.Step -> StepTree.Step) -> StepTree.StepTree -> Expectation
assertFocus id foci tree update expected =
    case Dict.get id foci of
        Nothing ->
            Expect.true "failed" False

        Just focus ->
            Expect.equal
                expected
                (Focus.update focus (updateStep update) tree)


cookedLog =
    Ansi.Log.init Ansi.Log.Cooked
