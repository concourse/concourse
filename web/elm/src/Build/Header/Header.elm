module Build.Header.Header exposing
    ( changeToBuild
    , handleCallback
    , handleDelivery
    , header
    , update
    , view
    )

import Api.Endpoints as Endpoints
import Application.Models exposing (Session)
import Build.Header.Models exposing (BuildPageType(..), HistoryItem, Model)
import Build.Header.Views as Views
import Build.StepTree.Models as STModels
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination exposing (Paginated)
import DateFormat
import Duration exposing (Duration)
import EffectTransformer exposing (ET)
import HoverState
import Html exposing (Html)
import List.Extra
import Maybe.Extra
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.ScrollDirection exposing (ScrollDirection(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import RemoteData exposing (WebData)
import Routes
import StrictEvents exposing (DeltaMode(..))
import Time


historyId : String
historyId =
    "builds"


header : Session -> Model r -> Views.Header
header session model =
    { leftWidgets =
        [ Views.Title model.name model.job
        , Views.Duration (duration session model)
        ]
    , rightWidgets =
        if isPipelineArchived session.pipelines model.job then
            []

        else
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

                 else if model.job /= Nothing then
                    let
                        isHovered =
                            HoverState.isHovered
                                RerunBuildButton
                                session.hovered
                    in
                    Just
                        { type_ = Views.Rerun
                        , isClickable = True
                        , backgroundShade =
                            if isHovered then
                                Views.Dark

                            else
                                Views.Light
                        , backgroundColor = model.status
                        , tooltip = isHovered
                        }

                 else
                    Nothing
                )
            , Views.Button
                (if model.job /= Nothing then
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
    , tabs = tabs model
    }


isPipelineArchived :
    WebData (List Concourse.Pipeline)
    -> Maybe Concourse.JobIdentifier
    -> Bool
isPipelineArchived pipelines jobId =
    case jobId of
        Just { pipelineName, teamName } ->
            pipelines
                |> RemoteData.withDefault []
                |> List.Extra.find (\p -> p.name == pipelineName && p.teamName == teamName)
                |> Maybe.map .archived
                |> Maybe.withDefault False

        Nothing ->
            False


tabs : Model r -> List Views.BuildTab
tabs model =
    model.history
        |> List.map
            (\b ->
                { id = b.id
                , name = b.name
                , background = b.status
                , href = Routes.buildRoute b.id b.name model.job
                , isCurrent = b.id == model.id
                }
            )


historyItem : Model r -> HistoryItem
historyItem model =
    { id = model.id
    , name = model.name
    , status = model.status
    , duration = model.duration
    }


changeToBuild : BuildPageType -> ET (Model r)
changeToBuild pageType ( model, effects ) =
    case pageType of
        JobBuildPage buildID ->
            ( model.history
                |> List.Extra.find (.name >> (==) buildID.buildName)
                |> Maybe.map
                    (\b ->
                        { model
                            | id = b.id
                            , status = b.status
                            , duration = b.duration
                            , name = b.name
                        }
                    )
                |> Maybe.withDefault model
            , effects
            )

        _ ->
            ( model, effects )


duration : Session -> Model r -> Views.BuildDuration
duration session model =
    case ( model.duration.startedAt, model.duration.finishedAt ) of
        ( Nothing, Nothing ) ->
            Views.Pending

        ( Nothing, Just finished ) ->
            Views.Cancelled (timestamp session.timeZone model.now finished)

        ( Just started, Nothing ) ->
            Views.Running (timestamp session.timeZone model.now started)

        ( Just started, Just finished ) ->
            Views.Finished
                { started = timestamp session.timeZone model.now started
                , finished = timestamp session.timeZone model.now finished
                , duration = timespan <| Duration.between started finished
                }


timestamp : Time.Zone -> Maybe Time.Posix -> Time.Posix -> Views.Timestamp
timestamp timeZone now time =
    let
        ago =
            Maybe.map (Duration.between time) now

        formatted =
            format timeZone time
    in
    case ago of
        Just a ->
            if a < 24 * 60 * 60 * 1000 then
                Views.Relative (timespan a) formatted

            else
                Views.Absolute formatted (Just <| timespan a)

        Nothing ->
            Views.Absolute formatted Nothing


format : Time.Zone -> Time.Posix -> String
format =
    DateFormat.format
        [ DateFormat.monthNameAbbreviated
        , DateFormat.text " "
        , DateFormat.dayOfMonthNumber
        , DateFormat.text " "
        , DateFormat.yearNumber
        , DateFormat.text " "
        , DateFormat.hourFixed
        , DateFormat.text ":"
        , DateFormat.minuteFixed
        , DateFormat.text ":"
        , DateFormat.secondFixed
        , DateFormat.text " "
        , DateFormat.amPmUppercase
        ]


timespan : Duration -> Views.Timespan
timespan dur =
    let
        seconds =
            dur // 1000

        remainingSeconds =
            remainderBy 60 seconds

        minutes =
            seconds // 60

        remainingMinutes =
            remainderBy 60 minutes

        hours =
            minutes // 60

        remainingHours =
            remainderBy 24 hours

        days =
            hours // 24
    in
    case ( ( days, remainingHours ), remainingMinutes, remainingSeconds ) of
        ( ( 0, 0 ), 0, s ) ->
            Views.JustSeconds s

        ( ( 0, 0 ), m, s ) ->
            Views.MinutesAndSeconds m s

        ( ( 0, h ), m, _ ) ->
            Views.HoursAndMinutes h m

        ( ( d, h ), _, _ ) ->
            Views.DaysAndHours d h


view : Session -> Model r -> Html Message
view session model =
    header session model |> Views.viewHeader


handleDelivery : Delivery -> ET (Model r)
handleDelivery delivery ( model, effects ) =
    case delivery of
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
            case model.job of
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
                    String.fromInt model.id == id

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

        BuildEventsReceived result ->
            Result.toMaybe result
                |> Maybe.map
                    (List.filter
                        (.url
                            >> String.endsWith
                                (Endpoints.BuildEventStream
                                    |> Endpoints.Build model.id
                                    |> Endpoints.toString []
                                )
                        )
                    )
                |> Maybe.map
                    (List.filterMap
                        (\{ data } ->
                            case data of
                                STModels.BuildStatus status date ->
                                    Just ( status, date )

                                _ ->
                                    Nothing
                        )
                    )
                |> Maybe.andThen List.Extra.last
                |> Maybe.map
                    (\( status, date ) ->
                        let
                            newStatus =
                                if Concourse.BuildStatus.isRunning model.status then
                                    status

                                else
                                    model.status

                            newDuration =
                                let
                                    dur =
                                        model.duration
                                in
                                { dur
                                    | finishedAt =
                                        if Concourse.BuildStatus.isRunning status then
                                            dur.finishedAt

                                        else
                                            Just date
                                }
                        in
                        ( { model
                            | history =
                                List.Extra.updateIf (.id >> (==) model.id)
                                    (\item ->
                                        { item
                                            | status = newStatus
                                            , duration = newDuration
                                        }
                                    )
                                    model.history
                            , duration = newDuration
                            , status = newStatus
                          }
                        , effects
                        )
                    )
                |> Maybe.withDefault ( model, effects )

        _ ->
            ( model, effects )


update : Message -> ET (Model r)
update msg ( model, effects ) =
    case msg of
        ScrollBuilds event ->
            let
                scrollFactor =
                    case event.deltaMode of
                        DeltaModePixel ->
                            1

                        DeltaModeLine ->
                            20

                        DeltaModePage ->
                            800

                scroll =
                    if event.deltaX == 0 then
                        [ Scroll (Sideways <| event.deltaY * scrollFactor) historyId ]

                    else
                        [ Scroll (Sideways <| -event.deltaX * scrollFactor) historyId ]

                checkVisibility =
                    case model.history |> List.Extra.last of
                        Just b ->
                            [ Effects.CheckIsVisible <| String.fromInt b.id ]

                        Nothing ->
                            []
            in
            ( model, effects ++ scroll ++ checkVisibility )

        Click RerunBuildButton ->
            ( model
            , effects
                ++ (model.job
                        |> Maybe.map
                            (\j ->
                                RerunJobBuild
                                    { teamName = j.teamName
                                    , pipelineName = j.pipelineName
                                    , jobName = j.jobName
                                    , buildName = model.name
                                    }
                            )
                        |> Maybe.Extra.toList
                   )
            )

        _ ->
            ( model, effects )


handleCallback : Callback -> ET (Model r)
handleCallback callback ( model, effects ) =
    case callback of
        BuildFetched (Ok b) ->
            handleBuildFetched b ( model, effects )

        BuildTriggered (Ok b) ->
            ( { model
                | history =
                    ({ id = b.id
                     , name = b.name
                     , status = b.status
                     , duration = b.duration
                     }
                        :: model.history
                    )
                        |> List.sortWith
                            (\n m ->
                                Maybe.map2
                                    (\( i, j ) ( k, l ) ->
                                        case compare i k of
                                            EQ ->
                                                compare j l

                                            x ->
                                                x
                                    )
                                    (buildName n.name)
                                    (buildName m.name)
                                    |> Maybe.withDefault EQ
                            )
                        |> List.reverse
              }
            , effects
                ++ [ NavigateTo <| Routes.toString <| Routes.buildRoute b.id b.name model.job ]
            )

        BuildHistoryFetched (Ok history) ->
            handleHistoryFetched history ( model, effects )

        BuildHistoryFetched (Err _) ->
            -- https://github.com/concourse/concourse/issues/3201
            ( { model | fetchingHistory = False }, effects )

        _ ->
            ( model, effects )


handleBuildFetched : Concourse.Build -> ET (Model r)
handleBuildFetched b ( model, effects ) =
    if not model.hasLoadedYet || model.id == b.id then
        ( { model
            | hasLoadedYet = True
            , history =
                List.Extra.setIf (.id >> (==) b.id)
                    { id = b.id
                    , name = b.name
                    , status = b.status
                    , duration = b.duration
                    }
                    model.history
            , fetchingHistory = True
            , duration = b.duration
            , status = b.status
            , job = b.job
            , id = b.id
            , name = b.name
          }
        , effects
        )

    else
        ( model, effects )


buildName : String -> Maybe ( Int, Int )
buildName s =
    case String.split "." s |> List.map String.toInt of
        [ Just n ] ->
            Just ( n, 0 )

        [ Just n, Just m ] ->
            Just ( n, m )

        _ ->
            Nothing


handleHistoryFetched : Paginated Concourse.Build -> ET (Model r)
handleHistoryFetched history ( model, effects ) =
    let
        newModel =
            { model
                | history =
                    model.history
                        ++ (history.content
                                |> List.map
                                    (\b ->
                                        { id = b.id
                                        , name = b.name
                                        , status = b.status
                                        , duration = b.duration
                                        }
                                    )
                           )
                , nextPage = history.pagination.nextPage
                , fetchingHistory = False
            }
    in
    case model.job of
        Just job ->
            if List.member (historyItem model) newModel.history then
                ( newModel
                , effects ++ [ CheckIsVisible <| String.fromInt <| model.id ]
                )

            else
                ( { newModel | fetchingHistory = True }
                , effects ++ [ FetchBuildHistory job history.pagination.nextPage ]
                )

        _ ->
            ( newModel, effects )
