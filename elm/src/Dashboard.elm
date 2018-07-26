port module Dashboard exposing (Model, Msg, init, update, subscriptions, view)

import BuildDuration
import Char
import Concourse
import Concourse.Cli
import Concourse.Info
import Concourse.Job
import Concourse.Pipeline
import Concourse.PipelineStatus
import Concourse.Resource
import DashboardHelpers exposing (..)
import DashboardPreview
import Date exposing (Date)
import Dict exposing (Dict)
import Dom
import Html exposing (Html)
import Html.Attributes exposing (class, classList, id, href, src, attribute, draggable)
import Html.Attributes.Aria exposing (ariaLabel)
import Html.Events exposing (on)
import Http
import Json.Decode
import Keyboard
import List.Extra
import Mouse
import NewTopBar
import NoPipeline exposing (view, Msg)
import Regex exposing (replace, regex, HowMany(AtMost))
import RemoteData
import Routes
import Simple.Fuzzy exposing (match, root, filter)
import StrictEvents exposing (onLeftClick)
import Task exposing (Task)
import Time exposing (Time)


type alias Ports =
    { title : String -> Cmd Msg
    }


port pinTeamNames : () -> Cmd msg


type alias PipelineIndex =
    Int


type DragState
    = NotDragging
    | Dragging Concourse.TeamName PipelineIndex


type DropState
    = NotDropping
    | Dropping PipelineIndex


type alias Flags =
    { csrfToken : String
    , turbulencePath : String
    , search : String
    }


type alias Model =
    { topBar : NewTopBar.Model
    , mPipelines : RemoteData.WebData (List Concourse.Pipeline)
    , pipelines : List Concourse.Pipeline
    , filteredPipelines : List Concourse.Pipeline
    , mJobs : RemoteData.WebData (List Concourse.Job)
    , pipelineJobs : Dict Int (List Concourse.Job)
    , pipelineResourceErrors : Dict ( String, String ) Bool
    , concourseVersion : String
    , csrfToken : String
    , turbulenceImgSrc : String
    , now : Maybe Time
    , showHelp : Bool
    , hideFooter : Bool
    , hideFooterCounter : Time
    , dragState : DragState
    , dropState : DropState
    }


type Msg
    = Noop
    | PipelinesResponse (RemoteData.WebData (List Concourse.Pipeline))
    | JobsResponse (RemoteData.WebData (List Concourse.Job))
    | ResourcesResponse (RemoteData.WebData (List Concourse.Resource))
    | VersionFetched (Result Http.Error String)
    | ClockTick Time.Time
    | AutoRefresh Time
    | ShowFooter
    | KeyPressed Keyboard.KeyCode
    | KeyDowns Keyboard.KeyCode
    | TopBarMsg NewTopBar.Msg
    | TogglePipelinePaused Concourse.Pipeline
    | PipelinePauseToggled Concourse.Pipeline (Result Http.Error ())
    | DragStart String Int
    | DragOver String Int
    | DragEnd


init : Ports -> Flags -> ( Model, Cmd Msg )
init ports flags =
    let
        ( topBar, topBarMsg ) =
            NewTopBar.init True flags.search
    in
        ( { topBar = topBar
          , mPipelines = RemoteData.NotAsked
          , pipelines = []
          , filteredPipelines = []
          , mJobs = RemoteData.NotAsked
          , pipelineJobs = Dict.empty
          , pipelineResourceErrors = Dict.empty
          , now = Nothing
          , csrfToken = flags.csrfToken
          , turbulenceImgSrc = flags.turbulencePath
          , concourseVersion = ""
          , showHelp = False
          , hideFooter = False
          , hideFooterCounter = 0
          , dragState = NotDragging
          , dropState = NotDropping
          }
        , Cmd.batch
            [ fetchPipelines
            , fetchVersion
            , getCurrentTime
            , Cmd.map TopBarMsg topBarMsg
            , pinTeamNames ()
            , ports.title <| "Dashboard" ++ " - "
            ]
        )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    let
        reload =
            Cmd.batch <|
                (if model.mPipelines == RemoteData.Loading then
                    []
                 else
                    [ fetchPipelines ]
                )
                    ++ [ fetchVersion, Cmd.map TopBarMsg NewTopBar.fetchUser ]
    in
        case msg of
            Noop ->
                ( model, Cmd.none )

            PipelinesResponse response ->
                case response of
                    RemoteData.Success pipelines ->
                        let
                            newModel =
                                { model | pipelines = pipelines }
                        in
                            ( { newModel | mPipelines = response, filteredPipelines = filter model.topBar.query newModel }, Cmd.batch [ fetchAllJobs, fetchAllResources ] )

                    _ ->
                        ( model, Cmd.none )

            JobsResponse response ->
                case ( response, model.mPipelines ) of
                    ( RemoteData.Success jobs, RemoteData.Success pipelines ) ->
                        ( { model | mJobs = response, pipelineJobs = jobsByPipelineId pipelines jobs }, Cmd.none )

                    _ ->
                        ( model, Cmd.none )

            ResourcesResponse response ->
                case ( response, model.mPipelines ) of
                    ( RemoteData.Success resources, RemoteData.Success pipelines ) ->
                        ( { model | pipelineResourceErrors = resourceErrorsByPipelineIdentifier resources }, Cmd.none )

                    _ ->
                        ( model, Cmd.none )

            VersionFetched (Ok version) ->
                ( { model | concourseVersion = version }, Cmd.none )

            VersionFetched (Err err) ->
                ( { model | concourseVersion = "" }, Cmd.none )

            ClockTick now ->
                if model.hideFooterCounter + Time.second > 5 * Time.second then
                    ( { model | now = Just now, hideFooter = True }, Cmd.none )
                else
                    ( { model | now = Just now, hideFooterCounter = model.hideFooterCounter + Time.second }, Cmd.none )

            AutoRefresh _ ->
                ( model
                , reload
                )

            KeyPressed keycode ->
                handleKeyPressed (Char.fromCode keycode) model

            KeyDowns keycode ->
                update (TopBarMsg (NewTopBar.KeyDown keycode)) model

            ShowFooter ->
                ( { model | hideFooter = False, hideFooterCounter = 0 }, Cmd.none )

            TopBarMsg msg ->
                let
                    ( newTopBar, newTopBarMsg ) =
                        NewTopBar.update msg model.topBar

                    newModel =
                        case msg of
                            NewTopBar.FilterMsg query ->
                                { model
                                    | topBar = newTopBar
                                    , filteredPipelines = filter query model
                                }

                            NewTopBar.KeyDown keycode ->
                                if keycode == 13 then
                                    { model
                                        | topBar = newTopBar
                                        , filteredPipelines = filter newTopBar.query model
                                    }
                                else
                                    { model | topBar = newTopBar }

                            _ ->
                                { model | topBar = newTopBar }

                    newMsg =
                        case msg of
                            NewTopBar.LoggedOut (Ok _) ->
                                reload

                            _ ->
                                Cmd.map TopBarMsg newTopBarMsg
                in
                    ( newModel, newMsg )

            TogglePipelinePaused pipeline ->
                ( model, togglePipelinePaused pipeline model.csrfToken )

            PipelinePauseToggled pipeline (Ok ()) ->
                let
                    togglePipelinePause : List Concourse.Pipeline -> List Concourse.Pipeline
                    togglePipelinePause pipelines =
                        List.Extra.updateIf
                            ((==) pipeline)
                            (\pipeline -> { pipeline | paused = not pipeline.paused })
                            pipelines
                in
                    ( { model
                        | pipelines = togglePipelinePause model.pipelines
                        , filteredPipelines = togglePipelinePause model.filteredPipelines
                      }
                    , Cmd.none
                    )

            PipelinePauseToggled _ (Err _) ->
                ( model, Cmd.none )

            DragStart teamName index ->
                ( { model | dragState = Dragging teamName index }, Cmd.none )

            DragOver teamName index ->
                ( { model | dropState = Dropping index }, Cmd.none )

            DragEnd ->
                case ( model.dragState, model.dropState ) of
                    ( Dragging teamName dragIndex, Dropping dropIndex ) ->
                        let
                            shiftPipelines : List Concourse.Pipeline -> List Concourse.Pipeline
                            shiftPipelines pipelines =
                                if dragIndex == dropIndex then
                                    pipelines
                                else
                                    case
                                        List.head <|
                                            List.drop dragIndex <|
                                                (List.filter ((==) teamName << .teamName) pipelines)
                                    of
                                        Nothing ->
                                            pipelines

                                        Just pipeline ->
                                            shiftPipelineTo pipeline dropIndex pipelines

                            pipelines =
                                if String.isEmpty model.topBar.query then
                                    shiftPipelines model.pipelines
                                else
                                    shiftPipelines model.filteredPipelines

                            newModel =
                                if String.isEmpty model.topBar.query then
                                    { model | pipelines = pipelines }
                                else
                                    { model | filteredPipelines = pipelines }
                        in
                            ( { newModel
                                | dragState = NotDragging
                                , dropState = NotDropping
                              }
                            , orderPipelines teamName pipelines model.csrfToken
                            )

                    _ ->
                        ( { model | dragState = NotDragging, dropState = NotDropping }, Cmd.none )


shiftPipelineTo : Concourse.Pipeline -> Int -> List Concourse.Pipeline -> List Concourse.Pipeline
shiftPipelineTo pipeline position pipelines =
    case pipelines of
        [] ->
            if position < 0 then
                []
            else
                [ pipeline ]

        p :: ps ->
            if p.teamName /= pipeline.teamName then
                p :: (shiftPipelineTo pipeline position ps)
            else if p == pipeline then
                shiftPipelineTo pipeline (position - 1) ps
            else if position == 0 then
                pipeline :: p :: (shiftPipelineTo pipeline (position - 1) ps)
            else
                p :: (shiftPipelineTo pipeline (position - 1) ps)


orderPipelines : String -> List Concourse.Pipeline -> Concourse.CSRFToken -> Cmd Msg
orderPipelines teamName pipelines csrfToken =
    Task.attempt (always Noop) <|
        Concourse.Pipeline.order
            teamName
            (List.map (.name) <| List.filter ((==) teamName << .teamName) pipelines)
            csrfToken


togglePipelinePaused : Concourse.Pipeline -> Concourse.CSRFToken -> Cmd Msg
togglePipelinePaused pipeline csrfToken =
    Task.attempt (PipelinePauseToggled pipeline) <|
        if pipeline.paused then
            Concourse.Pipeline.unpause pipeline.teamName pipeline.name csrfToken
        else
            Concourse.Pipeline.pause pipeline.teamName pipeline.name csrfToken


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ Time.every Time.second ClockTick
        , Time.every (5 * Time.second) AutoRefresh
        , Mouse.moves (\_ -> ShowFooter)
        , Mouse.clicks (\_ -> ShowFooter)
        , Keyboard.presses KeyPressed
        , Keyboard.downs KeyDowns
        ]


view : Model -> Html Msg
view model =
    Html.div [ class "page" ]
        [ Html.map TopBarMsg (NewTopBar.view model.topBar)
        , dashboardView model
        ]


dashboardView : Model -> Html Msg
dashboardView model =
    case ( model.mPipelines, model.mJobs ) of
        ( RemoteData.Success [], _ ) ->
            Html.map (\_ -> Noop) NoPipeline.view

        ( RemoteData.Success _, RemoteData.Success _ ) ->
            if List.length model.filteredPipelines > 0 then
                pipelinesView model model.filteredPipelines
            else if not (String.isEmpty model.topBar.query) then
                noResultsView (toString model.topBar.query)
            else
                pipelinesView model model.pipelines

        ( RemoteData.Failure _, _ ) ->
            turbulenceView model

        ( _, RemoteData.Failure _ ) ->
            turbulenceView model

        _ ->
            Html.text ""


noResultsView : String -> Html Msg
noResultsView query =
    let
        boldedQuery =
            Html.span [ class "monospace-bold" ] [ Html.text query ]
    in
        Html.div
            [ class "dashboard" ]
            [ Html.div [ class "dashboard-content " ]
                [ Html.div
                    [ class "dashboard-team-group" ]
                    [ Html.div [ class "pin-wrapper" ]
                        [ Html.div [ class "dashboard-team-name no-results" ]
                            [ Html.text "No results for "
                            , boldedQuery
                            , Html.text " matched your search."
                            ]
                        ]
                    ]
                ]
            ]


helpView : Model -> Html Msg
helpView model =
    Html.div
        [ classList
            [ ( "keyboard-help", True )
            , ( "hidden", not model.showHelp )
            ]
        ]
        [ Html.div [ class "help-title" ] [ Html.text "keyboard shortcuts" ]
        , Html.div [ class "help-line" ] [ Html.div [ class "keys" ] [ Html.span [ class "key" ] [ Html.text "/" ] ], Html.text "search" ]
        , Html.div [ class "help-line" ] [ Html.div [ class "keys" ] [ Html.span [ class "key" ] [ Html.text "?" ] ], Html.text "hide/show help" ]
        ]


footerView : Model -> Html Msg
footerView model =
    Html.div
        [ if model.hideFooter || model.showHelp then
            class "dashboard-footer hidden"
          else
            class "dashboard-footer"
        ]
        [ Html.div [ class "dashboard-legend" ]
            [ Html.div [ class "dashboard-status-pending" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "pending" ]
            , Html.div [ class "dashboard-paused" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "paused" ]
            , Html.div [ class "dashboard-running" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "running" ]
            , Html.div [ class "dashboard-status-failed" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "failing" ]
            , Html.div [ class "dashboard-status-errored" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "errored" ]
            , Html.div [ class "dashboard-status-aborted" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "aborted" ]
            , Html.div [ class "dashboard-status-succeeded" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "succeeded" ]
            , Html.div [ class "dashboard-status-separator" ] [ Html.text "|" ]
            , Html.div [ class "dashboard-high-density" ]
                [ Html.a [ class "toggle-high-density", href Routes.dashboardHdRoute, ariaLabel "Toggle high-density view" ]
                    [ Html.div [ class "dashboard-pipeline-icon hd-off" ] [], Html.text "high-density" ]
                ]
            ]
        , Html.div [ class "concourse-info" ]
            [ Html.div [ class "concourse-version" ]
                [ Html.text "version: v", Html.text model.concourseVersion ]
            , Html.div [ class "concourse-cli" ]
                [ Html.text "cli: "
                , Html.a [ href (Concourse.Cli.downloadUrl "amd64" "darwin"), ariaLabel "Download OS X CLI" ]
                    [ Html.i [ class "fa fa-apple" ] [] ]
                , Html.a [ href (Concourse.Cli.downloadUrl "amd64" "windows"), ariaLabel "Download Windows CLI" ]
                    [ Html.i [ class "fa fa-windows" ] [] ]
                , Html.a [ href (Concourse.Cli.downloadUrl "amd64" "linux"), ariaLabel "Download Linux CLI" ]
                    [ Html.i [ class "fa fa-linux" ] [] ]
                ]
            ]
        ]


turbulenceView : Model -> Html Msg
turbulenceView model =
    Html.div
        [ class "error-message" ]
        [ Html.div [ class "message" ]
            [ Html.img [ src model.turbulenceImgSrc, class "seatbelt" ] []
            , Html.p [] [ Html.text "experiencing turbulence" ]
            , Html.p [ class "explanation" ] []
            ]
        ]


pipelinesView : Model -> List Concourse.Pipeline -> Html Msg
pipelinesView model pipelines =
    let
        pipelinesByTeam =
            List.foldl
                (\pipelineWithJobs byTeam ->
                    groupPipelines byTeam ( pipelineWithJobs.pipeline.teamName, pipelineWithJobs )
                )
                []
                (pipelinesWithJobs model.pipelineJobs model.pipelineResourceErrors pipelines)

        emptyTeams =
            teamsWithoutPipelines model.topBar.teams <| Dict.fromList pipelinesByTeam

        pipelinesByTeamView =
            List.append
                (List.map (\( teamName, pipelines ) -> groupView model teamName (List.reverse pipelines))
                    pipelinesByTeam
                )
                (List.map (\team -> groupView model team.name [])
                    emptyTeams
                )
    in
        Html.div
            [ class "dashboard" ]
        <|
            [ Html.div [ class "dashboard-content" ] <| pipelinesByTeamView
            , footerView model
            , helpView model
            ]


handleKeyPressed : Char -> Model -> ( Model, Cmd Msg )
handleKeyPressed key model =
    case key of
        '/' ->
            ( model, Task.attempt (always Noop) (Dom.focus "search-input-field") )

        '?' ->
            ( { model | showHelp = not model.showHelp }, Cmd.none )

        _ ->
            update ShowFooter model


groupView : Model -> String -> List PipelineWithJobs -> Html Msg
groupView model teamName pipelines =
    let
        teamPipelines =
            if List.isEmpty pipelines then
                [ pipelineNotSetView ]
            else
                List.append
                    (List.indexedMap
                        (\i pipeline ->
                            Html.div [ class "pipeline-wrapper" ] [ pipelineDropAreaView model teamName i, pipelineView model pipeline i ]
                        )
                        pipelines
                    )
                    [ (pipelineDropAreaView model teamName (List.length pipelines)) ]
    in
        Html.div [ id teamName, class "dashboard-team-group", attribute "data-team-name" teamName ]
            [ Html.div [ class "pin-wrapper" ]
                [ Html.div [ class "dashboard-team-name" ] [ Html.text teamName ] ]
            , Html.div [ class "dashboard-team-pipelines" ] teamPipelines
            ]


pipelineView : Model -> PipelineWithJobs -> Int -> Html Msg
pipelineView model ({ pipeline, jobs, resourceError } as pipelineWithJobs) index =
    Html.div
        [ classList
            [ ( "dashboard-pipeline", True )
            , ( "dashboard-paused", pipeline.paused )
            , ( "dashboard-running", List.any (\job -> job.nextBuild /= Nothing) jobs )
            , ( "dashboard-status-" ++ Concourse.PipelineStatus.show (pipelineStatusFromJobs jobs False), not pipeline.paused )
            , ( "dragging", model.dragState == Dragging pipeline.teamName index )
            ]
        , attribute "data-pipeline-name" pipeline.name
        , attribute "ondragstart" "event.dataTransfer.setData('text/plain', '');"
        , draggable "true"
        , on "dragstart" (Json.Decode.succeed (DragStart pipeline.teamName index))
        , on "dragend" (Json.Decode.succeed DragEnd)
        ]
        [ Html.div [ class "dashboard-pipeline-banner" ] []
        , Html.div
            [ class "dashboard-pipeline-content" ]
            [ Html.a [ href <| Routes.pipelineRoute pipeline, draggable "false" ]
                [ Html.div
                    [ class "dashboard-pipeline-header" ]
                    [ Html.div [ class "dashboard-pipeline-name" ]
                        [ Html.text pipeline.name ]
                    , Html.div [ classList [ ( "dashboard-resource-error", resourceError ) ] ] []
                    ]
                ]
            , DashboardPreview.view jobs
            , Html.div [ class "dashboard-pipeline-footer" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] []
                , timeSincePipelineTransitioned model.now pipelineWithJobs
                , pauseToggleView pipeline
                ]
            ]
        ]


pipelineNotSetView : Html msg
pipelineNotSetView =
    Html.div [ class "pipeline-wrapper" ]
        [ Html.div
            [ class "dashboard-pipeline no-set"
            ]
            [ Html.div
                [ class "dashboard-pipeline-content" ]
                [ Html.div [ class "no-set-wrapper" ]
                    [ Html.text "no pipelines set" ]
                ]
            ]
        ]


pipelineDropAreaView : Model -> String -> Int -> Html Msg
pipelineDropAreaView model teamName index =
    let
        ( active, over ) =
            case ( model.dragState, model.dropState ) of
                ( Dragging team dragIndex, NotDropping ) ->
                    ( team == teamName, index == dragIndex )

                ( Dragging team dragIndex, Dropping dropIndex ) ->
                    ( team == teamName, index == dropIndex )

                _ ->
                    ( False, False )
    in
        Html.div
            [ classList [ ( "drop-area", True ), ( "active", active ), ( "over", over ), ( "animation", model.dropState /= NotDropping ) ]
            , on "dragenter" (Json.Decode.succeed (DragOver teamName index))
            ]
            [ Html.text "" ]


pauseToggleView : Concourse.Pipeline -> Html Msg
pauseToggleView pipeline =
    Html.a
        [ classList
            [ ( "pause-toggle", True )
            , ( "icon-play", pipeline.paused )
            , ( "icon-pause", not pipeline.paused )
            ]
        , onLeftClick <| TogglePipelinePaused pipeline
        ]
        []


timeSincePipelineTransitioned : Maybe Time -> PipelineWithJobs -> Html a
timeSincePipelineTransitioned time ({ jobs } as pipelineWithJobs) =
    let
        status =
            pipelineStatus pipelineWithJobs

        transitionedJobs =
            List.filter
                (\job ->
                    not <| xor (status == Concourse.PipelineStatusSucceeded) (Just Concourse.BuildStatusSucceeded == (Maybe.map .status job.finishedBuild))
                )
                jobs

        transitionedDurations =
            List.filterMap
                (\job ->
                    Maybe.map .duration job.transitionBuild
                )
                transitionedJobs

        sortedTransitionedDurations =
            List.sortBy
                (\duration ->
                    case duration.startedAt of
                        Just date ->
                            Time.inSeconds <| Date.toTime date

                        Nothing ->
                            0
                )
                transitionedDurations

        transitionedDuration =
            if status == Concourse.PipelineStatusSucceeded then
                List.head << List.reverse <| sortedTransitionedDurations
            else
                List.head <| sortedTransitionedDurations
    in
        case status of
            Concourse.PipelineStatusPaused ->
                Html.div [ class "build-duration" ] [ Html.text "paused" ]

            Concourse.PipelineStatusPending ->
                Html.div [ class "build-duration" ] [ Html.text "pending" ]

            Concourse.PipelineStatusRunning ->
                Html.div [ class "build-duration" ] [ Html.text "running" ]

            _ ->
                case ( time, transitionedDuration ) of
                    ( Just now, Just duration ) ->
                        BuildDuration.show duration now

                    _ ->
                        Html.text ""


pipelineStatus : PipelineWithJobs -> Concourse.PipelineStatus
pipelineStatus { pipeline, jobs } =
    if pipeline.paused then
        Concourse.PipelineStatusPaused
    else
        pipelineStatusFromJobs jobs True


fetchPipelines : Cmd Msg
fetchPipelines =
    Cmd.map PipelinesResponse <|
        RemoteData.asCmd Concourse.Pipeline.fetchPipelines


fetchAllJobs : Cmd Msg
fetchAllJobs =
    Cmd.map JobsResponse <|
        RemoteData.asCmd Concourse.Job.fetchAllJobs


fetchAllResources : Cmd Msg
fetchAllResources =
    Cmd.map ResourcesResponse <|
        RemoteData.asCmd Concourse.Resource.fetchAllResources


fetchVersion : Cmd Msg
fetchVersion =
    Concourse.Info.fetch
        |> Task.map (.version)
        |> Task.attempt VersionFetched


getCurrentTime : Cmd Msg
getCurrentTime =
    Task.perform ClockTick Time.now


filter : String -> Model -> List Concourse.Pipeline
filter queryStr model =
    let
        queries =
            queryStr
                |> replace (AtMost 1) (regex "team:\\s*") (\_ -> "team:")
                |> replace (AtMost 1) (regex "status:\\s*") (\_ -> "status:")
                |> String.words
    in
        filterByTerms model queries model.pipelines


filterByTerms : Model -> List String -> List Concourse.Pipeline -> List Concourse.Pipeline
filterByTerms model terms pipelines =
    case terms of
        [] ->
            pipelines

        x :: xs ->
            filterByTerms model xs (filterByTerm x (pipelinesWithJobs model.pipelineJobs model.pipelineResourceErrors pipelines))


filterByTerm : String -> List PipelineWithJobs -> List Concourse.Pipeline
filterByTerm term pipelines =
    let
        searchTeams =
            String.startsWith "team:" term

        searchStatus =
            String.startsWith "status:" term

        teamSearchTerm =
            if searchTeams then
                String.dropLeft 5 term
            else
                term

        statusSearchTerm =
            if searchStatus then
                String.dropLeft 7 term
            else
                term

        plist =
            List.map (\p -> p.pipeline) pipelines

        filterByStatus =
            fuzzySearch (\p -> pipelineStatus p |> Concourse.PipelineStatus.show) statusSearchTerm pipelines
    in
        if searchTeams then
            fuzzySearch .teamName teamSearchTerm plist
        else if searchStatus then
            List.map (\p -> p.pipeline) filterByStatus
        else
            fuzzySearch .name term plist


fuzzySearch : (a -> String) -> String -> List a -> List a
fuzzySearch map needle records =
    let
        negateSearch =
            String.startsWith "-" needle
    in
        if negateSearch then
            List.filter (not << (Simple.Fuzzy.match needle) << map) records
        else
            List.filter ((Simple.Fuzzy.match needle) << map) records
