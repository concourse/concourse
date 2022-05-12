module Build.Header.Header exposing
    ( changeToBuild
    , handleCallback
    , handleDelivery
    , header
    , tooltip
    , update
    , view
    )

import Api.Endpoints as Endpoints
import Application.Models exposing (Session)
import Build.Header.Models exposing (BuildPageType(..), CommentBarVisibility(..), HistoryItem, Model, commentBarIsVisible)
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
import Html.Attributes exposing (style)
import List.Extra
import Login.Login as Login
import Maybe.Extra
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects exposing (Effect(..), toHtmlID)
import Message.Message exposing (DomID(..), Message(..))
import Message.ScrollDirection exposing (ScrollDirection(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Routes
import SideBar.SideBar exposing (byPipelineId, lookupPipeline)
import StrictEvents exposing (DeltaMode(..))
import Time
import Tooltip
import UserState
import Views.CommentBar as CommentBar


historyId : String
historyId =
    "builds"


header : Session -> Model r -> Views.Header
header session model =
    let
        archived =
            case model.job of
                Just jobID ->
                    lookupPipeline (byPipelineId jobID) session
                        |> Maybe.map .archived
                        |> Maybe.withDefault False

                Nothing ->
                    False
    in
    { leftWidgets =
        [ Views.Title model.name model.job model.createdBy
        , Views.Duration (duration session model)
        ]
    , rightWidgets =
        (if model.authorized then
            [ Views.Button
                (Just
                    { type_ = Views.ToggleComment
                    , isClickable = True
                    , backgroundShade =
                        let
                            isHovered =
                                HoverState.isHovered
                                    ToggleBuildCommentButton
                                    session.hovered
                        in
                        case ( model.comment, isHovered ) of
                            ( Hidden _, False ) ->
                                Views.Light

                            ( Hidden _, True ) ->
                                Views.Dark

                            ( Visible _, False ) ->
                                Views.Dark

                            ( Visible _, True ) ->
                                Views.Light
                    , backgroundColor = model.status
                    }
                )
            ]

         else
            []
        )
            ++ (if archived then
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
                                }

                         else
                            Nothing
                        )
                    ]
               )
    , backgroundColor = model.status
    , tabs = tabs model
    , comment =
        if not model.authorized then
            Nothing

        else
            commentBarIsVisible model.comment
                |> Maybe.map
                    (\c ->
                        ( c
                        , { hover = session.hovered
                          , editable =
                                case model.job of
                                    Nothing ->
                                        False

                                    Just job ->
                                        UserState.isMember
                                            { teamName = job.teamName
                                            , userState = session.userState
                                            }
                          }
                        )
                    )
    }


buttonTooltip : String -> Tooltip.Tooltip
buttonTooltip text =
    { body = Html.text text
    , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.End }
    , arrow = Just 5
    , containerAttrs = Nothing
    }


tooltip : Model r -> Session -> Maybe Tooltip.Tooltip
tooltip model session =
    case session.hovered of
        HoverState.Tooltip TriggerBuildButton _ ->
            Just
                (buttonTooltip <|
                    if model.disableManualTrigger then
                        "manual triggering disabled in job config"

                    else
                        "trigger a new build"
                )

        HoverState.Tooltip ToggleBuildCommentButton _ ->
            Just
                (buttonTooltip <|
                    case model.comment of
                        Hidden comment ->
                            if String.isEmpty comment then
                                "add build comment"

                            else
                                "show build comment"

                        Visible _ ->
                            "hide build comment"
                )

        HoverState.Tooltip RerunBuildButton _ ->
            Just (buttonTooltip "re-run with the same inputs")

        HoverState.Tooltip AbortBuildButton _ ->
            Just (buttonTooltip "abort the current build")

        HoverState.Tooltip JobName _ ->
            Just (buttonTooltip "view job builds")

        HoverState.Tooltip (BuildTab id _) _ ->
            List.head (List.filter (\b -> b.id == id) model.history)
                |> Maybe.andThen
                    (\b ->
                        let
                            lines =
                                String.lines b.comment
                        in
                        if String.isEmpty b.comment then
                            Nothing

                        else
                            List.head lines
                                |> Maybe.map
                                    (\text ->
                                        { body = Html.text text
                                        , attachPosition = { direction = Tooltip.Bottom, alignment = Tooltip.Start }
                                        , arrow = Nothing
                                        , containerAttrs =
                                            Just
                                                ([ style "max-width" "350px"
                                                 , style "white-space" "nowrap"
                                                 , style "overflow" "hidden"
                                                 , style "text-overflow" "ellipsis"
                                                 ]
                                                    ++ Tooltip.defaultTooltipStyle
                                                )
                                        }
                                    )
                    )

        HoverState.Tooltip (UserDisplayName username) _ ->
            Login.tooltip username

        _ ->
            Nothing


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
                , hasComment = not (String.isEmpty b.comment)
                }
            )


historyItem : Model r -> HistoryItem
historyItem model =
    { id = model.id
    , name = model.name
    , status = model.status
    , duration = model.duration
    , createdBy = model.createdBy
    , comment =
        case model.comment of
            Hidden comment ->
                comment

            Visible commentBar ->
                CommentBar.getContent commentBar
    }


initBuildCommentBar : String -> ( CommentBar.Model, List Effect )
initBuildCommentBar content =
    let
        model =
            { id = BuildComment
            , style = CommentBar.defaultStyle
            , state =
                if String.isEmpty content then
                    CommentBar.Editing { content = content, cached = content }

                else
                    CommentBar.Viewing content
            }

        id =
            CommentBar.getTextareaID model
    in
    ( model
    , SyncTextareaHeight id
        :: (case model.state of
                CommentBar.Editing _ ->
                    [ Focus (toHtmlID id) ]

                _ ->
                    []
           )
    )


changeToBuild : BuildPageType -> ET (Model r)
changeToBuild pageType ( model, effects ) =
    case pageType of
        JobBuildPage buildID ->
            model.history
                |> List.Extra.find (.name >> (==) buildID.buildName)
                |> Maybe.map
                    (\b ->
                        let
                            ( commentBar, initEffects ) =
                                if String.isEmpty b.comment then
                                    ( Hidden b.comment, [] )

                                else
                                    let
                                        ( cb, effs ) =
                                            initBuildCommentBar b.comment
                                    in
                                    ( Visible cb, effs )

                            updatedModel =
                                { model
                                    | id = b.id
                                    , status = b.status
                                    , duration = b.duration
                                    , name = b.name
                                    , createdBy = b.createdBy
                                    , comment = commentBar
                                }
                        in
                        ( updatedModel, effects ++ initEffects )
                    )
                |> Maybe.withDefault ( model, effects )

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

        EventsReceived result ->
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
                                    , pipelineInstanceVars = j.pipelineInstanceVars
                                    , jobName = j.jobName
                                    , buildName = model.name
                                    }
                            )
                        |> Maybe.Extra.toList
                   )
            )

        Click ToggleBuildCommentButton ->
            case model.comment of
                Hidden comment ->
                    let
                        ( updatedCommentBar, updatedEffects ) =
                            initBuildCommentBar comment
                    in
                    ( { model | comment = Visible updatedCommentBar }
                    , effects ++ updatedEffects
                    )

                Visible commentBar ->
                    ( { model | comment = Hidden (CommentBar.getContent commentBar) }
                    , effects
                    )

        _ ->
            ( model, effects )


handleCallback : Callback -> ET (Model r)
handleCallback callback ( model, effects ) =
    case callback of
        BuildFetched (Ok b) ->
            handleBuildFetched b ( model, effects )

        BuildCommentSet id savedComment result ->
            let
                savedSuccessfully =
                    result |> Result.toMaybe |> Maybe.Extra.isJust

                updatedComment =
                    if model.id == id then
                        { model
                            | comment =
                                case model.comment of
                                    Hidden _ ->
                                        Hidden savedComment

                                    Visible commentBar ->
                                        Visible <| CommentBar.saveCallback ( savedSuccessfully, savedComment ) commentBar
                        }

                    else
                        model

                updatedHistory =
                    { updatedComment
                        | history =
                            List.Extra.updateIf (.id >> (==) id)
                                (\b ->
                                    { b | comment = savedComment }
                                )
                                model.history
                    }
            in
            ( updatedHistory, effects )

        BuildTriggered (Ok b) ->
            ( { model
                | history =
                    ({ id = b.id
                     , name = b.name
                     , status = b.status
                     , duration = b.duration
                     , createdBy = b.createdBy
                     , comment = b.comment
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
        let
            ( commentBar, commentBarEffects ) =
                if not model.hasLoadedYet then
                    if String.isEmpty b.comment then
                        ( Hidden b.comment, [] )

                    else
                        let
                            ( initCommentBar, effs ) =
                                initBuildCommentBar b.comment
                        in
                        ( Visible initCommentBar, effs )

                else
                    case model.comment of
                        Hidden _ ->
                            ( Hidden b.comment, [] )

                        Visible c ->
                            ( Visible (CommentBar.setCachedContent b.comment c), [] )
        in
        ( { model
            | hasLoadedYet = True
            , history =
                List.Extra.setIf (.id >> (==) b.id)
                    { id = b.id
                    , name = b.name
                    , status = b.status
                    , duration = b.duration
                    , createdBy = b.createdBy
                    , comment = b.comment
                    }
                    model.history
            , fetchingHistory = True
            , duration = b.duration
            , status = b.status
            , job = b.job
            , id = b.id
            , name = b.name
            , createdBy = b.createdBy
            , comment = commentBar
          }
        , effects ++ commentBarEffects
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
                                        , createdBy = b.createdBy
                                        , comment = b.comment
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
