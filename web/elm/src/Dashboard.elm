port module Dashboard exposing (Model, init, subscriptions, update, view)

import Char
import Concourse
import Concourse.Cli
import Concourse.Pipeline
import Concourse.PipelineStatus
import Concourse.User
import Css
import Dashboard.APIData as APIData
import Dashboard.Details as Details
import Dashboard.Group as Group
import Dashboard.GroupWithTag as GroupWithTag
import Dashboard.Msgs exposing (Msg(..))
import Dashboard.Pipeline as Pipeline
import Dashboard.SubState as SubState
import DashboardHd
import Dom
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes exposing (attribute, css, class, classList, draggable, href, id, src)
import Http
import Keyboard
import List.Extra
import Maybe.Extra
import Mouse
import Monocle.Common exposing ((=>), (<|>))
import Monocle.Optional
import Monocle.Lens
import MonocleHelpers exposing (..)
import NewTopBar
import NoPipeline
import Regex exposing (HowMany(All), regex, replace)
import RemoteData
import Routes
import Simple.Fuzzy exposing (filter, match, root)
import Task
import Time exposing (Time)
import Window


type alias Ports =
    { title : String -> Cmd Msg
    }


port tooltip : ( String, String ) -> Cmd msg



-- TODO all the crsfToken stuff in this file only gets actually used for ordering and toggling pipelines.
-- honestly it seems like it could live in a completely different module.


type alias Flags =
    { csrfToken : String
    , turbulencePath : String
    , search : String
    , highDensity : Bool
    , pipelineRunningKeyframes : String
    }


type DashboardError
    = NotAsked
    | Turbulence String
    | NoPipelines


type alias Model =
    { csrfToken : String
    , state : Result DashboardError SubState.SubState
    , topBar : NewTopBar.Model
    , turbulencePath : String -- this doesn't vary, it's more a prop (in the sense of react) than state. should be a way to use a thunk for the Turbulence case of DashboardState
    , highDensity : Bool
    , hoveredPipeline : Maybe Concourse.Pipeline
    , pipelineRunningKeyframes : String
    }


stateLens : Monocle.Lens.Lens Model (Result DashboardError SubState.SubState)
stateLens =
    Monocle.Lens.Lens .state (\b a -> { a | state = b })


substateOptional : Monocle.Optional.Optional Model SubState.SubState
substateOptional =
    Monocle.Optional.Optional (.state >> Result.toMaybe) (\s m -> { m | state = Ok s })


init : Ports -> Flags -> ( Model, Cmd Msg )
init ports flags =
    let
        ( topBar, topBarMsg ) =
            NewTopBar.init (not flags.highDensity) flags.search
    in
        ( { state = Err NotAsked
          , topBar = topBar
          , csrfToken = flags.csrfToken
          , turbulencePath = flags.turbulencePath
          , highDensity = flags.highDensity
          , hoveredPipeline = Nothing
          , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
          }
        , Cmd.batch
            [ fetchData
            , Cmd.map TopBarMsg topBarMsg
            , Group.pinTeamNames Group.stickyHeaderConfig
            , ports.title <| "Dashboard" ++ " - "
            ]
        )


handle : a -> a -> Result e v -> a
handle onError onSuccess result =
    case result of
        Ok _ ->
            onSuccess

        Err _ ->
            onError


substateLens : Monocle.Lens.Lens Model (Maybe SubState.SubState)
substateLens =
    Monocle.Lens.Lens (.state >> Result.toMaybe)
        (\mss model -> Maybe.map (\ss -> { model | state = Ok ss }) mss |> Maybe.withDefault model)


noop : Model -> ( Model, Cmd msg )
noop model =
    ( model, Cmd.none )


substate : String -> Bool -> ( Time.Time, ( APIData.APIData, Maybe Concourse.User ) ) -> Result DashboardError SubState.SubState
substate csrfToken highDensity ( now, ( apiData, user ) ) =
    apiData.pipelines
        |> List.head
        |> Maybe.map
            (always
                { teamData = SubState.teamData apiData user
                , details =
                    if highDensity then
                        Nothing
                    else
                        Just
                            { now = now
                            , dragState = Group.NotDragging
                            , dropState = Group.NotDropping
                            , showHelp = False
                            }
                , hideFooter = False
                , hideFooterCounter = 0
                , csrfToken = csrfToken
                }
            )
        |> Result.fromMaybe (NoPipelines)


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    let
        reload =
            Cmd.batch <|
                handle [] [ fetchData ] model.state
                    ++ [ Cmd.map TopBarMsg NewTopBar.fetchUser ]
    in
        case msg of
            Noop ->
                ( model, Cmd.none )

            APIDataFetched remoteData ->
                (case remoteData of
                    RemoteData.NotAsked ->
                        model |> stateLens.set (Err NotAsked)

                    RemoteData.Loading ->
                        model |> stateLens.set (Err NotAsked)

                    RemoteData.Failure _ ->
                        model |> stateLens.set (Err (Turbulence model.turbulencePath))

                    RemoteData.Success ( now, ( apiData, user ) ) ->
                        model
                            |> Monocle.Lens.modify stateLens
                                (Result.map
                                    (.set SubState.teamDataLens (SubState.teamData apiData user)
                                        >> .set (SubState.detailsOptional =|> Details.nowLens) now
                                        >> Ok
                                    )
                                    >> Result.withDefault (substate model.csrfToken model.highDensity ( now, ( apiData, user ) ))
                                )
                )
                    |> noop

            ClockTick now ->
                model
                    |> Monocle.Optional.modify substateOptional (SubState.tick now)
                    |> noop

            AutoRefresh _ ->
                ( model
                , reload
                )

            KeyPressed keycode ->
                handleKeyPressed (Char.fromCode keycode) model

            KeyDowns keycode ->
                update (TopBarMsg (NewTopBar.KeyDown keycode)) model

            ShowFooter ->
                model
                    |> Monocle.Optional.modify substateOptional SubState.showFooter
                    |> noop

            -- TODO pull the topbar logic right in here. right now there are wasted API calls and this crufty
            -- nonsense going on. however, this feels like a big change and not a big burning fire
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

            TogglePipelinePaused pipeline ->
                ( model, togglePipelinePaused pipeline model.csrfToken )

            PipelinePauseToggled pipeline (Ok ()) ->
                let
                    togglePipelinePause : List Concourse.Pipeline -> List Concourse.Pipeline
                    togglePipelinePause pipelines =
                        List.Extra.updateIf
                            ((==) pipeline)
                            -- TODO this lambda could be a utility/helper in the Concourse module
                            (\pipeline -> { pipeline | paused = not pipeline.paused })
                            pipelines
                in
                    ( model
                    , Cmd.none
                    )

            PipelinePauseToggled _ (Err _) ->
                ( model, Cmd.none )

            DragStart teamName index ->
                model
                    |> Monocle.Optional.modify
                        (substateOptional => SubState.detailsOptional)
                        ((Details.dragStateLens |> .set) <| Group.Dragging teamName index)
                    |> noop

            DragOver teamName index ->
                model
                    |> Monocle.Optional.modify
                        (substateOptional => SubState.detailsOptional)
                        ((Details.dropStateLens |> .set) <| Group.Dropping index)
                    |> noop

            TooltipHd pipelineName teamName ->
                ( model, DashboardHd.tooltipHd ( pipelineName, teamName ) )

            Tooltip pipelineName teamName ->
                ( model, tooltip ( pipelineName, teamName ) )

            DragEnd ->
                let
                    updatePipelines : ( Group.PipelineIndex, Group.PipelineIndex ) -> Group.Group -> ( Group.Group, Cmd Msg )
                    updatePipelines ( dragIndex, dropIndex ) group =
                        let
                            newGroup =
                                Group.shiftPipelines dragIndex dropIndex group
                        in
                            ( newGroup, orderPipelines newGroup.teamName newGroup.pipelines model.csrfToken )

                    dragDropOptional : Monocle.Optional.Optional Model ( Group.DragState, Group.DropState )
                    dragDropOptional =
                        substateOptional
                            => SubState.detailsOptional
                            =|> Monocle.Lens.tuple (Details.dragStateLens) (Details.dropStateLens)

                    dragDropIndexOptional : Monocle.Optional.Optional Model ( Group.PipelineIndex, Group.PipelineIndex )
                    dragDropIndexOptional =
                        dragDropOptional
                            => Monocle.Optional.zip
                                Group.dragIndexOptional
                                Group.dropIndexOptional

                    groupOptional : Monocle.Optional.Optional Model Group.Group
                    groupOptional =
                        (substateOptional
                            => SubState.detailsOptional
                            =|> Details.dragStateLens
                            => Group.teamNameOptional
                        )
                            >>= (\teamName ->
                                    substateOptional
                                        =|> SubState.teamDataLens
                                        =|> SubState.apiDataLens
                                        =|> Group.groupsLens
                                        => Group.findGroupOptional teamName
                                )

                    bigOptional : Monocle.Optional.Optional Model ( ( Group.PipelineIndex, Group.PipelineIndex ), Group.Group )
                    bigOptional =
                        Monocle.Optional.tuple
                            dragDropIndexOptional
                            groupOptional
                in
                    model
                        |> modifyWithEffect bigOptional
                            (\( t, g ) ->
                                let
                                    ( newG, msg ) =
                                        updatePipelines t g
                                in
                                    ( ( t, newG ), msg )
                            )
                        |> Tuple.mapFirst (dragDropOptional.set ( Group.NotDragging, Group.NotDropping ))

            PipelineButtonHover state ->
                ( { model | hoveredPipeline = state }, Cmd.none )


orderPipelines : String -> List Pipeline.PipelineWithJobs -> Concourse.CSRFToken -> Cmd Msg
orderPipelines teamName pipelines csrfToken =
    Task.attempt (always Noop) <|
        Concourse.Pipeline.order
            teamName
            (List.map (.name << .pipeline) <| pipelines)
            csrfToken



-- TODO this seems obsessed with pipelines. shouldn't be the dashboard's business


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
        , Window.resizes (TopBarMsg << NewTopBar.ScreenResized)
        ]


view : Model -> Html Msg
view model =
    Html.div [ class "page" ]
        [ (Html.map TopBarMsg) (NewTopBar.view model.topBar)
        , dashboardView model
        ]


dashboardView : Model -> Html Msg
dashboardView model =
    let
        mainContent =
            case model.state of
                Err NotAsked ->
                    [ Html.text "" ]

                Err (Turbulence path) ->
                    [ turbulenceView path ]

                Err NoPipelines ->
                    [ Html.div [ class "dashboard-no-content", css [ Css.height (Css.pct 100) ] ] [ (Html.map (always Noop) << Html.fromUnstyled) NoPipeline.view ] ]

                Ok substate ->
                    [ Html.div
                        [ class "dashboard-content" ]
                        (pipelinesView
                            { substate = substate
                            , query = (NewTopBar.query model.topBar)
                            , hoveredPipeline = model.hoveredPipeline
                            , pipelineRunningKeyframes = model.pipelineRunningKeyframes
                            }
                            ++ [ footerView substate ]
                        )
                    ]
    in
        Html.div
            [ classList [ ( .pageBodyClass Group.stickyHeaderConfig, True ), ( "dashboard-hd", model.highDensity ) ] ]
            mainContent


noResultsView : String -> Html Msg
noResultsView query =
    let
        boldedQuery =
            Html.span [ class "monospace-bold" ] [ Html.text query ]
    in
        Html.div
            [ class <| .pageBodyClass Group.stickyHeaderConfig ]
            [ Html.div [ class "dashboard-content " ]
                [ Html.div
                    [ class <| .sectionClass Group.stickyHeaderConfig ]
                    [ Html.div [ class "no-results" ]
                        [ Html.text "No results for "
                        , boldedQuery
                        , Html.text " matched your search."
                        ]
                    ]
                ]
            ]


helpView : Details.Details -> Html Msg
helpView details =
    Html.div
        [ classList
            [ ( "keyboard-help", True )
            , ( "hidden", not details.showHelp )
            ]
        ]
        [ Html.div [ class "help-title" ] [ Html.text "keyboard shortcuts" ]
        , Html.div [ class "help-line" ] [ Html.div [ class "keys" ] [ Html.span [ class "key" ] [ Html.text "/" ] ], Html.text "search" ]
        , Html.div [ class "help-line" ] [ Html.div [ class "keys" ] [ Html.span [ class "key" ] [ Html.text "?" ] ], Html.text "hide/show help" ]
        ]


toggleView : Bool -> Html Msg
toggleView highDensity =
    let
        hdClass =
            if highDensity then
                "hd-on"
            else
                "hd-off"

        route =
            if highDensity then
                Routes.dashboardRoute
            else
                Routes.dashboardHdRoute
    in
        Html.a [ class "toggle-high-density", href route, attribute "aria-label" "Toggle high-density view" ]
            [ Html.div [ class <| "dashboard-pipeline-icon " ++ hdClass ] [], Html.text "high-density" ]


footerView : SubState.SubState -> Html Msg
footerView substate =
    let
        showHelp =
            substate.details |> Maybe.map .showHelp |> Maybe.withDefault False
    in
        Html.div [] <|
            [ Html.div
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
                    , Html.div [ class "dashboard-high-density" ] [ substate.details |> Maybe.Extra.isJust |> not |> toggleView ]
                    ]
                , Html.div [ class "concourse-info" ]
                    [ Html.div [ class "concourse-version" ]
                        [ Html.text "version: v", substate.teamData |> SubState.apiData |> .version |> Html.text ]
                    , Html.div [ class "concourse-cli" ]
                        [ Html.text "cli: "
                        , Html.a [ href (Concourse.Cli.downloadUrl "amd64" "darwin"), attribute "aria-label" "Download OS X CLI" ]
                            [ Html.i [ class "fa fa-apple" ] [] ]
                        , Html.a [ href (Concourse.Cli.downloadUrl "amd64" "windows"), attribute "aria-label" "Download Windows CLI" ]
                            [ Html.i [ class "fa fa-windows" ] [] ]
                        , Html.a [ href (Concourse.Cli.downloadUrl "amd64" "linux"), attribute "aria-label" "Download Linux CLI" ]
                            [ Html.i [ class "fa fa-linux" ] [] ]
                        ]
                    ]
                ]
            , Html.div
                [ classList
                    [ ( "keyboard-help", True )
                    , ( "hidden", not showHelp )
                    ]
                ]
                [ Html.div [ class "help-title" ] [ Html.text "keyboard shortcuts" ]
                , Html.div [ class "help-line" ] [ Html.div [ class "keys" ] [ Html.span [ class "key" ] [ Html.text "/" ] ], Html.text "search" ]
                , Html.div [ class "help-line" ] [ Html.div [ class "keys" ] [ Html.span [ class "key" ] [ Html.text "?" ] ], Html.text "hide/show help" ]
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


pipelinesView :
    { substate : SubState.SubState
    , hoveredPipeline : Maybe Concourse.Pipeline
    , pipelineRunningKeyframes : String
    , query : String
    }
    -> List (Html Msg)
pipelinesView { substate, hoveredPipeline, pipelineRunningKeyframes, query } =
    let
        filteredGroups =
            substate.teamData |> SubState.apiData |> Group.groups |> filter query

        groupsToDisplay =
            if List.all (String.startsWith "team:") (filterTerms query) then
                filteredGroups
            else
                filteredGroups |> List.filter (.pipelines >> List.isEmpty >> not)

        highDensity =
            substate.details |> Maybe.Extra.isJust |> not

        groupViews =
            case substate.details of
                Just details ->
                    case substate.teamData of
                        SubState.Unauthenticated _ ->
                            List.map
                                (\g ->
                                    Group.view
                                        { header = (Group.headerView g)
                                        , dragState = details.dragState
                                        , dropState = details.dropState
                                        , now = details.now
                                        , hoveredPipeline = hoveredPipeline
                                        , pipelineRunningKeyframes = pipelineRunningKeyframes
                                        , group = g
                                        }
                                )
                                groupsToDisplay

                        SubState.Authenticated { user } ->
                            List.map
                                (\g ->
                                    Group.view
                                        { header = (GroupWithTag.headerView g False)
                                        , dragState = details.dragState
                                        , dropState = details.dropState
                                        , now = details.now
                                        , hoveredPipeline = hoveredPipeline
                                        , pipelineRunningKeyframes = pipelineRunningKeyframes
                                        , group = g.group
                                        }
                                )
                                (GroupWithTag.addTagsAndSort user groupsToDisplay)

                Nothing ->
                    case substate.teamData of
                        SubState.Unauthenticated _ ->
                            List.map
                                (\g ->
                                    Group.hdView
                                        (Group.headerView g)
                                        g.teamName
                                        g.pipelines
                                )
                                groupsToDisplay

                        SubState.Authenticated { user } ->
                            List.map
                                (\g ->
                                    Group.hdView
                                        (GroupWithTag.headerView g True)
                                        g.group.teamName
                                        g.group.pipelines
                                )
                                (GroupWithTag.addTagsAndSort user groupsToDisplay)
    in
        if List.isEmpty groupViews then
            [ noResultsView (toString query) ]
        else
            List.map Html.fromUnstyled groupViews


handleKeyPressed : Char -> Model -> ( Model, Cmd Msg )
handleKeyPressed key model =
    case key of
        '/' ->
            ( model, Task.attempt (always Noop) (Dom.focus "search-input-field") )

        '?' ->
            model
                |> Monocle.Optional.modify (substateOptional => SubState.detailsOptional) Details.toggleHelp
                |> noop

        _ ->
            update ShowFooter model


fetchData : Cmd Msg
fetchData =
    Group.remoteData
        |> Task.andThen remoteUser
        |> Task.map2 (,) Time.now
        |> RemoteData.asCmd
        |> Cmd.map APIDataFetched


remoteUser : APIData.APIData -> Task.Task Http.Error ( APIData.APIData, Maybe Concourse.User )
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
