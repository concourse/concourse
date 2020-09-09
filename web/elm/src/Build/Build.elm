module Build.Build exposing
    ( bodyId
    , changeToBuild
    , documentTitle
    , getScrollBehavior
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Api.Endpoints as Endpoints
import Application.Models exposing (Session)
import Assets
import Build.Header.Header as Header
import Build.Header.Models exposing (BuildPageType(..), CurrentOutput(..))
import Build.Models exposing (Model, toMaybe)
import Build.Output.Models exposing (OutputModel)
import Build.Output.Output
import Build.Shortcuts as Shortcuts
import Build.StepTree.Models as STModels
import Build.StepTree.StepTree as StepTree
import Build.Styles as Styles
import Colors
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import DateFormat
import Dict exposing (Dict)
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
        , tabindex
        , title
        )
import Html.Lazy
import Http
import List.Extra
import Login.Login as Login
import Maybe.Extra
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.ScrollDirection as ScrollDirection
import Message.Subscription as Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Routes
import SideBar.SideBar as SideBar
import StrictEvents exposing (onScroll)
import String
import Time
import Tooltip
import UpdateMsg exposing (UpdateMsg)
import Views.Icon as Icon
import Views.LoadingIndicator as LoadingIndicator
import Views.NotAuthorized as NotAuthorized
import Views.Spinner as Spinner
import Views.Styles
import Views.TopBar as TopBar


bodyId : String
bodyId =
    "build-body"


type alias Flags =
    { highlight : Routes.Highlight
    , pageType : BuildPageType
    , fromBuildPage : Maybe Build.Header.Models.BuildPageType
    }


type ScrollBehavior
    = ScrollWindow
    | ScrollToID String
    | NoScroll


init : Flags -> ( Model, List Effect )
init flags =
    changeToBuild
        flags
        ( { page = flags.pageType
          , id = 0
          , name =
                case flags.pageType of
                    OneOffBuildPage id ->
                        String.fromInt id

                    JobBuildPage { buildName } ->
                        buildName
          , now = Nothing
          , job = Nothing
          , disableManualTrigger = False
          , history = []
          , nextPage = Nothing
          , prep = Nothing
          , duration = { startedAt = Nothing, finishedAt = Nothing }
          , status = BuildStatusPending
          , output = Empty
          , autoScroll = True
          , isScrollToIdInProgress = False
          , previousKeyPress = Nothing
          , isTriggerBuildKeyDown = False
          , showHelp = False
          , highlight = flags.highlight
          , authorized = True
          , fetchingHistory = False
          , scrolledToCurrentBuild = False
          , shiftDown = False
          , isUserMenuExpanded = False
          , hasLoadedYet = False
          , notFound = False
          , reapTime = Nothing
          }
        , [ GetCurrentTime
          , GetCurrentTimeZone
          , FetchAllPipelines
          ]
        )


subscriptions : Model -> List Subscription
subscriptions model =
    let
        buildEventsUrl =
            model.output
                |> toMaybe
                |> Maybe.andThen .eventStreamUrlPath
    in
    [ OnClockTick OneSecond
    , OnClockTick FiveSeconds
    , OnKeyDown
    , OnKeyUp
    , OnElementVisible
    , OnScrolledToId
    ]
        ++ (case buildEventsUrl of
                Nothing ->
                    []

                Just url ->
                    [ Subscription.FromEventSource ( url, [ "end", "event" ] ) ]
           )


changeToBuild : Flags -> ET Model
changeToBuild { highlight, pageType, fromBuildPage } ( model, effects ) =
    let
        newModel =
            { model | page = pageType }
    in
    (if fromBuildPage == Just pageType then
        ( newModel, effects )

     else
        ( { newModel
            | prep = Nothing
            , output = Empty
            , autoScroll = True
            , highlight = highlight
          }
        , case pageType of
            OneOffBuildPage buildId ->
                effects
                    ++ [ CloseBuildEventStream, FetchBuild 0 buildId ]

            JobBuildPage jbi ->
                effects
                    ++ [ CloseBuildEventStream, FetchJobBuild jbi ]
        )
    )
        |> Header.changeToBuild pageType


extractTitle : Model -> String
extractTitle model =
    case ( model.hasLoadedYet, model.job, model.page ) of
        ( True, Just { jobName }, _ ) ->
            jobName ++ " #" ++ model.name

        ( _, _, JobBuildPage { jobName, buildName } ) ->
            jobName ++ " #" ++ buildName

        ( _, _, OneOffBuildPage id ) ->
            "#" ++ String.fromInt id


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    if model.notFound then
        UpdateMsg.NotFound

    else
        UpdateMsg.AOK


handleCallback : Callback -> ET Model
handleCallback action ( model, effects ) =
    (case action of
        BuildFetched (Ok build) ->
            handleBuildFetched build ( model, effects )

        BuildFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, effects ++ [ RedirectToLogin ] )

                    else if status.code == 404 then
                        ( { model
                            | prep = Nothing
                            , notFound = True
                          }
                        , effects
                        )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        BuildAborted (Ok ()) ->
            ( model, effects )

        BuildPrepFetched buildId (Ok buildPrep) ->
            if buildId == model.id then
                handleBuildPrepFetched buildPrep ( model, effects )

            else
                ( model, effects )

        BuildPrepFetched _ (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( { model | authorized = False }, effects )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        PlanAndResourcesFetched buildId (Ok planAndResources) ->
            updateOutput
                (Build.Output.Output.planAndResourcesFetched
                    buildId
                    planAndResources
                )
                ( model
                , effects
                    ++ [ Effects.OpenBuildEventStream
                            { url =
                                Endpoints.BuildEventStream
                                    |> Endpoints.Build buildId
                                    |> Endpoints.toString []
                            , eventTypes = [ "end", "event" ]
                            }
                       , SyncStickyBuildLogHeaders
                       ]
                )

        PlanAndResourcesFetched _ (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    let
                        isAborted =
                            model.status == BuildStatusAborted
                    in
                    if status.code == 404 && isAborted then
                        ( { model | output = Cancelled }
                        , effects
                        )

                    else if status.code == 401 then
                        ( { model | authorized = False }, effects )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

        BuildJobDetailsFetched (Ok job) ->
            ( { model | disableManualTrigger = job.disableManualTrigger }
            , effects
            )

        BuildJobDetailsFetched (Err _) ->
            -- https://github.com/concourse/concourse/issues/3201
            ( model, effects )

        _ ->
            ( model, effects )
    )
        |> Header.handleCallback action


handleDelivery : { a | hovered : HoverState.HoverState } -> Delivery -> ET Model
handleDelivery session delivery ( model, effects ) =
    (case delivery of
        ClockTicked OneSecond time ->
            ( { model | now = Just time }
            , effects
                ++ (case session.hovered of
                        HoverState.Hovered (ChangedStepLabel stepID text) ->
                            [ GetViewportOf
                                (ChangedStepLabel stepID text)
                            ]

                        HoverState.Hovered (StepState stepID) ->
                            [ GetViewportOf (StepState stepID) ]

                        _ ->
                            []
                   )
            )

        ClockTicked FiveSeconds _ ->
            ( model, effects ++ [ Effects.FetchAllPipelines ] )

        WindowResized _ _ ->
            ( model, effects ++ [ SyncStickyBuildLogHeaders ] )

        EventsReceived (Ok envelopes) ->
            let
                eventSourceClosed =
                    model.output
                        |> toMaybe
                        |> Maybe.map (.eventSourceOpened >> not)
                        |> Maybe.withDefault False

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

                ( newModel, newEffects ) =
                    updateOutput
                        (Build.Output.Output.handleEnvelopes envelopes)
                        (if eventSourceClosed && (envelopes |> List.map .data |> List.member STModels.NetworkError) then
                            ( { model | authorized = False }, effects )

                         else
                            case getScrollBehavior model of
                                ScrollWindow ->
                                    ( model
                                    , effects
                                        ++ [ Effects.Scroll
                                                ScrollDirection.ToBottom
                                                bodyId
                                           ]
                                    )

                                ScrollToID id ->
                                    ( { model
                                        | highlight = Routes.HighlightNothing
                                        , autoScroll = False
                                        , isScrollToIdInProgress = True
                                      }
                                    , effects
                                        ++ [ Effects.Scroll
                                                (ScrollDirection.ToId id)
                                                bodyId
                                           ]
                                    )

                                NoScroll ->
                                    ( model, effects )
                        )
            in
            case ( model.hasLoadedYet, buildStatus ) of
                ( True, Just ( status, _ ) ) ->
                    ( newModel
                    , if Concourse.BuildStatus.isRunning model.status then
                        newEffects ++ [ SetFavIcon (Just status) ]

                      else
                        newEffects
                    )

                _ ->
                    ( newModel, newEffects )

        ScrolledToId _ ->
            ( { model | isScrollToIdInProgress = False }, effects )

        _ ->
            ( model, effects )
    )
        |> Shortcuts.handleDelivery delivery
        |> Header.handleDelivery delivery


update : Message -> ET Model
update msg ( model, effects ) =
    (case msg of
        Click (BuildTab id name) ->
            ( model
            , effects
                ++ [ NavigateTo <|
                        Routes.toString <|
                            Routes.buildRoute id name model.job
                   ]
            )

        Click TriggerBuildButton ->
            (model.job
                |> Maybe.map (DoTriggerBuild >> (::) >> Tuple.mapSecond)
                |> Maybe.withDefault identity
            )
                ( model, effects )

        Click AbortBuildButton ->
            ( model, DoAbortBuild model.id :: effects )

        Click (StepHeader id) ->
            updateOutput
                (Build.Output.Output.handleStepTreeMsg <| StepTree.toggleStep id)
                ( model, effects ++ [ SyncStickyBuildLogHeaders ] )

        Click (StepSubHeader id i) ->
            updateOutput
                (Build.Output.Output.handleStepTreeMsg <| StepTree.toggleStepSubHeader id i)
                ( model, effects ++ [ SyncStickyBuildLogHeaders ] )

        Click (StepTab id tab) ->
            updateOutput
                (Build.Output.Output.handleStepTreeMsg <| StepTree.switchTab id tab)
                ( model, effects )

        SetHighlight id line ->
            updateOutput
                (Build.Output.Output.handleStepTreeMsg <| StepTree.setHighlight id line)
                ( model, effects )

        ExtendHighlight id line ->
            updateOutput
                (Build.Output.Output.handleStepTreeMsg <| StepTree.extendHighlight id line)
                ( model, effects )

        GoToRoute route ->
            ( model, effects ++ [ NavigateTo <| Routes.toString <| route ] )

        Scrolled { scrollHeight, scrollTop, clientHeight } ->
            ( { model
                | autoScroll =
                    (scrollHeight == scrollTop + clientHeight)
                        && not model.isScrollToIdInProgress
              }
            , effects
            )

        _ ->
            ( model, effects )
    )
        |> Header.update msg


getScrollBehavior : Model -> ScrollBehavior
getScrollBehavior model =
    case model.highlight of
        Routes.HighlightLine stepID lineNumber ->
            ScrollToID <| stepID ++ ":" ++ String.fromInt lineNumber

        Routes.HighlightRange stepID beginning end ->
            if beginning <= end then
                ScrollToID <| stepID ++ ":" ++ String.fromInt beginning

            else
                NoScroll

        Routes.HighlightNothing ->
            if model.autoScroll then
                if model.hasLoadedYet then
                    case model.status of
                        BuildStatusSucceeded ->
                            NoScroll

                        BuildStatusPending ->
                            NoScroll

                        _ ->
                            ScrollWindow

                else
                    NoScroll

            else
                NoScroll


updateOutput :
    (OutputModel -> ( OutputModel, List Effect ))
    -> ET Model
updateOutput updater ( model, effects ) =
    case model.output of
        Output output ->
            let
                ( newOutput, outputEffects ) =
                    updater output

                newModel =
                    { model
                        | output =
                            -- model.output must be equal-by-reference
                            -- to its previous value when passed
                            -- into `Html.Lazy.lazy3` below.
                            if newOutput /= output then
                                Output newOutput

                            else
                                model.output
                    }
            in
            ( newModel, effects ++ outputEffects )

        _ ->
            ( model, effects )


handleBuildFetched : Concourse.Build -> ET Model
handleBuildFetched build ( model, effects ) =
    let
        withBuild =
            { model
                | reapTime = build.reapTime
                , output =
                    if model.hasLoadedYet then
                        model.output

                    else
                        Empty
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
            if build.status == BuildStatusPending then
                ( withBuild, effects ++ pollUntilStarted build.id )

            else if build.reapTime == Nothing then
                case model.prep of
                    Nothing ->
                        initBuildOutput build ( withBuild, effects )

                    Just _ ->
                        let
                            ( newNewModel, newEffects ) =
                                initBuildOutput build ( withBuild, effects )
                        in
                        ( newNewModel
                        , newEffects
                            ++ [ FetchBuildPrep 1000 build.id ]
                        )

            else
                ( withBuild, effects )
    in
    if not model.hasLoadedYet || build.id == model.id then
        ( newModel
        , cmd
            ++ fetchJobAndHistory
            ++ [ SetFavIcon (Just build.status), Focus bodyId ]
        )

    else
        ( model, effects )


pollUntilStarted : Int -> List Effect
pollUntilStarted buildId =
    [ FetchBuild 1000 buildId
    , FetchBuildPrep 1000 buildId
    ]


initBuildOutput : Concourse.Build -> ET Model
initBuildOutput build ( model, effects ) =
    let
        ( output, outputCmd ) =
            Build.Output.Output.init model.highlight build
    in
    ( { model | output = Output output }
    , effects ++ outputCmd
    )


handleBuildPrepFetched : Concourse.BuildPrep -> ET Model
handleBuildPrepFetched buildPrep ( model, effects ) =
    ( { model | prep = Just buildPrep }
    , effects
    )


documentTitle : Model -> String
documentTitle =
    extractTitle


view : Session -> Model -> Html Message
view session model =
    let
        route =
            case model.page of
                OneOffBuildPage buildId ->
                    Routes.OneOffBuild
                        { id = buildId
                        , highlight = model.highlight
                        }

                JobBuildPage buildId ->
                    Routes.Build
                        { id = buildId
                        , highlight = model.highlight
                        }
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            [ SideBar.hamburgerMenu session
            , TopBar.concourseLogo
            , breadcrumbs session model
            , Login.view session.userState model
            ]
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
            [ SideBar.view session
                (model.job
                    |> Maybe.andThen (\j -> SideBar.lookupPipeline j.pipelineId session)
                )
            , viewBuildPage session model
            ]
        ]


tooltip : Model -> { a | hovered : HoverState.HoverState } -> Maybe Tooltip.Tooltip
tooltip _ { hovered } =
    case hovered of
        HoverState.Tooltip (ChangedStepLabel _ text) _ ->
            Just
                { body =
                    Html.div
                        Styles.changedStepTooltip
                        [ Html.text text ]
                , attachPosition =
                    { direction = Tooltip.Top
                    , alignment = Tooltip.Start
                    }
                , arrow = Just { size = 5, color = Colors.tooltipBackground }
                }

        _ ->
            Nothing


breadcrumbs : Session -> Model -> Html Message
breadcrumbs session model =
    case ( model.job, model.page ) of
        ( Just jobId, _ ) ->
            TopBar.breadcrumbs session <|
                Routes.Job
                    { id = jobId
                    , page = Nothing
                    }

        ( _, JobBuildPage buildId ) ->
            TopBar.breadcrumbs session <|
                Routes.Build
                    { id = buildId
                    , highlight = model.highlight
                    }

        _ ->
            Html.text ""


viewBuildPage : Session -> Model -> Html Message
viewBuildPage session model =
    if model.hasLoadedYet then
        Html.div
            [ class "with-fixed-header"
            , attribute "data-build-name" model.name
            , style "flex-grow" "1"
            , style "display" "flex"
            , style "flex-direction" "column"
            , style "overflow" "hidden"
            ]
            [ Header.view session model
            , body session model
            ]

    else
        LoadingIndicator.view


body :
    Session
    ->
        { a
            | prep : Maybe Concourse.BuildPrep
            , job : Maybe Concourse.JobIdentifier
            , status : BuildStatus
            , duration : Concourse.BuildDuration
            , reapTime : Maybe Time.Posix
            , id : Int
            , name : String
            , output : CurrentOutput
            , authorized : Bool
            , showHelp : Bool
        }
    -> Html Message
body session ({ prep, output, authorized, showHelp } as params) =
    Html.div
        ([ class "scrollable-body build-body"
         , id bodyId
         , tabindex 0
         , onScroll Scrolled
         ]
            ++ Styles.body
        )
    <|
        if authorized then
            [ viewBuildPrep prep
            , Html.Lazy.lazy3
                viewBuildOutput
                session.timeZone
                (projectOntoBuildPage session.hovered)
                output
            , Shortcuts.keyboardHelp showHelp
            ]
                ++ tombstone session.timeZone params

        else
            [ NotAuthorized.view ]


projectOntoBuildPage : HoverState.HoverState -> HoverState.HoverState
projectOntoBuildPage hovered =
    case hovered of
        HoverState.Hovered (ChangedStepLabel _ _) ->
            hovered

        HoverState.TooltipPending (ChangedStepLabel _ _) ->
            hovered

        HoverState.Tooltip (ChangedStepLabel _ _) _ ->
            hovered

        HoverState.Hovered (StepState _) ->
            hovered

        HoverState.TooltipPending (StepState _) ->
            hovered

        HoverState.Tooltip (StepState _) _ ->
            hovered

        HoverState.Hovered (StepTab _ _) ->
            hovered

        HoverState.TooltipPending (StepTab _ _) ->
            hovered

        HoverState.Tooltip (StepTab _ _) _ ->
            hovered

        _ ->
            HoverState.NoHover


tombstone :
    Time.Zone
    ->
        { a
            | job : Maybe Concourse.JobIdentifier
            , status : BuildStatus
            , duration : Concourse.BuildDuration
            , reapTime : Maybe Time.Posix
            , id : Int
            , name : String
        }
    -> List (Html Message)
tombstone timeZone model =
    let
        maybeBirthDate =
            Maybe.Extra.or model.duration.startedAt model.duration.finishedAt
    in
    case ( maybeBirthDate, model.reapTime ) of
        ( Just birthDate, Just reapTime ) ->
            [ Html.div
                [ class "tombstone" ]
                [ Html.div [ class "heading" ] [ Html.text "RIP" ]
                , Html.div
                    [ class "job-name" ]
                    [ model.job
                        |> Maybe.map .jobName
                        |> Maybe.withDefault "one-off build"
                        |> Html.text
                    ]
                , Html.div
                    [ class "build-name" ]
                    [ Html.text <| "build #" ++ model.name ]
                , Html.div
                    [ class "date" ]
                    [ Html.text <|
                        mmDDYY timeZone birthDate
                            ++ "-"
                            ++ mmDDYY timeZone reapTime
                    ]
                , Html.div
                    [ class "epitaph" ]
                    [ Html.text <|
                        case model.status of
                            BuildStatusSucceeded ->
                                "It passed, and now it has passed on."

                            BuildStatusFailed ->
                                "It failed, and now has been forgotten."

                            BuildStatusErrored ->
                                "It errored, but has found forgiveness."

                            BuildStatusAborted ->
                                "It was never given a chance."

                            _ ->
                                "I'm not dead yet."
                    ]
                ]
            , Html.div
                [ class "explanation" ]
                [ Html.text "This log has been "
                , Html.a
                    [ Html.Attributes.href "https://concourse-ci.org/jobs.html#job-build-log-retention" ]
                    [ Html.text "reaped." ]
                ]
            ]

        _ ->
            []


mmDDYY : Time.Zone -> Time.Posix -> String
mmDDYY =
    DateFormat.format
        [ DateFormat.monthFixed
        , DateFormat.text "/"
        , DateFormat.dayOfMonthFixed
        , DateFormat.text "/"
        , DateFormat.yearNumberLastTwo
        ]


viewBuildOutput : Time.Zone -> HoverState.HoverState -> CurrentOutput -> Html Message
viewBuildOutput timeZone hovered output =
    case output of
        Output o ->
            Build.Output.Output.view
                { timeZone = timeZone, hovered = hovered }
                o

        Cancelled ->
            Html.div
                Styles.errorLog
                [ Html.text "build cancelled" ]

        Empty ->
            Html.div [] []


viewBuildPrep : Maybe Concourse.BuildPrep -> Html Message
viewBuildPrep buildPrep =
    case buildPrep of
        Just prep ->
            Html.div [ class "build-step" ]
                [ Html.div
                    [ class "header"
                    , style "display" "flex"
                    , style "align-items" "center"
                    ]
                    [ Icon.icon
                        { sizePx = 15, image = Assets.CogsIcon }
                        [ style "margin" "6.5px"
                        , style "margin-right" "0.5px"
                        , style "background-size" "contain"
                        ]
                    , Html.h3 [] [ Html.text "preparing build" ]
                    ]
                , Html.div []
                    [ Html.ul
                        [ class "prep-status-list"
                        , style "font-size" "14px"
                        ]
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


viewBuildPrepInputs : Dict String Concourse.BuildPrepStatus -> List (Html Message)
viewBuildPrepInputs inputs =
    List.map viewBuildPrepInput (Dict.toList inputs)


viewBuildPrepInput : ( String, Concourse.BuildPrepStatus ) -> Html Message
viewBuildPrepInput ( name, status ) =
    viewBuildPrepLi ("discovering any new versions of " ++ name) status Dict.empty


viewBuildPrepDetails : Dict String String -> Html Message
viewBuildPrepDetails details =
    Html.ul [ class "details" ]
        (List.map viewDetailItem (Dict.toList details))


viewDetailItem : ( String, String ) -> Html Message
viewDetailItem ( name, status ) =
    Html.li []
        [ Html.text (name ++ " - " ++ status) ]


viewBuildPrepLi :
    String
    -> Concourse.BuildPrepStatus
    -> Dict String String
    -> Html Message
viewBuildPrepLi text status details =
    Html.li
        [ classList
            [ ( "prep-status", True )
            , ( "inactive", status == Concourse.BuildPrepStatusUnknown )
            ]
        ]
        [ Html.div
            [ style "align-items" "center"
            , style "display" "flex"
            ]
            [ viewBuildPrepStatus status
            , Html.span []
                [ Html.text text ]
            ]
        , viewBuildPrepDetails details
        ]


viewBuildPrepStatus : Concourse.BuildPrepStatus -> Html Message
viewBuildPrepStatus status =
    case status of
        Concourse.BuildPrepStatusUnknown ->
            Html.div
                [ title "thinking..." ]
                [ Spinner.spinner
                    { sizePx = 12
                    , margin = "0 8px 0 0"
                    }
                ]

        Concourse.BuildPrepStatusBlocking ->
            Html.div
                [ title "blocking" ]
                [ Spinner.spinner
                    { sizePx = 12
                    , margin = "0 8px 0 0"
                    }
                ]

        Concourse.BuildPrepStatusNotBlocking ->
            Icon.icon
                { sizePx = 12
                , image = Assets.NotBlockingCheckIcon
                }
                [ style "margin-right" "8px"
                , style "background-size" "contain"
                , title "not blocking"
                ]
