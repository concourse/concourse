module Build exposing
    ( Model
    , Page(..)
    , changeToBuild
    , getScrollBehavior
    , init
    , initJobBuildPage
    , subscriptions
    , update
    , updateWithMessage
    , view
    )

import Autoscroll
import Build.Effects exposing (Effect(..), toCmd)
import Build.Msgs exposing (HoveredButton(..), Msg(..))
import Build.Styles as Styles
import BuildDuration
import BuildOutput
import Char
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination exposing (Paginated)
import Date exposing (Date)
import Date.Format
import Debug
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes
    exposing
        ( action
        , attribute
        , class
        , classList
        , disabled
        , href
        , id
        , method
        , style
        , tabindex
        , title
        )
import Html.Events exposing (onBlur, onFocus, onMouseEnter, onMouseLeave)
import Html.Lazy
import Http
import Keyboard
import LoadingIndicator
import Maybe.Extra
import RemoteData exposing (WebData)
import Routes
import Scroll
import StepTree
import StrictEvents exposing (onLeftClick, onMouseWheel, onScroll)
import String
import Time exposing (Time)
import UpdateMsg exposing (UpdateMsg)
import Views


type alias Ports =
    { title : String -> Cmd Msg
    }


type Page
    = BuildPage Int
    | JobBuildPage Concourse.JobBuildIdentifier


initJobBuildPage :
    Concourse.TeamName
    -> Concourse.PipelineName
    -> Concourse.JobName
    -> Concourse.BuildName
    -> Page
initJobBuildPage teamName pipelineName jobName buildName =
    JobBuildPage
        { teamName = teamName
        , pipelineName = pipelineName
        , jobName = jobName
        , buildName = buildName
        }


type alias CurrentBuild =
    { build : Concourse.Build
    , prep : Maybe Concourse.BuildPrep
    , output : Maybe BuildOutput.Model
    }


type alias Model =
    { page : Page
    , now : Maybe Time.Time
    , job : Maybe Concourse.Job
    , history : List Concourse.Build
    , currentBuild : WebData CurrentBuild
    , browsingIndex : Int
    , autoScroll : Bool
    , ports : Ports
    , csrfToken : String
    , previousKeyPress : Maybe Char
    , previousTriggerBuildByKey : Bool
    , showHelp : Bool
    , hash : String
    , hoveredButton : HoveredButton
    }


type StepRenderingState
    = StepsLoading
    | StepsLiveUpdating
    | StepsComplete
    | NotAuthorized


type alias Flags =
    { csrfToken : String
    , hash : String
    }


init : Ports -> Flags -> Page -> ( Model, List Effect )
init ports flags page =
    let
        ( model, effects ) =
            changeToBuild
                page
                { page = page
                , now = Nothing
                , job = Nothing
                , history = []
                , currentBuild = RemoteData.NotAsked
                , browsingIndex = 0
                , autoScroll = True
                , ports = ports
                , csrfToken = flags.csrfToken
                , previousKeyPress = Nothing
                , previousTriggerBuildByKey = False
                , showHelp = False
                , hash = flags.hash
                , hoveredButton = Neither
                }
    in
    ( model, effects ++ [ GetCurrentTime ] )


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ Time.every Time.second ClockTick
        , Scroll.fromWindowBottom WindowScrolled
        , case
            model.currentBuild
                |> RemoteData.toMaybe
                |> Maybe.andThen .output
          of
            Nothing ->
                Sub.none

            Just buildOutput ->
                buildOutput.events
        , Keyboard.presses KeyPressed
        , Keyboard.ups KeyUped
        ]


changeToBuild : Page -> Model -> ( Model, List Effect )
changeToBuild page model =
    if model.browsingIndex > 0 && page == model.page then
        ( model, [] )

    else
        let
            newIndex =
                model.browsingIndex + 1

            newBuild =
                RemoteData.map
                    (\cb ->
                        { cb
                            | prep = Nothing
                            , output = Nothing
                        }
                    )
                    model.currentBuild
        in
        ( { model
            | browsingIndex = newIndex
            , currentBuild = newBuild
            , autoScroll = True
            , page = page
          }
        , case page of
            BuildPage buildId ->
                [ FetchBuild 0 newIndex buildId ]

            JobBuildPage jbi ->
                [ FetchJobBuild newIndex jbi ]
        )


extractTitle : Model -> String
extractTitle model =
    case ( model.currentBuild |> RemoteData.toMaybe, model.job ) of
        ( Just build, Just job ) ->
            job.name ++ ((" #" ++ build.build.name) ++ " - ")

        ( Just build, Nothing ) ->
            "#" ++ (toString build.build.id ++ " - ")

        _ ->
            ""


updateWithMessage : Msg -> Model -> ( Model, Cmd Msg, Maybe UpdateMsg )
updateWithMessage message model =
    let
        ( mdl, effects ) =
            update message model
    in
    case mdl.currentBuild of
        RemoteData.Failure _ ->
            ( mdl, Cmd.batch (List.map toCmd effects), Just UpdateMsg.NotFound )

        _ ->
            ( mdl, Cmd.batch (List.map toCmd effects), Nothing )


update : Msg -> Model -> ( Model, List Effect )
update action model =
    case action of
        Noop ->
            ( model, [] )

        SwitchToBuild build ->
            ( model, [ NewUrl <| Routes.buildRoute build ] )

        Hover state ->
            ( { model | hoveredButton = state }, [] )

        TriggerBuild job ->
            case job of
                Nothing ->
                    ( model, [] )

                Just someJob ->
                    ( model, [ DoTriggerBuild someJob model.csrfToken ] )

        BuildTriggered (Ok build) ->
            update
                (SwitchToBuild build)
                { model
                    | history = build :: model.history
                }

        BuildTriggered (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, [ RedirectToLogin "" ] )

                    else
                        ( model, [] )

                _ ->
                    ( model, [] )

        BuildFetched browsingIndex (Ok build) ->
            handleBuildFetched browsingIndex build model

        BuildFetched _ (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, [ RedirectToLogin "" ] )

                    else if status.code == 404 then
                        ( { model | currentBuild = RemoteData.Failure err }
                        , []
                        )

                    else
                        ( model, [] )

                _ ->
                    ( model, [] )

        AbortBuild buildId ->
            ( model, [ DoAbortBuild buildId model.csrfToken ] )

        BuildAborted (Ok ()) ->
            ( model, [] )

        BuildAborted (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, [ RedirectToLogin "" ] )

                    else
                        ( model, [] )

                _ ->
                    ( model, [] )

        BuildPrepFetched browsingIndex (Ok buildPrep) ->
            handleBuildPrepFetched browsingIndex buildPrep model

        BuildPrepFetched _ (Err err) ->
            flip always (Debug.log "failed to fetch build preparation" err) <|
                ( model, [] )

        PlanAndResourcesFetched result ->
            updateOutput (BuildOutput.planAndResourcesFetched result) model

        BuildEventsMsg action ->
            updateOutput (BuildOutput.handleEventsMsg action) model

        ToggleStep id ->
            updateOutput
                (BuildOutput.handleStepTreeMsg <| StepTree.toggleStep id)
                model

        Finished ->
            updateOutput
                (BuildOutput.handleStepTreeMsg <| StepTree.finished)
                model

        SwitchTab id tab ->
            updateOutput
                (BuildOutput.handleStepTreeMsg <| StepTree.switchTab id tab)
                model

        SetHighlight id line ->
            updateOutput
                (BuildOutput.handleStepTreeMsg <| StepTree.setHighlight id line)
                model

        ExtendHighlight id line ->
            updateOutput
                (BuildOutput.handleStepTreeMsg <| StepTree.extendHighlight id line)
                model

        BuildHistoryFetched (Err err) ->
            flip always (Debug.log "failed to fetch build history" err) <|
                ( model, [] )

        BuildHistoryFetched (Ok history) ->
            handleHistoryFetched history model

        BuildJobDetailsFetched (Ok job) ->
            handleBuildJobFetched job model

        BuildJobDetailsFetched (Err err) ->
            flip always (Debug.log "failed to fetch build job details" err) <|
                ( model, [] )

        RevealCurrentBuildInHistory ->
            ( model, [ ScrollToCurrentBuildInHistory ] )

        ScrollBuilds event ->
            if event.deltaX == 0 then
                ( model, [ DoScrollBuilds event.deltaY ] )

            else
                ( model, [ DoScrollBuilds -event.deltaX ] )

        ClockTick now ->
            ( { model | now = Just now }, [] )

        WindowScrolled fromBottom ->
            if fromBottom == 0 then
                ( { model | autoScroll = True }, [] )

            else
                ( { model | autoScroll = False }, [] )

        NavTo url ->
            ( model, [ NewUrl url ] )

        NewCSRFToken token ->
            ( { model | csrfToken = token }, [] )

        KeyPressed keycode ->
            handleKeyPressed (Char.fromCode keycode) model

        KeyUped keycode ->
            case Char.fromCode keycode of
                'T' ->
                    ( { model | previousTriggerBuildByKey = False }, [] )

                _ ->
                    ( model, [] )


updateOutput :
    (BuildOutput.Model
     -> ( BuildOutput.Model, List Effect, BuildOutput.OutMsg )
    )
    -> Model
    -> ( Model, List Effect )
updateOutput updater model =
    let
        currentBuild =
            model.currentBuild |> RemoteData.toMaybe
    in
    case ( currentBuild, currentBuild |> Maybe.andThen .output ) of
        ( Just currentBuild, Just output ) ->
            let
                ( newOutput, effects, outMsg ) =
                    updater output

                ( newModel, newCmd ) =
                    handleOutMsg outMsg
                        { model
                            | currentBuild =
                                RemoteData.Success
                                    { currentBuild
                                        | output = Just newOutput
                                    }
                        }
            in
            ( newModel, newCmd ++ effects )

        _ ->
            ( model, [] )


handleKeyPressed : Char -> Model -> ( Model, List Effect )
handleKeyPressed key model =
    let
        currentBuild =
            Maybe.map .build (model.currentBuild |> RemoteData.toMaybe)

        newModel =
            case ( model.previousKeyPress, key ) of
                ( Nothing, 'g' ) ->
                    { model | previousKeyPress = Just 'g' }

                _ ->
                    { model | previousKeyPress = Nothing }
    in
    case key of
        'h' ->
            case Maybe.andThen (nextBuild model.history) currentBuild of
                Just build ->
                    update (SwitchToBuild build) newModel

                Nothing ->
                    ( newModel, [] )

        'l' ->
            case Maybe.andThen (prevBuild model.history) currentBuild of
                Just build ->
                    update (SwitchToBuild build) newModel

                Nothing ->
                    ( newModel, [] )

        'j' ->
            ( newModel, [ ScrollDown ] )

        'k' ->
            ( newModel, [ ScrollUp ] )

        'T' ->
            if not model.previousTriggerBuildByKey then
                update
                    (TriggerBuild (currentBuild |> Maybe.andThen .job))
                    { newModel | previousTriggerBuildByKey = True }

            else
                ( newModel, [] )

        'A' ->
            if currentBuild == List.head model.history then
                case currentBuild of
                    Just build ->
                        update (AbortBuild build.id) newModel

                    Nothing ->
                        ( newModel, [] )

            else
                ( newModel, [] )

        'g' ->
            if model.previousKeyPress == Just 'g' then
                ( { newModel | autoScroll = False }, [ ScrollToWindowTop ] )

            else
                ( newModel, [] )

        'G' ->
            ( { newModel | autoScroll = True }, [ ScrollToWindowBottom ] )

        '?' ->
            ( { model | showHelp = not model.showHelp }, [] )

        _ ->
            ( newModel, [] )


nextBuild : List Concourse.Build -> Concourse.Build -> Maybe Concourse.Build
nextBuild builds build =
    case builds of
        first :: second :: rest ->
            if second == build then
                Just first

            else
                nextBuild (second :: rest) build

        _ ->
            Nothing


prevBuild : List Concourse.Build -> Concourse.Build -> Maybe Concourse.Build
prevBuild builds build =
    case builds of
        first :: second :: rest ->
            if first == build then
                Just second

            else
                prevBuild (second :: rest) build

        _ ->
            Nothing


handleBuildFetched : Int -> Concourse.Build -> Model -> ( Model, List Effect )
handleBuildFetched browsingIndex build model =
    if browsingIndex == model.browsingIndex then
        let
            currentBuild =
                case model.currentBuild |> RemoteData.toMaybe of
                    Nothing ->
                        { build = build
                        , prep = Nothing
                        , output = Nothing
                        }

                    Just currentBuild ->
                        { currentBuild | build = build }

            withBuild =
                { model
                    | currentBuild = RemoteData.Success currentBuild
                    , history = updateHistory build model.history
                }

            fetchJobAndHistory =
                case ( model.job, build.job ) of
                    ( Nothing, Just buildJob ) ->
                        [ FetchBuildJobDetails buildJob
                        , FetchBuildHistory buildJob Nothing
                        ]

                    _ ->
                        []

            ( newModel, cmd ) =
                if build.status == Concourse.BuildStatusPending then
                    ( withBuild, pollUntilStarted browsingIndex build.id )

                else if build.reapTime == Nothing then
                    case
                        model.currentBuild
                            |> RemoteData.toMaybe
                            |> Maybe.andThen .prep
                    of
                        Nothing ->
                            initBuildOutput build withBuild

                        Just _ ->
                            let
                                ( newModel, cmd ) =
                                    initBuildOutput build withBuild
                            in
                            ( newModel
                            , cmd
                                ++ [ FetchBuildPrep
                                        Time.second
                                        browsingIndex
                                        build.id
                                   ]
                            )

                else
                    ( withBuild, [] )
        in
        ( newModel
        , cmd
            ++ [ SetFavIcon build.status
               , SetTitle model.ports.title (extractTitle newModel)
               ]
            ++ fetchJobAndHistory
        )

    else
        ( model, [] )


pollUntilStarted : Int -> Int -> List Effect
pollUntilStarted browsingIndex buildId =
    [ FetchBuild Time.second browsingIndex buildId
    , FetchBuildPrep Time.second browsingIndex buildId
    ]


initBuildOutput : Concourse.Build -> Model -> ( Model, List Effect )
initBuildOutput build model =
    let
        ( output, outputCmd ) =
            BuildOutput.init { hash = model.hash } build
    in
    ( { model
        | currentBuild =
            RemoteData.map
                (\info -> { info | output = Just output })
                model.currentBuild
      }
    , outputCmd
    )


handleBuildJobFetched : Concourse.Job -> Model -> ( Model, List Effect )
handleBuildJobFetched job model =
    let
        withJobDetails =
            { model | job = Just job }
    in
    ( withJobDetails
    , [ SetTitle model.ports.title (extractTitle withJobDetails) ]
    )


handleHistoryFetched :
    Paginated Concourse.Build
    -> Model
    -> ( Model, List Effect )
handleHistoryFetched history model =
    let
        withBuilds =
            { model | history = List.append model.history history.content }

        currentBuild =
            model.currentBuild |> RemoteData.toMaybe
    in
    case
        ( history.pagination.nextPage
        , currentBuild |> Maybe.andThen (.job << .build)
        )
    of
        ( Nothing, _ ) ->
            ( withBuilds, [] )

        ( Just page, Just job ) ->
            ( withBuilds, [ FetchBuildHistory job (Just page) ] )

        ( Just url, Nothing ) ->
            Debug.crash "impossible"


handleBuildPrepFetched :
    Int
    -> Concourse.BuildPrep
    -> Model
    -> ( Model, List Effect )
handleBuildPrepFetched browsingIndex buildPrep model =
    if browsingIndex == model.browsingIndex then
        ( { model
            | currentBuild =
                RemoteData.map
                    (\info -> { info | prep = Just buildPrep })
                    model.currentBuild
          }
        , []
        )

    else
        ( model, [] )


view : Model -> Html Msg
view model =
    case model.currentBuild |> RemoteData.toMaybe of
        Just currentBuild ->
            Html.div
                [ class "with-fixed-header"
                , attribute "data-build-name" currentBuild.build.name
                ]
                [ viewBuildHeader currentBuild.build model
                , Html.div [ class "scrollable-body build-body" ] <|
                    [ viewBuildPrep currentBuild.prep
                    , Html.Lazy.lazy2 viewBuildOutput model.browsingIndex <|
                        currentBuild.output
                    , Html.div
                        [ classList
                            [ ( "keyboard-help", True )
                            , ( "hidden", not model.showHelp )
                            ]
                        ]
                        [ Html.div
                            [ class "help-title" ]
                            [ Html.text "keyboard shortcuts" ]
                        , Html.div
                            [ class "help-line" ]
                            [ Html.div
                                [ class "keys" ]
                                [ Html.span [ class "key" ] [ Html.text "h" ]
                                , Html.span [ class "key" ] [ Html.text "l" ]
                                ]
                            , Html.text "previous/next build"
                            ]
                        , Html.div
                            [ class "help-line" ]
                            [ Html.div
                                [ class "keys" ]
                                [ Html.span [ class "key" ] [ Html.text "j" ]
                                , Html.span [ class "key" ] [ Html.text "k" ]
                                ]
                            , Html.text "scroll down/up"
                            ]
                        , Html.div
                            [ class "help-line" ]
                            [ Html.div
                                [ class "keys" ]
                                [ Html.span [ class "key" ] [ Html.text "T" ] ]
                            , Html.text "trigger a new build"
                            ]
                        , Html.div
                            [ class "help-line" ]
                            [ Html.div
                                [ class "keys" ]
                                [ Html.span [ class "key" ] [ Html.text "A" ] ]
                            , Html.text "abort build"
                            ]
                        , Html.div
                            [ class "help-line" ]
                            [ Html.div
                                [ class "keys" ]
                                [ Html.span [ class "key" ] [ Html.text "gg" ] ]
                            , Html.text "scroll to the top"
                            ]
                        , Html.div
                            [ class "help-line" ]
                            [ Html.div
                                [ class "keys" ]
                                [ Html.span [ class "key" ] [ Html.text "G" ] ]
                            , Html.text "scroll to the bottom"
                            ]
                        , Html.div
                            [ class "help-line" ]
                            [ Html.div
                                [ class "keys" ]
                                [ Html.span [ class "key" ] [ Html.text "?" ] ]
                            , Html.text "hide/show help"
                            ]
                        ]
                    ]
                        ++ (let
                                build =
                                    currentBuild.build

                                maybeBirthDate =
                                    Maybe.Extra.or build.duration.startedAt build.duration.finishedAt
                            in
                            case ( maybeBirthDate, build.reapTime ) of
                                ( Just birthDate, Just reapTime ) ->
                                    [ Html.div
                                        [ class "tombstone" ]
                                        [ Html.div [ class "heading" ] [ Html.text "RIP" ]
                                        , Html.div
                                            [ class "job-name" ]
                                            [ Html.text <|
                                                Maybe.withDefault
                                                    "one-off build"
                                                <|
                                                    Maybe.map .jobName build.job
                                            ]
                                        , Html.div
                                            [ class "build-name" ]
                                            [ Html.text <|
                                                "build #"
                                                    ++ (case build.job of
                                                            Nothing ->
                                                                toString build.id

                                                            Just _ ->
                                                                build.name
                                                       )
                                            ]
                                        , Html.div
                                            [ class "date" ]
                                            [ Html.text <|
                                                mmDDYY birthDate
                                                    ++ "-"
                                                    ++ mmDDYY reapTime
                                            ]
                                        , Html.div
                                            [ class "epitaph" ]
                                            [ Html.text <|
                                                case build.status of
                                                    Concourse.BuildStatusSucceeded ->
                                                        "It passed, and now it has passed on."

                                                    Concourse.BuildStatusFailed ->
                                                        "It failed, and now has been forgotten."

                                                    Concourse.BuildStatusErrored ->
                                                        "It errored, but has found forgiveness."

                                                    Concourse.BuildStatusAborted ->
                                                        "It was never given a chance."

                                                    _ ->
                                                        "I'm not dead yet."
                                            ]
                                        ]
                                    , Html.div
                                        [ class "explanation" ]
                                        [ Html.text "This log has been "
                                        , Html.a
                                            [ Html.Attributes.href "https://concourse-ci.org/jobs.html#job-build-logs-to-retain" ]
                                            [ Html.text "reaped." ]
                                        ]
                                    ]

                                _ ->
                                    []
                           )
                ]

        _ ->
            LoadingIndicator.view


mmDDYY : Date -> String
mmDDYY d =
    Date.Format.format "%m/%d/" d ++ String.right 2 (Date.Format.format "%Y" d)


viewBuildOutput : Int -> Maybe BuildOutput.Model -> Html Msg
viewBuildOutput browsingIndex output =
    case output of
        Just o ->
            BuildOutput.view o

        Nothing ->
            Html.div [] []


viewBuildPrep : Maybe Concourse.BuildPrep -> Html Msg
viewBuildPrep prep =
    case prep of
        Just prep ->
            Html.div [ class "build-step" ]
                [ Html.div
                    [ class "header"
                    , style
                        [ ( "display", "flex" )
                        , ( "align-items", "center" )
                        ]
                    ]
                    [ Views.icon
                        { sizePx = 15, image = "ic_cogs.svg" }
                        [ style
                            [ ( "margin", "6.5px" )
                            , ( "margin-right", "0.5px" )
                            ]
                        ]
                    , Html.h3 [] [ Html.text "preparing build" ]
                    ]
                , Html.div []
                    [ Html.ul [ class "prep-status-list" ]
                        ([ viewBuildPrepLi "checking pipeline is not paused" prep.pausedPipeline Dict.empty
                         , viewBuildPrepLi "checking job is not paused" prep.pausedJob Dict.empty
                         ]
                            ++ viewBuildPrepInputs prep.inputs
                            ++ [ viewBuildPrepLi "waiting for a suitable set of input versions" prep.inputsSatisfied prep.missingInputReasons
                               , viewBuildPrepLi "checking max-in-flight is not reached" prep.maxRunningBuilds Dict.empty
                               ]
                        )
                    ]
                ]

        Nothing ->
            Html.div [] []


viewBuildPrepInputs : Dict String Concourse.BuildPrepStatus -> List (Html Msg)
viewBuildPrepInputs inputs =
    List.map viewBuildPrepInput (Dict.toList inputs)


viewBuildPrepInput : ( String, Concourse.BuildPrepStatus ) -> Html Msg
viewBuildPrepInput ( name, status ) =
    viewBuildPrepLi ("discovering any new versions of " ++ name) status Dict.empty


viewBuildPrepDetails : Dict String String -> Html Msg
viewBuildPrepDetails details =
    Html.ul [ class "details" ]
        (List.map viewDetailItem (Dict.toList details))


viewDetailItem : ( String, String ) -> Html Msg
viewDetailItem ( name, status ) =
    Html.li []
        [ Html.text (name ++ " - " ++ status) ]


viewBuildPrepLi : String -> Concourse.BuildPrepStatus -> Dict String String -> Html Msg
viewBuildPrepLi text status details =
    Html.li
        [ classList
            [ ( "prep-status", True )
            , ( "inactive", status == Concourse.BuildPrepStatusUnknown )
            ]
        ]
        [ Html.span [ class "marker" ]
            [ viewBuildPrepStatus status ]
        , Html.span []
            [ Html.text text ]
        , viewBuildPrepDetails details
        ]


viewBuildPrepStatus : Concourse.BuildPrepStatus -> Html Msg
viewBuildPrepStatus status =
    case status of
        Concourse.BuildPrepStatusUnknown ->
            Html.i [ class "fa fa-fw fa-circle-o-notch", title "thinking..." ] []

        Concourse.BuildPrepStatusBlocking ->
            Html.i [ class "fa fa-fw fa-spin fa-circle-o-notch inactive", title "blocking" ] []

        Concourse.BuildPrepStatusNotBlocking ->
            Html.i [ class "fa fa-fw fa-check", title "not blocking" ] []


viewBuildHeader : Concourse.Build -> Model -> Html Msg
viewBuildHeader build { now, job, history, hoveredButton } =
    let
        triggerButton =
            case job of
                Just { name, pipeline } ->
                    let
                        actionUrl =
                            "/teams/"
                                ++ pipeline.teamName
                                ++ "/pipelines/"
                                ++ pipeline.pipelineName
                                ++ "/jobs/"
                                ++ name
                                ++ "/builds"

                        buttonDisabled =
                            case job of
                                Nothing ->
                                    True

                                Just job ->
                                    job.disableManualTrigger

                        buttonHovered =
                            hoveredButton == Trigger

                        buttonHighlight =
                            buttonHovered && not buttonDisabled
                    in
                    Html.button
                        [ attribute "role" "button"
                        , attribute "tabindex" "0"
                        , attribute "aria-label" "Trigger Build"
                        , attribute "title" "Trigger Build"
                        , onLeftClick <| TriggerBuild build.job
                        , onMouseEnter <| Hover Trigger
                        , onFocus <| Hover Trigger
                        , onMouseLeave <| Hover Neither
                        , onBlur <| Hover Neither
                        , style <| Styles.triggerButton buttonDisabled
                        ]
                    <|
                        [ Html.div
                            [ style <| Styles.triggerIcon buttonHighlight ]
                            []
                        ]
                            ++ (if buttonDisabled && buttonHovered then
                                    [ Html.div
                                        [ style Styles.triggerTooltip ]
                                        [ Html.text <|
                                            "manual triggering disabled "
                                                ++ "in job config"
                                        ]
                                    ]

                                else
                                    []
                               )

                _ ->
                    Html.text ""

        abortButton =
            if Concourse.BuildStatus.isRunning build.status then
                Html.button
                    [ onLeftClick (AbortBuild build.id)
                    , attribute "role" "button"
                    , attribute "tabindex" "0"
                    , attribute "aria-label" "Abort Build"
                    , attribute "title" "Abort Build"
                    , onMouseEnter <| Hover Abort
                    , onFocus <| Hover Abort
                    , onMouseLeave <| Hover Neither
                    , onBlur <| Hover Neither
                    , style Styles.abortButton
                    ]
                    [ Html.div
                        [ style <| Styles.abortIcon <| hoveredButton == Abort ]
                        []
                    ]

            else
                Html.text ""

        buildTitle =
            case build.job of
                Just { jobName, teamName, pipelineName } ->
                    let
                        jobUrl =
                            "/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/jobs/" ++ jobName
                    in
                    Html.a
                        [ StrictEvents.onLeftClick <| NavTo jobUrl
                        , href jobUrl
                        ]
                        [ Html.span [ class "build-name" ] [ Html.text jobName ]
                        , Html.text (" #" ++ build.name)
                        ]

                _ ->
                    Html.text ("build #" ++ toString build.id)
    in
    Html.div [ class "fixed-header" ]
        [ Html.div
            [ id "build-header"
            , class ("build-header " ++ Concourse.BuildStatus.show build.status)
            , style
                [ ( "display", "flex" )
                , ( "justify-content", "space-between" )
                ]
            ]
            [ Html.div []
                [ Html.h1 [] [ buildTitle ]
                , case now of
                    Just n ->
                        BuildDuration.view build.duration n

                    Nothing ->
                        Html.text ""
                ]
            , Html.div
                [ style [ ( "display", "flex" ) ] ]
                [ abortButton, triggerButton ]
            ]
        , Html.div
            [ onMouseWheel ScrollBuilds
            ]
            [ lazyViewHistory build history ]
        ]


lazyViewHistory : Concourse.Build -> List Concourse.Build -> Html Msg
lazyViewHistory currentBuild builds =
    Html.Lazy.lazy2 viewHistory currentBuild builds


viewHistory : Concourse.Build -> List Concourse.Build -> Html Msg
viewHistory currentBuild builds =
    Html.ul [ id "builds" ]
        (List.map (viewHistoryItem currentBuild) builds)


viewHistoryItem : Concourse.Build -> Concourse.Build -> Html Msg
viewHistoryItem currentBuild build =
    Html.li
        [ if build.id == currentBuild.id then
            class (Concourse.BuildStatus.show currentBuild.status ++ " current")

          else
            class (Concourse.BuildStatus.show build.status)
        ]
        [ Html.a
            [ onLeftClick (SwitchToBuild build)
            , href (Routes.buildRoute build)
            ]
            [ Html.text build.name
            ]
        ]


durationTitle : Date -> List (Html Msg) -> Html Msg
durationTitle date content =
    Html.div [ title (Date.Format.format "%b" date) ] content


getScrollBehavior : Model -> Autoscroll.ScrollBehavior
getScrollBehavior model =
    case ( model.autoScroll, model.currentBuild |> RemoteData.toMaybe ) of
        ( False, _ ) ->
            Autoscroll.NoScroll

        ( True, Nothing ) ->
            Autoscroll.NoScroll

        ( True, Just cb ) ->
            case cb.build.status of
                Concourse.BuildStatusSucceeded ->
                    Autoscroll.NoScroll

                Concourse.BuildStatusPending ->
                    Autoscroll.NoScroll

                _ ->
                    Autoscroll.ScrollWindow


handleOutMsg : BuildOutput.OutMsg -> Model -> ( Model, List Effect )
handleOutMsg outMsg model =
    case outMsg of
        BuildOutput.OutNoop ->
            ( model, [] )

        BuildOutput.OutBuildStatus status date ->
            case model.currentBuild |> RemoteData.toMaybe of
                Nothing ->
                    ( model, [] )

                Just currentBuild ->
                    let
                        build =
                            currentBuild.build

                        duration =
                            build.duration

                        newDuration =
                            if Concourse.BuildStatus.isRunning status then
                                duration

                            else
                                { duration | finishedAt = Just date }

                        newStatus =
                            if Concourse.BuildStatus.isRunning build.status then
                                status

                            else
                                build.status

                        newBuild =
                            { build | status = newStatus, duration = newDuration }
                    in
                    ( { model
                        | history = updateHistory newBuild model.history
                        , currentBuild = RemoteData.Success { currentBuild | build = newBuild }
                      }
                    , if Concourse.BuildStatus.isRunning build.status then
                        [ SetFavIcon status ]

                      else
                        []
                    )


updateHistory : Concourse.Build -> List Concourse.Build -> List Concourse.Build
updateHistory newBuild =
    List.map <|
        \build ->
            if build.id == newBuild.id then
                newBuild

            else
                build
