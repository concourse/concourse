module Build.Header.Header exposing
    ( handleCallback
    , handleDelivery
    , header
    , update
    , view
    )

import Application.Models exposing (Session)
import Build.Header.Models exposing (BuildPageType(..), Model)
import Build.Header.Views as Views
import Build.Models exposing (toMaybe)
import Build.StepTree.Models as STModels
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination exposing (Paginated)
import EffectTransformer exposing (ET)
import HoverState
import Html exposing (Html)
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


historyId : String
historyId =
    "builds"


header : Session -> Model r -> Concourse.Build -> Views.Header
header session model build =
    { leftWidgets =
        [ Views.Title (name model) (currentJob model)
        , Views.Duration session.timeZone model.duration model.now
        ]
    , rightWidgets =
        [ Views.Button
            (if Concourse.BuildStatus.isRunning model.status then
                Just
                    { type_ = Views.Abort
                    , isClickable = True
                    , backgroundShade =
                        if
                            HoverState.isHovered
                                AbortBuildButton
                                session.hovered
                        then
                            Views.Dark

                        else
                            Views.Light
                    , backgroundColor = Concourse.BuildStatus.BuildStatusFailed
                    , tooltip = False
                    }

             else
                Nothing
            )
        , Views.Button
            (if currentJob model /= Nothing then
                let
                    isHovered =
                        HoverState.isHovered
                            TriggerBuildButton
                            session.hovered
                in
                Just
                    { type_ = Views.Trigger
                    , isClickable = not model.disableManualTrigger
                    , backgroundShade =
                        if isHovered then
                            Views.Dark

                        else
                            Views.Light
                    , backgroundColor = model.status
                    , tooltip = isHovered && model.disableManualTrigger
                    }

             else
                Nothing
            )
        ]
    , backgroundColor = model.status
    , history = Views.History build model.history
    }


name : Model r -> String
name { page } =
    case page of
        OneOffBuildPage id ->
            String.fromInt id

        JobBuildPage { buildName } ->
            buildName


view : Session -> Model r -> Concourse.Build -> Html Message
view session model build =
    header session model build |> Views.viewHeader


currentJob : Model r -> Maybe Concourse.JobIdentifier
currentJob =
    .build
        >> RemoteData.toMaybe
        >> Maybe.andThen .job


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
                    model.build
                        |> RemoteData.toMaybe
                        |> Maybe.map (.id >> String.fromInt)
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
            case ( model.build, result ) of
                ( RemoteData.Success build, Ok envelopes ) ->
                    case
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
                    of
                        Just ( status, date ) ->
                            ( let
                                newBuild =
                                    Concourse.receiveStatus status date build
                              in
                              { model
                                | history =
                                    updateHistory newBuild model.history
                                , duration = newBuild.duration
                                , status = newBuild.status
                              }
                            , effects
                            )

                        Nothing ->
                            ( model, effects )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


handleKeyPressed : Keyboard.KeyEvent -> ET (Model r)
handleKeyPressed keyEvent ( model, effects ) =
    let
        currentBuild =
            model.build |> RemoteData.toMaybe
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
                            (model.build
                                |> RemoteData.toMaybe
                                |> Maybe.map
                                    (.id >> DoAbortBuild >> (::) >> Tuple.mapSecond)
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

        BuildHistoryFetched (Err _) ->
            -- https://github.com/concourse/concourse/issues/3201
            ( { model | fetchingHistory = False }, effects )

        _ ->
            ( model, effects )


handleBuildFetched : Int -> Concourse.Build -> ET (Model r)
handleBuildFetched browsingIndex build ( model, effects ) =
    if browsingIndex == model.browsingIndex then
        ( { model
            | history = updateHistory build model.history
            , fetchingHistory = True
          }
        , effects
        )

    else
        ( model, effects )


handleHistoryFetched : Paginated Concourse.Build -> ET (Model r)
handleHistoryFetched history ( model, effects ) =
    let
        newModel =
            { model
                | history = List.append model.history history.content
                , nextPage = history.pagination.nextPage
                , fetchingHistory = False
            }
    in
    case ( model.build, currentJob model ) of
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
