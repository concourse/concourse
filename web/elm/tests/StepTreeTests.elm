module StepTreeTests exposing (..)

import Array
import Dict
import ElmTest exposing (..)
import Focus
import Regex
import String
import Ansi.Log
import Concourse.BuildPlan exposing (BuildPlan)
import Concourse.Version exposing (Version)
import StepTree


all : Test
all =
    suite "StepTree"
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


someVersionedStep : Maybe Version -> StepTree.StepID -> StepTree.StepName -> StepTree.StepState -> StepTree.Step
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
        { tree, foci } =
            StepTree.init emptyResources
                { id = "some-id"
                , step = Concourse.BuildPlan.Task "some-name"
                }
    in
        suite "init with Task"
            [ test "the tree" <|
                assertEqual
                    (StepTree.Task (someStep "some-id" "some-name" StepTree.StepStatePending))
                    tree
            , test "using the focus" <|
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

        { tree, foci } =
            StepTree.init emptyResources
                { id = "some-id"
                , step = Concourse.BuildPlan.Get "some-name" (Just version)
                }
    in
        suite "init with Get"
            [ test "the tree" <|
                assertEqual
                    (StepTree.Get (someVersionedStep (Just version) "some-id" "some-name" StepTree.StepStatePending))
                    tree
            , test "using the focus" <|
                assertFocus "some-id"
                    foci
                    tree
                    (\s -> { s | state = StepTree.StepStateSucceeded })
                    (StepTree.Get (someVersionedStep (Just version) "some-id" "some-name" StepTree.StepStateSucceeded))
            ]


initPut : Test
initPut =
    let
        { tree, foci } =
            StepTree.init emptyResources
                { id = "some-id"
                , step = Concourse.BuildPlan.Put "some-name"
                }
    in
        suite "init with Put"
            [ test "the tree" <|
                assertEqual
                    (StepTree.Put (someStep "some-id" "some-name" StepTree.StepStatePending))
                    tree
            , test "using the focus" <|
                assertFocus "some-id"
                    foci
                    tree
                    (\s -> { s | state = StepTree.StepStateSucceeded })
                    (StepTree.Put (someStep "some-id" "some-name" StepTree.StepStateSucceeded))
            ]


initDependentGet : Test
initDependentGet =
    let
        { tree, foci } =
            StepTree.init emptyResources
                { id = "some-id"
                , step = Concourse.BuildPlan.DependentGet "some-name"
                }
    in
        suite "init with DependentGet"
            [ test "the tree" <|
                assertEqual
                    (StepTree.DependentGet (someStep "some-id" "some-name" StepTree.StepStatePending))
                    tree
            , test "using the focus" <|
                assertFocus "some-id"
                    foci
                    tree
                    (\s -> { s | state = StepTree.StepStateSucceeded })
                    (StepTree.DependentGet (someStep "some-id" "some-name" StepTree.StepStateSucceeded))
            ]


initAggregate : Test
initAggregate =
    let
        { tree, foci } =
            StepTree.init emptyResources
                { id = "aggregate-id"
                , step =
                    Concourse.BuildPlan.Aggregate
                        << Array.fromList
                    <|
                        [ { id = "task-a-id", step = Concourse.BuildPlan.Task "task-a" }
                        , { id = "task-b-id", step = Concourse.BuildPlan.Task "task-b" }
                        ]
                }
    in
        suite "init with Aggregate"
            [ test "the tree" <|
                assertEqual
                    (StepTree.Aggregate
                        << Array.fromList
                     <|
                        [ StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending)
                        , StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending)
                        ]
                    )
                    tree
            , test "using the focus" <|
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
        { tree, foci } =
            StepTree.init emptyResources
                { id = "aggregate-id"
                , step =
                    Concourse.BuildPlan.Aggregate
                        << Array.fromList
                    <|
                        [ { id = "task-a-id", step = Concourse.BuildPlan.Task "task-a" }
                        , { id = "task-b-id", step = Concourse.BuildPlan.Task "task-b" }
                        , { id = "nested-aggregate-id"
                          , step =
                                Concourse.BuildPlan.Aggregate
                                    << Array.fromList
                                <|
                                    [ { id = "task-c-id", step = Concourse.BuildPlan.Task "task-c" }
                                    , { id = "task-d-id", step = Concourse.BuildPlan.Task "task-d" }
                                    ]
                          }
                        ]
                }
    in
        suite "init with Aggregate"
            [ test "the tree" <|
                assertEqual
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
        { tree, foci } =
            StepTree.init emptyResources
                { id = "on-success-id"
                , step =
                    Concourse.BuildPlan.OnSuccess <|
                        Concourse.BuildPlan.HookedPlan
                            { id = "task-a-id", step = Concourse.BuildPlan.Task "task-a" }
                            { id = "task-b-id", step = Concourse.BuildPlan.Task "task-b" }
                }
    in
        suite "init with OnSuccess"
            [ test "the tree" <|
                assertEqual
                    (StepTree.OnSuccess <|
                        StepTree.HookedStep
                            (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending))
                            (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending))
                    )
                    tree
            , test "updating a step via the focus" <|
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
        { tree, foci } =
            StepTree.init emptyResources
                { id = "on-success-id"
                , step =
                    Concourse.BuildPlan.OnFailure <|
                        Concourse.BuildPlan.HookedPlan
                            { id = "task-a-id", step = Concourse.BuildPlan.Task "task-a" }
                            { id = "task-b-id", step = Concourse.BuildPlan.Task "task-b" }
                }
    in
        suite "init with OnFailure"
            [ test "the tree" <|
                assertEqual
                    (StepTree.OnFailure <|
                        StepTree.HookedStep
                            (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending))
                            (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending))
                    )
                    tree
            , test "updating a step via the focus" <|
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
        { tree, foci } =
            StepTree.init emptyResources
                { id = "on-success-id"
                , step =
                    Concourse.BuildPlan.Ensure <|
                        Concourse.BuildPlan.HookedPlan
                            { id = "task-a-id", step = Concourse.BuildPlan.Task "task-a" }
                            { id = "task-b-id", step = Concourse.BuildPlan.Task "task-b" }
                }
    in
        suite "init with Ensure"
            [ test "the tree" <|
                assertEqual
                    (StepTree.Ensure <|
                        StepTree.HookedStep
                            (StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending))
                            (StepTree.Task (someStep "task-b-id" "task-b" StepTree.StepStatePending))
                    )
                    tree
            , test "updating a step via the focus" <|
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
        { tree, foci } =
            StepTree.init emptyResources
                { id = "on-success-id"
                , step =
                    Concourse.BuildPlan.Try { id = "task-a-id", step = Concourse.BuildPlan.Task "task-a" }
                }
    in
        suite "init with Try"
            [ test "the tree" <|
                assertEqual
                    (StepTree.Try <|
                        StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending)
                    )
                    tree
            , test "updating a step via the focus" <|
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
        { tree, foci } =
            StepTree.init emptyResources
                { id = "on-success-id"
                , step =
                    Concourse.BuildPlan.Timeout { id = "task-a-id", step = Concourse.BuildPlan.Task "task-a" }
                }
    in
        suite "init with Timeout"
            [ test "the tree" <|
                assertEqual
                    (StepTree.Timeout <|
                        StepTree.Task (someStep "task-a-id" "task-a" StepTree.StepStatePending)
                    )
                    tree
            , test "updating a step via the focus" <|
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


assertFocus id foci tree update expected =
    case Dict.get id foci of
        Nothing ->
            assert False

        Just focus ->
            assertEqual
                expected
                (Focus.update focus (updateStep update) tree)


cookedLog =
    Ansi.Log.init Ansi.Log.Cooked
