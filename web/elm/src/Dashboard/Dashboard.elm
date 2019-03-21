module Dashboard.Dashboard exposing
    ( handleCallback
    , handleDelivery
    , init
    , searchInputId
    , subscriptions
    , update
    , view
    )

import Array
import Concourse.Cli as Cli
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Dashboard.Details as Details
import Dashboard.Footer as Footer
import Dashboard.Group as Group
import Dashboard.Group.Models exposing (Group, Pipeline)
import Dashboard.Models as Models
    exposing
        ( DashboardError(..)
        , Model
        , SubState
        )
import Dashboard.Styles as Styles
import Dashboard.Text as Text
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , classList
        , draggable
        , href
        , id
        , placeholder
        , src
        , style
        , value
        )
import Html.Events
    exposing
        ( onBlur
        , onClick
        , onFocus
        , onInput
        , onMouseDown
        , onMouseEnter
        , onMouseLeave
        )
import Http
import Keycodes
import List.Extra
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message as Message exposing (Hoverable(..), Message(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Monocle.Common exposing ((<|>), (=>))
import Monocle.Lens
import Monocle.Optional
import MonocleHelpers exposing (..)
import Regex exposing (HowMany(All), regex, replace)
import RemoteData
import Routes
import ScreenSize exposing (ScreenSize)
import Simple.Fuzzy exposing (filter, match, root)
import TopBar.Model exposing (Dropdown(..))
import TopBar.Styles
import TopBar.TopBar as TopBar
import UserState exposing (UserState)
import Views.Login as Login


searchInputId : String
searchInputId =
    "search-input-field"


type alias Flags =
    { turbulencePath : String
    , searchType : Routes.SearchType
    , pipelineRunningKeyframes : String
    }


substateOptional : Monocle.Optional.Optional Model SubState
substateOptional =
    Monocle.Optional.Optional (.state >> RemoteData.toMaybe) (\s m -> { m | state = RemoteData.Success s })


init : Flags -> ( Model, List Effect )
init flags =
    let
        ( topBar, topBarEffects ) =
            TopBar.init
    in
    ( { state = RemoteData.NotAsked
      , turbulencePath = flags.turbulencePath
      , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
      , groups = []
      , version = ""
      , hovered = Nothing
      , userState = UserState.UserStateUnknown
      , hideFooter = False
      , hideFooterCounter = 0
      , showHelp = False
      , highDensity = flags.searchType == Routes.HighDensity
      , query = Routes.extractQuery flags.searchType
      , isUserMenuExpanded = topBar.isUserMenuExpanded
      , dropdown = topBar.dropdown
      , screenSize = topBar.screenSize
      , shiftDown = topBar.shiftDown
      }
    , [ FetchData
      , PinTeamNames Group.stickyHeaderConfig
      , SetTitle <| "Dashboard" ++ " - "
      , GetScreenSize
      ]
        ++ topBarEffects
    )


handleCallback : Callback -> ET Model
handleCallback msg =
    TopBar.handleCallback msg >> handleCallbackBody msg


handleCallbackBody : Callback -> ET Model
handleCallbackBody msg ( model, effects ) =
    case msg of
        APIDataFetched (Err _) ->
            ( { model | state = RemoteData.Failure (Turbulence model.turbulencePath) }, effects )

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
                                    RemoteData.Success (Models.tick now substate)
                            }

                        _ ->
                            { model
                                | state =
                                    RemoteData.Success
                                        { now = now
                                        , dragState = Models.NotDragging
                                        , dropState = Models.NotDropping
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
                    | groups = groups
                    , highDensity = False
                    , version = apiData.version
                    , userState = userState
                  }
                , effects
                    ++ [ ModifyUrl <|
                            Routes.toString <|
                                Routes.dashboardRoute False
                       ]
                )

            else
                ( { newModel
                    | groups = groups
                    , version = apiData.version
                    , userState = userState
                  }
                , effects
                )

        LoggedOut (Ok ()) ->
            ( { model | userState = UserState.UserStateLoggedOut }
            , effects
                ++ [ NavigateTo <|
                        Routes.toString <|
                            Routes.dashboardRoute <|
                                model.highDensity
                   , FetchData
                   ]
            )

        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, effects )

        ScreenResized size ->
            let
                newSize =
                    ScreenSize.fromWindowSize size
            in
            ( { model | screenSize = newSize }, effects )

        PipelineToggled _ (Ok ()) ->
            ( model, effects ++ [ FetchData ] )

        PipelineToggled _ (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    ( model
                    , if status.code == 401 then
                        effects ++ [ RedirectToLogin ]

                      else
                        effects
                    )

                _ ->
                    ( model, effects )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery =
    TopBar.handleDelivery delivery
        >> Footer.handleDelivery delivery
        >> handleDeliveryBody delivery


handleDeliveryBody : Delivery -> ET Model
handleDeliveryBody delivery ( model, effects ) =
    case delivery of
        ClockTicked OneSecond time ->
            ( { model | state = RemoteData.map (Models.tick time) model.state }, effects )

        ClockTicked FiveSeconds _ ->
            ( model, effects ++ [ FetchData ] )

        KeyUp keyCode ->
            if keyCode == Keycodes.shift then
                ( { model | shiftDown = False }, effects )

            else
                ( model, effects )

        KeyDown keyCode ->
            if keyCode == Keycodes.shift then
                ( { model | shiftDown = True }, effects )

            else
                let
                    options =
                        dropdownOptions model
                in
                case keyCode of
                    -- up arrow
                    38 ->
                        ( { model
                            | dropdown =
                                arrowUp options model.dropdown
                          }
                        , effects
                        )

                    -- down arrow
                    40 ->
                        ( { model
                            | dropdown =
                                arrowDown options model.dropdown
                          }
                        , effects
                        )

                    -- enter key
                    13 ->
                        case model.dropdown of
                            Shown { selectedIdx } ->
                                case selectedIdx of
                                    Nothing ->
                                        ( model, effects )

                                    Just selectedIdx ->
                                        let
                                            options =
                                                Array.fromList (dropdownOptions model)

                                            selectedItem =
                                                Array.get selectedIdx options
                                                    |> Maybe.withDefault model.query
                                        in
                                        ( { model
                                            | dropdown = Shown { selectedIdx = Nothing }
                                            , query = selectedItem
                                          }
                                        , [ ModifyUrl <|
                                                Routes.toString <|
                                                    Routes.Dashboard (Routes.Normal (Just selectedItem))
                                          ]
                                        )

                            _ ->
                                ( model, effects )

                    -- escape key
                    27 ->
                        ( model, effects ++ [ Blur searchInputId ] )

                    -- '/'
                    191 ->
                        ( model
                        , if model.shiftDown then
                            effects

                          else
                            effects ++ [ Focus searchInputId ]
                        )

                    -- any other keycode
                    _ ->
                        ( model, effects )

        _ ->
            ( model, effects )


dropdownOptions : { a | query : String, groups : List Group } -> List String
dropdownOptions { query, groups } =
    case String.trim query of
        "" ->
            [ "status: ", "team: " ]

        "status:" ->
            [ "status: paused"
            , "status: pending"
            , "status: failed"
            , "status: errored"
            , "status: aborted"
            , "status: running"
            , "status: succeeded"
            ]

        "team:" ->
            groups
                |> List.take 10
                |> List.map (\group -> "team: " ++ group.teamName)

        _ ->
            []


arrowUp : List a -> Dropdown -> Dropdown
arrowUp options dropdown =
    case dropdown of
        Shown { selectedIdx } ->
            case selectedIdx of
                Nothing ->
                    let
                        lastItem =
                            List.length options - 1
                    in
                    Shown { selectedIdx = Just lastItem }

                Just selectedIdx ->
                    let
                        newSelection =
                            (selectedIdx - 1) % List.length options
                    in
                    Shown { selectedIdx = Just newSelection }

        Hidden ->
            Hidden


arrowDown : List a -> Dropdown -> Dropdown
arrowDown options dropdown =
    case dropdown of
        Shown { selectedIdx } ->
            case selectedIdx of
                Nothing ->
                    Shown { selectedIdx = Just 0 }

                Just selectedIdx ->
                    let
                        newSelection =
                            (selectedIdx + 1) % List.length options
                    in
                    Shown { selectedIdx = Just newSelection }

        Hidden ->
            Hidden


update : Message -> ET Model
update msg =
    TopBar.update msg >> updateBody msg


updateBody : Message -> ET Model
updateBody msg ( model, effects ) =
    case msg of
        FilterMsg query ->
            ( { model | query = query }
            , effects
                ++ [ Focus searchInputId
                   , ModifyUrl <|
                        Routes.toString <|
                            Routes.Dashboard (Routes.Normal (Just query))
                   ]
            )

        ShowSearchInput ->
            showSearchInput ( model, effects )

        DragStart teamName index ->
            let
                newModel =
                    { model | state = RemoteData.map (\s -> { s | dragState = Models.Dragging teamName index }) model.state }
            in
            ( newModel, effects )

        DragOver teamName index ->
            let
                newModel =
                    { model | state = RemoteData.map (\s -> { s | dropState = Models.Dropping index }) model.state }
            in
            ( newModel, effects )

        TooltipHd pipelineName teamName ->
            ( model, effects ++ [ ShowTooltipHd ( pipelineName, teamName ) ] )

        Tooltip pipelineName teamName ->
            ( model, effects ++ [ ShowTooltip ( pipelineName, teamName ) ] )

        DragEnd ->
            let
                updatePipelines :
                    ( Group.PipelineIndex, Group.PipelineIndex )
                    -> Group
                    -> ( Group, List Effect )
                updatePipelines ( dragIndex, dropIndex ) group =
                    let
                        newGroup =
                            Group.shiftPipelines dragIndex dropIndex group
                    in
                    ( newGroup
                    , [ SendOrderPipelinesRequest newGroup.teamName newGroup.pipelines ]
                    )

                dragDropOptional : Monocle.Optional.Optional Model ( Models.DragState, Models.DropState )
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

                groupsLens : Monocle.Lens.Lens Model (List Group)
                groupsLens =
                    Monocle.Lens.Lens .groups (\b a -> { a | groups = b })

                groupOptional : Monocle.Optional.Optional Model Group
                groupOptional =
                    (substateOptional
                        =|> Details.dragStateLens
                        => Group.teamNameOptional
                    )
                        >>= (\teamName ->
                                groupsLens
                                    <|= Group.findGroupOptional teamName
                            )

                bigOptional : Monocle.Optional.Optional Model ( ( Group.PipelineIndex, Group.PipelineIndex ), Group )
                bigOptional =
                    Monocle.Optional.tuple
                        dragDropIndexOptional
                        groupOptional

                ( newModel, unAccumulatedEffects ) =
                    model
                        |> modifyWithEffect bigOptional
                            (\( t, g ) ->
                                let
                                    ( newG, msg ) =
                                        updatePipelines t g
                                in
                                ( ( t, newG ), msg )
                            )
                        |> Tuple.mapFirst (dragDropOptional.set ( Models.NotDragging, Models.NotDropping ))
            in
            ( newModel, effects ++ unAccumulatedEffects )

        Hover hovered ->
            ( { model | hovered = hovered }, effects )

        LogOut ->
            ( { model | state = RemoteData.NotAsked }, effects )

        TogglePipelinePaused pipelineId isPaused ->
            let
                newGroups =
                    model.groups
                        |> List.Extra.updateIf
                            (.teamName >> (==) pipelineId.teamName)
                            (\g ->
                                let
                                    newPipelines =
                                        g.pipelines
                                            |> List.Extra.updateIf
                                                (.name >> (==) pipelineId.pipelineName)
                                                (\p -> { p | isToggleLoading = True })
                                in
                                { g | pipelines = newPipelines }
                            )
            in
            ( { model | groups = newGroups }
            , effects
                ++ [ SendTogglePipelineRequest pipelineId isPaused ]
            )

        _ ->
            ( model, effects )


showSearchInput : ET Model
showSearchInput ( model, effects ) =
    if model.highDensity then
        ( model, effects )

    else
        let
            isDropDownHidden =
                model.dropdown == TopBar.Model.Hidden

            isMobile =
                model.screenSize == ScreenSize.Mobile

            newModel =
                { model | dropdown = Shown { selectedIdx = Nothing } }
        in
        if isDropDownHidden && isMobile && model.query == "" then
            ( newModel, effects ++ [ Focus searchInputId ] )

        else
            ( model, effects )


subscriptions : Model -> List Subscription
subscriptions model =
    [ OnClockTick OneSecond
    , OnClockTick FiveSeconds
    , OnMouse
    , OnKeyDown
    , OnKeyUp
    , OnWindowResize
    ]


view : UserState -> Model -> Html Message
view userState model =
    Html.div
        [ style TopBar.Styles.pageIncludingTopBar
        , id "page-including-top-bar"
        ]
        [ Html.div
            [ id "top-bar-app"
            , style <| TopBar.Styles.topBar False
            ]
          <|
            [ TopBar.viewConcourseLogo ]
                ++ (case model.highDensity of
                        False ->
                            let
                                isDropDownHidden =
                                    model.dropdown == TopBar.Model.Hidden

                                isMobile =
                                    model.screenSize == ScreenSize.Mobile

                                noPipelines =
                                    model.groups
                                        |> List.concatMap .pipelines
                                        |> List.isEmpty
                            in
                            if noPipelines then
                                [ Login.view userState model False ]

                            else if isDropDownHidden && isMobile && model.query == "" then
                                [ Html.div
                                    [ style <|
                                        Styles.showSearchContainer model
                                    ]
                                    [ Html.a
                                        [ id "show-search-button"
                                        , onClick ShowSearchInput
                                        , style TopBar.Styles.searchButton
                                        ]
                                        []
                                    ]
                                , Login.view userState model False
                                ]

                            else if isMobile then
                                [ viewSearch model ]

                            else
                                [ viewSearch model
                                , Login.view userState model False
                                ]

                        _ ->
                            [ Login.view userState model False ]
                   )
        , Html.div
            [ id "page-below-top-bar", style TopBar.Styles.pageBelowTopBar ]
            (dashboardView model)
        ]


dashboardView : Model -> List (Html Message)
dashboardView model =
    case model.state of
        RemoteData.NotAsked ->
            [ Html.text "" ]

        RemoteData.Loading ->
            [ Html.text "" ]

        RemoteData.Failure (Turbulence path) ->
            [ turbulenceView path ]

        RemoteData.Success substate ->
            [ Html.div
                [ class <| .pageBodyClass Group.stickyHeaderConfig
                , style <| Styles.content model.highDensity
                ]
              <|
                welcomeCard model
                    :: pipelinesView
                        { groups = model.groups
                        , substate = substate
                        , query = model.query
                        , hovered = model.hovered
                        , pipelineRunningKeyframes =
                            model.pipelineRunningKeyframes
                        , userState = model.userState
                        , highDensity = model.highDensity
                        }
            , Footer.view model
            ]


viewSearch :
    { a
        | screenSize : ScreenSize
        , query : String
        , dropdown : Dropdown
        , groups : List Group
    }
    -> Html Message
viewSearch ({ screenSize, query } as params) =
    Html.div
        [ id "search-container"
        , style (Styles.searchContainer screenSize)
        ]
        ([ Html.input
            [ id searchInputId
            , style (Styles.searchInput screenSize)
            , placeholder "search"
            , attribute "autocomplete" "off"
            , value query
            , onFocus FocusMsg
            , onBlur BlurMsg
            , onInput FilterMsg
            ]
            []
         , Html.div
            [ id "search-clear"
            , onClick (FilterMsg "")
            , style (Styles.searchClearButton (String.length query > 0))
            ]
            []
         ]
            ++ viewDropdownItems params
        )


viewDropdownItems :
    { a
        | query : String
        , dropdown : Dropdown
        , groups : List Group
        , screenSize : ScreenSize
    }
    -> List (Html Message)
viewDropdownItems ({ dropdown, screenSize } as model) =
    case dropdown of
        Hidden ->
            []

        Shown { selectedIdx } ->
            let
                dropdownItem : Int -> String -> Html Message
                dropdownItem idx text =
                    Html.li
                        [ onMouseDown (FilterMsg text)
                        , style (Styles.dropdownItem (Just idx == selectedIdx))
                        ]
                        [ Html.text text ]
            in
            [ Html.ul
                [ id "search-dropdown"
                , style (Styles.dropdownContainer screenSize)
                ]
                (List.indexedMap dropdownItem (dropdownOptions model))
            ]


welcomeCard :
    { a
        | hovered : Maybe Hoverable
        , groups : List Group
        , userState : UserState.UserState
    }
    -> Html Message
welcomeCard { hovered, groups, userState } =
    let
        noPipelines =
            List.isEmpty (groups |> List.concatMap .pipelines)

        cliIcon : Maybe Hoverable -> Cli.Cli -> Html Message
        cliIcon hovered cli =
            Html.a
                [ href (Cli.downloadUrl cli)
                , attribute "aria-label" <| Cli.label cli
                , style <|
                    Styles.topCliIcon
                        { hovered =
                            hovered
                                == (Just <| Message.WelcomeCardCliIcon cli)
                        , cli = cli
                        }
                , id <| "top-cli-" ++ Cli.id cli
                , onMouseEnter <| Hover <| Just <| Message.WelcomeCardCliIcon cli
                , onMouseLeave <| Hover Nothing
                ]
                []
    in
    if noPipelines then
        Html.div
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
                        ++ List.map (cliIcon hovered) Cli.clis
                , Html.div
                    []
                    [ Html.text Text.setPipelineInstructions ]
                ]
                    ++ loginInstruction userState
            , Html.pre
                [ style Styles.asciiArt ]
                [ Html.text Text.asciiArt ]
            ]

    else
        Html.text ""


loginInstruction : UserState.UserState -> List (Html Message)
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


noResultsView : String -> Html Message
noResultsView query =
    let
        boldedQuery =
            Html.span [ class "monospace-bold" ] [ Html.text query ]
    in
    Html.div
        [ class "no-results"
        , style Styles.noResults
        ]
        [ Html.text "No results for "
        , boldedQuery
        , Html.text " matched your search."
        ]


turbulenceView : String -> Html Message
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
    { groups : List Group
    , substate : Models.SubState
    , hovered : Maybe Message.Hoverable
    , pipelineRunningKeyframes : String
    , query : String
    , userState : UserState.UserState
    , highDensity : Bool
    }
    -> List (Html Message)
pipelinesView { groups, substate, hovered, pipelineRunningKeyframes, query, userState, highDensity } =
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
                    |> List.concatMap (Group.hdView pipelineRunningKeyframes)

            else
                groupsToDisplay
                    |> List.map
                        (Group.view
                            { dragState = substate.dragState
                            , dropState = substate.dropState
                            , now = substate.now
                            , hovered = hovered
                            , pipelineRunningKeyframes = pipelineRunningKeyframes
                            , userState = userState
                            }
                        )
    in
    if List.isEmpty groupViews && not (String.isEmpty query) then
        [ noResultsView (toString query) ]

    else
        groupViews


filterTerms : String -> List String
filterTerms =
    replace All (regex "team:\\s*") (\_ -> "team:")
        >> replace All (regex "status:\\s*") (\_ -> "status:")
        >> String.words
        >> List.filter (not << String.isEmpty)


filter : String -> List Group -> List Group
filter =
    filterTerms >> flip (List.foldl filterGroupsByTerm)


filterPipelinesByTerm : String -> Group -> Group
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


filterGroupsByTerm : String -> List Group -> List Group
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
