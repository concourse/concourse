module StepTreeTests exposing
    ( all
    , initAggregate
    , initAggregateNested
    , initEnsure
    , initGet
    , initOnFailure
    , initOnSuccess
    , initPut
    , initTask
    , initTimeout
    , initTry
    )

import Ansi.Log
import Array
import Build.Models as Models
import Build.StepTree as StepTree
import Concourse exposing (BuildStep(..), HookedPlan)
import Dict
import Expect exposing (..)
import Focus
import Test exposing (..)


all : Test
all =
    describe "StepTree"
        [ initTask
        , initGet
        , initPut
        , initAggregate
        , initAggregateNested
        , initOnSuccess
        , initOnFailure
        , initEnsure
        , initTry
        , initTimeout
        ]


someStep : Models.StepID -> Models.StepName -> Models.StepState -> Models.Step
someStep =
    someVersionedStep Nothing


someVersionedStep : Maybe Models.Version -> Models.StepID -> Models.StepName -> Models.StepState -> Models.Step
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
    , timestamps = Dict.empty
    }


emptyResources : Concourse.BuildResources
emptyResources =
    { inputs = [], outputs = [] }


initTask : Test
initTask =
    let
        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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


initGet : Test
initGet =
    let
        version =
            Dict.fromList [ ( "some", "version" ) ]

        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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
        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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
        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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
        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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


initOnSuccess : Test
initOnSuccess =
    let
        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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
        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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
        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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
        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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
        { tree, foci, finished } =
            StepTree.init Models.HighlightNothing
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


updateStep : (Models.Step -> Models.Step) -> Models.StepTree -> Models.StepTree
updateStep f tree =
    case tree of
        Models.Task step ->
            Models.Task (f step)

        Models.Get step ->
            Models.Get (f step)

        Models.Put step ->
            Models.Put (f step)

        _ ->
            tree


assertFocus : Models.StepID -> Dict.Dict Models.StepID Models.StepFocus -> Models.StepTree -> (Models.Step -> Models.Step) -> Models.StepTree -> Expectation
assertFocus id foci tree update expected =
    case Dict.get id foci of
        Nothing ->
            Expect.true "failed" False

        Just focus ->
            Expect.equal
                expected
                (Focus.update focus (updateStep update) tree)


cookedLog : Ansi.Log.Model
cookedLog =
    Ansi.Log.init Ansi.Log.Cooked
