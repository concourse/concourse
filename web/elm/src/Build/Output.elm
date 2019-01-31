module Build.Output exposing
    ( OutMsg(..)
    , handleEventsMsg
    , handleStepTreeMsg
    , init
    , planAndResourcesFetched
    , subscribeToEvents
    , view
    )

import Ansi.Log
import Array exposing (Array)
import Build.Models
    exposing
        ( BuildEvent(..)
        , OutputModel
        , OutputState(..)
        , StepID
        , StepState(..)
        , StepTree
        , StepTreeModel
        )
import Build.Msgs exposing (EventsMsg(..), Msg(..))
import Build.StepTree as StepTree
import Build.Styles as Styles
import Concourse
import Concourse.BuildEvents as BuildEvents
import Concourse.BuildStatus
import Date exposing (Date)
import Dict exposing (Dict)
import Effects exposing (Effect(..))
import EventSource
import Html exposing (Html)
import Html.Attributes
    exposing
        ( action
        , class
        , classList
        , id
        , method
        , style
        , title
        )
import Http
import LoadingIndicator
import NotAuthorized
import Subscription


type OutMsg
    = OutNoop
    | OutBuildStatus Concourse.BuildStatus Date


type alias Flags =
    { hash : String }


init : Flags -> Concourse.Build -> ( OutputModel, List Effect )
init flags build =
    let
        outputState =
            if Concourse.BuildStatus.isRunning build.status then
                StepsLiveUpdating

            else
                StepsLoading

        model =
            { steps = Nothing
            , errors = Nothing
            , state = outputState
            , events = Nothing
            , eventSourceOpened = False
            , highlight = StepTree.parseHighlight flags.hash
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
    -> Result Http.Error ( Concourse.BuildPlan, Concourse.BuildResources )
    -> OutputModel
    -> ( OutputModel, List Effect, OutMsg )
planAndResourcesFetched buildId result model =
    ( case result of
        Err err ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        { model | events = Just buildId }

                    else
                        model

                _ ->
                    flip always (Debug.log "failed to fetch plan" err) <|
                        model

        Ok ( plan, resources ) ->
            { model
                | steps = Just (StepTree.init model.highlight resources plan)
                , events = Just buildId
            }
    , []
    , OutNoop
    )


handleEventsMsg :
    EventsMsg
    -> OutputModel
    -> ( OutputModel, List Effect, OutMsg )
handleEventsMsg action model =
    case action of
        Opened ->
            ( { model | eventSourceOpened = True }, [], OutNoop )

        Errored ->
            if model.eventSourceOpened then
                -- connection could have dropped out of the blue; just let the browser
                -- handle reconnecting
                ( model, [], OutNoop )

            else
                -- assume request was rejected because auth is required; no way to
                -- really tell
                ( { model | state = NotAuthorized }, [], OutNoop )

        Events (Ok events) ->
            Array.foldl handleEvent_ ( model, [], OutNoop ) events

        Events (Err err) ->
            flip always (Debug.log "failed to get event" err) <|
                ( model, [], OutNoop )


handleEvent_ :
    BuildEvent
    -> ( OutputModel, List Effect, OutMsg )
    -> ( OutputModel, List Effect, OutMsg )
handleEvent_ ev ( m, msgpassedin, outmsgpassedin ) =
    let
        ( m1, msgfromhandleevent, outmsgfromhandleevent ) =
            handleEvent ev m
    in
    ( m1
    , if msgfromhandleevent == [] then
        msgpassedin

      else
        msgfromhandleevent
    , if outmsgfromhandleevent == OutNoop then
        outmsgpassedin

      else
        outmsgfromhandleevent
    )


handleEvent :
    BuildEvent
    -> OutputModel
    -> ( OutputModel, List Effect, OutMsg )
handleEvent event model =
    case event of
        Log origin output time ->
            ( updateStep origin.id (setRunning << appendStepLog output time) model
            , []
            , OutNoop
            )

        Error origin message ->
            ( updateStep origin.id (setStepError message) model
            , []
            , OutNoop
            )

        Initialize origin ->
            ( updateStep origin.id setRunning model
            , []
            , OutNoop
            )

        StartTask origin ->
            ( updateStep origin.id setRunning model
            , []
            , OutNoop
            )

        FinishTask origin exitStatus ->
            ( updateStep origin.id (finishStep exitStatus) model
            , []
            , OutNoop
            )

        FinishGet origin exitStatus version metadata ->
            ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
            , []
            , OutNoop
            )

        FinishPut origin exitStatus version metadata ->
            ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
            , []
            , OutNoop
            )

        BuildStatus status date ->
            case model.steps of
                Just st ->
                    let
                        ( newSt, effects ) =
                            if not <| Concourse.BuildStatus.isRunning status then
                                ( { st | finished = True }, [] )

                            else
                                ( st, [] )
                    in
                    ( { model | steps = Just newSt }, effects, OutBuildStatus status date )

                Nothing ->
                    ( model, [], OutBuildStatus status date )

        BuildError message ->
            ( { model
                | errors =
                    Just <|
                        Ansi.Log.update message <|
                            Maybe.withDefault (Ansi.Log.init Ansi.Log.Cooked) model.errors
              }
            , []
            , OutNoop
            )

        End ->
            ( { model | state = StepsComplete, events = Nothing }, [], OutNoop )


updateStep : StepID -> (StepTree -> StepTree) -> OutputModel -> OutputModel
updateStep id update model =
    { model | steps = Maybe.map (StepTree.updateAt id update) model.steps }


setRunning : StepTree -> StepTree
setRunning =
    setStepState StepStateRunning


appendStepLog : String -> Maybe Date -> StepTree -> StepTree
appendStepLog output mtime tree =
    flip StepTree.map tree <|
        \step ->
            let
                outputLineCount =
                    Ansi.Log.update output (Ansi.Log.init Ansi.Log.Cooked) |> .lines |> Array.length

                logLineCount =
                    max (Array.length step.log.lines - 1) 0

                setLineTimestamp line timestamps =
                    Dict.update line (\mval -> mtime) timestamps

                newTimestamps =
                    List.foldl
                        setLineTimestamp
                        step.timestamps
                        (List.range logLineCount (logLineCount + outputLineCount - 1))

                newLog =
                    Ansi.Log.update output step.log
            in
            { step | log = newLog, timestamps = newTimestamps }


setStepError : String -> StepTree -> StepTree
setStepError message tree =
    StepTree.map
        (\step ->
            { step
                | state = StepStateErrored
                , error = Just message
            }
        )
        tree


finishStep : Int -> StepTree -> StepTree
finishStep exitStatus tree =
    let
        stepState =
            if exitStatus == 0 then
                StepStateSucceeded

            else
                StepStateFailed
    in
    setStepState stepState tree


setResourceInfo : Concourse.Version -> Concourse.Metadata -> StepTree -> StepTree
setResourceInfo version metadata tree =
    StepTree.map (\step -> { step | version = Just version, metadata = metadata }) tree


setStepState : StepState -> StepTree -> StepTree
setStepState state tree =
    StepTree.map (\step -> { step | state = state }) tree


subscribeToEvents : Int -> Subscription.Subscription Msg
subscribeToEvents buildId =
    Subscription.map BuildEventsMsg
        (Subscription.FromEventSource
            ( "/api/v1/builds/" ++ toString buildId ++ "/events"
            , [ "end", "event" ]
            )
            parseMsg
        )


parseMsg : EventSource.Msg -> EventsMsg
parseMsg msg =
    case msg of
        EventSource.Events evs ->
            Events (BuildEvents.parseEvents evs)

        EventSource.Opened ->
            Opened

        EventSource.Errored ->
            Errored


view : Concourse.Build -> OutputModel -> Html Msg
view build { steps, errors, state } =
    Html.div [ class "steps" ]
        [ viewErrors errors
        , viewStepTree build steps state
        ]


viewStepTree :
    Concourse.Build
    -> Maybe StepTreeModel
    -> OutputState
    -> Html Msg
viewStepTree build steps state =
    case ( state, steps ) of
        ( StepsLoading, _ ) ->
            LoadingIndicator.view

        ( NotAuthorized, _ ) ->
            NotAuthorized.view

        ( StepsLiveUpdating, Just root ) ->
            StepTree.view root

        ( StepsComplete, Just root ) ->
            StepTree.view root

        ( _, Nothing ) ->
            Html.div [] []


viewErrors : Maybe Ansi.Log.Model -> Html msg
viewErrors errors =
    case errors of
        Nothing ->
            Html.div [] []

        Just log ->
            Html.div [ class "build-step" ]
                [ Html.div [ class "header" ]
                    [ Html.div
                        [ style <|
                            Styles.stepStatusIcon "ic-exclamation-triangle"
                        ]
                        []
                    , Html.h3 [] [ Html.text "error" ]
                    ]
                , Html.div [ class "step-body build-errors-body" ]
                    [ Html.pre
                        []
                        (Array.toList (Array.map Ansi.Log.viewLine log.lines))
                    ]
                ]
