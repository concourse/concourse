module NewTopBar exposing (Model, Msg(FilterMsg, UserFetched, KeyDown), init, fetchUser, update, view)

import Array
import Concourse
import Concourse.User
import Concourse.Team
import Dom
import Html exposing (Html)
import Html.Attributes exposing (class, classList, href, id, src, type_, placeholder, value)
import Html.Events exposing (..)
import Keyboard
import RemoteData exposing (RemoteData)
import Task


type alias Model =
    { user : RemoteData.WebData Concourse.User
    , teams : RemoteData.WebData (List Concourse.Team)
    , query : String
    , showSearch : Bool
    , showAutocomplete : Bool
    , selectionMade : Bool
    , selection : Int
    }


type Msg
    = Noop
    | UserFetched (RemoteData.WebData Concourse.User)
    | TeamsFetched (RemoteData.WebData (List Concourse.Team))
    | FilterMsg String
    | FocusMsg
    | BlurMsg
    | SelectMsg Int
    | KeyDown Keyboard.KeyCode


init : Bool -> ( Model, Cmd Msg )
init showSearch =
    ( { user = RemoteData.Loading
      , teams = RemoteData.Loading
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

        UserFetched response ->
            ( { model | user = response }, Cmd.none )

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


showUserInfo : Model -> Html Msg
showUserInfo model =
    case model.user of
        RemoteData.NotAsked ->
            Html.text "n/a"

        RemoteData.Loading ->
            Html.text "loading"

        RemoteData.Success user ->
            Html.text (userDisplayName user)

        RemoteData.Failure _ ->
            Html.a
                [ href "/sky/login"
                , Html.Attributes.attribute "aria-label" "Log In"
                ]
                [ Html.text "login"
                ]


userDisplayName : Concourse.User -> String
userDisplayName user =
    Maybe.withDefault user.id <|
        List.head <|
            List.filter (not << String.isEmpty) [ user.userName, user.name, user.email ]


view : Model -> Html Msg
view model =
    Html.div [ class "module-topbar" ]
        [ Html.div [ class "topbar-logo" ] [ Html.a [ class "logo-image-link", href "#" ] [] ]
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
                    [ class "search-clear-button"
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
        , Html.div [ class "topbar-login" ]
            [ Html.div [ class "topbar-user-info" ]
                [ showUserInfo model ]
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
    case model.query of
        "" ->
            [ "status:", "team:" ]

        "status:" ->
            [ "status:paused", "status:pending", "status:failed", "status:errored", "status:aborted", "status:running", "status:succeeded" ]

        "team:" ->
            case model.teams of
                RemoteData.Success teams ->
                    List.map (\team -> "team:" ++ team.name) <| List.take 10 teams

                _ ->
                    []

        _ ->
            []
