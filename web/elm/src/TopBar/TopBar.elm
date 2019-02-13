module TopBar.TopBar exposing
    ( Flags
    , handleCallback
    , init
    , query
    , queryStringFromSearch
    , update
    , view
    )

import Array
import Callback exposing (Callback(..))
import Char
import Concourse
import Dict
import Effects exposing (Effect(..))
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes as HA
    exposing
        ( attribute
        , class
        , css
        , href
        , id
        , placeholder
        , src
        , style
        , type_
        , value
        )
import Html.Styled.Events exposing (..)
import Http
import TopBar.Model
    exposing
        ( Dropdown(..)
        , MiddleSection(..)
        , Model
        , PipelineState(..)
        , isPaused
        )
import TopBar.Msgs exposing (Msg(..))
import TopBar.Styles as Styles
import QueryString
import RemoteData exposing (RemoteData)
import Routes
import ScreenSize exposing (ScreenSize(..))
import UserState exposing (UserState(..))
import Window


type alias Flags =
    { route : Routes.Route }


query : Model -> String
query model =
    case model.middleSection of
        SearchBar { query } ->
            query

        _ ->
            ""


init : Flags -> ( Model, List Effect )
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


handleCallback : Callback -> Model -> ( Model, List Effect )
handleCallback callback model =
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
            , [ NavigateTo <| Routes.toString redirectUrl ]
            )

        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, [] )

        APIDataFetched (Ok ( time, data )) ->
            ( { model
                | teams = RemoteData.Success data.teams
                , middleSection =
                    if data.pipelines == [] then
                        Empty

                    else
                        model.middleSection
              }
            , []
            )

        APIDataFetched (Err err) ->
            ( { model | teams = RemoteData.Failure err, middleSection = Empty }, [] )

        ScreenResized size ->
            ( screenResize size model, [] )

        _ ->
            ( model, [] )


update : Msg -> Model -> ( Model, List Effect )
update msg model =
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
            , [ ForceFocus "search-input-field"
              , ModifyUrl (queryStringFromSearch query)
              ]
            )

        LogIn ->
            ( model, [ RedirectToLogin ] )

        LogOut ->
            ( model, [ SendLogOutRequest ] )

        ToggleUserMenu ->
            ( { model | isUserMenuExpanded = not model.isUserMenuExpanded }, [] )

        TogglePinIconDropdown ->
            ( { model | isPinMenuExpanded = not model.isPinMenuExpanded }, [] )

        FocusMsg ->
            let
                newModel =
                    case model.middleSection of
                        SearchBar r ->
                            { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Nothing } } }

                        _ ->
                            model
            in
            ( newModel, [] )

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
            ( newModel, [] )

        KeyDown keycode ->
            case keycode of
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
                                            , []
                                            )

                                        Just selectedIdx ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }

                                                newSelection =
                                                    (selectedIdx - 1) % List.length options
                                            in
                                            ( { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Just newSelection } } }
                                            , []
                                            )

                                _ ->
                                    ( model, [] )

                        _ ->
                            ( model, [] )

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
                                            , []
                                            )

                                        Just selectedIdx ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }

                                                newSelection =
                                                    (selectedIdx + 1) % List.length options
                                            in
                                            ( { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Just newSelection } } }
                                            , []
                                            )

                                _ ->
                                    ( model, [] )

                        _ ->
                            ( model, [] )

                -- enter key
                13 ->
                    case model.middleSection of
                        SearchBar r ->
                            case r.dropdown of
                                Shown { selectedIdx } ->
                                    case selectedIdx of
                                        Nothing ->
                                            ( model, [] )

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
                                            , []
                                            )

                                _ ->
                                    ( model, [] )

                        _ ->
                            ( model, [] )

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
                                            ( newModel, [] )

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
                                            ( newModel, [] )

                                _ ->
                                    ( model, [] )

                        _ ->
                            ( model, [] )

                -- any other keycode
                _ ->
                    case model.middleSection of
                        SearchBar r ->
                            ( { model | middleSection = SearchBar { r | dropdown = Shown { selectedIdx = Nothing } } }, [] )

                        _ ->
                            ( model, [] )

        ShowSearchInput ->
            showSearchInput model

        KeyPressed keycode ->
            case Char.fromCode keycode of
                '/' ->
                    ( model, [ ForceFocus "search-input-field" ] )

                _ ->
                    ( model, [] )

        ResizeScreen size ->
            ( screenResize size model, [] )

        GoToPinnedResource route ->
            ( model, [ NavigateTo (Routes.toString route) ] )

        Noop ->
            ( model, [] )


screenResize : Window.Size -> Model -> Model
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


showSearchInput : Model -> ( Model, List Effect )
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


view : UserState -> PipelineState -> Model -> Html Msg
view userState pipelineState model =
    Html.div
        [ id "top-bar-app"
        , style <| Styles.topBar <| isPaused pipelineState
        ]
        (viewConcourseLogo
            ++ viewMiddleSection model
            ++ viewPin pipelineState model
            ++ viewLogin userState model (isPaused pipelineState)
        )


viewLogin : UserState -> Model -> Bool -> List (Html Msg)
viewLogin userState model isPaused =
    if showLogin model then
        [ Html.div [ id "login-component", style Styles.loginComponent ] <|
            viewLoginState userState model.isUserMenuExpanded isPaused
        ]

    else
        []


showLogin : Model -> Bool
showLogin model =
    case model.middleSection of
        SearchBar _ ->
            model.screenSize /= Mobile

        _ ->
            True


viewLoginState : UserState -> Bool -> Bool -> List (Html Msg)
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


viewMiddleSection : Model -> List (Html Msg)
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


viewSearch : { query : String, dropdown : Dropdown } -> Model -> List (Html Msg)
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


viewDropdownItems : { query : String, dropdown : Dropdown } -> Model -> List (Html Msg)
viewDropdownItems { query, dropdown } model =
    case dropdown of
        Hidden ->
            []

        Shown { selectedIdx } ->
            let
                dropdownItem : Int -> String -> Html Msg
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


viewConcourseLogo : List (Html Msg)
viewConcourseLogo =
    [ Html.a
        [ style Styles.concourseLogo, href "/" ]
        []
    ]


viewBreadcrumbs : Routes.Route -> List (Html Msg)
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


breadcrumbComponent : String -> String -> List (Html Msg)
breadcrumbComponent componentType name =
    [ Html.div
        [ style (Styles.breadcrumbComponent componentType) ]
        []
    , Html.text <| decodeName name
    ]


viewBreadcrumbSeparator : Html Msg
viewBreadcrumbSeparator =
    Html.li
        [ class "breadcrumb-separator", style Styles.breadcrumbItem ]
        [ Html.text "/" ]


viewPipelineBreadcrumb : Concourse.PipelineIdentifier -> Html Msg
viewPipelineBreadcrumb pipelineId =
    Html.li
        [ id "breadcrumb-pipeline", style Styles.breadcrumbItem ]
        [ Html.a
            [ href <|
                Routes.toString <|
                    Routes.Pipeline { id = pipelineId, groups = [] }
            ]
            (breadcrumbComponent "pipeline" pipelineId.pipelineName)
        ]


viewJobBreadcrumb : String -> Html Msg
viewJobBreadcrumb jobName =
    Html.li
        [ id "breadcrumb-job", style Styles.breadcrumbItem ]
        (breadcrumbComponent "job" jobName)


viewResourceBreadcrumb : String -> Html Msg
viewResourceBreadcrumb resourceName =
    Html.li
        [ id "breadcrumb-resource", style Styles.breadcrumbItem ]
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


viewPin : PipelineState -> Model -> List (Html Msg)
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


viewPinDropdown : List ( String, Concourse.Version ) -> Concourse.PipelineIdentifier -> Model -> List (Html Msg)
viewPinDropdown pinnedResources pipeline model =
    if model.isPinMenuExpanded then
        [ Html.ul
            [ style Styles.pinIconDropdown ]
            (pinnedResources
                |> List.map
                    (\( resourceName, pinnedVersion ) ->
                        Html.li
                            [ onClick <|
                                GoToPinnedResource <|
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
