module Build.Output.Output exposing
    ( filterHoverState
    , handleEnvelopes
    , handleStepTreeMsg
    , init
    , planAndResourcesFetched
    , view
    )

import Ansi.Log
import Api.Endpoints as Endpoints
import Array
import Build.Output.Models exposing (OutputModel, OutputState(..))
import Build.StepTree.Models as StepTree
    exposing
        ( BuildEvent(..)
        , BuildEventEnvelope
        , Step
        , StepState(..)
        , StepTreeModel
        )
import Build.StepTree.StepTree
import Concourse
import Concourse.BuildStatus
import Dict
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (class)
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Routes exposing (StepID)
import Time
import Views.LoadingIndicator as LoadingIndicator


init : Routes.Highlight -> Concourse.Build -> ( OutputModel, List Effect )
init highlight build =
    let
        outputState =
            if Concourse.BuildStatus.isRunning build.status then
                StepsLiveUpdating

            else
                StepsLoading

        model =
            { steps = Nothing
            , state = outputState
            , eventStreamUrlPath = Nothing
            , eventSourceOpened = False
            , highlight = highlight
            }

        fetch =
            if build.job /= Nothing then
                [ FetchBuildPlanAndResources build.id ]

            else
                [ FetchBuildPlan build.id ]
    in
    ( model, fetch )


handleStepTreeMsg :
    (StepTreeModel -> ( StepTreeModel, List Effect ))
    -> OutputModel
    -> ( OutputModel, List Effect )
handleStepTreeMsg action model =
    case model.steps of
        Just st ->
            let
                ( newModel, effects ) =
                    action st
            in
            ( { model | steps = Just newModel }, effects )

        _ ->
            ( model, [] )


planAndResourcesFetched :
    Concourse.BuildId
    -> ( Concourse.BuildPlan, Concourse.BuildResources )
    -> OutputModel
    -> ( OutputModel, List Effect )
planAndResourcesFetched buildId ( plan, resources ) model =
    let
        url =
            Endpoints.BuildEventStream
                |> Endpoints.Build buildId
                |> Endpoints.toString []
    in
    ( { model
        | steps =
            Just
                (Build.StepTree.StepTree.init
                    model.highlight
                    resources
                    plan
                )
        , eventStreamUrlPath = Just url
      }
    , []
    )


handleEnvelopes :
    List BuildEventEnvelope
    -> OutputModel
    -> ( OutputModel, List Effect )
handleEnvelopes envelopes model =
    envelopes
        |> List.reverse
        |> List.foldr handleEnvelope ( model, [] )


handleEnvelope :
    BuildEventEnvelope
    -> ( OutputModel, List Effect )
    -> ( OutputModel, List Effect )
handleEnvelope { url, data } ( model, effects ) =
    if
        model.eventStreamUrlPath
            |> Maybe.map (\p -> String.endsWith p url)
            |> Maybe.withDefault False
    then
        handleEvent data ( model, effects )

    else
        ( model, effects )


handleEvent :
    BuildEvent
    -> ( OutputModel, List Effect )
    -> ( OutputModel, List Effect )
handleEvent event ( model, effects ) =
    case event of
        Opened ->
            ( { model | eventSourceOpened = True }
            , effects
            )

        Log origin output time ->
            ( updateStep origin.id (setRunning << appendStepLog output time) model
            , effects
            )

        WaitingForWorker origin time ->
            ( updateStep origin.id (setRunning << appendStepLog "\u{001B}[1mno suitable workers found, waiting for worker...\u{001B}[0m\n" time) model
            , effects
            )

        SelectedWorker origin output time ->
            ( updateStep origin.id (setRunning << appendStepLog ("\u{001B}[1mselected worker: \u{001B}[0m" ++ output ++ "\n") time) model
            , effects
            )

        Error origin message time ->
            ( updateStep origin.id (setStepError message time) model
            , effects
            )

        InitializeTask origin time ->
            ( updateStep origin.id (setInitialize time) model
            , effects
            )

        StartTask origin time ->
            ( updateStep origin.id (setStart time) model
            , effects
            )

        FinishTask origin exitStatus time ->
            ( updateStep origin.id (finishStep (exitStatus == 0) (Just time)) model
            , effects
            )

        Initialize origin time ->
            ( updateStep origin.id (setInitialize time) model
            , effects
            )

        Start origin time ->
            ( updateStep origin.id (setStart time) model
            , effects
            )

        Finish origin time succeeded ->
            ( updateStep origin.id (finishStep succeeded (Just time)) model
            , effects
            )

        InitializeGet origin time ->
            ( updateStep origin.id (setInitialize time) model
            , effects
            )

        StartGet origin time ->
            ( updateStep origin.id (setStart time) model
            , effects
            )

        FinishGet origin exitStatus version metadata time ->
            ( updateStep origin.id (finishStep (exitStatus == 0) time << setResourceInfo version metadata) model
            , effects
            )

        InitializePut origin time ->
            ( updateStep origin.id (setInitialize time) model
            , effects
            )

        StartPut origin time ->
            ( updateStep origin.id (setStart time) model
            , effects
            )

        FinishPut origin exitStatus version metadata time ->
            ( updateStep origin.id (finishStep (exitStatus == 0) time << setResourceInfo version metadata) model
            , effects
            )

        SetPipelineChanged origin changed ->
            ( updateStep origin.id (setSetPipelineChanged changed) model
            , effects
            )

        BuildStatus status _ ->
            let
                newSt =
                    model.steps
                        |> Maybe.map
                            (\st ->
                                if Concourse.BuildStatus.isRunning status then
                                    st

                                else
                                    Build.StepTree.StepTree.finished st
                            )
            in
            ( { model | steps = newSt }, effects )

        ImageCheck { id } plan ->
            ( { model | steps = Maybe.map (Build.StepTree.StepTree.setImageCheck id plan) model.steps }
            , effects
            )

        ImageGet { id } plan ->
            ( { model | steps = Maybe.map (Build.StepTree.StepTree.setImageGet id plan) model.steps }
            , effects
            )

        End ->
            ( { model | state = StepsComplete, eventStreamUrlPath = Nothing }
            , effects
            )

        NetworkError ->
            ( model, effects )


updateStep : StepID -> (Step -> Step) -> OutputModel -> OutputModel
updateStep id update model =
    { model | steps = Maybe.map (StepTree.updateAt id update) model.steps }


setRunning : Step -> Step
setRunning =
    setStepState StepStateRunning


appendStepLog : String -> Maybe Time.Posix -> Step -> Step
appendStepLog output mtime step =
    let
        outputLineCount =
            Ansi.Log.update output (Ansi.Log.init Ansi.Log.Cooked)
                |> .lines
                |> Array.length

        lastLineNo =
            max (Array.length step.log.lines) 1

        setLineTimestamp lineNo timestamps =
            Dict.update lineNo (always mtime) timestamps

        newTimestamps =
            List.foldl
                setLineTimestamp
                step.timestamps
                (List.range lastLineNo (lastLineNo + outputLineCount - 1))

        newLog =
            Ansi.Log.update output step.log
    in
    { step | log = newLog, timestamps = newTimestamps }


setStepError : String -> Time.Posix -> Step -> Step
setStepError message time step =
    { step
        | state = StepStateErrored
        , error = Just message
        , finish = Just time
    }


setStart : Time.Posix -> Step -> Step
setStart time step =
    setStepStart time (setStepState StepStateRunning step)


setInitialize : Time.Posix -> Step -> Step
setInitialize time step =
    setStepInitialize time (setStepState StepStateRunning step)


finishStep : Bool -> Maybe Time.Posix -> Step -> Step
finishStep succeeded mtime step =
    let
        stepState =
            if succeeded then
                StepStateSucceeded

            else
                StepStateFailed
    in
    setStepFinish mtime (setStepState stepState step)


setResourceInfo : Concourse.Version -> Concourse.Metadata -> Step -> Step
setResourceInfo version metadata step =
    { step | version = Just version, metadata = metadata }


setStepState : StepState -> Step -> Step
setStepState state step =
    { step | state = state }


setStepInitialize : Time.Posix -> Step -> Step
setStepInitialize time step =
    { step | initialize = Just time }


setStepStart : Time.Posix -> Step -> Step
setStepStart time step =
    { step | start = Just time }


setStepFinish : Maybe Time.Posix -> Step -> Step
setStepFinish mtime step =
    { step | finish = mtime }


setSetPipelineChanged : Bool -> Step -> Step
setSetPipelineChanged changed step =
    { step | changed = changed }


view :
    { timeZone : Time.Zone, hovered : HoverState.HoverState }
    -> OutputModel
    -> Html Message
view session { steps, state } =
    Html.div [ class "steps" ] [ viewStepTree session steps state ]


viewStepTree :
    { timeZone : Time.Zone, hovered : HoverState.HoverState }
    -> Maybe StepTreeModel
    -> OutputState
    -> Html Message
viewStepTree session steps state =
    case ( state, steps ) of
        ( StepsLoading, _ ) ->
            LoadingIndicator.view

        ( StepsLiveUpdating, Just root ) ->
            Build.StepTree.StepTree.view session root

        ( StepsComplete, Just root ) ->
            Build.StepTree.StepTree.view session root

        ( _, Nothing ) ->
            Html.div [] []


filterHoverState : HoverState.HoverState -> HoverState.HoverState
filterHoverState hovered =
    case hovered of
        HoverState.TooltipPending (StepState _) ->
            hovered

        HoverState.Tooltip (StepState _) _ ->
            hovered

        HoverState.Hovered (StepTab _ _) ->
            hovered

        _ ->
            HoverState.NoHover
