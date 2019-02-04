module NewestTopBar exposing
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
import NewTopBar.Model exposing (Dropdown(..), Model, SearchBar(..))
import NewTopBar.Msgs exposing (Msg(..))
import NewTopBar.Styles as Styles
import QueryString
import RemoteData exposing (RemoteData)
import Routes
import ScreenSize exposing (ScreenSize(..))
import TopBar exposing (userDisplayName)
import UserState exposing (UserState(..))
import Window


type alias Flags =
    { route : Routes.Route }


query : Model -> String
query model =
    case model.searchBar of
        Visible r ->
            r.query

        Minified ->
            ""

        Gone ->
            ""


init : Flags -> ( Model, List Effect )
init { route } =
    let
        isHd =
            route == Routes.Dashboard Routes.HighDensity

        searchBar =
            case route of
                Routes.Dashboard (Routes.Normal search) ->
                    Visible { query = Maybe.withDefault "" search, dropdown = Hidden }

                _ ->
                    Gone
    in
    ( { userState = UserStateUnknown
      , isUserMenuExpanded = False
      , searchBar = searchBar
      , teams = RemoteData.Loading
      , route = route
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
                | userState = UserStateLoggedOut
                , isUserMenuExpanded = False
                , teams = RemoteData.Loading
              }
            , [ NavigateTo redirectUrl ]
            )

        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, [] )

        APIDataFetched (Ok ( time, data )) ->
            ( { model
                | teams = RemoteData.Success data.teams
                , userState =
                    case data.user of
                        Just user ->
                            UserStateLoggedIn user

                        Nothing ->
                            UserStateLoggedOut
                , searchBar =
                    if data.pipelines == [] then
                        Gone

                    else
                        model.searchBar
              }
            , []
            )

        APIDataFetched (Err err) ->
            ( { model
                | teams = RemoteData.Failure err
                , userState = UserStateLoggedOut
                , searchBar = Gone
              }
            , []
            )

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
                    case model.searchBar of
                        Visible r ->
                            { model | searchBar = Visible { r | query = query } }

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

        FocusMsg ->
            let
                newModel =
                    case model.searchBar of
                        Visible r ->
                            { model | searchBar = Visible { r | dropdown = Shown { selectedIdx = Nothing } } }

                        _ ->
                            model
            in
            ( newModel, [] )

        BlurMsg ->
            let
                newModel =
                    case model.searchBar of
                        Visible r ->
                            if model.screenSize == Mobile && r.query == "" then
                                { model | searchBar = Minified }

                            else
                                { model | searchBar = Visible { r | dropdown = Hidden } }

                        _ ->
                            model
            in
            ( newModel, [] )

        KeyDown keycode ->
            case model.searchBar of
                Visible r ->
                    case r.dropdown of
                        Hidden ->
                            ( model, [] )

                        Shown { selectedIdx } ->
                            case selectedIdx of
                                Nothing ->
                                    case keycode of
                                        -- up arrow
                                        38 ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }

                                                lastItem =
                                                    List.length options - 1
                                            in
                                            ( { model | searchBar = Visible { r | dropdown = Shown { selectedIdx = Just lastItem } } }, [] )

                                        -- down arrow
                                        40 ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }
                                            in
                                            ( { model | searchBar = Visible { r | dropdown = Shown { selectedIdx = Just 0 } } }, [] )

                                        -- escape key
                                        27 ->
                                            ( model, [ ForceFocus "search-input-field" ] )

                                        _ ->
                                            ( model, [] )

                                Just selectionIdx ->
                                    case keycode of
                                        -- enter key
                                        13 ->
                                            let
                                                options =
                                                    Array.fromList (dropdownOptions { query = r.query, teams = model.teams })

                                                selectedItem =
                                                    Maybe.withDefault r.query (Array.get selectionIdx options)
                                            in
                                            ( { model
                                                | searchBar =
                                                    Visible
                                                        { r
                                                            | dropdown = Shown { selectedIdx = Nothing }
                                                            , query = selectedItem
                                                        }
                                              }
                                            , []
                                            )

                                        -- up arrow
                                        38 ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }

                                                newSelection =
                                                    (selectionIdx - 1) % List.length options
                                            in
                                            ( { model | searchBar = Visible { r | dropdown = Shown { selectedIdx = Just newSelection } } }, [] )

                                        -- down arrow
                                        40 ->
                                            let
                                                options =
                                                    dropdownOptions { query = r.query, teams = model.teams }

                                                newSelection =
                                                    (selectionIdx + 1) % List.length options
                                            in
                                            ( { model | searchBar = Visible { r | dropdown = Shown { selectedIdx = Just newSelection } } }, [] )

                                        -- escape key
                                        27 ->
                                            ( model, [ ForceFocus "search-input-field" ] )

                                        _ ->
                                            ( { model | searchBar = Visible { r | dropdown = Shown { selectedIdx = Nothing } } }, [] )

                Minified ->
                    ( model, [] )

                Gone ->
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

        Noop ->
            ( model, [] )


screenResize : Window.Size -> Model -> Model
screenResize size model =
    let
        newSize =
            ScreenSize.fromWindowSize size

        newSearchBar =
            case ( model.searchBar, newSize ) of
                ( Gone, _ ) ->
                    Gone

                ( Visible r, ScreenSize.Mobile ) ->
                    if String.isEmpty r.query then
                        case model.screenSize of
                            ScreenSize.Desktop ->
                                Minified

                            ScreenSize.BigDesktop ->
                                Minified

                            ScreenSize.Mobile ->
                                Visible r

                    else
                        Visible r

                ( Visible r, ScreenSize.Desktop ) ->
                    Visible r

                ( Visible r, ScreenSize.BigDesktop ) ->
                    Visible r

                ( Minified, ScreenSize.Desktop ) ->
                    Visible { query = "", dropdown = Hidden }

                ( Minified, ScreenSize.BigDesktop ) ->
                    Visible { query = "", dropdown = Hidden }

                ( Minified, ScreenSize.Mobile ) ->
                    Minified
    in
    { model | screenSize = newSize, searchBar = newSearchBar }


showSearchInput : Model -> ( Model, List Effect )
showSearchInput model =
    let
        newModel =
            { model | searchBar = Visible { query = "", dropdown = Hidden } }
    in
    case model.searchBar of
        Minified ->
            ( newModel, [ ForceFocus "search-input-field" ] )

        Visible _ ->
            ( model, [] )

        Gone ->
            Debug.log "attempting to show search input when search is gone" ( model, [] )


view : Model -> Html Msg
view model =
    Html.div [ id "top-bar-app", style Styles.topBar ] <|
        viewConcourseLogo
            ++ viewBreadcrumbs model
            ++ viewMiddleSection model
            ++ viewLogin model


viewBreadcrumbs : Model -> List (Html Msg)
viewBreadcrumbs model =
    List.intersperse viewBreadcrumbSeparator
        (case model.route of
            Routes.Pipeline teamName pipelineName _ ->
                viewPipelineBreadcrumb (Routes.toString model.route) pipelineName

            Routes.Build teamName pipelineName jobName buildNumber _ ->
                viewPipelineBreadcrumb (Routes.toString (Routes.Pipeline teamName pipelineName [])) pipelineName
                    ++ viewJobBreadcrumb (Routes.toString (Routes.Job teamName pipelineName jobName Nothing))

            Routes.Resource teamName pipelineName resourceName _ ->
                viewPipelineBreadcrumb (Routes.toString (Routes.Pipeline teamName pipelineName [])) pipelineName
                    ++ viewResourceBreadcrumb resourceName

            Routes.Job teamName pipelineName jobName _ ->
                viewPipelineBreadcrumb (Routes.toString (Routes.Pipeline teamName pipelineName [])) pipelineName
                    ++ viewJobBreadcrumb jobName

            _ ->
                []
        )


viewLogin : Model -> List (Html Msg)
viewLogin model =
    if model.screenSize /= Mobile || model.searchBar == Minified then
        [ Html.div [ id "login-component", style Styles.loginComponent ] <| viewLoginState model ]

    else
        []


viewLoginState : { a | userState : UserState, isUserMenuExpanded : Bool } -> List (Html Msg)
viewLoginState { userState, isUserMenuExpanded } =
    case userState of
        UserStateUnknown ->
            []

        UserStateLoggedOut ->
            [ Html.div
                [ href "/sky/login"
                , HA.attribute "aria-label" "Log In"
                , id "login-container"
                , onClick LogIn
                , style Styles.loginContainer
                ]
                [ Html.div [ style Styles.loginItem, id "login-item" ] [ Html.a [ href "/sky/login" ] [ Html.text "login" ] ] ]
            ]

        UserStateLoggedIn user ->
            [ Html.div
                [ id "login-container"
                , onClick ToggleUserMenu
                , style Styles.loginContainer
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


viewMiddleSection : Model -> List (Html Msg)
viewMiddleSection model =
    case model.searchBar of
        Gone ->
            []

        Minified ->
            [ Html.div [ style <| Styles.showSearchContainer model ]
                [ Html.a
                    [ id "show-search-button"
                    , onClick ShowSearchInput
                    , style Styles.searchButton
                    ]
                    []
                ]
            ]

        Visible r ->
            viewSearch r model


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
        [ style Styles.concourseLogo, href "#" ]
        []
    ]


breadcrumbComponent : String -> String -> List (Html Msg)
breadcrumbComponent componentType name =
    [ Html.div
        [ style (Styles.breadcrumbComponent componentType) ]
        []
    , Html.text <| decodeName name
    ]


viewBreadcrumbSeparator : Html Msg
viewBreadcrumbSeparator =
    Html.li [ class "breadcrumb-separator", style Styles.breadcrumbContainer ] [ Html.text "/" ]


viewPipelineBreadcrumb : String -> String -> List (Html Msg)
viewPipelineBreadcrumb url pipelineName =
    [ Html.li [ style Styles.breadcrumbContainer, id "breadcrumb-pipeline" ]
        [ Html.a
            [ href url ]
          <|
            breadcrumbComponent "pipeline" pipelineName
        ]
    ]


viewJobBreadcrumb : String -> List (Html Msg)
viewJobBreadcrumb jobName =
    [ Html.li [ id "breadcrumb-job", style Styles.breadcrumbContainer ] <| breadcrumbComponent "job" jobName ]


viewResourceBreadcrumb : String -> List (Html Msg)
viewResourceBreadcrumb resourceName =
    [ Html.li [ id "breadcrumb-resource", style Styles.breadcrumbContainer ] <| breadcrumbComponent "resource" resourceName ]


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
