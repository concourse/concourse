module StepTreeTests exposing
    ( all
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
        , initRun
        , initGet
        , initPut
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


someStep : Routes.StepID -> Concourse.BuildStep -> Models.StepState -> Models.Step
someStep =
    someVersionedStep Nothing


someVersionedStep : Maybe Models.Version -> Routes.StepID -> Concourse.BuildStep -> Models.StepState -> Models.Step
someVersionedStep version id buildStep state =
    { id = id
    , buildStep = buildStep
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
    , initializationExpanded = False
    , imageCheck = Nothing
    , imageGet = Nothing
    }


someExpandedStep : Routes.StepID -> Concourse.BuildStep -> Models.StepState -> Models.Step
someExpandedStep id buildStep state =
    someStep id buildStep state |> (\s -> { s | expanded = True })


emptyResources : Concourse.BuildResources
emptyResources =
    { inputs = [], outputs = [] }


task : String -> BuildStep
task s =
    BuildStepTask <| "task-" ++ s


initTask : Test
initTask =
    let
        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = task "some-name"
                }
    in
    describe "init with Task"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.Task "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps
                    [ someStep "some-id" (task "some-name") Models.StepStatePending ]
                    steps
        ]


initSetPipeline : Test
initSetPipeline =
    let
        step =
            BuildStepSetPipeline "some-name" Dict.empty

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = step
                }
    in
    describe "init with SetPipeline"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.SetPipeline "some-id") tree
        , test "the steps" <|
            \_ ->
                assertSteps [ someStep "some-id" step Models.StepStatePending ] steps
        ]


initLoadVar : Test
initLoadVar =
    let
        step =
            BuildStepLoadVar "some-name"

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = step
                }
    in
    describe "init with LoadVar"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.LoadVar "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps [ someStep "some-id" step Models.StepStatePending ] steps
        ]


initCheck : Test
initCheck =
    let
        step =
            BuildStepCheck "some-name" Nothing

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = step
                }
    in
    describe "init with Check"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.Check "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps [ someStep "some-id" step Models.StepStatePending ] steps
        ]


initRun : Test
initRun =
    let
        step =
            BuildStepRun "some-message"

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = step
                }
    in
    describe "init with Run"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.Run "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps [ someStep "some-id" step Models.StepStatePending ] steps
        ]


initGet : Test
initGet =
    let
        version =
            Dict.fromList [ ( "some", "version" ) ]

        step =
            BuildStepGet "some-name" (Just "some-resource") (Just version) Nothing

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = step
                }
    in
    describe "init with Get"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.Get "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps [ someVersionedStep (Just version) "some-id" step Models.StepStatePending ] steps
        ]


initPut : Test
initPut =
    let
        step =
            BuildStepPut "some-name" (Just "some-resource") Nothing

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "some-id"
                , step = step
                }
    in
    describe "init with Put"
        [ test "the tree" <|
            \_ ->
                Expect.equal (Models.Put "some-id") tree
        , test "the step" <|
            \_ ->
                assertSteps [ someStep "some-id" step Models.StepStatePending ] steps
        ]


initAcross : Test
initAcross =
    let
        rootStep =
            BuildStepAcross
                { vars = [ "var" ]
                , steps =
                    [ { values = [ JsonString "v1" ]
                      , step = { id = "task-a-id", step = task "a" }
                      }
                    , { values = [ JsonString "v2" ]
                      , step = { id = "task-b-id", step = task "b" }
                      }
                    ]
                }

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "across-id"
                , step = rootStep
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
                    [ someStep "across-id" rootStep Models.StepStatePending
                    , someExpandedStep "task-a-id" (task "a") Models.StepStatePending
                    , someExpandedStep "task-b-id" (task "b") Models.StepStatePending
                    ]
                    steps
        ]


initAcrossNested : Test
initAcrossNested =
    let
        nestedAcross =
            BuildStepAcross
                { vars = [ "var2" ]
                , steps =
                    [ { values = [ JsonString "b1" ]
                      , step = { id = "task-a-id", step = task "a" }
                      }
                    , { values = [ JsonString "b2" ]
                      , step = { id = "task-b-id", step = task "b" }
                      }
                    ]
                }

        rootAcross =
            BuildStepAcross
                { vars = [ "var1" ]
                , steps =
                    [ { values = [ JsonString "a1" ]
                      , step =
                            { id = "nested-across-id"
                            , step = nestedAcross
                            }
                      }
                    ]
                }

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "across-id"
                , step = rootAcross
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
                    [ someStep "across-id" rootAcross Models.StepStatePending
                    , someExpandedStep "nested-across-id" nestedAcross Models.StepStatePending
                    , someExpandedStep "task-a-id" (task "a") Models.StepStatePending
                    , someExpandedStep "task-b-id" (task "b") Models.StepStatePending
                    ]
                    steps
        ]


initAcrossWithDo : Test
initAcrossWithDo =
    let
        do =
            BuildStepDo <|
                Array.fromList
                    [ { id = "task-a-id", step = task "a" }
                    , { id = "task-b-id", step = task "b" }
                    ]

        across =
            BuildStepAcross
                { vars = [ "var" ]
                , steps =
                    [ { values = [ JsonString "v1" ]
                      , step =
                            { id = "do-id"
                            , step = do
                            }
                      }
                    ]
                }

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "across-id"
                , step = across
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
                    [ someStep "across-id" across Models.StepStatePending
                    , someStep "task-a-id" (task "a") Models.StepStatePending
                    , someStep "task-b-id" (task "b") Models.StepStatePending
                    ]
                    steps
        ]


initInParallel : Test
initInParallel =
    let
        step =
            BuildStepInParallel
                << Array.fromList
            <|
                [ { id = "task-a-id", step = task "a" }
                , { id = "task-b-id", step = task "b" }
                ]

        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "parallel-id"
                , step = step
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
                    [ someStep "task-a-id" (task "a") Models.StepStatePending
                    , someStep "task-b-id" (task "b") Models.StepStatePending
                    ]
                    steps
        ]


initInParallelNested : Test
initInParallelNested =
    let
        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "parallel-id"
                , step =
                    BuildStepInParallel
                        << Array.fromList
                    <|
                        [ { id = "task-a-id", step = task "a" }
                        , { id = "task-b-id", step = task "b" }
                        , { id = "nested-parallel-id"
                          , step =
                                BuildStepInParallel
                                    << Array.fromList
                                <|
                                    [ { id = "task-c-id", step = task "c" }
                                    , { id = "task-d-id", step = task "d" }
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
                    [ someStep "task-a-id" (task "a") Models.StepStatePending
                    , someStep "task-b-id" (task "b") Models.StepStatePending
                    , someStep "task-c-id" (task "c") Models.StepStatePending
                    , someStep "task-d-id" (task "d") Models.StepStatePending
                    ]
                    steps
        ]


initOnSuccess : Test
initOnSuccess =
    let
        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepOnSuccess <|
                        HookedPlan
                            { id = "task-a-id", step = task "a" }
                            { id = "task-b-id", step = task "b" }
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
                    [ someStep "task-a-id" (task "a") Models.StepStatePending
                    , someStep "task-b-id" (task "b") Models.StepStatePending
                    ]
                    steps
        ]


initOnFailure : Test
initOnFailure =
    let
        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepOnFailure <|
                        HookedPlan
                            { id = "task-a-id", step = task "a" }
                            { id = "task-b-id", step = task "b" }
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
                    [ someStep "task-a-id" (task "a") Models.StepStatePending
                    , someStep "task-b-id" (task "b") Models.StepStatePending
                    ]
                    steps
        ]


initEnsure : Test
initEnsure =
    let
        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepEnsure <|
                        HookedPlan
                            { id = "task-a-id", step = task "a" }
                            { id = "task-b-id", step = task "b" }
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
                    [ someStep "task-a-id" (task "a") Models.StepStatePending
                    , someStep "task-b-id" (task "b") Models.StepStatePending
                    ]
                    steps
        ]


initTry : Test
initTry =
    let
        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepTry { id = "task-a-id", step = task "a" }
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
                assertSteps [ someStep "task-a-id" (task "a") Models.StepStatePending ] steps
        ]


initTimeout : Test
initTimeout =
    let
        { tree, steps } =
            StepTree.init Nothing
                Routes.HighlightNothing
                emptyResources
                { id = "on-success-id"
                , step =
                    BuildStepTimeout { id = "task-a-id", step = task "a" }
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
                assertSteps [ someStep "task-a-id" (task "a") Models.StepStatePending ] steps
        ]


assertSteps : List Models.Step -> Dict Routes.StepID Models.Step -> Expectation
assertSteps expected actual =
    Expect.equalDicts (Dict.fromList (List.map (\s -> ( s.id, s )) expected)) actual


cookedLog : Ansi.Log.Model
cookedLog =
    Ansi.Log.init Ansi.Log.Cooked
