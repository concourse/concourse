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
import Date exposing (Date)
import Dict exposing (Dict)
import Routes exposing (Highlight, StepID)


type alias StepTreeModel =
    { tree : StepTree
    , foci : Dict StepID StepFocus
    , highlight : Highlight
    , tooltip : Maybe StepID
    }


type StepTree
    = Task Step
    | Get Step
    | Put Step
    | Aggregate (Array StepTree)
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
    { update : (StepTree -> StepTree) -> StepTree -> StepTree }


type alias Step =
    { id : StepID
    , name : StepName
    , state : StepState
    , log : Ansi.Log.Model
    , error : Maybe String
    , expanded : Maybe Bool
    , version : Maybe Version
    , metadata : List MetadataField
    , firstOccurrence : Bool
    , timestamps : Dict Int Date
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
    = BuildStatus Concourse.BuildStatus Date
    | Initialize Origin
    | StartTask Origin
    | FinishTask Origin Int
    | FinishGet Origin Int Concourse.Version Concourse.Metadata
    | FinishPut Origin Int Concourse.Version Concourse.Metadata
    | Log Origin String (Maybe Date)
    | Error Origin String
    | BuildError String
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
            Debug.crash "impossible (non-retry tab focus)"


updateAt : StepID -> (StepTree -> StepTree) -> StepTreeModel -> StepTreeModel
updateAt id update root =
    case Dict.get id root.foci of
        Nothing ->
            Debug.crash ("updateAt: id " ++ id ++ " not found")

        Just focus ->
            { root | tree = focus.update update root.tree }


map : (Step -> Step) -> StepTree -> StepTree
map f tree =
    case tree of
        Task step ->
            Task (f step)

        Get step ->
            Get (f step)

        Put step ->
            Put (f step)

        _ ->
            tree


wrapMultiStep : Int -> Dict StepID StepFocus -> Dict StepID StepFocus
wrapMultiStep i =
    Dict.map (\_ subFocus -> { update = \upd tree -> setMultiStepIndex i (subFocus.update upd) tree })


wrapStep : StepID -> StepFocus -> StepFocus
wrapStep id subFocus =
    { update = \upd tree -> updateStep (subFocus.update upd) tree }


wrapHook : StepID -> StepFocus -> StepFocus
wrapHook id subFocus =
    { update = \upd tree -> updateHook (subFocus.update upd) tree }


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
            Debug.crash "impossible"


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
            Debug.crash "impossible"


getMultiStepIndex : Int -> StepTree -> StepTree
getMultiStepIndex idx tree =
    let
        steps =
            case tree of
                Aggregate trees ->
                    trees

                Do trees ->
                    trees

                Retry _ _ _ trees ->
                    trees

                _ ->
                    Debug.crash "impossible"
    in
    case Array.get idx steps of
        Just sub ->
            sub

        Nothing ->
            Debug.crash "impossible"


setMultiStepIndex : Int -> (StepTree -> StepTree) -> StepTree -> StepTree
setMultiStepIndex idx update tree =
    case tree of
        Aggregate trees ->
            Aggregate (Array.set idx (update (getMultiStepIndex idx tree)) trees)

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
            Debug.crash "impossible"


finishTree : StepTree -> StepTree
finishTree root =
    case root of
        Task step ->
            Task (finishStep step)

        Get step ->
            Get (finishStep step)

        Put step ->
            Put (finishStep step)

        Aggregate trees ->
            Aggregate (Array.map finishTree trees)

        Do trees ->
            Do (Array.map finishTree trees)

        OnSuccess hookedStep ->
            OnSuccess { hookedStep | step = finishTree hookedStep.step }

        OnFailure hookedStep ->
            OnFailure { hookedStep | step = finishTree hookedStep.step }

        OnAbort hookedStep ->
            OnAbort { hookedStep | step = finishTree hookedStep.step }
        
        OnError hookedStep ->
            OnError { hookedStep | step = finishTree hookedStep.step }

        Ensure hookedStep ->
            Ensure { hookedStep | step = finishTree hookedStep.step }

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
