module BuildOutput exposing (init, update, view, Model, Msg, OutMsg(..))

import Ansi.Log
import Array exposing (Array)
import Dict exposing (Dict)
import Date exposing (Date)
import Html exposing (Html)
import Html.Attributes exposing (action, class, classList, id, method, title)
import Http
import Task exposing (Task)
import Concourse
import Concourse.BuildPlan
import Concourse.BuildEvents
import Concourse.BuildStatus
import Concourse.BuildResources exposing (empty, fetch)
import LoadingIndicator
import StepTree exposing (StepTree)
import Routes


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
    | LoginRequired


type Msg
    = Noop
    | PlanAndResourcesFetched (Result Http.Error ( Concourse.BuildPlan, Concourse.BuildResources ))
    | BuildEventsMsg Concourse.BuildEvents.Msg
    | StepTreeMsg StepTree.Msg


type OutMsg
    = OutNoop
    | OutBuildStatus Concourse.BuildStatus Date


type alias Flags =
    { hash : String }


init : Flags -> Concourse.Build -> ( Model, Cmd Msg )
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
                fetchBuildPlanAndResources model.build.id
            else
                fetchBuildPlan model.build.id
    in
        ( model, fetch )


update : Msg -> Model -> ( Model, Cmd Msg, OutMsg )
update action model =
    case action of
        Noop ->
            ( model, Cmd.none, OutNoop )

        PlanAndResourcesFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        ( { model | events = subscribeToEvents model.build.id }
                        , Cmd.none
                        , OutNoop
                        )
                    else
                        ( model, Cmd.none, OutNoop )

                _ ->
                    flip always (Debug.log ("failed to fetch plan") (err)) <|
                        ( model, Cmd.none, OutNoop )

        PlanAndResourcesFetched (Ok ( plan, resources )) ->
            ( { model
                | steps = Just (StepTree.init model.highlight resources plan)
                , events = subscribeToEvents model.build.id
              }
            , Cmd.none
            , OutNoop
            )

        BuildEventsMsg action ->
            handleEventsMsg action model

        StepTreeMsg action ->
            case model.steps of
                Just st ->
                    let
                        ( newModel, newMsg ) =
                            StepTree.update action st
                    in
                        ( { model | steps = Just newModel }, Cmd.map StepTreeMsg newMsg, OutNoop )

                _ ->
                    ( model, Cmd.none, OutNoop )


handleEventsMsg : Concourse.BuildEvents.Msg -> Model -> ( Model, Cmd Msg, OutMsg )
handleEventsMsg action model =
    case action of
        Concourse.BuildEvents.Opened ->
            ( { model | eventSourceOpened = True }, Cmd.none, OutNoop )

        Concourse.BuildEvents.Errored ->
            if model.eventSourceOpened then
                -- connection could have dropped out of the blue; just let the browser
                -- handle reconnecting
                ( model, Cmd.none, OutNoop )
            else
                -- assume request was rejected because auth is required; no way to
                -- really tell
                ( { model | state = LoginRequired }, Cmd.none, OutNoop )

        Concourse.BuildEvents.Events (Ok events) ->
            Array.foldl handleEvent_ ( model, Cmd.none, OutNoop ) events

        Concourse.BuildEvents.Events (Err err) ->
            flip always (Debug.log ("failed to get event") (err)) <|
                ( model, Cmd.none, OutNoop )


handleEvent_ : Concourse.BuildEvents.BuildEvent -> ( Model, Cmd Msg, OutMsg ) -> ( Model, Cmd Msg, OutMsg )
handleEvent_ ev ( m, msgpassedin, outmsgpassedin ) =
    let
        ( m1, msgfromhandleevent, outmsgfromhandleevent ) =
            handleEvent ev m
    in
        ( m1
        , case ( msgpassedin == Cmd.none, msgfromhandleevent == Cmd.none ) of
            ( True, True ) ->
                Cmd.none

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


handleEvent : Concourse.BuildEvents.BuildEvent -> Model -> ( Model, Cmd Msg, OutMsg )
handleEvent event model =
    case event of
        Concourse.BuildEvents.Log origin output time ->
            ( updateStep origin.id (setRunning << appendStepLog output time) model
            , Cmd.none
            , OutNoop
            )

        Concourse.BuildEvents.Error origin message ->
            ( updateStep origin.id (setStepError message) model
            , Cmd.none
            , OutNoop
            )

        Concourse.BuildEvents.Initialize origin ->
            ( updateStep origin.id setRunning model
            , Cmd.none
            , OutNoop
            )

        Concourse.BuildEvents.StartTask origin ->
            ( updateStep origin.id setRunning model
            , Cmd.none
            , OutNoop
            )

        Concourse.BuildEvents.FinishTask origin exitStatus ->
            ( updateStep origin.id (finishStep exitStatus) model
            , Cmd.none
            , OutNoop
            )

        Concourse.BuildEvents.FinishGet origin exitStatus version metadata ->
            ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
            , Cmd.none
            , OutNoop
            )

        Concourse.BuildEvents.FinishPut origin exitStatus version metadata ->
            ( updateStep origin.id (finishStep exitStatus << setResourceInfo version metadata) model
            , Cmd.none
            , OutNoop
            )

        Concourse.BuildEvents.BuildStatus status date ->
            case model.steps of
                Just st ->
                    let
                        ( newSt, newMsg ) =
                            if not <| Concourse.BuildStatus.isRunning status then
                                StepTree.update StepTree.Finished st
                            else
                                ( st, Cmd.none )
                    in
                        ( { model | steps = Just newSt }, Cmd.map StepTreeMsg newMsg, OutBuildStatus status date )

                Nothing ->
                    ( model, Cmd.none, OutBuildStatus status date )

        Concourse.BuildEvents.BuildError message ->
            ( { model
                | errors =
                    Just <|
                        Ansi.Log.update message <|
                            Maybe.withDefault (Ansi.Log.init Ansi.Log.Cooked) model.errors
              }
            , Cmd.none
            , OutNoop
            )

        Concourse.BuildEvents.End ->
            ( { model | state = StepsComplete, events = Sub.none }, Cmd.none, OutNoop )


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
                    max ((Array.length step.log.lines) - 1) 0

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


fetchBuildPlanAndResources : Int -> Cmd Msg
fetchBuildPlanAndResources buildId =
    Task.attempt PlanAndResourcesFetched <|
        Task.map2 (,) (Concourse.BuildPlan.fetch buildId) (Concourse.BuildResources.fetch buildId)


fetchBuildPlan : Int -> Cmd Msg
fetchBuildPlan buildId =
    Task.attempt PlanAndResourcesFetched <|
        Task.map (flip (,) Concourse.BuildResources.empty) (Concourse.BuildPlan.fetch buildId)


subscribeToEvents : Int -> Sub Msg
subscribeToEvents buildId =
    Sub.map BuildEventsMsg (Concourse.BuildEvents.subscribe buildId)


view : Model -> Html Msg
view { build, steps, errors, state } =
    Html.div [ class "steps" ]
        [ viewErrors errors
        , viewStepTree build steps state
        ]


viewStepTree : Concourse.Build -> Maybe StepTree.Model -> OutputState -> Html Msg
viewStepTree build steps state =
    case ( state, steps ) of
        ( StepsLoading, _ ) ->
            LoadingIndicator.view

        ( LoginRequired, _ ) ->
            viewLoginButton build

        ( StepsLiveUpdating, Just root ) ->
            Html.map StepTreeMsg (StepTree.view root)

        ( StepsComplete, Just root ) ->
            Html.map StepTreeMsg (StepTree.view root)

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


viewLoginButton : Concourse.Build -> Html msg
viewLoginButton build =
    Html.form
        [ class "build-login"
        , Html.Attributes.method "get"
        , Html.Attributes.action "/sky/login"
        ]
        [ Html.input
            [ Html.Attributes.type_ "submit"
            , Html.Attributes.value "log in to view"
            ]
            []
        , Html.input
            [ Html.Attributes.type_ "hidden"
            , Html.Attributes.name "redirect_uri"
            , Html.Attributes.value (Routes.buildRoute build)
            ]
            []
        ]
