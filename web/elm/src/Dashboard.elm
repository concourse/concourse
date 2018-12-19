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
import Dashboard.APIData as APIData
import Dashboard.Details as Details
import Dashboard.Footer as Footer
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


type alias Model =
    { csrfToken : String
    , state : Result DashboardError SubState.SubState
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
    , searchBar : SearchBar
    , hideFooter : Bool
    , hideFooterCounter : Int
    , showHelp : Bool
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
          , hoveredTopCliIcon = Nothing
          , screenSize = ScreenSize.Desktop
          , version = ""
          , userState = UserState.UserStateUnknown
          , userMenuVisible = False
          , hideFooter = False
          , hideFooterCounter = 0
          , showHelp = False
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
            ( let
                newModel =
                    Footer.tick model
              in
                case model.state of
                    Ok substate ->
                        { newModel | state = Ok (SubState.tick now substate) }

                    _ ->
                        newModel
            , []
            )

        AutoRefresh _ ->
            ( model
            , [ FetchData ]
            )

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

        TopCliHover state ->
            ( { model | hoveredTopCliIcon = state }, [] )

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
            ( { model | state = Err NotAsked }, [ SendLogOutRequest ] )

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
                , [ NewUrl redirectUrl, FetchData ]
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

                Ok substate ->
                    [ Html.div
                        [ class "dashboard-content" ]
                      <|
                        (if List.isEmpty (model.groups |> List.concatMap .pipelines) then
                            [ noPipelinesCard model
                            ]
                         else
                            []
                        )
                            ++ (pipelinesView
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
                        ++ (List.map Html.fromUnstyled <| Footer.view model)
    in
        Html.div
            [ classList
                [ ( .pageBodyClass Group.stickyHeaderConfig, True )
                , ( "dashboard-hd", model.highDensity )
                ]
            ]
            mainContent


noPipelinesCard : { a | hoveredTopCliIcon : Maybe Cli.Cli } -> Html Msg
noPipelinesCard { hoveredTopCliIcon } =
    let
        cliIcon : Cli.Cli -> Maybe Cli.Cli -> Html Msg
        cliIcon cli hoveredTopCliIcon =
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
                    , style <|
                        Styles.topCliIcon
                            { hovered = hoveredTopCliIcon == Just cli
                            , image = icon ++ "_logo.svg"
                            }
                    , id <| "top-cli-" ++ cliName
                    , onMouseEnter <| TopCliHover <| Just cli
                    , onMouseLeave <| TopCliHover Nothing
                    ]
                    []
    in
        Html.div
            [ id "no-pipelines-card"
            , style Styles.noPipelinesCard
            ]
            [ Html.div
                [ style Styles.noPipelinesCardTitle ]
                [ Html.text "welcome to concourse!" ]
            , Html.div
                [ style Styles.noPipelinesCardBody ]
                [ Html.div
                    [ style
                        [ ( "display", "flex" )
                        , ( "align-items", "center" )
                        ]
                    ]
                    [ Html.div
                        [ style [ ( "margin-right", "10px" ) ] ]
                        [ Html.text "first, download the CLI tools:" ]
                    , cliIcon Cli.OSX hoveredTopCliIcon
                    , cliIcon Cli.Windows hoveredTopCliIcon
                    , cliIcon Cli.Linux hoveredTopCliIcon
                    ]
                , Html.div
                    []
                    [ Html.text "then, use `fly set-pipeline` to set up your new pipeline" ]
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
            ( model, [ FocusSearchInput ] )

        '?' ->
            ( Footer.toggleHelp model, [] )

        _ ->
            ( Footer.showFooter model, [] )


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
