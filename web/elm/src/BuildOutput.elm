module BuildOutput exposing
    ( Model
    , OutMsg(..)
    , handleEventsMsg
    , handleStepTreeMsg
    , init
    , planAndResourcesFetched
    , view
    )

import Ansi.Log
import Array exposing (Array)
import Build.Effects exposing (Effect(..))
import Build.Msgs exposing (Msg(..))
import Concourse
import Concourse.BuildEvents
import Concourse.BuildStatus
import Date exposing (Date)
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (action, class, classList, id, method, title)
import Http
import LoadingIndicator
import NotAuthorized
import StepTree exposing (StepTree)


type alias Model =
    { build : Concourse.Build
    , steps : Maybe StepTree.Model
    , errors : Maybe Ansi.Log.Model
    , state : OutputState
    , eventSourceOpened : Bool
    , events : Sub Msg
    , highlight : StepTree.Highlight
    }


type OutputState
    = StepsLoading
    | StepsLiveUpdating
    | StepsComplete
    | NotAuthorized


type OutMsg
    = OutNoop
    | OutBuildStatus Concourse.BuildStatus Date


type alias Flags =
    { hash : String }


init : Flags -> Concourse.Build -> ( Model, List Effect )
init flags build =
    let
        outputState =
            if Concourse.BuildStatus.isRunning build.status then
                StepsLiveUpdating

            else
                StepsLoading

        model =
            { build = build
            , steps = Nothing
            , errors = Nothing
            , state = outputState
            , events = Sub.none
            , eventSourceOpened = False
            , highlight = StepTree.parseHighlight flags.hash
            }

        fetch =
            if build.job /= Nothing then
                [ FetchBuildPlanAndResources model.build.id ]

            else
                [ FetchBuildPlan model.build.id ]
    in
    ( model, fetch )


handleStepTreeMsg :
    (StepTree.Model -> ( StepTree.Model, List Effect ))
    -> Model
    -> ( Model, List Effect, OutMsg )
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
    Result Http.Error ( Concourse.BuildPlan, Concourse.BuildResources )
    -> Model
    -> ( Model, List Effect, OutMsg )
planAndResourcesFetched result model =
    case result of
        Err err ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        ( { model | events = subscribeToEvents model.build.id }
                        , []
                        , OutNoop
                        )

                    else
                        ( model, [], OutNoop )

                _ ->
                    flip always (Debug.log "failed to fetch plan" err) <|
                        ( model, [], OutNoop )

        Ok ( plan, resources ) ->
            ( { model
                | steps = Just (StepTree.init model.highlight resources plan)
                , events = subscribeToEvents model.build.id
              }
            , []
            , OutNoop
            )


handleEventsMsg :
    Concourse.BuildEvents.Msg
    -> Model
    -> ( Model, List Effect, OutMsg )
handleEventsMsg action model =
    case action of
        Concourse.BuildEvents.Opened ->
            ( { model | eventSourceOpened = True }, [], OutNoop )

        Concourse.BuildEvents.Errored ->
            if model.eventSourceOpened then
                -- connection could have dropped out of the blue; just let the browser
                -- handle reconnecting
                ( model, [], OutNoop )

            else
                -- assume request was rejected because auth is required; no way to
                -- really tell
                ( { model | state = NotAuthorized }, [], OutNoop )

        Concourse.BuildEvents.Events (Ok events) ->
            Array.foldl handleEvent_ ( model, [], OutNoop ) events

        Concourse.BuildEvents.Events (Err err) ->
            flip always (Debug.log "failed to get event" err) <|
                ( model, [], OutNoop )


handleEvent_ :
    Concourse.BuildEvents.BuildEvent
    -> ( Model, List Effect, OutMsg )
    -> ( Model, List Effect, OutMsg )
handleEvent_ ev ( m, msgpassedin, outmsgpassedin ) =
    let
        ( m1, msgfromhandleevent, outmsgfromhandleevent ) =
            handleEvent ev m
    in
    ( m1
    , case ( msgpassedin == [], msgfromhandleevent == [] ) of
        ( True, True ) ->
            []

        ( False, True ) ->
            msgpassedin

        otherwise ->
            msgfromhandleevent
    , case ( outmsgpassedin == OutNoop, outmsgfromhandleevent == OutNoop ) of
        ( True, True ) ->
            OutNoop

        ( False, True ) ->
            outmsgpassedin

        otherwise ->
            outmsgfromhandleevent
    )


handleEvent :
    Concourse.BuildEvents.BuildEvent
    -> Model
    -> ( Model, List Effect, OutMsg )
handleEvent event model =
    case event of
        Concourse.BuildEvents.Log origin output time ->
            ( updateStep origin.id (setRunning << appendStepLog output time) model
            , []
            , OutNoop
            )

        Concourse.BuildEvents.Error origin message ->
            ( updateStep origin.id (setStepError message) model
            , []
            , OutNoop
            )

        Concourse.BuildEvents.Initialize origin ->
            ( updateStep origin.id setRunning model
            , []
            , OutNoop
            )

        Concourse.BuildEvents.StartTask origin ->
            ( updateStep origin.id setRunning model
            , []
            , OutNoop
            )

        Concourse.BuildEvents.FinishTask origin exitStatus ->
            ( updateStep origin.id (finishStep exitStatus) model
            , []
            , OutNoop
            )

        Concourse.BuildEvents.FinishGet origin exitStatus version metadata ->
            ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
            , []
            , OutNoop
            )

        Concourse.BuildEvents.FinishPut origin exitStatus version metadata ->
            ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
            , []
            , OutNoop
            )

        Concourse.BuildEvents.BuildStatus status date ->
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

        Concourse.BuildEvents.BuildError message ->
            ( { model
                | errors =
                    Just <|
                        Ansi.Log.update message <|
                            Maybe.withDefault (Ansi.Log.init Ansi.Log.Cooked) model.errors
              }
            , []
            , OutNoop
            )

        Concourse.BuildEvents.End ->
            ( { model | state = StepsComplete, events = Sub.none }, [], OutNoop )


updateStep : StepTree.StepID -> (StepTree -> StepTree) -> Model -> Model
updateStep id update model =
    { model | steps = Maybe.map (StepTree.updateAt id update) model.steps }


setRunning : StepTree -> StepTree
setRunning =
    setStepState StepTree.StepStateRunning


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
                | state = StepTree.StepStateErrored
                , error = Just message
            }
        )
        tree


finishStep : Int -> StepTree -> StepTree
finishStep exitStatus tree =
    let
        stepState =
            if exitStatus == 0 then
                StepTree.StepStateSucceeded

            else
                StepTree.StepStateFailed
    in
    setStepState stepState tree


setResourceInfo : Concourse.Version -> Concourse.Metadata -> StepTree -> StepTree
setResourceInfo version metadata tree =
    StepTree.map (\step -> { step | version = Just version, metadata = metadata }) tree


setStepState : StepTree.StepState -> StepTree -> StepTree
setStepState state tree =
    StepTree.map (\step -> { step | state = state }) tree


subscribeToEvents : Int -> Sub Msg
subscribeToEvents buildId =
    Sub.map BuildEventsMsg (Concourse.BuildEvents.subscribe buildId)


view : Model -> Html Msg
view { build, steps, errors, state } =
    Html.div [ class "steps" ]
        [ viewErrors errors
        , viewStepTree build steps state
        ]


viewStepTree :
    Concourse.Build
    -> Maybe StepTree.Model
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
                    [ Html.i [ class "left fa fa-fw fa-exclamation-triangle" ] []
                    , Html.h3 [] [ Html.text "error" ]
                    ]
                , Html.div [ class "step-body build-errors-body" ] [ Ansi.Log.view log ]
                ]
