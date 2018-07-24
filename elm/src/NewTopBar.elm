module NewTopBar exposing (Model, Msg(FilterMsg, UserFetched, KeyDown, LoggedOut), UserState(UserStateLoggedIn), init, fetchUser, update, view)

import Array
import Concourse
import Concourse.User
import Concourse.Team
import Dom
import Html exposing (Html)
import Html.Attributes exposing (class, classList, href, id, src, type_, placeholder, value)
import Html.Events exposing (..)
import Http
import Keyboard
import LoginRedirect
import Navigation exposing (Location)
import RemoteData exposing (RemoteData)
import Task
import TopBar exposing (userDisplayName)


type alias Model =
    { teams : RemoteData.WebData (List Concourse.Team)
    , userState : UserState
    , userMenuVisible : Bool
    , query : String
    , showSearch : Bool
    , showAutocomplete : Bool
    , selectionMade : Bool
    , selection : Int
    }


type UserState
    = UserStateLoggedIn Concourse.User
    | UserStateLoggedOut
    | UserStateUnknown


type Msg
    = Noop
    | UserFetched (RemoteData.WebData Concourse.User)
    | TeamsFetched (RemoteData.WebData (List Concourse.Team))
    | LogIn
    | LogOut
    | LoggedOut (Result Http.Error ())
    | FilterMsg String
    | FocusMsg
    | BlurMsg
    | SelectMsg Int
    | KeyDown Keyboard.KeyCode
    | ToggleUserMenu


init : Bool -> ( Model, Cmd Msg )
init showSearch =
    ( { teams = RemoteData.Loading
      , userState = UserStateUnknown
      , userMenuVisible = False
      , query = ""
      , showSearch = showSearch
      , showAutocomplete = False
      , selectionMade = False
      , selection = 0
      }
    , Cmd.batch [ fetchUser, fetchTeams ]
    )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Noop ->
            ( model, Cmd.none )

        FilterMsg query ->
            ( { model | query = query }, Task.attempt (always Noop) (Dom.focus "search-input-field") )

        UserFetched user ->
            case user of
                RemoteData.Success user ->
                    ( { model | userState = UserStateLoggedIn user }, Cmd.none )

                _ ->
                    ( { model | userState = UserStateLoggedOut }
                    , Cmd.none
                    )

        LogIn ->
            ( model
            , LoginRedirect.requestLoginRedirect ""
            )

        LogOut ->
            ( model, logOut )

        LoggedOut (Ok _) ->
            let
                redirectUrl =
                    case model.showSearch of
                        True ->
                            "/dashboard"

                        False ->
                            "/dashboard/hd"
            in
                ( { model
                    | userState = UserStateLoggedOut
                    , userMenuVisible = False
                  }
                , Navigation.newUrl redirectUrl
                )

        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, Cmd.none )

        ToggleUserMenu ->
            ( { model | userMenuVisible = not model.userMenuVisible }, Cmd.none )

        TeamsFetched response ->
            ( { model | teams = response }, Cmd.none )

        FocusMsg ->
            ( { model | showAutocomplete = True }, Cmd.none )

        BlurMsg ->
            ( { model | showAutocomplete = False, selectionMade = False, selection = 0 }, Cmd.none )

        SelectMsg index ->
            ( { model | selectionMade = True, selection = index + 1 }, Cmd.none )

        KeyDown keycode ->
            if not model.showAutocomplete then
                ( { model | selectionMade = False, selection = 0 }, Cmd.none )
            else
                case keycode of
                    -- enter key
                    13 ->
                        if not model.selectionMade then
                            ( model, Cmd.none )
                        else
                            let
                                options =
                                    Array.fromList (autocompleteOptions model)

                                index =
                                    (model.selection - 1) % (Array.length options)

                                selectedItem =
                                    case Array.get index options of
                                        Nothing ->
                                            model.query

                                        Just item ->
                                            item
                            in
                                ( { model | selectionMade = False, selection = 0, query = selectedItem }, Cmd.none )

                    -- up arrow
                    38 ->
                        ( { model | selectionMade = True, selection = model.selection - 1 }, Cmd.none )

                    -- down arrow
                    40 ->
                        ( { model | selectionMade = True, selection = model.selection + 1 }, Cmd.none )

                    -- escape key
                    27 ->
                        ( model, Task.attempt (always Noop) (Dom.blur "search-input-field") )

                    _ ->
                        ( { model | selectionMade = False, selection = 0 }, Cmd.none )


viewUserState : UserState -> Bool -> Html Msg
viewUserState userState userMenuVisible =
    case userState of
        UserStateUnknown ->
            Html.text ""

        UserStateLoggedOut ->
            Html.div [ class "user-id", onClick LogIn ]
                [ Html.a
                    [ href "/sky/login"
                    , Html.Attributes.attribute "aria-label" "Log In"
                    , class "login-button"
                    ]
                    [ Html.text "login"
                    ]
                ]

        UserStateLoggedIn user ->
            Html.div [ class "user-info" ]
                [ Html.div [ class "user-id", onClick ToggleUserMenu ]
                    [ Html.text <|
                        userDisplayName user
                    ]
                , Html.div [ classList [ ( "user-menu", True ), ( "hidden", not userMenuVisible ) ], onClick LogOut ]
                    [ Html.a
                        [ Html.Attributes.attribute "aria-label" "Log Out"
                        ]
                        [ Html.text "logout"
                        ]
                    ]
                ]


view : Model -> Html Msg
view model =
    Html.div [ class "module-topbar" ]
        [ Html.div [ class "topbar-logo" ] [ Html.a [ class "logo-image-link", href "#" ] [] ]
        , Html.div [ class "topbar-login" ]
            [ Html.div [ class "topbar-user-info" ]
                [ viewUserState model.userState model.userMenuVisible
                ]
            ]
        , Html.div [ classList [ ( "topbar-search", True ), ( "hidden", not model.showSearch ) ] ]
            [ Html.div
                [ class "topbar-search-form" ]
                [ Html.input
                    [ class "search-input-field"
                    , id "search-input-field"
                    , type_ "text"
                    , placeholder "search"
                    , onInput FilterMsg
                    , onFocus FocusMsg
                    , onBlur BlurMsg
                    , value model.query
                    ]
                    []
                , Html.span
                    [ classList [ ( "search-clear-button", True ), ( "active", not <| String.isEmpty model.query ) ]
                    , id "search-clear-button"
                    , onClick (FilterMsg "")
                    ]
                    []
                ]
            , Html.ul [ classList [ ( "hidden", not model.showAutocomplete ), ( "search-options", True ) ] ] <|
                let
                    options =
                        autocompleteOptions model
                in
                    List.indexedMap
                        (\index option ->
                            Html.li
                                [ classList
                                    [ ( "search-option", True )
                                    , ( "active", model.selectionMade && index == (model.selection - 1) % (List.length options) )
                                    ]
                                , onMouseDown (FilterMsg option)
                                , onMouseOver (SelectMsg index)
                                ]
                                [ Html.text option ]
                        )
                        options
            ]
        ]


fetchUser : Cmd Msg
fetchUser =
    Cmd.map UserFetched <|
        RemoteData.asCmd Concourse.User.fetchUser


fetchTeams : Cmd Msg
fetchTeams =
    Cmd.map TeamsFetched <|
        RemoteData.asCmd Concourse.Team.fetchTeams


autocompleteOptions : Model -> List String
autocompleteOptions model =
    case String.trim model.query of
        "" ->
            [ "status: ", "team: " ]

        "status:" ->
            [ "status: paused", "status: pending", "status: failed", "status: errored", "status: aborted", "status: running", "status: succeeded" ]

        "team:" ->
            case model.teams of
                RemoteData.Success teams ->
                    List.map (\team -> "team: " ++ team.name) <| List.take 10 teams

                _ ->
                    []

        _ ->
            []


logOut : Cmd Msg
logOut =
    Task.attempt LoggedOut Concourse.User.logOut
