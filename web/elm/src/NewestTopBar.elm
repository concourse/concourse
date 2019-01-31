module NewestTopBar exposing
    ( Flags
    , Model
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
import NewTopBar.Msgs exposing (Msg(..))
import NewTopBar.Styles as Styles
import QueryString
import RemoteData exposing (RemoteData)
import Routes
import ScreenSize exposing (ScreenSize(..))
import SearchBar exposing (Autocomplete(..), SearchBar(..))
import TopBar exposing (userDisplayName)
import UserState exposing (UserState(..))
import Window


type alias Model =
    { userState : UserState
    , isUserMenuExpanded : Bool
    , searchBar : SearchBar
    , teams : RemoteData.WebData (List Concourse.Team)
    , route : Routes.ConcourseRoute
    , screenSize : ScreenSize
    , highDensity : Bool
    , hasPipelines : Bool
    }


type alias Flags =
    { route : Routes.ConcourseRoute
    , isHd : Bool
    }


query : Model -> String
query model =
    case model.searchBar of
        Expanded r ->
            r.query

        Collapsed ->
            ""


querySearchForRoute : Routes.ConcourseRoute -> String
querySearchForRoute route =
    QueryString.one QueryString.string "search" route.queries
        |> Maybe.withDefault ""


init : Flags -> ( Model, List Effect )
init { route, isHd } =
    let
        showSearch =
            route.logical == Routes.Dashboard { isHd = isHd }

        searchBar =
            if showSearch then
                Expanded { query = querySearchForRoute route, autocomplete = Hidden }

            else
                Collapsed
    in
    ( { userState = UserStateUnknown
      , isUserMenuExpanded = False
      , searchBar = searchBar
      , teams = RemoteData.Loading
      , route = route
      , screenSize = Desktop
      , highDensity = isHd
      , hasPipelines = True
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
                    case model.searchBar of
                        Collapsed ->
                            Routes.dashboardRoute True

                        Expanded _ ->
                            Routes.dashboardRoute False
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
                , hasPipelines = data.pipelines /= []
              }
            , []
            )

        APIDataFetched (Err err) ->
            ( { model
                | teams = RemoteData.Failure err
                , userState = UserStateLoggedOut
                , hasPipelines = False
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
                        Expanded r ->
                            { model | searchBar = Expanded { r | query = query } }

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
                        Expanded r ->
                            { model | searchBar = Expanded { r | autocomplete = Shown { selectedIdx = Nothing } } }

                        Collapsed ->
                            model
            in
            ( newModel, [] )

        BlurMsg ->
            let
                newModel =
                    case model.searchBar of
                        Expanded r ->
                            if model.screenSize == Mobile && query model == "" then
                                { model | searchBar = Collapsed }

                            else
                                { model | searchBar = Expanded { r | autocomplete = Hidden } }

                        Collapsed ->
                            model
            in
            ( newModel, [] )

        KeyDown keycode ->
            case model.searchBar of
                Expanded r ->
                    case r.autocomplete of
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
                                                    autocompleteOptions { query = r.query, teams = model.teams }

                                                lastItem =
                                                    List.length options - 1
                                            in
                                            ( { model | searchBar = Expanded { r | autocomplete = Shown { selectedIdx = Just lastItem } } }, [] )

                                        -- down arrow
                                        40 ->
                                            let
                                                options =
                                                    autocompleteOptions { query = r.query, teams = model.teams }
                                            in
                                            ( { model | searchBar = Expanded { r | autocomplete = Shown { selectedIdx = Just 0 } } }, [] )

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
                                                    Array.fromList (autocompleteOptions { query = r.query, teams = model.teams })

                                                selectedItem =
                                                    Maybe.withDefault r.query (Array.get selectionIdx options)
                                            in
                                            ( { model
                                                | searchBar =
                                                    Expanded
                                                        { r
                                                            | autocomplete = Shown { selectedIdx = Nothing }
                                                            , query = selectedItem
                                                        }
                                              }
                                            , []
                                            )

                                        -- up arrow
                                        38 ->
                                            let
                                                options =
                                                    autocompleteOptions { query = r.query, teams = model.teams }

                                                newSelection =
                                                    (selectionIdx - 1) % List.length options
                                            in
                                            ( { model | searchBar = Expanded { r | autocomplete = Shown { selectedIdx = Just newSelection } } }, [] )

                                        -- down arrow
                                        40 ->
                                            let
                                                options =
                                                    autocompleteOptions { query = r.query, teams = model.teams }

                                                newSelection =
                                                    (selectionIdx + 1) % List.length options
                                            in
                                            ( { model | searchBar = Expanded { r | autocomplete = Shown { selectedIdx = Just newSelection } } }, [] )

                                        -- escape key
                                        27 ->
                                            ( model, [ ForceFocus "search-input-field" ] )

                                        _ ->
                                            ( { model | searchBar = Expanded { r | autocomplete = Shown { selectedIdx = Nothing } } }, [] )

                Collapsed ->
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
    in
    { model
        | screenSize = newSize
        , searchBar =
            SearchBar.screenSizeChanged
                { oldSize = model.screenSize
                , newSize = newSize
                }
                model.searchBar
    }


showSearchInput : Model -> ( Model, List Effect )
showSearchInput model =
    let
        newModel =
            { model | searchBar = Expanded { query = "", autocomplete = Hidden } }
    in
    case model.searchBar of
        Collapsed ->
            ( newModel, [ ForceFocus "search-input-field" ] )

        Expanded _ ->
            ( model, [] )


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
        (case model.route.logical of
            Routes.Pipeline teamName pipelineName ->
                viewPipelineBreadcrumb (Routes.toString model.route.logical) pipelineName

            Routes.Build teamName pipelineName jobName buildNumber ->
                viewPipelineBreadcrumb (Routes.toString (Routes.Pipeline teamName pipelineName)) pipelineName
                    ++ viewJobBreadcrumb (Routes.toString (Routes.Job teamName pipelineName jobName))

            Routes.Resource teamName pipelineName resourceName ->
                viewPipelineBreadcrumb (Routes.toString (Routes.Pipeline teamName pipelineName)) pipelineName
                    ++ viewResourceBreadcrumb resourceName

            Routes.Job teamName pipelineName jobName ->
                viewPipelineBreadcrumb (Routes.toString (Routes.Pipeline teamName pipelineName)) pipelineName
                    ++ viewJobBreadcrumb jobName

            _ ->
                []
        )


viewLogin : Model -> List (Html Msg)
viewLogin model =
    if model.screenSize /= Mobile || model.searchBar == Collapsed then
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
    if model.hasPipelines && not model.highDensity then
        case model.searchBar of
            Collapsed ->
                [ Html.div [ style <| Styles.showSearchContainer model ]
                    [ Html.a
                        [ id "show-search-button"
                        , onClick ShowSearchInput
                        , style Styles.searchButton
                        ]
                        []
                    ]
                ]

            Expanded r ->
                viewSearch model

    else
        []


viewSearch : Model -> List (Html Msg)
viewSearch model =
    [ Html.div
        [ id "search-container"
        , style (Styles.searchContainer model.screenSize)
        ]
        ([ Html.input
            [ id "search-input-field"
            , style (Styles.searchInput model.screenSize)
            , placeholder "search"
            , value (query model)
            , onFocus FocusMsg
            , onBlur BlurMsg
            , onInput FilterMsg
            ]
            []
         , Html.div
            [ id "search-clear"
            , onClick (FilterMsg "")
            , style (Styles.searchClearButton (String.length (query model) > 0))
            ]
            []
         ]
            ++ viewDropdownItems model
        )
    ]


viewDropdownItems : Model -> List (Html Msg)
viewDropdownItems model =
    case model.searchBar of
        Expanded r ->
            case r.autocomplete of
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
                            case String.trim r.query of
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

        Collapsed ->
            []


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


autocompleteOptions : { a | query : String, teams : RemoteData.WebData (List Concourse.Team) } -> List String
autocompleteOptions { query, teams } =
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
