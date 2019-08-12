module Build.Output.Output exposing
    ( OutMsg(..)
    , handleEnvelopes
    , handleStepTreeMsg
    , init
    , planAndResourcesFetched
    , view
    )

import Ansi.Log
import Array
import Build.Output.Models exposing (OutputModel, OutputState(..))
import Build.StepTree.Models as StepTree
    exposing
        ( BuildEvent(..)
        , BuildEventEnvelope
        , StepState(..)
        , StepTree
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
import Message.Message exposing (Message(..))
import Routes exposing (StepID)
import Time
import Views.LoadingIndicator as LoadingIndicator


type OutMsg
    = OutNoop
    | OutBuildStatus Concourse.BuildStatus Time.Posix


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
    -> ( OutputModel, List Effect, OutMsg )
handleStepTreeMsg action model =
    case model.steps of
        Just st ->
            let
                ( newModel, effects ) =
                    action st
            in
            ( { model | steps = Just newModel }, effects, OutNoop )

        _ ->
            ( model, [], OutNoop )


planAndResourcesFetched :
    Concourse.BuildId
    -> ( Concourse.BuildPlan, Concourse.BuildResources )
    -> OutputModel
    -> ( OutputModel, List Effect, OutMsg )
planAndResourcesFetched buildId ( plan, resources ) model =
    let
        url =
            "/api/v1/builds/" ++ String.fromInt buildId ++ "/events"
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
    , OutNoop
    )


handleEnvelopes :
    List BuildEventEnvelope
    -> OutputModel
    -> ( OutputModel, List Effect, OutMsg )
handleEnvelopes envelopes model =
    envelopes
        |> List.reverse
        |> List.foldr handleEnvelope ( model, [], OutNoop )


handleEnvelope :
    BuildEventEnvelope
    -> ( OutputModel, List Effect, OutMsg )
    -> ( OutputModel, List Effect, OutMsg )
handleEnvelope { url, data } ( model, effects, outmsg ) =
    if
        model.eventStreamUrlPath
            |> Maybe.map (\p -> String.endsWith p url)
            |> Maybe.withDefault False
    then
        handleEvent data ( model, effects, outmsg )

    else
        ( model, effects, outmsg )


handleEvent :
    BuildEvent
    -> ( OutputModel, List Effect, OutMsg )
    -> ( OutputModel, List Effect, OutMsg )
handleEvent event ( model, effects, outmsg ) =
    case event of
        Opened ->
            ( { model | eventSourceOpened = True }
            , effects
            , outmsg
            )

        Log origin output time ->
            ( updateStep origin.id (setRunning << appendStepLog output time) model
            , effects
            , outmsg
            )

        Error origin message time ->
            ( updateStep origin.id (setStepError message time) model
            , effects
            , outmsg
            )

        InitializeTask origin time ->
            ( updateStep origin.id (setInitialize time) model
            , effects
            , outmsg
            )

        StartTask origin time ->
            ( updateStep origin.id (setStart time) model
            , effects
            , outmsg
            )

        FinishTask origin exitStatus time ->
            ( updateStep origin.id (finishStep exitStatus (Just time)) model
            , effects
            , outmsg
            )

        InitializeGet origin time ->
            ( updateStep origin.id (setInitialize time) model
            , effects
            , outmsg
            )

        StartGet origin time ->
            ( updateStep origin.id (setStart time) model
            , effects
            , outmsg
            )

        FinishGet origin exitStatus version metadata time ->
            ( updateStep origin.id (finishStep exitStatus time << setResourceInfo version metadata) model
            , effects
            , outmsg
            )

        InitializePut origin time ->
            ( updateStep origin.id (setInitialize time) model
            , effects
            , outmsg
            )

        StartPut origin time ->
            ( updateStep origin.id (setStart time) model
            , effects
            , outmsg
            )

        FinishPut origin exitStatus version metadata time ->
            ( updateStep origin.id (finishStep exitStatus time << setResourceInfo version metadata) model
            , effects
            , outmsg
            )

        BuildStatus status date ->
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
            ( { model | steps = newSt }, effects, OutBuildStatus status date )

        End ->
            ( { model | state = StepsComplete, eventStreamUrlPath = Nothing }
            , effects
            , outmsg
            )

        NetworkError ->
            ( model, effects, outmsg )


updateStep : StepID -> (StepTree -> StepTree) -> OutputModel -> OutputModel
updateStep id update model =
    { model | steps = Maybe.map (StepTree.updateAt id update) model.steps }


setRunning : StepTree -> StepTree
setRunning =
    setStepState StepStateRunning


appendStepLog : String -> Maybe Time.Posix -> StepTree -> StepTree
appendStepLog output mtime tree =
    (\a -> StepTree.map a tree) <|
        \step ->
            let
                outputLineCount =
                    Ansi.Log.update output (Ansi.Log.init Ansi.Log.Cooked) |> .lines |> Array.length

                logLineCount =
                    max (Array.length step.log.lines - 1) 0

                setLineTimestamp line timestamps =
                    Dict.update line (always mtime) timestamps

                newTimestamps =
                    List.foldl
                        setLineTimestamp
                        step.timestamps
                        (List.range logLineCount (logLineCount + outputLineCount - 1))

                newLog =
                    Ansi.Log.update output step.log
            in
            { step | log = newLog, timestamps = newTimestamps }


setStepError : String -> Time.Posix -> StepTree -> StepTree
setStepError message time tree =
    StepTree.map
        (\step ->
            { step
                | state = StepStateErrored
                , error = Just message
                , finish = Just time
            }
        )
        tree


setStart : Time.Posix -> StepTree -> StepTree
setStart time tree =
    setStepStart time (setStepState StepStateRunning tree)


setInitialize : Time.Posix -> StepTree -> StepTree
setInitialize time tree =
    setStepInitialize time (setStepState StepStateRunning tree)


finishStep : Int -> Maybe Time.Posix -> StepTree -> StepTree
finishStep exitStatus mtime tree =
    let
        stepState =
            if exitStatus == 0 then
                StepStateSucceeded

            else
                StepStateFailed
    in
    setStepFinish mtime (setStepState stepState tree)


setResourceInfo : Concourse.Version -> Concourse.Metadata -> StepTree -> StepTree
setResourceInfo version metadata tree =
    StepTree.map (\step -> { step | version = Just version, metadata = metadata }) tree


setStepState : StepState -> StepTree -> StepTree
setStepState state tree =
    StepTree.map (\step -> { step | state = state }) tree


setStepInitialize : Time.Posix -> StepTree -> StepTree
setStepInitialize time tree =
    StepTree.map (\step -> { step | initialize = Just time }) tree


setStepStart : Time.Posix -> StepTree -> StepTree
setStepStart time tree =
    StepTree.map (\step -> { step | start = Just time }) tree


setStepFinish : Maybe Time.Posix -> StepTree -> StepTree
setStepFinish mtime tree =
    StepTree.map (\step -> { step | finish = mtime }) tree


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
