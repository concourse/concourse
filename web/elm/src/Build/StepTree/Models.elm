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
    , finishTree
    , focusRetry
    , map
    , updateAt
    , wrapHook
    , wrapMultiStep
    , wrapStep
    )

import Ansi.Log
import Array exposing (Array)
import Concourse
import Concourse.BuildStatus exposing (BuildStatus)
import Dict exposing (Dict)
import Routes exposing (Highlight, StepID)
import Time


type alias StepTreeModel =
    { tree : StepTree
    , foci : Dict StepID StepFocus
    , highlight : Highlight
    }


type StepTree
    = Task Step
    | SetPipeline Step
    | ArtifactInput Step
    | Get Step
    | ArtifactOutput Step
    | Put Step
    | Aggregate (Array StepTree)
    | InParallel (Array StepTree)
    | Do (Array StepTree)
    | OnSuccess HookedStep
    | OnFailure HookedStep
    | OnAbort HookedStep
    | OnError HookedStep
    | Ensure HookedStep
    | Try StepTree
    | Retry StepID Int TabFocus (Array StepTree)
    | Timeout StepTree


type alias StepFocus =
    (StepTree -> StepTree) -> StepTree -> StepTree


type alias Step =
    { id : StepID
    , name : StepName
    , state : StepState
    , log : Ansi.Log.Model
    , error : Maybe String
    , expanded : Bool
    , version : Maybe Version
    , metadata : List MetadataField
    , firstOccurrence : Bool
    , timestamps : Dict Int Time.Posix
    , initialize : Maybe Time.Posix
    , start : Maybe Time.Posix
    , finish : Maybe Time.Posix
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


type alias Version =
    Dict String String


type alias MetadataField =
    { name : String
    , value : String
    }


type alias HookedStep =
    { step : StepTree
    , hook : StepTree
    }


type TabFocus
    = Auto
    | User


type alias BuildEventEnvelope =
    { data : BuildEvent
    , url : String
    }


type BuildEvent
    = BuildStatus BuildStatus Time.Posix
    | InitializeTask Origin Time.Posix
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
    | Log Origin String (Maybe Time.Posix)
    | Error Origin String Time.Posix
    | End
    | Opened
    | NetworkError


type alias Origin =
    { source : String
    , id : String
    }



-- model manipulation functions


focusRetry : Int -> StepTree -> StepTree
focusRetry tab tree =
    case tree of
        Retry id _ _ steps ->
            Retry id tab User steps

        _ ->
            -- impossible (non-retry tab focus)
            tree


updateAt : StepID -> (StepTree -> StepTree) -> StepTreeModel -> StepTreeModel
updateAt id update root =
    case Dict.get id root.foci of
        Nothing ->
            -- updateAt: id " ++ id ++ " not found"
            root

        Just focus ->
            { root | tree = focus update root.tree }


map : (Step -> Step) -> StepTree -> StepTree
map f tree =
    case tree of
        Task step ->
            Task (f step)

        Get step ->
            Get (f step)

        Put step ->
            Put (f step)

        SetPipeline step ->
            SetPipeline (f step)

        _ ->
            tree


wrapMultiStep : Int -> Dict StepID StepFocus -> Dict StepID StepFocus
wrapMultiStep i =
    Dict.map (\_ subFocus -> subFocus >> setMultiStepIndex i)


wrapStep : StepFocus -> StepFocus
wrapStep subFocus =
    subFocus >> updateStep


wrapHook : StepFocus -> StepFocus
wrapHook subFocus =
    subFocus >> updateHook


updateStep : (StepTree -> StepTree) -> StepTree -> StepTree
updateStep update tree =
    case tree of
        OnSuccess hookedStep ->
            OnSuccess { hookedStep | step = update hookedStep.step }

        OnFailure hookedStep ->
            OnFailure { hookedStep | step = update hookedStep.step }

        OnAbort hookedStep ->
            OnAbort { hookedStep | step = update hookedStep.step }

        OnError hookedStep ->
            OnError { hookedStep | step = update hookedStep.step }

        Ensure hookedStep ->
            Ensure { hookedStep | step = update hookedStep.step }

        Try step ->
            Try (update step)

        Timeout step ->
            Timeout (update step)

        _ ->
            --impossible
            tree


updateHook : (StepTree -> StepTree) -> StepTree -> StepTree
updateHook update tree =
    case tree of
        OnSuccess hookedStep ->
            OnSuccess { hookedStep | hook = update hookedStep.hook }

        OnFailure hookedStep ->
            OnFailure { hookedStep | hook = update hookedStep.hook }

        OnAbort hookedStep ->
            OnAbort { hookedStep | hook = update hookedStep.hook }

        OnError hookedStep ->
            OnError { hookedStep | hook = update hookedStep.hook }

        Ensure hookedStep ->
            Ensure { hookedStep | hook = update hookedStep.hook }

        _ ->
            -- impossible
            tree


getMultiStepIndex : Int -> StepTree -> StepTree
getMultiStepIndex idx tree =
    let
        steps =
            case tree of
                Aggregate trees ->
                    trees

                InParallel trees ->
                    trees

                Do trees ->
                    trees

                Retry _ _ _ trees ->
                    trees

                _ ->
                    -- impossible
                    Array.fromList []
    in
    case Array.get idx steps of
        Just sub ->
            sub

        Nothing ->
            -- impossible
            tree


setMultiStepIndex : Int -> (StepTree -> StepTree) -> StepTree -> StepTree
setMultiStepIndex idx update tree =
    case tree of
        Aggregate trees ->
            Aggregate (Array.set idx (update (getMultiStepIndex idx tree)) trees)

        InParallel trees ->
            InParallel (Array.set idx (update (getMultiStepIndex idx tree)) trees)

        Do trees ->
            Do (Array.set idx (update (getMultiStepIndex idx tree)) trees)

        Retry id tab focus trees ->
            let
                updatedSteps =
                    Array.set idx (update (getMultiStepIndex idx tree)) trees
            in
            case focus of
                Auto ->
                    Retry id (idx + 1) Auto updatedSteps

                User ->
                    Retry id tab User updatedSteps

        _ ->
            -- impossible
            tree


finishTree : StepTree -> StepTree
finishTree root =
    case root of
        Task step ->
            Task (finishStep step)

        ArtifactInput step ->
            ArtifactInput (finishStep step)

        Get step ->
            Get (finishStep step)

        ArtifactOutput step ->
            ArtifactOutput { step | state = StepStateSucceeded }

        Put step ->
            Put (finishStep step)

        SetPipeline step ->
            SetPipeline (finishStep step)

        Aggregate trees ->
            Aggregate (Array.map finishTree trees)

        InParallel trees ->
            InParallel (Array.map finishTree trees)

        Do trees ->
            Do (Array.map finishTree trees)

        OnSuccess hookedStep ->
            OnSuccess (finishHookedStep hookedStep)

        OnFailure hookedStep ->
            OnFailure (finishHookedStep hookedStep)

        OnAbort hookedStep ->
            OnAbort (finishHookedStep hookedStep)

        OnError hookedStep ->
            OnError (finishHookedStep hookedStep)

        Ensure hookedStep ->
            Ensure (finishHookedStep hookedStep)

        Try tree ->
            Try (finishTree tree)

        Retry id tab focus trees ->
            Retry id tab focus (Array.map finishTree trees)

        Timeout tree ->
            Timeout (finishTree tree)


finishStep : Step -> Step
finishStep step =
    let
        newState =
            case step.state of
                StepStateRunning ->
                    StepStateInterrupted

                StepStatePending ->
                    StepStateCancelled

                otherwise ->
                    otherwise
    in
    { step | state = newState }


finishHookedStep : HookedStep -> HookedStep
finishHookedStep hooked =
    { hooked
        | step = finishTree hooked.step
        , hook = finishTree hooked.hook
    }
