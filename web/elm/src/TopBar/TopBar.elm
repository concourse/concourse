module TopBar.TopBar exposing
    ( Flags
    , handleCallback
    , handleDelivery
    , init
    , query
    , queryStringFromSearch
    , update
    , view
    )

import Array
import Concourse
import Dict
import Html exposing (Html)
import Html.Attributes as HA
    exposing
        ( attribute
        , class
        , href
        , id
        , placeholder
        , src
        , style
        , type_
        , value
        )
import Html.Events exposing (..)
import Http
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Message(..))
import Message.Subscription exposing (Delivery(..))
import QueryString
import RemoteData exposing (RemoteData)
import Routes
import ScreenSize exposing (ScreenSize(..))
import TopBar.Model
    exposing
        ( Dropdown(..)
        , MiddleSection(..)
        , Model
        , PipelineState(..)
        , isPaused
        )
import TopBar.Styles as Styles
import UserState exposing (UserState(..))
import Window


type alias Flags =
    { route : Routes.Route }


query : Model r -> String
query model =
    case model.middleSection of
        SearchBar { query } ->
            query

        _ ->
            ""


init : Flags -> ( Model {}, List Effect )
init { route } =
    let
        isHd =
            route == Routes.Dashboard { searchType = Routes.HighDensity }

        middleSection =
            case route of
                Routes.Dashboard { searchType } ->
                    case searchType of
                        Routes.Normal search ->
                            SearchBar { query = Maybe.withDefault "" search, dropdown = Hidden }

                        Routes.HighDensity ->
                            Empty

                _ ->
                    Breadcrumbs route
    in
    ( { isUserMenuExpanded = False
      , isPinMenuExpanded = False
      , middleSection = middleSection
      , teams = RemoteData.Loading
      , screenSize = Desktop
      , highDensity = isHd
      }
    , [ GetScreenSize ]
    )


queryStringFromSearch : String -> String
queryStringFromSearch query =
    case query of
        "" ->
            QueryString.render QueryString.empty

        query ->
            QueryString.render <|
                QueryString.add "search" query QueryString.empty


handleCallback : Callback -> ( Model r, List Effect ) -> ( Model r, List Effect )
handleCallback callback ( model, effects ) =
    case callback of
        LoggedOut (Ok ()) ->
            let
                redirectUrl =
                    Routes.dashboardRoute model.highDensity
            in
            ( { model
                | isUserMenuExpanded = False
                , teams = RemoteData.Loading
              }
            , effects ++ [ NavigateTo <| Routes.toString redirectUrl ]
            )

        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, effects )

        APIDataFetched (Ok ( time, data )) ->
            ( { model
                | teams = RemoteData.Success data.teams
                , middleSection =
                    if data.pipelines == [] then
                        Empty

                    else
                        model.middleSection
              }
            , effects
            )

        APIDataFetched (Err err) ->
            ( { model | teams = RemoteData.Failure err, middleSection = Empty }, effects )

        ScreenResized size ->
            ( screenResize size model, effects )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ( Model r, List Effect ) -> ( Model r, List Effect )
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keyCode ->
            case keyCode of
                -- up arrow
                38 ->
                    case model.middleSection of
                        SearchBar r ->
                            case r.dropdown of
                                Shown { selectedIdx } ->
                                    case selectedIdx of
                                        Nothing ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }

                                                lastItem =
                                                    List.length options - 1
                                            in
                                            ( { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Just lastItem } } }
                                            , effects
                                            )

                                        Just selectedIdx ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }

                                                newSelection =
                                                    (selectedIdx - 1) % List.length options
                                            in
                                            ( { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Just newSelection } } }
                                            , effects
                                            )

                                _ ->
                                    ( model, effects )

                        _ ->
                            ( model, effects )

                -- down arrow
                40 ->
                    case model.middleSection of
                        SearchBar r ->
                            case r.dropdown of
                                Shown { selectedIdx } ->
                                    case selectedIdx of
                                        Nothing ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }
                                            in
                                            ( { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Just 0 } } }
                                            , effects
                                            )

                                        Just selectedIdx ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }

                                                newSelection =
                                                    (selectedIdx + 1) % List.length options
                                            in
                                            ( { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Just newSelection } } }
                                            , effects
                                            )

                                _ ->
                                    ( model, effects )

                        _ ->
                            ( model, effects )

                -- enter key
                13 ->
                    case model.middleSection of
                        SearchBar r ->
                            case r.dropdown of
                                Shown { selectedIdx } ->
                                    case selectedIdx of
                                        Nothing ->
                                            ( model, effects )

                                        Just selectedIdx ->
                                            let
                                                options =
                                                    Array.fromList (dropdownOptions { query = r.query, teams = model.teams })

                                                selectedItem =
                                                    Maybe.withDefault r.query (Array.get selectedIdx options)
                                            in
                                            ( { model
                                                | middleSection =
                                                    SearchBar
                                                        { r
                                                            | dropdown = Shown { selectedIdx = Nothing }
                                                            , query = selectedItem
                                                        }
                                              }
                                            , effects
                                            )

                                _ ->
                                    ( model, effects )

                        _ ->
                            ( model, effects )

                -- escape key
                27 ->
                    case model.middleSection of
                        SearchBar r ->
                            case r.dropdown of
                                Shown { selectedIdx } ->
                                    case selectedIdx of
                                        Nothing ->
                                            let
                                                newModel =
                                                    case model.middleSection of
                                                        SearchBar r ->
                                                            if model.screenSize == Mobile && r.query == "" then
                                                                { model | middleSection = MinifiedSearch }

                                                            else
                                                                { model | middleSection = SearchBar { r | dropdown = Hidden } }

                                                        _ ->
                                                            model
                                            in
                                            ( newModel, effects )

                                        Just selectedIdx ->
                                            let
                                                newModel =
                                                    case model.middleSection of
                                                        SearchBar r ->
                                                            if model.screenSize == Mobile && r.query == "" then
                                                                { model | middleSection = MinifiedSearch }

                                                            else
                                                                { model | middleSection = SearchBar { r | dropdown = Hidden } }

                                                        _ ->
                                                            model
                                            in
                                            ( newModel, effects )

                                _ ->
                                    ( model, effects )

                        _ ->
                            ( model, effects )

                -- '/'
                191 ->
                    ( model, effects ++ [ ForceFocus "search-input-field" ] )

                -- any other keycode
                _ ->
                    case model.middleSection of
                        SearchBar r ->
                            ( { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Nothing } } }, effects )

                        _ ->
                            ( model, effects )

        WindowResized size ->
            ( screenResize size model, effects )

        _ ->
            ( model, effects )


update : Message -> ( Model r, List Effect ) -> ( Model r, List Effect )
update msg ( model, effects ) =
    case msg of
        FilterMsg query ->
            let
                newModel =
                    case model.middleSection of
                        SearchBar r ->
                            { model | middleSection = SearchBar { r | query = query } }

                        _ ->
                            model
            in
            ( newModel
            , effects
                ++ [ ForceFocus "search-input-field"
                   , ModifyUrl (queryStringFromSearch query)
                   ]
            )

        GoToRoute route ->
            ( model, effects ++ [ NavigateTo (Routes.toString route) ] )

        LogIn ->
            ( model, effects ++ [ RedirectToLogin ] )

        LogOut ->
            ( model, effects ++ [ SendLogOutRequest ] )

        ToggleUserMenu ->
            ( { model | isUserMenuExpanded = not model.isUserMenuExpanded }, effects )

        TogglePinIconDropdown ->
            ( { model | isPinMenuExpanded = not model.isPinMenuExpanded }, effects )

        TogglePipelinePaused pipelineIdentifier isPaused ->
            ( model, effects ++ [ SendTogglePipelineRequest pipelineIdentifier isPaused ] )

        FocusMsg ->
            let
                newModel =
                    case model.middleSection of
                        SearchBar r ->
                            { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Nothing } } }

                        _ ->
                            model
            in
            ( newModel, effects )

        BlurMsg ->
            let
                newModel =
                    case model.middleSection of
                        SearchBar r ->
                            if model.screenSize == Mobile && r.query == "" then
                                { model | middleSection = MinifiedSearch }

                            else
                                { model | middleSection = SearchBar { r | dropdown = Hidden } }

                        _ ->
                            model
            in
            ( newModel, effects )

        ShowSearchInput ->
            showSearchInput model

        _ ->
            ( model, effects )


screenResize : Window.Size -> Model r -> Model r
screenResize size model =
    let
        newSize =
            ScreenSize.fromWindowSize size

        newMiddleSection =
            case model.middleSection of
                Breadcrumbs r ->
                    Breadcrumbs r

                Empty ->
                    Empty

                SearchBar q ->
                    if String.isEmpty q.query && newSize == Mobile && model.screenSize /= Mobile then
                        MinifiedSearch

                    else
                        SearchBar q

                MinifiedSearch ->
                    case newSize of
                        ScreenSize.Desktop ->
                            SearchBar { query = "", dropdown = Hidden }

                        ScreenSize.BigDesktop ->
                            SearchBar { query = "", dropdown = Hidden }

                        ScreenSize.Mobile ->
                            MinifiedSearch
    in
    { model | screenSize = newSize, middleSection = newMiddleSection }


showSearchInput : Model r -> ( Model r, List Effect )
showSearchInput model =
    let
        newModel =
            { model | middleSection = SearchBar { query = "", dropdown = Hidden } }
    in
    case model.middleSection of
        MinifiedSearch ->
            ( newModel, [ ForceFocus "search-input-field" ] )

        SearchBar _ ->
            ( model, [] )

        Empty ->
            Debug.log "attempting to show search input when search is gone" ( model, [] )

        Breadcrumbs _ ->
            Debug.log "attempting to show search input on a breadcrumbs page" ( model, [] )


view : UserState -> PipelineState -> Model r -> Html Message
view userState pipelineState model =
    Html.div
        [ id "top-bar-app"
        , style <| Styles.topBar <| isPaused pipelineState
        ]
        (viewConcourseLogo
            ++ viewMiddleSection model
            ++ viewPin pipelineState model
            ++ viewPauseToggle pipelineState
            ++ viewLogin userState model (isPaused pipelineState)
        )


viewPauseToggle : PipelineState -> List (Html Message)
viewPauseToggle pipelineState =
    case pipelineState of
        HasPipeline { isPaused, pipeline } ->
            [ Html.a
                [ id "top-bar-pause-pipeline"
                , style (Styles.pausePipelineButton isPaused)
                , onClick <| TogglePipelinePaused pipeline isPaused
                ]
                []
            ]

        _ ->
            []


viewLogin : UserState -> Model r -> Bool -> List (Html Message)
viewLogin userState model isPaused =
    if showLogin model then
        [ Html.div [ id "login-component", style Styles.loginComponent ] <|
            viewLoginState userState model.isUserMenuExpanded isPaused
        ]

    else
        []


showLogin : Model r -> Bool
showLogin model =
    case model.middleSection of
        SearchBar _ ->
            model.screenSize /= Mobile

        _ ->
            True


viewLoginState : UserState -> Bool -> Bool -> List (Html Message)
viewLoginState userState isUserMenuExpanded isPaused =
    case userState of
        UserStateUnknown ->
            []

        UserStateLoggedOut ->
            [ Html.div
                [ href "/sky/login"
                , HA.attribute "aria-label" "Log In"
                , id "login-container"
                , onClick LogIn
                , style (Styles.loginContainer isPaused)
                ]
                [ Html.div [ style Styles.loginItem, id "login-item" ] [ Html.a [ href "/sky/login" ] [ Html.text "login" ] ] ]
            ]

        UserStateLoggedIn user ->
            [ Html.div
                [ id "login-container"
                , onClick ToggleUserMenu
                , style (Styles.loginContainer isPaused)
                ]
                [ Html.div [ id "user-id", style Styles.loginItem ]
                    ([ Html.div [ style Styles.loginText ] [ Html.text (userDisplayName user) ] ]
                        ++ (if isUserMenuExpanded then
                                [ Html.div [ id "logout-button", style Styles.logoutButton, onClick LogOut ] [ Html.text "logout" ] ]

                            else
                                []
                           )
                    )
                ]
            ]


userDisplayName : Concourse.User -> String
userDisplayName user =
    Maybe.withDefault user.id <|
        List.head <|
            List.filter (not << String.isEmpty) [ user.userName, user.name, user.email ]


viewMiddleSection : Model r -> List (Html Message)
viewMiddleSection model =
    case model.middleSection of
        Empty ->
            []

        MinifiedSearch ->
            [ Html.div [ style <| Styles.showSearchContainer model ]
                [ Html.a
                    [ id "show-search-button"
                    , onClick ShowSearchInput
                    , style Styles.searchButton
                    ]
                    []
                ]
            ]

        SearchBar r ->
            viewSearch r model

        Breadcrumbs r ->
            [ Html.div [ id "breadcrumbs", style Styles.breadcrumbContainer ] (viewBreadcrumbs r) ]


viewSearch : { query : String, dropdown : Dropdown } -> Model r -> List (Html Message)
viewSearch r model =
    [ Html.div
        [ id "search-container"
        , style (Styles.searchContainer model.screenSize)
        ]
        ([ Html.input
            [ id "search-input-field"
            , style (Styles.searchInput model.screenSize)
            , placeholder "search"
            , value r.query
            , onFocus FocusMsg
            , onBlur BlurMsg
            , onInput FilterMsg
            ]
            []
         , Html.div
            [ id "search-clear"
            , onClick (FilterMsg "")
            , style (Styles.searchClearButton (String.length r.query > 0))
            ]
            []
         ]
            ++ viewDropdownItems r model
        )
    ]


viewDropdownItems : { query : String, dropdown : Dropdown } -> Model r -> List (Html Message)
viewDropdownItems { query, dropdown } model =
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

                itemList : List String
                itemList =
                    case String.trim query of
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
                            model.teams
                                |> RemoteData.withDefault []
                                |> List.take 10
                                |> List.map (\t -> "team: " ++ t.name)

                        "" ->
                            [ "status:", "team:" ]

                        _ ->
                            []
            in
            [ Html.ul
                [ id "search-dropdown"
                , style (Styles.dropdownContainer model.screenSize)
                ]
                (List.indexedMap dropdownItem itemList)
            ]


viewConcourseLogo : List (Html Message)
viewConcourseLogo =
    [ Html.a
        [ style Styles.concourseLogo, href "/" ]
        []
    ]


viewBreadcrumbs : Routes.Route -> List (Html Message)
viewBreadcrumbs route =
    case route of
        Routes.Pipeline { id } ->
            [ viewPipelineBreadcrumb { teamName = id.teamName, pipelineName = id.pipelineName } ]

        Routes.Build { id } ->
            [ viewPipelineBreadcrumb { teamName = id.teamName, pipelineName = id.pipelineName }
            , viewBreadcrumbSeparator
            , viewJobBreadcrumb id.jobName
            ]

        Routes.Resource { id } ->
            [ viewPipelineBreadcrumb { teamName = id.teamName, pipelineName = id.pipelineName }
            , viewBreadcrumbSeparator
            , viewResourceBreadcrumb id.resourceName
            ]

        Routes.Job { id } ->
            [ viewPipelineBreadcrumb { teamName = id.teamName, pipelineName = id.pipelineName }
            , viewBreadcrumbSeparator
            , viewJobBreadcrumb id.jobName
            ]

        _ ->
            []


breadcrumbComponent : String -> String -> List (Html Message)
breadcrumbComponent componentType name =
    [ Html.div
        [ style (Styles.breadcrumbComponent componentType) ]
        []
    , Html.text <| decodeName name
    ]


viewBreadcrumbSeparator : Html Message
viewBreadcrumbSeparator =
    Html.li
        [ class "breadcrumb-separator", style <| Styles.breadcrumbItem False ]
        [ Html.text "/" ]


viewPipelineBreadcrumb : Concourse.PipelineIdentifier -> Html Message
viewPipelineBreadcrumb pipelineId =
    Html.li
        [ id "breadcrumb-pipeline"
        , style <| Styles.breadcrumbItem True
        , onClick <| GoToRoute <| Routes.Pipeline { id = pipelineId, groups = [] }
        ]
        (breadcrumbComponent "pipeline" pipelineId.pipelineName)


viewJobBreadcrumb : String -> Html Message
viewJobBreadcrumb jobName =
    Html.li
        [ id "breadcrumb-job"
        , style <| Styles.breadcrumbItem False
        ]
        (breadcrumbComponent "job" jobName)


viewResourceBreadcrumb : String -> Html Message
viewResourceBreadcrumb resourceName =
    Html.li
        [ id "breadcrumb-resource"
        , style <| Styles.breadcrumbItem False
        ]
        (breadcrumbComponent "resource" resourceName)


decodeName : String -> String
decodeName name =
    Maybe.withDefault name (Http.decodeUri name)


dropdownOptions : { a | query : String, teams : RemoteData.WebData (List Concourse.Team) } -> List String
dropdownOptions { query, teams } =
    case String.trim query of
        "" ->
            [ "status: ", "team: " ]

        "status:" ->
            [ "status: paused", "status: pending", "status: failed", "status: errored", "status: aborted", "status: running", "status: succeeded" ]

        "team:" ->
            case teams of
                RemoteData.Success ts ->
                    List.map (\team -> "team: " ++ team.name) <| List.take 10 ts

                _ ->
                    []

        _ ->
            []


viewPin : PipelineState -> Model r -> List (Html Message)
viewPin pipelineState model =
    case pipelineState of
        HasPipeline { pinnedResources, pipeline } ->
            [ Html.div
                [ style <| Styles.pinIconContainer model.isPinMenuExpanded
                , id "pin-icon"
                ]
                [ if List.length pinnedResources > 0 then
                    Html.div
                        [ style <| Styles.pinIcon
                        , onMouseEnter TogglePinIconDropdown
                        , onMouseLeave TogglePinIconDropdown
                        ]
                        ([ Html.div
                            [ style Styles.pinBadge
                            , id "pin-badge"
                            ]
                            [ Html.div [] [ Html.text <| toString <| List.length pinnedResources ]
                            ]
                         ]
                            ++ viewPinDropdown pinnedResources pipeline model
                        )

                  else
                    Html.div [ style <| Styles.pinIcon ] []
                ]
            ]

        None ->
            []


viewPinDropdown : List ( String, Concourse.Version ) -> Concourse.PipelineIdentifier -> Model r -> List (Html Message)
viewPinDropdown pinnedResources pipeline model =
    if model.isPinMenuExpanded then
        [ Html.ul
            [ style Styles.pinIconDropdown ]
            (pinnedResources
                |> List.map
                    (\( resourceName, pinnedVersion ) ->
                        Html.li
                            [ onClick <|
                                GoToRoute <|
                                    Routes.Resource
                                        { id =
                                            { teamName = pipeline.teamName
                                            , pipelineName = pipeline.pipelineName
                                            , resourceName = resourceName
                                            }
                                        , page = Nothing
                                        }
                            , style Styles.pinDropdownCursor
                            ]
                            [ Html.div
                                [ style Styles.pinText ]
                                [ Html.text resourceName ]
                            , Html.table []
                                (pinnedVersion
                                    |> Dict.toList
                                    |> List.map
                                        (\( k, v ) ->
                                            Html.tr []
                                                [ Html.td [] [ Html.text k ]
                                                , Html.td [] [ Html.text v ]
                                                ]
                                        )
                                )
                            ]
                    )
            )
        , Html.div [ style Styles.pinHoverHighlight ] []
        ]

    else
        []
