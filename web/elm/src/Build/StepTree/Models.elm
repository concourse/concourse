module Build.StepTree.Models exposing
    ( BuildEvent(..)
    , BuildEventEnvelope
    , HookedStep
    , MetadataField
    , Origin
    , Step
    , StepFocus
    , StepName
    , StepState(..)
    , StepTree(..)
    , StepTreeModel
    , TabFocus(..)
    , Version
    , focusTabbed
    , isActive
    , lastActive
    , mostSevereStepState
    , showStepState
    , toggleSubHeaderExpanded
    , treeIsActive
    , updateAt
    , updateTreeNodeAt
    )

import Ansi.Log
import Array exposing (Array)
import Concourse
import Concourse.BuildStatus exposing (BuildStatus)
import Dict exposing (Dict)
import List.Extra
import Maybe.Extra
import Ordering exposing (Ordering)
import Routes exposing (Highlight, StepID)
import Time


type alias StepTreeModel =
    { tree : StepTree
    , steps : Dict StepID Step
    , highlight : Highlight
    , resources : Concourse.BuildResources
    , buildId : Maybe Concourse.JobBuildIdentifier
    }


type StepTree
    = Task StepID
    | Check StepID
    | Get StepID
    | Run StepID
    | Put StepID
    | SetPipeline StepID
    | LoadVar StepID
    | ArtifactInput StepID
    | ArtifactOutput StepID
    | InParallel (Array StepTree)
    | Across StepID (List String) (List (List Concourse.JsonValue)) (Array StepTree)
    | Retry StepID (Array StepTree)
    | Do (Array StepTree)
    | OnSuccess HookedStep
    | OnFailure HookedStep
    | OnAbort HookedStep
    | OnError HookedStep
    | Ensure HookedStep
    | Try StepTree
    | Timeout StepTree


type alias HookedStep =
    { step : StepTree
    , hook : StepTree
    }


type alias StepFocus =
    (StepTree -> StepTree) -> StepTree -> StepTree


type alias Step =
    { id : StepID
    , buildStep : Concourse.BuildStep
    , state : StepState
    , log : Ansi.Log.Model
    , error : Maybe String
    , expanded : Bool
    , version : Maybe Version
    , metadata : List MetadataField
    , changed : Bool
    , timestamps : Dict Int Time.Posix
    , initialize : Maybe Time.Posix
    , start : Maybe Time.Posix
    , finish : Maybe Time.Posix
    , tabFocus : TabFocus
    , expandedHeaders : Dict Int Bool
    , initializationExpanded : Bool
    , imageCheck : Maybe StepTree
    , imageGet : Maybe StepTree
    }


type alias StepName =
    String


type StepState
    = StepStatePending
    | StepStateRunning
    | StepStateInterrupted
    | StepStateCancelled
    | StepStateSucceeded
    | StepStateFailed
    | StepStateErrored


showStepState : StepState -> String
showStepState state =
    case state of
        StepStatePending ->
            "pending"

        StepStateRunning ->
            "running"

        StepStateInterrupted ->
            "interrupted"

        StepStateCancelled ->
            "cancelled"

        StepStateSucceeded ->
            "succeeded"

        StepStateFailed ->
            "failed"

        StepStateErrored ->
            "errored"


stepStateOrdering : Ordering StepState
stepStateOrdering =
    Ordering.explicit
        [ StepStateFailed
        , StepStateErrored
        , StepStateInterrupted
        , StepStateCancelled
        , StepStateRunning
        , StepStatePending
        , StepStateSucceeded
        ]


mostSevereStepState : StepTreeModel -> StepTree -> StepState
mostSevereStepState model stepTree =
    activeTreeSteps model stepTree
        |> List.foldl
            (\step state ->
                case stepStateOrdering step.state state of
                    LT ->
                        step.state

                    _ ->
                        state
            )
            StepStateSucceeded


type alias Version =
    Dict String String


type alias MetadataField =
    { name : String
    , value : String
    }


type TabFocus
    = Auto
    | Manual Int


type alias BuildEventEnvelope =
    { data : BuildEvent
    , url : String
    }


type BuildEvent
    = BuildStatus BuildStatus Time.Posix
    | InitializeTask Origin Time.Posix
    | InitializeCheck Origin Time.Posix String
    | StartTask Origin Time.Posix
    | FinishTask Origin Int Time.Posix
    | Initialize Origin Time.Posix
    | Start Origin Time.Posix
    | Finish Origin Time.Posix Bool
    | InitializeGet Origin Time.Posix
    | StartGet Origin Time.Posix
    | FinishGet Origin Int Concourse.Version Concourse.Metadata (Maybe Time.Posix)
    | InitializePut Origin Time.Posix
    | StartPut Origin Time.Posix
    | FinishPut Origin Int Concourse.Version Concourse.Metadata (Maybe Time.Posix)
    | SetPipelineChanged Origin Bool
    | Log Origin String (Maybe Time.Posix)
    | WaitingForWorker Origin (Maybe Time.Posix)
    | SelectedWorker Origin String (Maybe Time.Posix)
    | StreamingVolume Origin String String (Maybe Time.Posix)
    | Error Origin String Time.Posix
    | ImageCheck Origin Concourse.BuildPlan
    | ImageGet Origin Concourse.BuildPlan
    | AcrossSubsteps Origin (List Concourse.AcrossSubstep)
    | End
    | Opened
    | NetworkError


type alias Origin =
    { source : String
    , id : String
    }



-- model manipulation functions


focusTabbed : Int -> Step -> Step
focusTabbed tab step =
    { step | tabFocus = Manual tab }


toggleSubHeaderExpanded : Int -> Step -> Step
toggleSubHeaderExpanded idx step =
    { step | expandedHeaders = Dict.update idx (Just << not << Maybe.withDefault False) step.expandedHeaders }


updateAt : StepID -> (Step -> Step) -> StepTreeModel -> StepTreeModel
updateAt id update model =
    { model | steps = Dict.update id (Maybe.map update) model.steps }


activeStepIds : StepTreeModel -> StepTree -> List StepID
activeStepIds model tree =
    let
        hooked step hook state =
            activeStepIds model step
                ++ (if mostSevereStepState model step == state then
                        activeStepIds model hook

                    else
                        []
                   )
    in
    case tree of
        Task stepId ->
            [ stepId ]

        Check stepId ->
            [ stepId ]

        Get stepId ->
            [ stepId ]

        Run stepId ->
            [ stepId ]

        Put stepId ->
            [ stepId ]

        ArtifactInput stepId ->
            [ stepId ]

        ArtifactOutput stepId ->
            [ stepId ]

        SetPipeline stepId ->
            [ stepId ]

        LoadVar stepId ->
            [ stepId ]

        InParallel trees ->
            List.concatMap (activeStepIds model) (Array.toList trees)

        Do trees ->
            List.concatMap (activeStepIds model) (Array.toList trees)

        Across _ _ _ trees ->
            List.concatMap (activeStepIds model) (Array.toList trees)

        OnSuccess { step, hook } ->
            hooked step hook StepStateSucceeded

        OnFailure { step, hook } ->
            hooked step hook StepStateFailed

        OnAbort { step, hook } ->
            hooked step hook StepStateInterrupted

        OnError { step, hook } ->
            hooked step hook StepStateErrored

        Ensure { step, hook } ->
            activeStepIds model step ++ activeStepIds model hook

        Try subTree ->
            activeStepIds model subTree

        Timeout subTree ->
            activeStepIds model subTree

        Retry _ trees ->
            trees
                |> Array.toList
                |> List.Extra.takeWhile (mostSevereStepState model >> (/=) StepStateSucceeded)
                |> List.concatMap (activeStepIds model)


updateTreeNodeAt : StepID -> (StepTree -> StepTree) -> StepTree -> StepTree
updateTreeNodeAt id fn tree =
    let
        updateSelf stepId =
            if stepId == id then
                fn tree

            else
                tree
    in
    case tree of
        Task stepId ->
            updateSelf stepId

        Check stepId ->
            updateSelf stepId

        Get stepId ->
            updateSelf stepId

        Run stepId ->
            updateSelf stepId

        Put stepId ->
            updateSelf stepId

        ArtifactInput stepId ->
            updateSelf stepId

        ArtifactOutput stepId ->
            updateSelf stepId

        SetPipeline stepId ->
            updateSelf stepId

        LoadVar stepId ->
            updateSelf stepId

        InParallel trees ->
            InParallel <| Array.map (updateTreeNodeAt id fn) trees

        Do trees ->
            Do <| Array.map (updateTreeNodeAt id fn) trees

        Across stepId vars vals trees ->
            let
                withUpdatedChildren =
                    Across stepId vars vals <| Array.map (updateTreeNodeAt stepId fn) trees
            in
            if stepId == id then
                fn withUpdatedChildren

            else
                withUpdatedChildren

        OnSuccess { step, hook } ->
            OnSuccess
                { step = updateTreeNodeAt id fn step
                , hook = updateTreeNodeAt id fn hook
                }

        OnFailure { step, hook } ->
            OnFailure
                { step = updateTreeNodeAt id fn step
                , hook = updateTreeNodeAt id fn hook
                }

        OnAbort { step, hook } ->
            OnAbort
                { step = updateTreeNodeAt id fn step
                , hook = updateTreeNodeAt id fn hook
                }

        OnError { step, hook } ->
            OnError
                { step = updateTreeNodeAt id fn step
                , hook = updateTreeNodeAt id fn hook
                }

        Ensure { step, hook } ->
            Ensure
                { step = updateTreeNodeAt id fn step
                , hook = updateTreeNodeAt id fn hook
                }

        Try subTree ->
            Try <| updateTreeNodeAt id fn subTree

        Timeout subTree ->
            Timeout <| updateTreeNodeAt id fn subTree

        Retry stepId trees ->
            let
                withUpdatedChildren =
                    Retry stepId <| Array.map (updateTreeNodeAt stepId fn) trees
            in
            if stepId == id then
                fn withUpdatedChildren

            else
                withUpdatedChildren


activeTreeSteps : StepTreeModel -> StepTree -> List Step
activeTreeSteps model stepTree =
    activeStepIds model stepTree
        |> List.map (\id -> Dict.get id model.steps)
        |> Maybe.Extra.values


treeIsActive : StepTreeModel -> StepTree -> Bool
treeIsActive model stepTree =
    activeTreeSteps model stepTree
        |> List.any (.state >> isActive)


lastActive : StepTreeModel -> Array StepTree -> Maybe Int
lastActive model trees =
    Array.toIndexedList trees
        |> List.reverse
        |> List.filter (Tuple.second >> treeIsActive model)
        |> List.head
        |> Maybe.map Tuple.first


isActive : StepState -> Bool
isActive state =
    state /= StepStatePending && state /= StepStateCancelled
