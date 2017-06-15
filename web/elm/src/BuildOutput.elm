module BuildOutput exposing (init, update, view, Model, Msg, OutMsg(..))

import Ansi.Log
import Date exposing (Date)
import Html exposing (Html)
import Html.Attributes exposing (action, class, classList, id, method, title)
import Http
import Task exposing (Task)
import Concourse
import Concourse.Build
import Concourse.BuildPlan
import Concourse.BuildEvents
import Concourse.BuildStatus
import Concourse.BuildResources exposing (empty, fetch)
import LoadingIndicator
import StepTree exposing (StepTree)


type alias Model =
    { build : Concourse.Build
    , steps : Maybe StepTree.Model
    , errors : Maybe Ansi.Log.Model
    , state : OutputState
    , eventSourceOpened : Bool
    , events : Sub Msg
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


init : Concourse.Build -> ( Model, Cmd Msg )
init build =
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
                | steps = Just (StepTree.init resources plan)
                , events = subscribeToEvents model.build.id
              }
            , Cmd.none
            , OutNoop
            )

        BuildEventsMsg action ->
            handleEventsMsg action model

        StepTreeMsg action ->
            ( { model | steps = Maybe.map (StepTree.update action) model.steps }
            , Cmd.none
            , OutNoop
            )


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

        Concourse.BuildEvents.Event (Ok event) ->
            handleEvent event model

        Concourse.BuildEvents.Event (Err err) ->
            flip always (Debug.log ("failed to get event") (err)) <|
                ( model, Cmd.none, OutNoop )

        Concourse.BuildEvents.End ->
            ( { model | state = StepsComplete, events = Sub.none }, Cmd.none, OutNoop )


handleEvent : Concourse.BuildEvents.BuildEvent -> Model -> ( Model, Cmd Msg, OutMsg )
handleEvent event model =
    case event of
        Concourse.BuildEvents.Log origin output ->
            ( updateStep origin.id (setRunning << appendStepLog output) model
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
            ( { model
                | steps =
                    if not <| Concourse.BuildStatus.isRunning status then
                        Maybe.map (StepTree.update StepTree.Finished) model.steps
                    else
                        model.steps
              }
            , Cmd.none
            , OutBuildStatus status date
            )

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


updateStep : StepTree.StepID -> (StepTree -> StepTree) -> Model -> Model
updateStep id update model =
    { model | steps = Maybe.map (StepTree.updateAt id update) model.steps }


setRunning : StepTree -> StepTree
setRunning =
    setStepState StepTree.StepStateRunning


appendStepLog : String -> StepTree -> StepTree
appendStepLog output tree =
    StepTree.map (\step -> { step | log = Ansi.Log.update output step.log }) tree


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
        , Html.Attributes.action "/login"
        ]
        [ Html.input
            [ Html.Attributes.type_ "submit"
            , Html.Attributes.value "log in to view"
            ]
            []
        , Html.input
            [ Html.Attributes.type_ "hidden"
            , Html.Attributes.name "redirect"
            , Html.Attributes.value (Concourse.Build.url build)
            ]
            []
        ]
