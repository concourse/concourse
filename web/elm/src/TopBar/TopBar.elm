module TopBar.TopBar exposing
    ( Flags
    , handleCallback
    , handleDelivery
    , init
    , queryStringFromSearch
    , searchInputId
    , update
    , viewBreadcrumbs
    , viewConcourseLogo
    , viewSearch
    )

import Array
import Concourse
import Dashboard.Group.Models exposing (Group)
import EffectTransformer exposing (ET)
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
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Hoverable(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import QueryString
import Routes
import ScreenSize exposing (ScreenSize(..))
import TopBar.Model
    exposing
        ( Dropdown(..)
        , Model
        , PipelineState
        , isPaused
        )
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


handleCallback : Callback -> ET (Model r)
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


handleDelivery : Delivery -> ET (Model r)
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


update : Message -> ET (Model r)
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

        TogglePipelinePaused _ _ ->
            ( model, effects )

        FocusMsg ->
            let
                newModel =
                    { model | dropdown = Shown { selectedIdx = Nothing } }
            in
            ( newModel, effects )

        BlurMsg ->
            let
                newModel =
                    { model | dropdown = Hidden }
            in
            ( newModel, effects )

        ShowSearchInput ->
            showSearchInput ( model, effects )

        _ ->
            ( model, effects )


screenResize : Window.Size -> Model r -> Model r
screenResize size model =
    let
        newSize =
            ScreenSize.fromWindowSize size

        newModel =
            { model | screenSize = newSize }
    in
    case newSize of
        ScreenSize.Desktop ->
            { newModel | dropdown = Hidden }

        ScreenSize.BigDesktop ->
            { newModel | dropdown = Hidden }

        ScreenSize.Mobile ->
            newModel


showSearchInput : ET (Model r)
showSearchInput ( model, effects ) =
    case model.route of
        Routes.Dashboard (Routes.Normal query) ->
            let
                q =
                    Maybe.withDefault "" query

                isDropDownHidden =
                    model.dropdown == TopBar.Model.Hidden

                isMobile =
                    model.screenSize == ScreenSize.Mobile

                newModel =
                    { model | dropdown = Shown { selectedIdx = Nothing } }
            in
            if isDropDownHidden && isMobile && q == "" then
                ( newModel, effects ++ [ Focus searchInputId ] )

            else
                ( model, effects )

        _ ->
            ( model, effects )


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


viewSearch :
    { a
        | screenSize : ScreenSize
        , route : Routes.Route
        , dropdown : Dropdown
        , groups : List Group
    }
    -> Html Message
viewSearch ({ screenSize, route } as params) =
    let
        query =
            Routes.extractQuery route
    in
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
        | route : Routes.Route
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


viewConcourseLogo : Html Message
viewConcourseLogo =
    Html.a [ style Styles.concourseLogo, href "/" ] []


viewBreadcrumbs : Routes.Route -> Html Message
viewBreadcrumbs route =
    Html.div
        [ id "breadcrumbs", style Styles.breadcrumbContainer ]
    <|
        case route of
            Routes.Pipeline { id } ->
                [ viewPipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                ]

            Routes.Build { id } ->
                [ viewPipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , viewBreadcrumbSeparator
                , viewJobBreadcrumb id.jobName
                ]

            Routes.Resource { id } ->
                [ viewPipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , viewBreadcrumbSeparator
                , viewResourceBreadcrumb id.resourceName
                ]

            Routes.Job { id } ->
                [ viewPipelineBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
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
