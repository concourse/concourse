port module Dashboard
    exposing
        ( Model
        , Effect(..)
        , init
        , subscriptions
        , update
        , view
        , toCmd
        )

import Array
import Char
import Concourse
import Concourse.Cli as Cli
import Concourse.Pipeline
import Concourse.PipelineStatus
import Concourse.User
import Css
import Dashboard.APIData as APIData
import Dashboard.Details as Details
import Dashboard.Group as Group
import Dashboard.Models as Models
import Dashboard.Msgs as Msgs exposing (Msg(..))
import Dashboard.SubState as SubState
import Dashboard.Styles as Styles
import Dom
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes
    exposing
        ( attribute
        , css
        , class
        , classList
        , draggable
        , href
        , id
        , src
        , style
        )
import Html.Styled.Events exposing (onMouseEnter, onMouseLeave)
import Http
import Keyboard
import LoginRedirect
import Mouse
import Monocle.Common exposing ((=>), (<|>))
import Monocle.Optional
import Monocle.Lens
import MonocleHelpers exposing (..)
import Navigation
import NewTopBar
import NoPipeline
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Regex exposing (HowMany(All), regex, replace)
import RemoteData
import Routes
import ScreenSize
import SearchBar exposing (SearchBar(..))
import Simple.Fuzzy exposing (filter, match, root)
import Task
import Time exposing (Time)
import UserState
import Window


type alias Ports =
    { title : String -> Cmd Msg
    }


port tooltip : ( String, String ) -> Cmd msg


port tooltipHd : ( String, String ) -> Cmd msg


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
    , turbulencePath : String
    , highDensity : Bool
    , hoveredPipeline : Maybe Models.Pipeline
    , pipelineRunningKeyframes : String
    , groups : List Group.Group
    , hoveredCliIcon : Maybe Cli.Cli
    , screenSize : ScreenSize.ScreenSize
    , version : String
    , userState : UserState.UserState
    , userMenuVisible : Bool
    , searchBar : SearchBar
    }


type Effect
    = FetchData
    | FocusSearchInput
    | ModifyUrl String
    | NewUrl String
    | SendTogglePipelineRequest { pipeline : Models.Pipeline, csrfToken : Concourse.CSRFToken }
    | ShowTooltip ( String, String )
    | ShowTooltipHd ( String, String )
    | SendOrderPipelinesRequest String (List Models.Pipeline) Concourse.CSRFToken
    | RedirectToLogin String
    | SendLogOutRequest


toCmd : Effect -> Cmd Msg
toCmd effect =
    case effect of
        FetchData ->
            fetchData

        FocusSearchInput ->
            Task.attempt (always Noop) (Dom.focus "search-input-field")

        ModifyUrl url ->
            Navigation.modifyUrl url

        NewUrl url ->
            Navigation.newUrl url

        SendTogglePipelineRequest { pipeline, csrfToken } ->
            togglePipelinePaused { pipeline = pipeline, csrfToken = csrfToken }

        ShowTooltip ( teamName, pipelineName ) ->
            tooltip ( teamName, pipelineName )

        ShowTooltipHd ( teamName, pipelineName ) ->
            tooltipHd ( teamName, pipelineName )

        SendOrderPipelinesRequest teamName pipelines csrfToken ->
            orderPipelines teamName pipelines csrfToken

        RedirectToLogin s ->
            LoginRedirect.requestLoginRedirect s

        SendLogOutRequest ->
            NewTopBar.logOut


stateLens : Monocle.Lens.Lens Model (Result DashboardError SubState.SubState)
stateLens =
    Monocle.Lens.Lens .state (\b a -> { a | state = b })


substateOptional : Monocle.Optional.Optional Model SubState.SubState
substateOptional =
    Monocle.Optional.Optional (.state >> Result.toMaybe) (\s m -> { m | state = Ok s })


init : Ports -> Flags -> ( Model, Cmd Msg )
init ports flags =
    let
        searchBar =
            Expanded
                { query = flags.search
                , selectionMade = False
                , showAutocomplete = False
                , selection = 0
                }
    in
        ( { state = Err NotAsked
          , csrfToken = flags.csrfToken
          , turbulencePath = flags.turbulencePath
          , highDensity = flags.highDensity
          , hoveredPipeline = Nothing
          , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
          , groups = []
          , hoveredCliIcon = Nothing
          , screenSize = ScreenSize.Desktop
          , version = ""
          , userState = UserState.UserStateUnknown
          , userMenuVisible = False
          , searchBar = searchBar
          }
        , Cmd.batch
            [ fetchData
            , Group.pinTeamNames Group.stickyHeaderConfig
            , ports.title <| "Dashboard" ++ " - "
            , Task.perform ScreenResized Window.size
            ]
        )


substateLens : Monocle.Lens.Lens Model (Maybe SubState.SubState)
substateLens =
    Monocle.Lens.Lens (.state >> Result.toMaybe)
        (\mss model -> Maybe.map (\ss -> { model | state = Ok ss }) mss |> Maybe.withDefault model)


noop : Model -> ( Model, List Effect )
noop model =
    ( model, [] )


update : Msg -> Model -> ( Model, List Effect )
update msg model =
    case msg of
        Noop ->
            ( model, [] )

        APIDataFetched remoteData ->
            (case remoteData of
                RemoteData.NotAsked ->
                    ( { model | state = Err NotAsked }, [] )

                RemoteData.Loading ->
                    ( { model | state = Err NotAsked }, [] )

                RemoteData.Failure _ ->
                    ( { model | state = Err (Turbulence model.turbulencePath) }
                    , []
                    )

                RemoteData.Success ( now, apiData ) ->
                    let
                        groups =
                            Group.groups apiData

                        noPipelines =
                            List.isEmpty <| List.concatMap .pipelines groups

                        newModel =
                            if noPipelines then
                                { model | state = Err NoPipelines }
                            else
                                case model.state of
                                    Ok substate ->
                                        { model
                                            | state =
                                                Ok (SubState.tick now substate)
                                        }

                                    _ ->
                                        { model
                                            | state =
                                                Ok
                                                    { hideFooter = False
                                                    , hideFooterCounter = 0
                                                    , now = now
                                                    , dragState = Group.NotDragging
                                                    , dropState = Group.NotDropping
                                                    , showHelp = False
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
                            , [ ModifyUrl Routes.dashboardRoute ]
                            )
                        else
                            ( { newModel
                                | groups = groups
                                , version = apiData.version
                                , userState = userState
                              }
                            , []
                            )
            )

        ClockTick now ->
            ( case model.state of
                Ok substate ->
                    { model | state = Ok (SubState.tick now substate) }

                _ ->
                    model
            , []
            )

        AutoRefresh _ ->
            ( model
            , [ FetchData ]
            )

        KeyPressed keycode ->
            handleKeyPressed (Char.fromCode keycode) model

        ShowFooter ->
            ( case model.state of
                Ok substate ->
                    { model | state = Ok (SubState.showFooter substate) }

                _ ->
                    model
            , []
            )

        TogglePipelinePaused pipeline ->
            ( model
            , [ SendTogglePipelineRequest
                    { pipeline = pipeline, csrfToken = model.csrfToken }
              ]
            )

        DragStart teamName index ->
            model
                |> Monocle.Optional.modify
                    substateOptional
                    ((Details.dragStateLens |> .set) <| Group.Dragging teamName index)
                |> noop

        DragOver teamName index ->
            model
                |> Monocle.Optional.modify
                    substateOptional
                    ((Details.dropStateLens |> .set) <| Group.Dropping index)
                |> noop

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
                                (Details.dragStateLens)
                                (Details.dropStateLens)

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

        FilterMsg query ->
            let
                newModel =
                    case model.searchBar of
                        Expanded r ->
                            { model | searchBar = Expanded { r | query = query } }

                        _ ->
                            model
            in
                ( newModel
                , [ FocusSearchInput, ModifyUrl (NewTopBar.queryStringFromSearch query) ]
                )

        LogIn ->
            ( model, [ RedirectToLogin "" ] )

        LogOut ->
            ( { model | state = Err NotAsked }, [ SendLogOutRequest, FetchData ] )

        LoggedOut (Ok ()) ->
            let
                redirectUrl =
                    if model.highDensity then
                        Routes.dashboardHdRoute
                    else
                        Routes.dashboardRoute
            in
                ( { model
                    | userState = UserState.UserStateLoggedOut
                    , userMenuVisible = False
                  }
                , [ NewUrl redirectUrl ]
                )

        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, [] )

        ToggleUserMenu ->
            ( { model | userMenuVisible = not model.userMenuVisible }, [] )

        FocusMsg ->
            let
                newModel =
                    case model.searchBar of
                        Expanded r ->
                            { model
                                | searchBar =
                                    Expanded
                                        { r
                                            | showAutocomplete = True
                                        }
                            }

                        _ ->
                            model
            in
                ( newModel, [] )

        BlurMsg ->
            let
                newModel =
                    case model.searchBar of
                        Expanded r ->
                            case model.screenSize of
                                ScreenSize.Mobile ->
                                    if String.isEmpty r.query then
                                        { model | searchBar = Collapsed }
                                    else
                                        { model
                                            | searchBar =
                                                Expanded
                                                    { r
                                                        | showAutocomplete = False
                                                        , selectionMade = False
                                                        , selection = 0
                                                    }
                                        }

                                ScreenSize.Desktop ->
                                    { model
                                        | searchBar =
                                            Expanded
                                                { r
                                                    | showAutocomplete = False
                                                    , selectionMade = False
                                                    , selection = 0
                                                }
                                    }

                                ScreenSize.BigDesktop ->
                                    { model
                                        | searchBar =
                                            Expanded
                                                { r
                                                    | showAutocomplete = False
                                                    , selectionMade = False
                                                    , selection = 0
                                                }
                                    }

                        _ ->
                            model
            in
                ( newModel, [] )

        SelectMsg index ->
            let
                newModel =
                    case model.searchBar of
                        Expanded r ->
                            { model
                                | searchBar =
                                    Expanded
                                        { r
                                            | selectionMade = True
                                            , selection = index + 1
                                        }
                            }

                        _ ->
                            model
            in
                ( newModel, [] )

        KeyDowns keycode ->
            case model.searchBar of
                Expanded r ->
                    if not r.showAutocomplete then
                        ( { model
                            | searchBar =
                                Expanded
                                    { r
                                        | selectionMade = False
                                        , selection = 0
                                    }
                          }
                        , []
                        )
                    else
                        case keycode of
                            -- enter key
                            13 ->
                                if not r.selectionMade then
                                    ( model, [] )
                                else
                                    let
                                        options =
                                            Array.fromList
                                                (NewTopBar.autocompleteOptions
                                                    { query = r.query
                                                    , groups = model.groups
                                                    }
                                                )

                                        index =
                                            (r.selection - 1) % Array.length options

                                        selectedItem =
                                            case Array.get index options of
                                                Nothing ->
                                                    r.query

                                                Just item ->
                                                    item
                                    in
                                        ( { model
                                            | searchBar =
                                                Expanded
                                                    { r
                                                        | selectionMade = False
                                                        , selection = 0
                                                        , query = selectedItem
                                                    }
                                          }
                                        , []
                                        )

                            -- up arrow
                            38 ->
                                ( { model
                                    | searchBar =
                                        Expanded
                                            { r
                                                | selectionMade = True
                                                , selection = r.selection - 1
                                            }
                                  }
                                , []
                                )

                            -- down arrow
                            40 ->
                                ( { model
                                    | searchBar =
                                        Expanded
                                            { r
                                                | selectionMade = True
                                                , selection = r.selection + 1
                                            }
                                  }
                                , []
                                )

                            -- escape key
                            27 ->
                                ( model, [ FocusSearchInput ] )

                            _ ->
                                ( { model
                                    | searchBar =
                                        Expanded
                                            { r
                                                | selectionMade = False
                                                , selection = 0
                                            }
                                  }
                                , []
                                )

                _ ->
                    ( model, [] )

        ShowSearchInput ->
            let
                newModel =
                    { model
                        | searchBar =
                            Expanded
                                { query = ""
                                , selectionMade = False
                                , showAutocomplete = False
                                , selection = 0
                                }
                    }
            in
                case model.searchBar of
                    Collapsed ->
                        ( newModel, [ FocusSearchInput ] )

                    _ ->
                        ( model, [] )

        ScreenResized size ->
            let
                newSize =
                    ScreenSize.fromWindowSize size
            in
                ( { model
                    | screenSize = newSize
                    , searchBar =
                        SearchBar.screenSizeChanged
                            { oldSize = model.screenSize
                            , newSize = newSize
                            }
                            model.searchBar
                  }
                , []
                )


orderPipelines : String -> List Models.Pipeline -> Concourse.CSRFToken -> Cmd Msg
orderPipelines teamName pipelines csrfToken =
    Task.attempt (always Noop) <|
        Concourse.Pipeline.order
            teamName
            (List.map .name pipelines)
            csrfToken



-- TODO this seems obsessed with pipelines. shouldn't be the dashboard's business


togglePipelinePaused : { pipeline : Models.Pipeline, csrfToken : Concourse.CSRFToken } -> Cmd Msg
togglePipelinePaused { pipeline, csrfToken } =
    Task.attempt (always Noop) <|
        if pipeline.status == PipelineStatus.PipelineStatusPaused then
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
        , Window.resizes Msgs.ScreenResized
        ]


view : Model -> Html Msg
view model =
    Html.div [ class "page" ]
        [ NewTopBar.view model
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
                            { groups = model.groups
                            , substate = substate
                            , query = NewTopBar.query model
                            , hoveredPipeline = model.hoveredPipeline
                            , pipelineRunningKeyframes = model.pipelineRunningKeyframes
                            , userState = model.userState
                            , highDensity = model.highDensity
                            }
                        )
                    ]
                        ++ footerView
                            { substate = substate
                            , hoveredCliIcon = model.hoveredCliIcon
                            , screenSize = model.screenSize
                            , version = model.version
                            , highDensity = model.highDensity
                            }
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


helpView : Details.Details r -> Html Msg
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
        route =
            if highDensity then
                Routes.dashboardRoute
            else
                Routes.dashboardHdRoute
    in
        Html.a
            [ style Styles.highDensityToggle
            , href route
            , attribute "aria-label" "Toggle high-density view"
            ]
            [ Html.div [ style <| Styles.highDensityIcon highDensity ] []
            , Html.text "high-density"
            ]


footerView :
    { substate : SubState.SubState
    , hoveredCliIcon : Maybe Cli.Cli
    , screenSize : ScreenSize.ScreenSize
    , version : String
    , highDensity : Bool
    }
    -> List (Html Msg)
footerView { substate, hoveredCliIcon, screenSize, version, highDensity } =
    if substate.showHelp then
        [ keyboardHelpView ]
    else if not substate.hideFooter then
        [ infoView
            { substate = substate
            , hoveredCliIcon = hoveredCliIcon
            , screenSize = screenSize
            , version = version
            , highDensity = highDensity
            }
        ]
    else
        []


legendItem : PipelineStatus -> Html Msg
legendItem status =
    Html.div [ style Styles.legendItem ]
        [ Html.div
            [ style <| Styles.pipelineStatusIcon status ]
            []
        , Html.div [ style [ ( "width", "10px" ) ] ] []
        , Html.text <| PipelineStatus.show status
        ]


infoView :
    { substate : SubState.SubState
    , hoveredCliIcon : Maybe Cli.Cli
    , screenSize : ScreenSize.ScreenSize
    , version : String
    , highDensity : Bool
    }
    -> Html Msg
infoView { substate, hoveredCliIcon, screenSize, version, highDensity } =
    let
        legendSeparator : ScreenSize.ScreenSize -> List (Html Msg)
        legendSeparator screenSize =
            case screenSize of
                ScreenSize.Mobile ->
                    []

                ScreenSize.Desktop ->
                    [ Html.div
                        [ style Styles.legendSeparator ]
                        [ Html.text "|" ]
                    ]

                ScreenSize.BigDesktop ->
                    [ Html.div
                        [ style Styles.legendSeparator ]
                        [ Html.text "|" ]
                    ]

        cliIcon : Cli.Cli -> Maybe Cli.Cli -> Html Msg
        cliIcon cli hoveredCliIcon =
            let
                ( cliName, ariaText, icon ) =
                    case cli of
                        Cli.OSX ->
                            ( "osx", "OS X", "apple" )

                        Cli.Windows ->
                            ( "windows", "Windows", "windows" )

                        Cli.Linux ->
                            ( "linux", "Linux", "linux" )
            in
                Html.a
                    [ href (Cli.downloadUrl "amd64" cliName)
                    , attribute "aria-label" <| "Download " ++ ariaText ++ " CLI"
                    , style <| Styles.infoCliIcon (hoveredCliIcon == Just cli)
                    , id <| "cli-" ++ cliName
                    , onMouseEnter <| CliHover <| Just cli
                    , onMouseLeave <| CliHover Nothing
                    ]
                    [ Html.i [ class <| "fa fa-" ++ icon ] [] ]
    in
        Html.div
            [ id "dashboard-info"
            , style <| Styles.infoBar screenSize
            ]
            [ Html.div
                [ id "legend"
                , style Styles.legend
                ]
              <|
                List.map legendItem
                    [ PipelineStatusPending False
                    , PipelineStatusPaused
                    ]
                    ++ [ Html.div [ style Styles.legendItem ]
                            [ Html.div
                                [ style
                                    [ ( "background-image", "url(public/images/ic_running_legend.svg)" )
                                    , ( "height", "20px" )
                                    , ( "width", "20px" )
                                    , ( "background-repeat", "no-repeat" )
                                    , ( "background-position", "50% 50%" )
                                    ]
                                ]
                                []
                            , Html.div [ style [ ( "width", "10px" ) ] ] []
                            , Html.text "running"
                            ]
                       ]
                    ++ List.map legendItem
                        [ PipelineStatusFailed PipelineStatus.Running
                        , PipelineStatusErrored PipelineStatus.Running
                        , PipelineStatusAborted PipelineStatus.Running
                        , PipelineStatusSucceeded PipelineStatus.Running
                        ]
                    ++ legendSeparator screenSize
                    ++ [ toggleView highDensity ]
            , Html.div [ id "concourse-info", style Styles.info ]
                [ Html.div [ style Styles.infoItem ]
                    [ Html.text <| "version: v" ++ version ]
                , Html.div [ style Styles.infoItem ]
                    [ Html.span
                        [ style [ ( "margin-right", "10px" ) ] ]
                        [ Html.text "cli: " ]
                    , cliIcon Cli.OSX hoveredCliIcon
                    , cliIcon Cli.Windows hoveredCliIcon
                    , cliIcon Cli.Linux hoveredCliIcon
                    ]
                ]
            ]


keyboardHelpView : Html Msg
keyboardHelpView =
    Html.div
        [ classList
            [ ( "keyboard-help", True )
            ]
        ]
        [ Html.div [ class "help-title" ] [ Html.text "keyboard shortcuts" ]
        , Html.div [ class "help-line" ] [ Html.div [ class "keys" ] [ Html.span [ class "key" ] [ Html.text "/" ] ], Html.text "search" ]
        , Html.div [ class "help-line" ] [ Html.div [ class "keys" ] [ Html.span [ class "key" ] [ Html.text "?" ] ], Html.text "hide/show help" ]
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
        if List.isEmpty groupViews then
            [ noResultsView (toString query) ]
        else
            List.map Html.fromUnstyled groupViews


handleKeyPressed : Char -> Model -> ( Model, List Effect )
handleKeyPressed key model =
    case key of
        '/' ->
            ( model, [ FocusSearchInput ] )

        '?' ->
            model
                |> Monocle.Optional.modify substateOptional Details.toggleHelp
                |> noop

        _ ->
            update ShowFooter model


fetchData : Cmd Msg
fetchData =
    APIData.remoteData
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
            fuzzySearch (.status >> Concourse.PipelineStatus.show) statusSearchTerm pipelines
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
