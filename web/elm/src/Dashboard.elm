module Dashboard exposing
    ( Model
    , handleCallback
    , init
    , subscriptions
    , update
    , view
    )

import Callback exposing (Callback(..))
import Char
import Concourse
import Concourse.Cli as Cli
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Concourse.User
import Dashboard.APIData as APIData
import Dashboard.Details as Details
import Dashboard.Footer as Footer
import Dashboard.Group as Group
import Dashboard.Models as Models
import Dashboard.Msgs as Msgs exposing (Msg(..), fromDashboardMsg)
import Dashboard.Styles as Styles
import Dashboard.SubState as SubState
import Dashboard.Text as Text
import Effects exposing (Effect(..))
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes
    exposing
        ( attribute
        , class
        , classList
        , css
        , draggable
        , href
        , id
        , src
        , style
        )
import Html.Styled.Events exposing (onMouseEnter, onMouseLeave)
import Http
import Monocle.Common exposing ((<|>), (=>))
import Monocle.Lens
import Monocle.Optional
import MonocleHelpers exposing (..)
import NewTopBar.Model
import NewTopBar.Msgs
import NewestTopBar as NewTopBar
import Regex exposing (HowMany(All), regex, replace)
import RemoteData
import Routes
import ScreenSize
import Simple.Fuzzy exposing (filter, match, root)
import Subscription exposing (Subscription(..))
import Task
import Time exposing (Time)
import UserState exposing (UserState)


type alias Flags =
    { csrfToken : String
    , turbulencePath : String
    , search : String
    , highDensity : Bool
    , pipelineRunningKeyframes : String
    , route : Routes.Route
    }


type DashboardError
    = Turbulence String


type alias Model =
    { csrfToken : String
    , state : RemoteData.RemoteData DashboardError SubState.SubState
    , turbulencePath : String
    , highDensity : Bool
    , hoveredPipeline : Maybe Models.Pipeline
    , pipelineRunningKeyframes : String
    , groups : List Group.Group
    , hoveredCliIcon : Maybe Cli.Cli
    , hoveredTopCliIcon : Maybe Cli.Cli
    , screenSize : ScreenSize.ScreenSize
    , version : String
    , userState : UserState.UserState
    , userMenuVisible : Bool
    , topBar : NewTopBar.Model.Model
    , hideFooter : Bool
    , hideFooterCounter : Int
    , showHelp : Bool
    }


substateOptional : Monocle.Optional.Optional Model SubState.SubState
substateOptional =
    Monocle.Optional.Optional (.state >> RemoteData.toMaybe) (\s m -> { m | state = RemoteData.Success s })


init : Flags -> ( Model, List Effect )
init flags =
    let
        ( topBar, topBarEffects ) =
            NewTopBar.init { route = flags.route }
    in
    ( { state = RemoteData.NotAsked
      , csrfToken = flags.csrfToken
      , turbulencePath = flags.turbulencePath
      , highDensity = flags.highDensity
      , hoveredPipeline = Nothing
      , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
      , groups = []
      , hoveredCliIcon = Nothing
      , hoveredTopCliIcon = Nothing
      , screenSize = ScreenSize.Desktop
      , version = ""
      , userState = UserState.UserStateUnknown
      , userMenuVisible = False
      , hideFooter = False
      , hideFooterCounter = 0
      , showHelp = False
      , topBar = topBar
      }
    , [ FetchData
      , PinTeamNames Group.stickyHeaderConfig
      , SetTitle <| "Dashboard" ++ " - "
      , GetScreenSize
      ]
        ++ topBarEffects
    )


handleCallback : Callback -> Model -> ( Model, List Effect )
handleCallback msg model =
    let
        ( newTopBar, topBarEffects ) =
            NewTopBar.handleCallback msg model.topBar

        ( newModel, dashboardEffects ) =
            handleCallbackWithoutTopBar msg model
    in
    ( { newModel | topBar = newTopBar }
    , topBarEffects ++ dashboardEffects
    )


handleCallbackWithoutTopBar : Callback -> Model -> ( Model, List Effect )
handleCallbackWithoutTopBar msg model =
    case msg of
        APIDataFetched (Err _) ->
            ( { model | state = RemoteData.Failure (Turbulence model.turbulencePath) }, [] )

        APIDataFetched (Ok ( now, apiData )) ->
            let
                groups =
                    Group.groups apiData

                noPipelines =
                    List.isEmpty <| List.concatMap .pipelines groups

                newModel =
                    case model.state of
                        RemoteData.Success substate ->
                            { model
                                | state =
                                    RemoteData.Success (SubState.tick now substate)
                            }

                        _ ->
                            { model
                                | state =
                                    RemoteData.Success
                                        { now = now
                                        , dragState = Group.NotDragging
                                        , dropState = Group.NotDropping
                                        }
                            }

                userState =
                    case apiData.user of
                        Just u ->
                            UserState.UserStateLoggedIn u

                        Nothing ->
                            UserState.UserStateLoggedOut
            in
            if model.highDensity && noPipelines then
                ( { newModel
                    | highDensity = False
                    , groups = groups
                    , version = apiData.version
                    , userState = userState
                  }
                , [ ModifyUrl (Routes.dashboardRoute False) ]
                )

            else
                ( { newModel
                    | groups = groups
                    , version = apiData.version
                    , userState = userState
                  }
                , []
                )

        LoggedOut (Ok ()) ->
            ( { model
                | userState = UserState.UserStateLoggedOut
                , userMenuVisible = False
              }
            , [ NavigateTo (Routes.dashboardRoute model.highDensity)
              , FetchData
              ]
            )

        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, [] )

        ScreenResized size ->
            let
                newSize =
                    ScreenSize.fromWindowSize size
            in
            ( { model | screenSize = newSize }, [] )

        _ ->
            ( model, [] )


update : Msg -> Model -> ( Model, List Effect )
update msg model =
    let
        ( newTopBar, topBarEffects ) =
            NewTopBar.update (fromDashboardMsg msg) model.topBar

        ( newModel, dashboardEffects ) =
            updateWithoutTopBar msg model
    in
    ( { newModel | topBar = newTopBar }
    , topBarEffects ++ dashboardEffects
    )


updateWithoutTopBar : Msg -> Model -> ( Model, List Effect )
updateWithoutTopBar msg model =
    case msg of
        ClockTick now ->
            ( let
                newModel =
                    Footer.tick model
              in
              { newModel | state = RemoteData.map (SubState.tick now) newModel.state }
            , []
            )

        AutoRefresh _ ->
            ( model, [ FetchData ] )

        KeyPressed keycode ->
            handleKeyPressed (Char.fromCode keycode) model

        ShowFooter ->
            ( Footer.showFooter model, [] )

        TogglePipelinePaused pipeline ->
            ( model
            , [ SendTogglePipelineRequest
                    { pipeline = pipeline, csrfToken = model.csrfToken }
              ]
            )

        DragStart teamName index ->
            let
                newModel =
                    { model | state = RemoteData.map (\s -> { s | dragState = Group.Dragging teamName index }) model.state }
            in
            ( newModel, [] )

        DragOver teamName index ->
            let
                newModel =
                    { model | state = RemoteData.map (\s -> { s | dropState = Group.Dropping index }) model.state }
            in
            ( newModel, [] )

        TooltipHd pipelineName teamName ->
            ( model, [ ShowTooltipHd ( pipelineName, teamName ) ] )

        Tooltip pipelineName teamName ->
            ( model, [ ShowTooltip ( pipelineName, teamName ) ] )

        DragEnd ->
            let
                updatePipelines :
                    ( Group.PipelineIndex, Group.PipelineIndex )
                    -> Group.Group
                    -> ( Group.Group, List Effect )
                updatePipelines ( dragIndex, dropIndex ) group =
                    let
                        newGroup =
                            Group.shiftPipelines dragIndex dropIndex group
                    in
                    ( newGroup
                    , [ SendOrderPipelinesRequest
                            newGroup.teamName
                            newGroup.pipelines
                            model.csrfToken
                      ]
                    )

                dragDropOptional : Monocle.Optional.Optional Model ( Group.DragState, Group.DropState )
                dragDropOptional =
                    substateOptional
                        =|> Monocle.Lens.tuple
                                Details.dragStateLens
                                Details.dropStateLens

                dragDropIndexOptional : Monocle.Optional.Optional Model ( Group.PipelineIndex, Group.PipelineIndex )
                dragDropIndexOptional =
                    dragDropOptional
                        => Monocle.Optional.zip
                            Group.dragIndexOptional
                            Group.dropIndexOptional

                groupsLens : Monocle.Lens.Lens Model (List Group.Group)
                groupsLens =
                    Monocle.Lens.Lens .groups (\b a -> { a | groups = b })

                groupOptional : Monocle.Optional.Optional Model Group.Group
                groupOptional =
                    (substateOptional
                        =|> Details.dragStateLens
                        => Group.teamNameOptional
                    )
                        >>= (\teamName ->
                                groupsLens
                                    <|= Group.findGroupOptional teamName
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
            ( { model | hoveredPipeline = state }, [] )

        CliHover state ->
            ( { model | hoveredCliIcon = state }, [] )

        TopCliHover state ->
            ( { model | hoveredTopCliIcon = state }, [] )

        ResizeScreen size ->
            ( { model | screenSize = ScreenSize.fromWindowSize size }, [] )

        FromTopBar NewTopBar.Msgs.LogOut ->
            ( { model | state = RemoteData.NotAsked }, [] )

        FromTopBar NewTopBar.Msgs.ToggleUserMenu ->
            ( { model | userMenuVisible = not model.userMenuVisible }, [] )

        FromTopBar m ->
            ( model, [] )


subscriptions : Model -> List (Subscription Msg)
subscriptions model =
    [ OnClockTick Time.second ClockTick
    , OnClockTick (5 * Time.second) AutoRefresh
    , OnMouseMove ShowFooter
    , OnMouseClick ShowFooter
    , OnKeyPress KeyPressed
    , OnKeyDown (NewTopBar.Msgs.KeyDown >> FromTopBar)
    , OnWindowResize Msgs.ResizeScreen
    ]


view : UserState -> Model -> Html Msg
view userState model =
    Html.div
        [ class "page"
        , style
            [ ( "-webkit-font-smoothing", "antialiased" )
            , ( "font-weight", "700" )
            ]
        ]
        [ Html.map FromTopBar (NewTopBar.view userState NewTopBar.Model.None model.topBar)
        , dashboardView model
        ]


dashboardView : Model -> Html Msg
dashboardView model =
    let
        mainContent =
            case model.state of
                RemoteData.NotAsked ->
                    [ Html.text "" ]

                RemoteData.Loading ->
                    [ Html.text "" ]

                RemoteData.Failure (Turbulence path) ->
                    [ turbulenceView path ]

                RemoteData.Success substate ->
                    [ Html.div
                        [ class "dashboard-content" ]
                      <|
                        welcomeCard model
                            ++ pipelinesView
                                { groups = model.groups
                                , substate = substate
                                , query = NewTopBar.query model.topBar
                                , hoveredPipeline = model.hoveredPipeline
                                , pipelineRunningKeyframes =
                                    model.pipelineRunningKeyframes
                                , userState = model.userState
                                , highDensity = model.highDensity
                                }
                    ]
                        ++ (List.map Html.fromUnstyled <| Footer.view model)
    in
    Html.div
        [ classList
            [ ( .pageBodyClass Group.stickyHeaderConfig, True )
            , ( "dashboard-hd", model.highDensity )
            ]
        ]
        mainContent


welcomeCard :
    { a
        | hoveredTopCliIcon : Maybe Cli.Cli
        , groups : List Group.Group
        , userState : UserState.UserState
    }
    -> List (Html Msg)
welcomeCard { hoveredTopCliIcon, groups, userState } =
    let
        noPipelines =
            List.isEmpty (groups |> List.concatMap .pipelines)

        cliIcon : Maybe Cli.Cli -> Cli.Cli -> Html Msg
        cliIcon hoveredTopCliIcon cli =
            Html.a
                [ href (Cli.downloadUrl cli)
                , attribute "aria-label" <| Cli.label cli
                , style <|
                    Styles.topCliIcon
                        { hovered = hoveredTopCliIcon == Just cli
                        , cli = cli
                        }
                , id <| "top-cli-" ++ Cli.id cli
                , onMouseEnter <| TopCliHover <| Just cli
                , onMouseLeave <| TopCliHover Nothing
                ]
                []
    in
    if noPipelines then
        [ Html.div
            [ id "welcome-card"
            , style Styles.welcomeCard
            ]
            [ Html.div
                [ style Styles.welcomeCardTitle ]
                [ Html.text Text.welcome ]
            , Html.div
                [ style Styles.welcomeCardBody ]
              <|
                [ Html.div
                    [ style
                        [ ( "display", "flex" )
                        , ( "align-items", "center" )
                        ]
                    ]
                  <|
                    [ Html.div
                        [ style [ ( "margin-right", "10px" ) ] ]
                        [ Html.text Text.cliInstructions ]
                    ]
                        ++ List.map (cliIcon hoveredTopCliIcon) Cli.clis
                , Html.div
                    []
                    [ Html.text Text.setPipelineInstructions ]
                ]
                    ++ loginInstruction userState
            , Html.pre
                [ style Styles.asciiArt ]
                [ Html.text Text.asciiArt ]
            ]
        ]

    else
        []


loginInstruction : UserState.UserState -> List (Html Msg)
loginInstruction userState =
    case userState of
        UserState.UserStateLoggedIn _ ->
            []

        _ ->
            [ Html.div
                [ id "login-instruction"
                , style [ ( "line-height", "42px" ) ]
                ]
                [ Html.text "login "
                , Html.a
                    [ href "/login"
                    , style [ ( "text-decoration", "underline" ) ]
                    ]
                    [ Html.text "here" ]
                ]
            ]


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


helpView : { a | showHelp : Bool } -> Html Msg
helpView { showHelp } =
    Html.div
        [ classList
            [ ( "keyboard-help", True )
            , ( "hidden", not showHelp )
            ]
        ]
        [ Html.div
            [ class "help-title" ]
            [ Html.text "keyboard shortcuts" ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span
                    [ class "key" ]
                    [ Html.text "/" ]
                ]
            , Html.text "search"
            ]
        , Html.div [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span
                    [ class "key" ]
                    [ Html.text "?" ]
                ]
            , Html.text "hide/show help"
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
    { groups : List Group.Group
    , substate : SubState.SubState
    , hoveredPipeline : Maybe Models.Pipeline
    , pipelineRunningKeyframes : String
    , query : String
    , userState : UserState.UserState
    , highDensity : Bool
    }
    -> List (Html Msg)
pipelinesView { groups, substate, hoveredPipeline, pipelineRunningKeyframes, query, userState, highDensity } =
    let
        filteredGroups =
            groups |> filter query |> List.sortWith Group.ordering

        groupsToDisplay =
            if List.all (String.startsWith "team:") (filterTerms query) then
                filteredGroups

            else
                filteredGroups |> List.filter (.pipelines >> List.isEmpty >> not)

        groupViews =
            if highDensity then
                groupsToDisplay
                    |> List.map (Group.hdView pipelineRunningKeyframes)

            else
                groupsToDisplay
                    |> List.map
                        (Group.view
                            { dragState = substate.dragState
                            , dropState = substate.dropState
                            , now = substate.now
                            , hoveredPipeline = hoveredPipeline
                            , pipelineRunningKeyframes = pipelineRunningKeyframes
                            }
                        )
    in
    if List.isEmpty groupViews && (not <| String.isEmpty query) then
        [ noResultsView (toString query) ]

    else
        List.map Html.fromUnstyled groupViews


handleKeyPressed : Char -> Footer.Model r -> ( Footer.Model r, List Effect )
handleKeyPressed key model =
    case key of
        '/' ->
            ( model, [] )

        '?' ->
            ( Footer.toggleHelp model, [] )

        _ ->
            ( Footer.showFooter model, [] )


remoteUser : APIData.APIData -> Task.Task Http.Error ( APIData.APIData, Maybe Concourse.User )
remoteUser d =
    Concourse.User.fetchUser
        |> Task.map ((,) d << Just)
        |> Task.onError (always <| Task.succeed <| ( d, Nothing ))


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
            fuzzySearch (.status >> PipelineStatus.show) statusSearchTerm pipelines
    in
    { group
        | pipelines =
            if searchStatus then
                filterByStatus

            else
                fuzzySearch .name term pipelines
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
