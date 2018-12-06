module NewTopBar
    exposing
        ( Model
        , autocompleteOptions
        , init
        , query
        , view
        , logOut
        , queryStringFromSearch
        , showSearchInput
        )

import Concourse
import Concourse.Team
import Concourse.User
import Dashboard.Msgs exposing (Msg(..))
import Dom
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes as HA
    exposing
        ( css
        , href
        , id
        , placeholder
        , src
        , type_
        , value
        )
import Html.Styled.Events exposing (..)
import NewTopBar.Styles as Styles
import QueryString
import RemoteData exposing (RemoteData)
import ScreenSize exposing (ScreenSize(..))
import SearchBar exposing (SearchBar(..))
import Task
import TopBar exposing (userDisplayName)
import UserState exposing (UserState(..))
import Window


type alias Model r =
    { r
        | userState : UserState
        , userMenuVisible : Bool
        , searchBar : SearchBar
        , teams : RemoteData.WebData (List Concourse.Team)
    }


query : Model r -> String
query model =
    case model.searchBar of
        Expanded r ->
            r.query

        _ ->
            ""


init : Bool -> String -> ( Model {}, Cmd Msg )
init showSearch query =
    let
        searchBar =
            if showSearch then
                Expanded
                    { query = query
                    , selectionMade = False
                    , showAutocomplete = False
                    , selection = 0
                    , screenSize = Desktop
                    }
            else
                Invisible
    in
        ( { userState = UserStateUnknown
          , userMenuVisible = False
          , searchBar = searchBar
          , teams = RemoteData.Loading
          }
        , Cmd.batch
            [ fetchTeams
            , Task.perform ScreenResized Window.size
            ]
        )


queryStringFromSearch : String -> String
queryStringFromSearch query =
    case query of
        "" ->
            QueryString.render QueryString.empty

        query ->
            QueryString.render <|
                QueryString.add "search" query QueryString.empty


showSearchInput : Model r -> ( Model r, Cmd Msg )
showSearchInput model =
    let
        newModel =
            { model
                | searchBar =
                    Expanded
                        { query = ""
                        , selectionMade = False
                        , showAutocomplete = False
                        , selection = 0
                        , screenSize = Mobile
                        }
            }
    in
        case model.searchBar of
            Collapsed ->
                ( newModel, Task.attempt (always Noop) (Dom.focus "search-input-field") )

            _ ->
                ( model, Cmd.none )


viewUserState : { a | userState : UserState, userMenuVisible : Bool } -> List (Html Msg)
viewUserState { userState, userMenuVisible } =
    case userState of
        UserStateUnknown ->
            []

        UserStateLoggedOut ->
            [ Html.div
                [ href "/sky/login"
                , HA.attribute "aria-label" "Log In"
                , id "login-button"
                , onClick LogIn
                , css Styles.menuButton
                ]
                [ Html.div [] [ Html.text "login" ] ]
            ]

        UserStateLoggedIn user ->
            [ Html.div
                [ id "user-id"
                , onClick ToggleUserMenu
                , css Styles.menuButton
                ]
                [ Html.div [ css Styles.userName ] [ Html.text (userDisplayName user) ] ]
            ]
                ++ (if userMenuVisible then
                        [ Html.div
                            [ HA.attribute "aria-label" "Log Out"
                            , onClick LogOut
                            , css Styles.logoutButton
                            , id "logout-button"
                            ]
                            [ Html.div [] [ Html.text "logout" ] ]
                        ]
                    else
                        []
                   )


searchInput : { a | query : String, screenSize : ScreenSize } -> List (Html Msg)
searchInput { query, screenSize } =
    [ Html.div [ css Styles.searchForm ] <|
        [ Html.input
            [ id "search-input-field"
            , type_ "text"
            , placeholder "search"
            , onInput FilterMsg
            , onFocus FocusMsg
            , onBlur BlurMsg
            , value query
            , css <| Styles.searchInput screenSize
            ]
            []
        , Html.span
            [ css <| Styles.searchClearButton (not <| String.isEmpty query)
            , id "search-clear-button"
            , onClick (FilterMsg "")
            ]
            []
        ]
    ]


view : Model r -> Html Msg
view model =
    Html.div [ css Styles.topBar ] <|
        viewConcourseLogo
            ++ viewMiddleSection model
            ++ viewUserInfo model


viewConcourseLogo : List (Html Msg)
viewConcourseLogo =
    [ Html.a
        [ css Styles.concourseLogo, href "#" ]
        []
    ]


viewMiddleSection : Model r -> List (Html Msg)
viewMiddleSection model =
    case model.searchBar of
        Invisible ->
            []

        Collapsed ->
            [ Html.div [ css <| Styles.middleSection model.searchBar ]
                [ Html.a
                    [ id "search-button"
                    , onClick ShowSearchInput
                    , css Styles.searchButton
                    ]
                    []
                ]
            ]

        Expanded r ->
            [ Html.div [ css <| Styles.middleSection model.searchBar ] <|
                (searchInput r
                    ++ (if r.showAutocomplete then
                            [ Html.ul
                                [ css <| Styles.searchOptionsList r.screenSize ]
                                (viewAutocomplete
                                    { query = r.query
                                    , teams = model.teams
                                    , selectionMade = r.selectionMade
                                    , selection = r.selection
                                    , screenSize = r.screenSize
                                    }
                                )
                            ]
                        else
                            []
                       )
                )
            ]


viewAutocomplete :
    { a
        | query : String
        , teams : RemoteData.WebData (List Concourse.Team)
        , selectionMade : Bool
        , selection : Int
        , screenSize : ScreenSize
    }
    -> List (Html Msg)
viewAutocomplete r =
    let
        options =
            autocompleteOptions r
    in
        options
            |> List.indexedMap
                (\index option ->
                    let
                        active =
                            r.selectionMade && index == (r.selection - 1) % List.length options
                    in
                        Html.li
                            [ onMouseDown (FilterMsg option)
                            , onMouseOver (SelectMsg index)
                            , css <| Styles.searchOption { screenSize = r.screenSize, active = active }
                            ]
                            [ Html.text option ]
                )


viewUserInfo : Model r -> List (Html Msg)
viewUserInfo model =
    case model.searchBar of
        Expanded r ->
            case r.screenSize of
                Mobile ->
                    []

                Desktop ->
                    [ Html.div [ css Styles.userInfo ] (viewUserState model) ]

                BigDesktop ->
                    [ Html.div [ css Styles.userInfo ] (viewUserState model) ]

        _ ->
            [ Html.div [ css Styles.userInfo ] (viewUserState model) ]


fetchTeams : Cmd Msg
fetchTeams =
    Cmd.map TeamsFetched <|
        RemoteData.asCmd Concourse.Team.fetchTeams


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


logOut : Cmd Msg
logOut =
    Task.attempt LoggedOut Concourse.User.logOut
