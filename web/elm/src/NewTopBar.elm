module NewTopBar
    exposing
        ( Model
        , autocompleteOptions
        , query
        , view
        , logOut
        , queryStringFromSearch
        )

import Concourse.User
import Dashboard.Msgs exposing (Msg(..))
import Dashboard.Group as Group
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
import ScreenSize exposing (ScreenSize(..))
import SearchBar exposing (SearchBar(..))
import Task
import TopBar exposing (userDisplayName)
import UserState exposing (UserState(..))


type alias Model r =
    { r
        | userState : UserState
        , userMenuVisible : Bool
        , searchBar : SearchBar
        , groups : List Group.Group
        , screenSize : ScreenSize
        , highDensity : Bool
    }


query : Model r -> String
query model =
    case model.searchBar of
        Expanded r ->
            r.query

        _ ->
            ""


queryStringFromSearch : String -> String
queryStringFromSearch query =
    case query of
        "" ->
            QueryString.render QueryString.empty

        query ->
            QueryString.render <|
                QueryString.add "search" query QueryString.empty


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
    if hideSearch model then
        []
    else
        case model.searchBar of
            Collapsed ->
                [ Html.div [ css <| Styles.middleSection model ]
                    [ Html.a
                        [ id "search-button"
                        , onClick ShowSearchInput
                        , css Styles.searchButton
                        ]
                        []
                    ]
                ]

            Expanded r ->
                [ Html.div [ css <| Styles.middleSection model ] <|
                    (searchInput { query = r.query, screenSize = model.screenSize }
                        ++ (if r.showAutocomplete then
                                [ Html.ul
                                    [ css <| Styles.searchOptionsList model.screenSize ]
                                    (viewAutocomplete
                                        { query = r.query
                                        , groups = model.groups
                                        , selectionMade = r.selectionMade
                                        , selection = r.selection
                                        , screenSize = model.screenSize
                                        }
                                    )
                                ]
                            else
                                []
                           )
                    )
                ]


hideSearch : { a | highDensity : Bool, groups : List Group.Group } -> Bool
hideSearch model =
    model.highDensity
        || List.isEmpty (model.groups |> List.concatMap .pipelines)


viewAutocomplete :
    { a
        | query : String
        , groups : List Group.Group
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
        Expanded _ ->
            case model.screenSize of
                Mobile ->
                    []

                Desktop ->
                    [ Html.div [ css Styles.userInfo ] (viewUserState model) ]

                BigDesktop ->
                    [ Html.div [ css Styles.userInfo ] (viewUserState model) ]

        _ ->
            [ Html.div [ css Styles.userInfo ] (viewUserState model) ]


autocompleteOptions : { a | query : String, groups : List Group.Group } -> List String
autocompleteOptions { query, groups } =
    case String.trim query of
        "" ->
            [ "status: ", "team: " ]

        "status:" ->
            [ "status: paused", "status: pending", "status: failed", "status: errored", "status: aborted", "status: running", "status: succeeded" ]

        "team:" ->
            List.map (\group -> "team: " ++ group.teamName) <| List.take 10 groups

        _ ->
            []


logOut : Cmd Msg
logOut =
    Task.attempt LoggedOut Concourse.User.logOut
