module Build.Header.Header exposing (handleCallback, handleDelivery, update, viewBuildHeader)

import Application.Models exposing (Session)
import Build.Header.Models exposing (Model)
import Build.Models exposing (BuildPageType(..), toMaybe)
import Build.StepTree.Models as STModels
import Build.Styles as Styles
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination exposing (Paginated)
import EffectTransformer exposing (ET)
import HoverState
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , classList
        , href
        , id
        , style
        )
import Html.Events exposing (onBlur, onFocus, onMouseEnter, onMouseLeave)
import Html.Lazy
import Keyboard
import List.Extra
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects exposing (Effect(..), ScrollDirection(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import RemoteData
import Routes
import StrictEvents exposing (onLeftClick, onMouseWheel)
import Views.BuildDuration as BuildDuration
import Views.Icon as Icon


historyId : String
historyId =
    "builds"


viewBuildHeader :
    Session
    -> Model r
    -> Concourse.Build
    -> Html Message
viewBuildHeader session model build =
    let
        triggerButton =
            case currentJob model of
                Just _ ->
                    let
                        buttonDisabled =
                            model.disableManualTrigger

                        buttonHovered =
                            HoverState.isHovered
                                TriggerBuildButton
                                session.hovered
                    in
                    Html.button
                        ([ attribute "role" "button"
                         , attribute "tabindex" "0"
                         , attribute "aria-label" "Trigger Build"
                         , attribute "title" "Trigger Build"
                         , onLeftClick <| Click TriggerBuildButton
                         , onMouseEnter <| Hover <| Just TriggerBuildButton
                         , onFocus <| Hover <| Just TriggerBuildButton
                         , onMouseLeave <| Hover Nothing
                         , onBlur <| Hover Nothing
                         ]
                            ++ Styles.triggerButton
                                buttonDisabled
                                buttonHovered
                                build.status
                        )
                    <|
                        [ Icon.icon
                            { sizePx = 40
                            , image = "ic-add-circle-outline-white.svg"
                            }
                            []
                        ]
                            ++ (if buttonDisabled && buttonHovered then
                                    [ Html.div
                                        Styles.triggerTooltip
                                        [ Html.text <|
                                            "manual triggering disabled "
                                                ++ "in job config"
                                        ]
                                    ]

                                else
                                    []
                               )

                Nothing ->
                    Html.text ""

        abortHovered =
            HoverState.isHovered AbortBuildButton session.hovered

        abortButton =
            if Concourse.BuildStatus.isRunning build.status then
                Html.button
                    ([ onLeftClick (Click <| AbortBuildButton)
                     , attribute "role" "button"
                     , attribute "tabindex" "0"
                     , attribute "aria-label" "Abort Build"
                     , attribute "title" "Abort Build"
                     , onMouseEnter <| Hover <| Just AbortBuildButton
                     , onFocus <| Hover <| Just AbortBuildButton
                     , onMouseLeave <| Hover Nothing
                     , onBlur <| Hover Nothing
                     ]
                        ++ Styles.abortButton abortHovered
                    )
                    [ Icon.icon
                        { sizePx = 40
                        , image = "ic-abort-circle-outline-white.svg"
                        }
                        []
                    ]

            else
                Html.text ""

        buildTitle =
            case build.job of
                Just jobId ->
                    let
                        jobRoute =
                            Routes.Job { id = jobId, page = Nothing }
                    in
                    Html.a
                        [ href <| Routes.toString jobRoute ]
                        [ Html.span [ class "build-name" ] [ Html.text jobId.jobName ]
                        , Html.text (" #" ++ build.name)
                        ]

                _ ->
                    Html.text ("build #" ++ String.fromInt build.id)
    in
    Html.div [ class "fixed-header" ]
        [ Html.div
            ([ id "build-header"
             , class "build-header"
             ]
                ++ Styles.header build.status
            )
            [ Html.div []
                [ Html.h1 [] [ buildTitle ]
                , case model.now of
                    Just n ->
                        BuildDuration.view session.timeZone build.duration n

                    Nothing ->
                        Html.text ""
                ]
            , Html.div
                [ style "display" "flex" ]
                [ abortButton, triggerButton ]
            ]
        , Html.div
            [ onMouseWheel ScrollBuilds ]
            [ lazyViewHistory build model.history ]
        ]


currentJob : Model r -> Maybe Concourse.JobIdentifier
currentJob =
    .currentBuild
        >> RemoteData.toMaybe
        >> Maybe.map .build
        >> Maybe.andThen .job


lazyViewHistory : Concourse.Build -> List Concourse.Build -> Html Message
lazyViewHistory currentBuild builds =
    Html.Lazy.lazy2 viewHistory currentBuild builds


viewHistory : Concourse.Build -> List Concourse.Build -> Html Message
viewHistory currentBuild builds =
    Html.ul [ id historyId ]
        (List.map (viewHistoryItem currentBuild) builds)


viewHistoryItem : Concourse.Build -> Concourse.Build -> Html Message
viewHistoryItem currentBuild build =
    Html.li
        ([ classList [ ( "current", build.id == currentBuild.id ) ]
         , id <| String.fromInt build.id
         ]
            ++ Styles.historyItem build.status
        )
        [ Html.a
            [ onLeftClick <| Click <| BuildTab build
            , href <| Routes.toString <| Routes.buildRoute build
            ]
            [ Html.text build.name ]
        ]


handleDelivery : Delivery -> ET (Model r)
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keyEvent ->
            handleKeyPressed keyEvent ( model, effects )

        KeyUp keyEvent ->
            case keyEvent.code of
                Keyboard.T ->
                    ( { model | previousTriggerBuildByKey = False }, effects )

                _ ->
                    ( model, effects )

        ElementVisible ( id, True ) ->
            let
                lastBuildVisible =
                    model.history
                        |> List.Extra.last
                        |> Maybe.map .id
                        |> Maybe.map String.fromInt
                        |> Maybe.map ((==) id)
                        |> Maybe.withDefault False

                hasNextPage =
                    model.nextPage /= Nothing

                needsToFetchMorePages =
                    not model.fetchingHistory && lastBuildVisible && hasNextPage
            in
            case currentJob model of
                Just job ->
                    if needsToFetchMorePages then
                        ( { model | fetchingHistory = True }
                        , effects ++ [ FetchBuildHistory job model.nextPage ]
                        )

                    else
                        ( model, effects )

                Nothing ->
                    ( model, effects )

        ElementVisible ( id, False ) ->
            let
                currentBuildInvisible =
                    model.currentBuild
                        |> RemoteData.toMaybe
                        |> Maybe.map (.build >> .id >> String.fromInt)
                        |> Maybe.map ((==) id)
                        |> Maybe.withDefault False

                shouldScroll =
                    currentBuildInvisible && not model.scrolledToCurrentBuild
            in
            ( { model | scrolledToCurrentBuild = True }
            , effects
                ++ (if shouldScroll then
                        [ Scroll (ToId id) historyId ]

                    else
                        []
                   )
            )

        EventsReceived result ->
            case result of
                Ok envelopes ->
                    let
                        currentBuild =
                            model.currentBuild |> RemoteData.toMaybe

                        buildStatus =
                            envelopes
                                |> List.filterMap
                                    (\{ data } ->
                                        case data of
                                            STModels.BuildStatus status date ->
                                                Just ( status, date )

                                            _ ->
                                                Nothing
                                    )
                                |> List.Extra.last
                    in
                    case ( currentBuild, currentBuild |> Maybe.andThen (.output >> toMaybe) ) of
                        ( Just cb, Just _ ) ->
                            case buildStatus of
                                Nothing ->
                                    ( model, effects )

                                Just ( status, date ) ->
                                    ( { model
                                        | history =
                                            updateHistory
                                                (Concourse.receiveStatus status date cb.build)
                                                model.history
                                      }
                                    , effects
                                    )

                        _ ->
                            ( model, effects )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


handleKeyPressed : Keyboard.KeyEvent -> ET (Model r)
handleKeyPressed keyEvent ( model, effects ) =
    let
        currentBuild =
            model.currentBuild |> RemoteData.toMaybe |> Maybe.map .build
    in
    if Keyboard.hasControlModifier keyEvent then
        ( model, effects )

    else
        case ( keyEvent.code, keyEvent.shiftKey ) of
            ( Keyboard.H, False ) ->
                case Maybe.andThen (nextBuild model.history) currentBuild of
                    Just build ->
                        ( model
                        , effects
                            ++ [ NavigateTo <| Routes.toString <| Routes.buildRoute build ]
                        )

                    Nothing ->
                        ( model, effects )

            ( Keyboard.L, False ) ->
                case
                    Maybe.andThen (prevBuild model.history) currentBuild
                of
                    Just build ->
                        ( model
                        , effects
                            ++ [ NavigateTo <| Routes.toString <| Routes.buildRoute build ]
                        )

                    Nothing ->
                        ( model, effects )

            ( Keyboard.T, True ) ->
                if not model.previousTriggerBuildByKey then
                    (currentJob model
                        |> Maybe.map (DoTriggerBuild >> (::) >> Tuple.mapSecond)
                        |> Maybe.withDefault identity
                    )
                        ( { model | previousTriggerBuildByKey = True }, effects )

                else
                    ( model, effects )

            ( Keyboard.A, True ) ->
                if currentBuild == List.head model.history then
                    case currentBuild of
                        Just _ ->
                            (model.currentBuild
                                |> RemoteData.toMaybe
                                |> Maybe.map
                                    (.build >> .id >> DoAbortBuild >> (::) >> Tuple.mapSecond)
                                |> Maybe.withDefault identity
                            )
                                ( model, effects )

                        Nothing ->
                            ( model, effects )

                else
                    ( model, effects )

            _ ->
                ( model, effects )


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


update : Message -> ET (Model r)
update msg ( model, effects ) =
    case msg of
        ScrollBuilds event ->
            let
                scroll =
                    if event.deltaX == 0 then
                        [ Scroll (Sideways event.deltaY) historyId ]

                    else
                        [ Scroll (Sideways -event.deltaX) historyId ]

                checkVisibility =
                    case model.history |> List.Extra.last of
                        Just build ->
                            [ Effects.CheckIsVisible <| String.fromInt build.id ]

                        Nothing ->
                            []
            in
            ( model, effects ++ scroll ++ checkVisibility )

        _ ->
            ( model, effects )


handleCallback : Callback -> ET (Model r)
handleCallback callback ( model, effects ) =
    case callback of
        BuildFetched (Ok ( browsingIndex, build )) ->
            handleBuildFetched browsingIndex build ( model, effects )

        BuildTriggered (Ok build) ->
            ( { model | history = build :: model.history }
            , effects
                ++ [ NavigateTo <| Routes.toString <| Routes.buildRoute build ]
            )

        BuildHistoryFetched (Ok history) ->
            handleHistoryFetched history ( model, effects )

        _ ->
            ( model, effects )


handleBuildFetched : Int -> Concourse.Build -> ET (Model r)
handleBuildFetched browsingIndex build ( model, effects ) =
    if browsingIndex == model.browsingIndex then
        ( { model | history = updateHistory build model.history }
        , effects
        )

    else
        ( model, effects )


handleHistoryFetched : Paginated Concourse.Build -> ET (Model r)
handleHistoryFetched history ( model, effects ) =
    let
        currentBuild =
            model.currentBuild |> RemoteData.map .build

        newModel =
            { model
                | history = List.append model.history history.content
                , nextPage = history.pagination.nextPage
                , fetchingHistory = False
            }
    in
    case ( currentBuild, currentJob model ) of
        ( RemoteData.Success build, Just job ) ->
            if List.member build newModel.history then
                ( newModel, effects ++ [ CheckIsVisible <| String.fromInt build.id ] )

            else
                ( { newModel | fetchingHistory = True }, effects ++ [ FetchBuildHistory job history.pagination.nextPage ] )

        _ ->
            ( newModel, effects )


updateHistory : Concourse.Build -> List Concourse.Build -> List Concourse.Build
updateHistory newBuild =
    List.map <|
        \build ->
            if build.id == newBuild.id then
                newBuild

            else
                build
