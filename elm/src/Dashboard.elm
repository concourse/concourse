port module Dashboard exposing (Model, Msg, init, subscriptions, update, view)

import Char
import Concourse
import Concourse.Cli
import Concourse.Pipeline
import Concourse.PipelineStatus
import Concourse.User
import DashboardHelpers exposing (..)
import Dashboard.Group as Group
import Dashboard.GroupWithTag as GroupWithTag
import Dashboard.Pipeline as Pipeline
import Dom
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, draggable, href, id, src)
import Html.Attributes.Aria exposing (ariaLabel)
import Http
import Keyboard
import List.Extra
import Mouse
import NewTopBar
import NoPipeline exposing (Msg, view)
import Regex exposing (HowMany(All), regex, replace)
import RemoteData
import Routes
import Simple.Fuzzy exposing (filter, match, root)
import Task
import Time exposing (Time)


type alias Ports =
    { title : String -> Cmd Msg
    }


port pinTeamNames : () -> Cmd msg


port tooltip : ( String, String ) -> Cmd msg


type alias Flags =
    { csrfToken : String
    , turbulencePath : String
    , search : String
    }


type DashboardState
    = NotAsked
    | Turbulence String
    | NoPipelines
    | HasData SubState


type alias SubState =
    { csrfToken : String
    , dragState : Pipeline.DragState
    , dropState : Pipeline.DropState
    , hideFooter : Bool
    , hideFooterCounter : Time
    , now : Maybe Time
    , teamData : TeamData
    }


type alias Model =
    { csrfToken : String
    , state : DashboardState
    , topBar : NewTopBar.Model
    , turbulencePath : String
    , showHelp : Bool
    }


type TeamData
    = Unauthenticated
        { apiData : Group.APIData
        }
    | Authenticated
        { apiData : Group.APIData
        , user : Concourse.User
        }


teamApiData : TeamData -> Group.APIData
teamApiData teamData =
    case teamData of
        Unauthenticated { apiData } ->
            apiData

        Authenticated { apiData } ->
            apiData


type Msg
    = Noop
    | APIDataFetched (RemoteData.WebData ( Group.APIData, Maybe Concourse.User ))
    | ClockTick Time.Time
    | AutoRefresh Time
    | ShowFooter
    | KeyPressed Keyboard.KeyCode
    | KeyDowns Keyboard.KeyCode
    | TopBarMsg NewTopBar.Msg
    | PipelinePauseToggled Concourse.Pipeline (Result Http.Error ())
    | PipelineMsg Pipeline.Msg


init : Ports -> Flags -> ( Model, Cmd Msg )
init ports flags =
    let
        ( topBar, topBarMsg ) =
            NewTopBar.init True flags.search
    in
        ( { state = NotAsked
          , topBar = topBar
          , csrfToken = flags.csrfToken
          , turbulencePath = flags.turbulencePath
          , showHelp = False
          }
        , Cmd.batch
            [ fetchData
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
                (case model.state of
                    HasData _ ->
                        [ fetchData ]

                    _ ->
                        []
                )
                    ++ [ Cmd.map TopBarMsg NewTopBar.fetchUser ]
    in
        case msg of
            Noop ->
                ( model, Cmd.none )

            APIDataFetched remoteData ->
                let
                    state =
                        case remoteData of
                            RemoteData.NotAsked ->
                                NotAsked

                            RemoteData.Loading ->
                                NotAsked

                            RemoteData.Failure _ ->
                                Turbulence ""

                            RemoteData.Success ( apiData, user ) ->
                                case apiData.pipelines of
                                    [] ->
                                        NoPipelines

                                    _ ->
                                        let
                                            teamData =
                                                user
                                                    |> Maybe.map (\u -> Authenticated { apiData = apiData, user = u })
                                                    |> Maybe.withDefault (Unauthenticated { apiData = apiData })
                                        in
                                            HasData
                                                { teamData = teamData
                                                , now = Nothing
                                                , hideFooter = False
                                                , hideFooterCounter = 0
                                                , dragState = Pipeline.NotDragging
                                                , dropState = Pipeline.NotDropping
                                                , csrfToken = model.csrfToken
                                                }
                in
                    ( { model | state = state }
                    , Cmd.none
                    )

            ClockTick now ->
                case model.state of
                    HasData substate ->
                        if substate.hideFooterCounter + Time.second > 5 * Time.second then
                            ( { model | state = HasData { substate | now = Just now, hideFooter = True } }, Cmd.none )
                        else
                            ( { model | state = HasData { substate | now = Just now, hideFooterCounter = substate.hideFooterCounter + Time.second } }, Cmd.none )

                    _ ->
                        ( model, Cmd.none )

            AutoRefresh _ ->
                ( model
                , reload
                )

            KeyPressed keycode ->
                handleKeyPressed (Char.fromCode keycode) model

            KeyDowns keycode ->
                update (TopBarMsg (NewTopBar.KeyDown keycode)) model

            ShowFooter ->
                case model.state of
                    HasData substate ->
                        ( { model | state = HasData { substate | hideFooter = False, hideFooterCounter = 0 } }, Cmd.none )

                    _ ->
                        ( model, Cmd.none )

            TopBarMsg msg ->
                let
                    ( newTopBar, newTopBarMsg ) =
                        NewTopBar.update msg model.topBar

                    newMsg =
                        case msg of
                            NewTopBar.LoggedOut (Ok _) ->
                                reload

                            _ ->
                                Cmd.map TopBarMsg newTopBarMsg
                in
                    ( { model | topBar = newTopBar }, newMsg )

            PipelineMsg (Pipeline.TogglePipelinePaused pipeline) ->
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
                    ( model
                      -- | filteredPipelines = togglePipelinePause model.filteredPipelines
                    , Cmd.none
                    )

            PipelinePauseToggled _ (Err _) ->
                ( model, Cmd.none )

            PipelineMsg (Pipeline.DragStart teamName index) ->
                case model.state of
                    HasData substate ->
                        ( { model | state = HasData { substate | dragState = Pipeline.Dragging teamName index } }, Cmd.none )

                    _ ->
                        ( model, Cmd.none )

            PipelineMsg (Pipeline.DragOver teamName index) ->
                case model.state of
                    HasData substate ->
                        ( { model | state = HasData { substate | dropState = Pipeline.Dropping index } }, Cmd.none )

                    _ ->
                        ( model, Cmd.none )

            PipelineMsg (Pipeline.Tooltip pipelineName teamName) ->
                ( model, tooltip ( pipelineName, teamName ) )

            PipelineMsg Pipeline.DragEnd ->
                case model.state of
                    HasData substate ->
                        case ( substate.dragState, substate.dropState ) of
                            ( Pipeline.Dragging teamName dragIndex, Pipeline.Dropping dropIndex ) ->
                                let
                                    shiftPipelines : List PipelineWithJobs -> List PipelineWithJobs
                                    shiftPipelines pipelines =
                                        if dragIndex == dropIndex then
                                            pipelines
                                        else
                                            case
                                                List.head <|
                                                    List.drop dragIndex <|
                                                        pipelines
                                            of
                                                Nothing ->
                                                    pipelines

                                                Just pipeline ->
                                                    shiftPipelineTo pipeline dropIndex pipelines

                                    groups =
                                        substate.teamData |> teamApiData |> Group.groups

                                    filteredPipelines =
                                        groups
                                            |> List.Extra.find (.teamName >> (==) teamName)
                                            |> Maybe.map (.pipelines >> shiftPipelines)

                                    newGroups =
                                        filteredPipelines
                                            |> Maybe.map (\fps -> groups |> List.Extra.updateIf (.teamName >> (==) teamName) (\g -> { g | pipelines = fps }))
                                            |> Maybe.withDefault groups

                                    newSubstate =
                                        { substate
                                            | dragState = Pipeline.NotDragging
                                            , dropState = Pipeline.NotDropping
                                        }

                                    newModel =
                                        case newSubstate.teamData of
                                            Unauthenticated { apiData } ->
                                                HasData { newSubstate | teamData = Unauthenticated { apiData = Group.apiData newGroups } }

                                            Authenticated { apiData, user } ->
                                                HasData { newSubstate | teamData = Authenticated { apiData = Group.apiData newGroups, user = user } }
                                in
                                    ( { model | state = newModel }
                                    , filteredPipelines
                                        |> Maybe.map (\ps -> orderPipelines teamName ps model.csrfToken)
                                        |> Maybe.withDefault reload
                                    )

                            _ ->
                                ( { model | state = HasData { substate | dragState = Pipeline.NotDragging, dropState = Pipeline.NotDropping } }
                                , Cmd.none
                                )

                    _ ->
                        ( model, Cmd.none )


shiftPipelineTo : PipelineWithJobs -> Int -> List PipelineWithJobs -> List PipelineWithJobs
shiftPipelineTo ({ pipeline } as pipelineWithJobs) position pipelines =
    case pipelines of
        [] ->
            if position < 0 then
                []
            else
                [ pipelineWithJobs ]

        p :: ps ->
            if p.pipeline.teamName /= pipeline.teamName then
                p :: shiftPipelineTo pipelineWithJobs position ps
            else if p.pipeline == pipeline then
                shiftPipelineTo pipelineWithJobs (position - 1) ps
            else if position == 0 then
                pipelineWithJobs :: p :: shiftPipelineTo pipelineWithJobs (position - 1) ps
            else
                p :: shiftPipelineTo pipelineWithJobs (position - 1) ps


orderPipelines : String -> List PipelineWithJobs -> Concourse.CSRFToken -> Cmd Msg
orderPipelines teamName pipelines csrfToken =
    Task.attempt (always Noop) <|
        Concourse.Pipeline.order
            teamName
            (List.map (.name << .pipeline) <| pipelines)
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
    let
        mainContent =
            case model.state of
                NotAsked ->
                    Html.text ""

                Turbulence path ->
                    turbulenceView path

                NoPipelines ->
                    Html.map (always Noop) NoPipeline.view

                HasData substate ->
                    pipelinesView substate model.showHelp model.topBar.query
    in
        Html.div
            [ class "dashboard" ]
            [ mainContent
            , helpView model
            ]


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


footerView : SubState -> Bool -> Html Msg
footerView substate showHelp =
    Html.div
        [ if substate.hideFooter || showHelp then
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
                [ Html.text "version: v", substate.teamData |> teamApiData |> .version |> Html.text ]
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


turbulenceView : String -> Html Msg
turbulenceView path =
    Html.div
        [ class "error-message" ]
        [ Html.div [ class "message" ]
            [ Html.img [ src path, class "seatbelt" ] []
            , Html.p [] [ Html.text "experiencing turbulence" ]
            , Html.p [ class "explanation" ] []
            ]
        ]


pipelinesView : SubState -> Bool -> String -> Html Msg
pipelinesView substate showHelp query =
    let
        filteredGroups =
            substate.teamData |> teamApiData |> Group.groups |> filter query

        groupsToDisplay =
            if List.all (String.startsWith "team:") (filterTerms query) then
                filteredGroups
            else
                filteredGroups |> List.filter (.pipelines >> List.isEmpty >> not)

        groupViews =
            case substate.teamData of
                Unauthenticated _ ->
                    List.map (\g -> Group.view (Group.headerView g) substate.dragState substate.dropState substate.now g) groupsToDisplay

                Authenticated { user } ->
                    List.map (\g -> Group.view (GroupWithTag.headerView g) substate.dragState substate.dropState substate.now g.group) (GroupWithTag.addTagsAndSort user groupsToDisplay)
    in
        if List.isEmpty groupViews then
            noResultsView (toString query)
        else
            Html.div [ class "dashboard-content" ] <|
                (List.map (Html.map PipelineMsg) groupViews ++ [ footerView substate showHelp ])


handleKeyPressed : Char -> Model -> ( Model, Cmd Msg )
handleKeyPressed key model =
    case key of
        '/' ->
            ( model, Task.attempt (always Noop) (Dom.focus "search-input-field") )

        '?' ->
            ( { model | showHelp = not model.showHelp }, Cmd.none )

        _ ->
            update ShowFooter model


fetchData : Cmd Msg
fetchData =
    Group.remoteData |> Task.andThen remoteUser |> RemoteData.asCmd |> Cmd.map APIDataFetched


remoteUser : Group.APIData -> Task.Task Http.Error ( Group.APIData, Maybe Concourse.User )
remoteUser d =
    Concourse.User.fetchUser
        |> Task.map ((,) d << Just)
        |> Task.onError (always <| Task.succeed <| ( d, Nothing ))


getCurrentTime : Cmd Msg
getCurrentTime =
    Task.perform ClockTick Time.now


filterTerms : String -> List String
filterTerms =
    replace All (regex "team:\\s*") (\_ -> "team:")
        >> replace All (regex "status:\\s*") (\_ -> "status:")
        >> String.words
        >> List.filter (not << String.isEmpty)


filter : String -> List Group.Group -> List Group.Group
filter =
    filterTerms >> flip (List.foldl filterGroupsByTerm)


filterPipelinesByTerm : String -> Group.Group -> Group.Group
filterPipelinesByTerm term ({ pipelines } as group) =
    let
        searchStatus =
            String.startsWith "status:" term

        statusSearchTerm =
            if searchStatus then
                String.dropLeft 7 term
            else
                term

        filterByStatus =
            fuzzySearch (Pipeline.pipelineStatus >> Concourse.PipelineStatus.show) statusSearchTerm pipelines
    in
        { group
            | pipelines =
                if searchStatus then
                    filterByStatus
                else
                    fuzzySearch (.pipeline >> .name) term pipelines
        }


filterGroupsByTerm : String -> List Group.Group -> List Group.Group
filterGroupsByTerm term groups =
    let
        searchTeams =
            String.startsWith "team:" term

        teamSearchTerm =
            if searchTeams then
                String.dropLeft 5 term
            else
                term
    in
        if searchTeams then
            fuzzySearch .teamName teamSearchTerm groups
        else
            groups |> List.map (filterPipelinesByTerm term)


fuzzySearch : (a -> String) -> String -> List a -> List a
fuzzySearch map needle records =
    let
        negateSearch =
            String.startsWith "-" needle
    in
        if negateSearch then
            List.filter (not << Simple.Fuzzy.match needle << map) records
        else
            List.filter (Simple.Fuzzy.match needle << map) records
