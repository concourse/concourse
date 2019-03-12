module TopBar.TopBar exposing
    ( Flags
    , handleCallback
    , handleDelivery
    , init
    , queryStringFromSearch
    , searchInputId
    , update
    , view
    )

import Array
import Callback exposing (Callback(..))
import Concourse
import Dashboard.Group exposing (Group)
import Dict
import Effects exposing (Effect(..))
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
import Keycodes
import QueryString
import Routes
import ScreenSize exposing (ScreenSize(..))
import Subscription exposing (Delivery(..))
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
import UserState exposing (UserState(..))
import Window


searchInputId : String
searchInputId =
    "search-input-field"


type alias Flags =
    { route : Routes.Route }


init : Flags -> ( Model {}, List Effect )
init { route } =
    ( { isUserMenuExpanded = False
      , isPinMenuExpanded = False
      , route = route
      , groups = []
      , dropdown = Hidden
      , screenSize = Desktop
      , shiftDown = False
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
                    Routes.dashboardRoute (model.route == Routes.Dashboard Routes.HighDensity)
            in
            ( { model | isUserMenuExpanded = False }
            , effects ++ [ NavigateTo <| Routes.toString redirectUrl ]
            )

        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, effects )

        ScreenResized size ->
            ( screenResize size model, effects )

        _ ->
            ( model, effects )


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


handleDelivery : Delivery -> ( Model r, List Effect ) -> ( Model r, List Effect )
handleDelivery delivery ( model, effects ) =
    case delivery of
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
                        case middleSection model of
                            SearchBar ->
                                ( { model
                                    | dropdown =
                                        arrowUp options model.dropdown
                                  }
                                , effects
                                )

                            _ ->
                                ( model, effects )

                    -- down arrow
                    40 ->
                        case middleSection model of
                            SearchBar ->
                                ( { model
                                    | dropdown =
                                        arrowDown options model.dropdown
                                  }
                                , effects
                                )

                            _ ->
                                ( model, effects )

                    -- enter key
                    13 ->
                        case middleSection model of
                            SearchBar ->
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
                                                            |> Maybe.withDefault (Routes.extractQuery model.route)
                                                in
                                                ( { model
                                                    | dropdown = Shown { selectedIdx = Nothing }
                                                    , route = Routes.Dashboard (Routes.Normal (Just selectedItem))
                                                  }
                                                , [ ModifyUrl <|
                                                        queryStringFromSearch
                                                            selectedItem
                                                  ]
                                                )

                                    _ ->
                                        ( model, effects )

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

        WindowResized size ->
            ( screenResize size model, effects )

        _ ->
            ( model, effects )


update : Msg -> ( Model r, List Effect ) -> ( Model r, List Effect )
update msg ( model, effects ) =
    case msg of
        FilterMsg query ->
            ( { model | route = Routes.Dashboard (Routes.Normal (Just query)) }
            , effects
                ++ [ Focus searchInputId
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

        FocusMsg ->
            let
                newModel =
                    case middleSection model of
                        SearchBar ->
                            { model | dropdown = Shown { selectedIdx = Nothing } }

                        _ ->
                            model
            in
            ( newModel, effects )

        BlurMsg ->
            let
                newModel =
                    case middleSection model of
                        SearchBar ->
                            { model | dropdown = Hidden }

                        _ ->
                            model
            in
            ( newModel, effects )

        ShowSearchInput ->
            showSearchInput model


screenResize : Window.Size -> Model r -> Model r
screenResize size model =
    let
        newSize =
            ScreenSize.fromWindowSize size

        newModel =
            { model | screenSize = newSize }
    in
    case middleSection model of
        Breadcrumbs r ->
            newModel

        Empty ->
            newModel

        SearchBar ->
            newModel

        MinifiedSearch ->
            case newSize of
                ScreenSize.Desktop ->
                    { newModel | dropdown = Hidden }

                ScreenSize.BigDesktop ->
                    { newModel | dropdown = Hidden }

                ScreenSize.Mobile ->
                    newModel


showSearchInput : Model r -> ( Model r, List Effect )
showSearchInput model =
    let
        newModel =
            { model | dropdown = Shown { selectedIdx = Nothing } }
    in
    case middleSection model of
        MinifiedSearch ->
            ( newModel, [ Focus searchInputId ] )

        SearchBar ->
            ( model, [] )

        Empty ->
            Debug.log "attempting to show search input when search is gone"
                ( model, [] )

        Breadcrumbs _ ->
            Debug.log "attempting to show search input on a breadcrumbs page"
                ( model, [] )


view : UserState -> PipelineState -> Model r -> Html Msg
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


viewLogin : UserState -> Model r -> Bool -> List (Html Msg)
viewLogin userState model isPaused =
    if showLogin model then
        [ Html.div [ id "login-component", style Styles.loginComponent ] <|
            viewLoginState userState model.isUserMenuExpanded isPaused
        ]

    else
        []


showLogin : Model r -> Bool
showLogin model =
    model.screenSize /= Mobile || middleSection model /= SearchBar


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


viewMiddleSection : Model r -> List (Html Msg)
viewMiddleSection model =
    case middleSection model of
        Empty ->
            []

        MinifiedSearch ->
            [ Html.div
                [ style <|
                    Styles.showSearchContainer
                        { screenSize = model.screenSize
                        , highDensity = model.route == Routes.Dashboard Routes.HighDensity
                        }
                ]
                [ Html.a
                    [ id "show-search-button"
                    , onClick ShowSearchInput
                    , style Styles.searchButton
                    ]
                    []
                ]
            ]

        SearchBar ->
            viewSearch model

        Breadcrumbs r ->
            [ Html.div [ id "breadcrumbs", style Styles.breadcrumbContainer ] (viewBreadcrumbs r) ]


middleSection : Model r -> MiddleSection
middleSection { route, dropdown, screenSize, groups } =
    case route of
        Routes.Dashboard (Routes.Normal query) ->
            let
                q =
                    Maybe.withDefault "" query
            in
            if groups |> List.concatMap .pipelines |> List.isEmpty then
                Empty

            else if dropdown == Hidden && screenSize == Mobile && q == "" then
                MinifiedSearch

            else
                SearchBar

        Routes.Dashboard Routes.HighDensity ->
            Empty

        _ ->
            Breadcrumbs route


viewSearch :
    { a
        | screenSize : ScreenSize
        , route : Routes.Route
        , dropdown : Dropdown
        , groups : List Group
    }
    -> List (Html Msg)
viewSearch ({ screenSize, route } as params) =
    let
        query =
            Routes.extractQuery route
    in
    [ Html.div
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
    ]


viewDropdownItems :
    { a
        | route : Routes.Route
        , dropdown : Dropdown
        , groups : List Group
        , screenSize : ScreenSize
    }
    -> List (Html Msg)
viewDropdownItems ({ dropdown, screenSize } as model) =
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
            in
            [ Html.ul
                [ id "search-dropdown"
                , style (Styles.dropdownContainer screenSize)
                ]
                (List.indexedMap dropdownItem (dropdownOptions model))
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
        [ class "breadcrumb-separator", style <| Styles.breadcrumbItem False ]
        [ Html.text "/" ]


viewPipelineBreadcrumb : Concourse.PipelineIdentifier -> Html Msg
viewPipelineBreadcrumb pipelineId =
    Html.li
        [ id "breadcrumb-pipeline"
        , style <| Styles.breadcrumbItem True
        , onClick <| GoToRoute <| Routes.Pipeline { id = pipelineId, groups = [] }
        ]
        (breadcrumbComponent "pipeline" pipelineId.pipelineName)


viewJobBreadcrumb : String -> Html Msg
viewJobBreadcrumb jobName =
    Html.li
        [ id "breadcrumb-job"
        , style <| Styles.breadcrumbItem False
        ]
        (breadcrumbComponent "job" jobName)


viewResourceBreadcrumb : String -> Html Msg
viewResourceBreadcrumb resourceName =
    Html.li
        [ id "breadcrumb-resource"
        , style <| Styles.breadcrumbItem False
        ]
        (breadcrumbComponent "resource" resourceName)


decodeName : String -> String
decodeName name =
    Maybe.withDefault name (Http.decodeUri name)


dropdownOptions : { a | route : Routes.Route, groups : List Group } -> List String
dropdownOptions { route, groups } =
    case Routes.extractQuery route |> String.trim of
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


viewPin : PipelineState -> Model r -> List (Html Msg)
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


viewPinDropdown : List ( String, Concourse.Version ) -> Concourse.PipelineIdentifier -> Model r -> List (Html Msg)
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
