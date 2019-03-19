module Dashboard.Dashboard exposing
    ( handleCallback
    , handleDelivery
    , init
    , subscriptions
    , update
    , view
    )

import Char
import Concourse.Cli as Cli
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Dashboard.Details as Details
import Dashboard.Footer as Footer
import Dashboard.Group as Group
import Dashboard.Models as Models
    exposing
        ( DashboardError(..)
        , Model
        , Pipeline
        , SubState
        )
import Dashboard.Styles as Styles
import Dashboard.Text as Text
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , classList
        , draggable
        , href
        , id
        , src
        , style
        )
import Html.Events exposing (onMouseEnter, onMouseLeave)
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
import ScreenSize
import Simple.Fuzzy exposing (filter, match, root)
import TopBar.Model
import TopBar.Styles
import TopBar.TopBar as TopBar
import UserState exposing (UserState)


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
            TopBar.init { route = Routes.Dashboard { searchType = flags.searchType } }
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
      , isUserMenuExpanded = topBar.isUserMenuExpanded
      , isPinMenuExpanded = topBar.isPinMenuExpanded
      , middleSection = topBar.middleSection
      , teams = topBar.teams
      , screenSize = topBar.screenSize
      , highDensity = topBar.highDensity
      }
    , [ FetchData
      , PinTeamNames Group.stickyHeaderConfig
      , SetTitle <| "Dashboard" ++ " - "
      , GetScreenSize
      ]
        ++ topBarEffects
    )


handleCallback : Callback -> ( Model, List Effect ) -> ( Model, List Effect )
handleCallback msg =
    TopBar.handleCallback msg >> handleCallbackWithoutTopBar msg


handleCallbackWithoutTopBar : Callback -> ( Model, List Effect ) -> ( Model, List Effect )
handleCallbackWithoutTopBar msg ( model, effects ) =
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
                    | highDensity = False
                    , groups = groups
                    , version = apiData.version
                    , userState = userState
                  }
                , effects ++ [ ModifyUrl <| Routes.toString <| Routes.dashboardRoute False ]
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
                ++ [ NavigateTo <| Routes.toString <| Routes.dashboardRoute model.highDensity
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

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ( Model, List Effect ) -> ( Model, List Effect )
handleDelivery delivery =
    TopBar.handleDelivery delivery >> handleDeliveryWithoutTopBar delivery


handleDeliveryWithoutTopBar : Delivery -> ( Model, List Effect ) -> ( Model, List Effect )
handleDeliveryWithoutTopBar delivery ( model, effects ) =
    case delivery of
        KeyDown keycode ->
            handleKeyPressed (Char.fromCode keycode) ( model, effects )

        Moused ->
            ( Footer.showFooter model, effects )

        ClockTicked OneSecond time ->
            ( let
                newModel =
                    Footer.tick model
              in
              { newModel | state = RemoteData.map (Models.tick time) newModel.state }
            , effects
            )

        ClockTicked FiveSeconds _ ->
            ( model, effects ++ [ FetchData ] )

        _ ->
            ( model, effects )


update : Message -> ( Model, List Effect ) -> ( Model, List Effect )
update msg =
    TopBar.update msg >> updateBody msg


updateBody : Message -> ( Model, List Effect ) -> ( Model, List Effect )
updateBody msg ( model, effects ) =
    case msg of
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
                    -> Models.Group
                    -> ( Models.Group, List Effect )
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

                groupsLens : Monocle.Lens.Lens Model (List Models.Group)
                groupsLens =
                    Monocle.Lens.Lens .groups (\b a -> { a | groups = b })

                groupOptional : Monocle.Optional.Optional Model Models.Group
                groupOptional =
                    (substateOptional
                        =|> Details.dragStateLens
                        => Group.teamNameOptional
                    )
                        >>= (\teamName ->
                                groupsLens
                                    <|= Group.findGroupOptional teamName
                            )

                bigOptional : Monocle.Optional.Optional Model ( ( Group.PipelineIndex, Group.PipelineIndex ), Models.Group )
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

        _ ->
            ( model, effects )


subscriptions : Model -> List Subscription
subscriptions model =
    [ OnClockTick OneSecond
    , OnClockTick FiveSeconds
    , OnMouse
    , OnKeyDown
    , OnWindowResize
    ]


view : UserState -> Model -> Html Message
view userState model =
    Html.div []
        [ Html.div
            [ style TopBar.Styles.pageIncludingTopBar, id "page-including-top-bar" ]
            [ TopBar.view userState TopBar.Model.None model
            , Html.div [ id "page-below-top-bar", style TopBar.Styles.pageBelowTopBar ]
                [ dashboardView model
                ]
            ]
        ]


dashboardView : Model -> Html Message
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
                                , query = TopBar.query model
                                , hovered = model.hovered
                                , pipelineRunningKeyframes =
                                    model.pipelineRunningKeyframes
                                , userState = model.userState
                                , highDensity = model.highDensity
                                }
                    ]
                        ++ Footer.view model
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
        | hovered : Maybe Hoverable
        , groups : List Models.Group
        , userState : UserState.UserState
    }
    -> List (Html Message)
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
        ]

    else
        []


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


helpView : { a | showHelp : Bool } -> Html Message
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
    { groups : List Models.Group
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
                    |> List.map (Group.hdView pipelineRunningKeyframes)

            else
                groupsToDisplay
                    |> List.map
                        (Group.view
                            { dragState = substate.dragState
                            , dropState = substate.dropState
                            , now = substate.now
                            , hovered = hovered
                            , pipelineRunningKeyframes = pipelineRunningKeyframes
                            }
                        )
    in
    if List.isEmpty groupViews && not (String.isEmpty query) then
        [ noResultsView (toString query) ]

    else
        groupViews


handleKeyPressed :
    Char
    -> ( Models.FooterModel r, List Effect )
    -> ( Models.FooterModel r, List Effect )
handleKeyPressed key ( model, effects ) =
    case key of
        '/' ->
            ( model, effects )

        '?' ->
            ( Footer.toggleHelp model, effects )

        _ ->
            ( Footer.showFooter model, effects )


filterTerms : String -> List String
filterTerms =
    replace All (regex "team:\\s*") (\_ -> "team:")
        >> replace All (regex "status:\\s*") (\_ -> "status:")
        >> String.words
        >> List.filter (not << String.isEmpty)


filter : String -> List Models.Group -> List Models.Group
filter =
    filterTerms >> flip (List.foldl filterGroupsByTerm)


filterPipelinesByTerm : String -> Models.Group -> Models.Group
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


filterGroupsByTerm : String -> List Models.Group -> List Models.Group
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
