module StepTreeTests where

import Array
import Dict
import ElmTest exposing (..)
import Focus
import Regex
import String
import Ansi.Log

import BuildPlan
import StepTree

all : Test
all =
  suite "StepTree"
    [ initTask
    , initGet
    , initPut
    , initDependentGet
    , initAggregate
    , initOnSuccess
    , initOnFailure
    , initEnsure
    , initTry
    , initTimeout
    ]

initTask : Test
initTask =
  let
    {tree, foci} =
      StepTree.init
        { id = "some-id"
        , step = BuildPlan.Task "some-name"
        }
  in
    suite "init with Task"
      [ test "the tree" <|
          assertEqual
            (StepTree.Task
              { name = "some-name"
              , state = StepTree.StepStatePending
              , log = cookedLog
              })
            tree
      , test "using the focus" <|
          assertFocus "some-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.Task
              { name = "some-name"
              , state = StepTree.StepStateSucceeded
              , log = cookedLog
              })
      ]

initGet : Test
initGet =
  let
    version = Dict.fromList [("some", "version")]
    {tree, foci} =
      StepTree.init
        { id = "some-id"
        , step = BuildPlan.Get "some-name" (Just version)
        }
  in
    suite "init with Get"
      [ test "the tree" <|
          assertEqual
            (StepTree.Get
              { name = "some-name"
              , state = StepTree.StepStatePending
              , log = cookedLog
              }
              (Just version))
            tree
      , test "using the focus" <|
          assertFocus "some-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.Get
              { name = "some-name"
              , state = StepTree.StepStateSucceeded
              , log = cookedLog
              }
              (Just version))
      ]

initPut : Test
initPut =
  let
    {tree, foci} =
      StepTree.init
        { id = "some-id"
        , step = BuildPlan.Put "some-name"
        }
  in
    suite "init with Put"
      [ test "the tree" <|
          assertEqual
            (StepTree.Put
              { name = "some-name"
              , state = StepTree.StepStatePending
              , log = cookedLog
              })
            tree
      , test "using the focus" <|
          assertFocus "some-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.Put
              { name = "some-name"
              , state = StepTree.StepStateSucceeded
              , log = cookedLog
              })
      ]

initDependentGet : Test
initDependentGet =
  let
    {tree, foci} =
      StepTree.init
        { id = "some-id"
        , step = BuildPlan.DependentGet "some-name"
        }
  in
    suite "init with DependentGet"
      [ test "the tree" <|
          assertEqual
            (StepTree.DependentGet
              { name = "some-name"
              , state = StepTree.StepStatePending
              , log = cookedLog
              })
            tree
      , test "using the focus" <|
          assertFocus "some-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.DependentGet
              { name = "some-name"
              , state = StepTree.StepStateSucceeded
              , log = cookedLog
              })
      ]

initAggregate : Test
initAggregate =
  let
    {tree, foci} =
      StepTree.init
        { id = "aggregate-id"
        , step =
            BuildPlan.Aggregate << Array.fromList <|
              [ { id = "task-a-id", step = BuildPlan.Task "task-a" }
              , { id = "task-b-id", step = BuildPlan.Task "task-b" }
              ]
        }
  in
    suite "init with Aggregate"
      [ test "the tree" <|
          assertEqual
            (StepTree.Aggregate << Array.fromList <|
              [ StepTree.Task
                  { name = "task-a"
                  , state = StepTree.StepStatePending
                  , log = cookedLog
                  }
              , StepTree.Task
                  { name = "task-b"
                  , state = StepTree.StepStatePending
                  , log = cookedLog
                  }
              ])
            tree
      , test "using the focus" <|
          assertFocus "task-a-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.Aggregate << Array.fromList <|
              [ StepTree.Task
                  { name = "task-a"
                  , state = StepTree.StepStateSucceeded
                  , log = cookedLog
                  }
              , StepTree.Task
                  { name = "task-b"
                  , state = StepTree.StepStatePending
                  , log = cookedLog
                  }
              ])
      ]

initOnSuccess : Test
initOnSuccess =
  let
    {tree, foci} =
      StepTree.init
        { id = "on-success-id"
        , step =
            BuildPlan.OnSuccess <| BuildPlan.HookedPlan
              { id = "task-a-id", step = BuildPlan.Task "task-a" }
              { id = "task-b-id", step = BuildPlan.Task "task-b" }
        }
  in
    suite "init with OnSuccess"
      [ test "the tree" <|
          assertEqual
            (StepTree.OnSuccess <| StepTree.HookedStep
              (StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStatePending
                , log = cookedLog
                })
              (StepTree.Task
                { name = "task-b"
                , state = StepTree.StepStatePending
                , log = cookedLog
                }))
            tree
      , test "updating a step via the focus" <|
          assertFocus "task-a-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.OnSuccess <| StepTree.HookedStep
              (StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStateSucceeded
                , log = cookedLog
                })
              (StepTree.Task
                { name = "task-b"
                , state = StepTree.StepStatePending
                , log = cookedLog
                }))
      , test "updating a hook via the focus" <|
          assertFocus "task-b-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.OnSuccess <| StepTree.HookedStep
              (StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStatePending
                , log = cookedLog
                })
              (StepTree.Task
                { name = "task-b"
                , state = StepTree.StepStateSucceeded
                , log = cookedLog
                }))
      ]

initOnFailure : Test
initOnFailure =
  let
    {tree, foci} =
      StepTree.init
        { id = "on-success-id"
        , step =
            BuildPlan.OnFailure <| BuildPlan.HookedPlan
              { id = "task-a-id", step = BuildPlan.Task "task-a" }
              { id = "task-b-id", step = BuildPlan.Task "task-b" }
        }
  in
    suite "init with OnFailure"
      [ test "the tree" <|
          assertEqual
            (StepTree.OnFailure <| StepTree.HookedStep
              (StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStatePending
                , log = cookedLog
                })
              (StepTree.Task
                { name = "task-b"
                , state = StepTree.StepStatePending
                , log = cookedLog
                }))
            tree
      , test "updating a step via the focus" <|
          assertFocus "task-a-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.OnFailure <| StepTree.HookedStep
              (StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStateSucceeded
                , log = cookedLog
                })
              (StepTree.Task
                { name = "task-b"
                , state = StepTree.StepStatePending
                , log = cookedLog
                }))
      , test "updating a hook via the focus" <|
          assertFocus "task-b-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.OnFailure <| StepTree.HookedStep
              (StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStatePending
                , log = cookedLog
                })
              (StepTree.Task
                { name = "task-b"
                , state = StepTree.StepStateSucceeded
                , log = cookedLog
                }))
      ]

initEnsure : Test
initEnsure =
  let
    {tree, foci} =
      StepTree.init
        { id = "on-success-id"
        , step =
            BuildPlan.Ensure <| BuildPlan.HookedPlan
              { id = "task-a-id", step = BuildPlan.Task "task-a" }
              { id = "task-b-id", step = BuildPlan.Task "task-b" }
        }
  in
    suite "init with Ensure"
      [ test "the tree" <|
          assertEqual
            (StepTree.Ensure <| StepTree.HookedStep
              (StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStatePending
                , log = cookedLog
                })
              (StepTree.Task
                { name = "task-b"
                , state = StepTree.StepStatePending
                , log = cookedLog
                }))
            tree
      , test "updating a step via the focus" <|
          assertFocus "task-a-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.Ensure <| StepTree.HookedStep
              (StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStateSucceeded
                , log = cookedLog
                })
              (StepTree.Task
                { name = "task-b"
                , state = StepTree.StepStatePending
                , log = cookedLog
                }))
      , test "updating a hook via the focus" <|
          assertFocus "task-b-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.Ensure <| StepTree.HookedStep
              (StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStatePending
                , log = cookedLog
                })
              (StepTree.Task
                { name = "task-b"
                , state = StepTree.StepStateSucceeded
                , log = cookedLog
                }))
      ]

initTry : Test
initTry =
  let
    {tree, foci} =
      StepTree.init
        { id = "on-success-id"
        , step =
            BuildPlan.Try { id = "task-a-id", step = BuildPlan.Task "task-a" }
        }
  in
    suite "init with Try"
      [ test "the tree" <|
          assertEqual
            (StepTree.Try <|
              StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStatePending
                , log = cookedLog
                })
            tree
      , test "updating a step via the focus" <|
          assertFocus "task-a-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.Try <|
              StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStateSucceeded
                , log = cookedLog
                })
      ]

initTimeout : Test
initTimeout =
  let
    {tree, foci} =
      StepTree.init
        { id = "on-success-id"
        , step =
            BuildPlan.Timeout { id = "task-a-id", step = BuildPlan.Task "task-a" }
        }
  in
    suite "init with Timeout"
      [ test "the tree" <|
          assertEqual
            (StepTree.Timeout <|
              StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStatePending
                , log = cookedLog
                })
            tree
      , test "updating a step via the focus" <|
          assertFocus "task-a-id" foci tree
            (\s -> { s | state = StepTree.StepStateSucceeded })
            (StepTree.Timeout <|
              StepTree.Task
                { name = "task-a"
                , state = StepTree.StepStateSucceeded
                , log = cookedLog
                })
      ]


updateStep : (StepTree.Step -> StepTree.Step) -> StepTree.StepTree -> StepTree.StepTree
updateStep f tree =
  case tree of
    StepTree.Task step ->
      StepTree.Task (f step)

    StepTree.Get step version ->
      StepTree.Get (f step) version

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
