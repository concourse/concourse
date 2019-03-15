module Login exposing (view)

import Concourse
import Html exposing (Html)
import Html.Attributes exposing (attribute, href, id, style)
import Html.Events exposing (onClick)
import ScreenSize exposing (ScreenSize(..))
import TopBar.Model exposing (MiddleSection(..))
import TopBar.Styles as Styles
import UserState exposing (UserState(..))


type Msg
    = LogIn
    | ToggleUserMenu
    | LogOut


view :
    UserState
    ->
        { a
            | screenSize : ScreenSize
            , middleSection : MiddleSection
            , isUserMenuExpanded : Bool
        }
    -> Bool
    -> Html Msg
view userState model isPaused =
    if showLogin model then
        Html.div [ id "login-component", style Styles.loginComponent ] <|
            viewLoginState userState model.isUserMenuExpanded isPaused

    else
        Html.text ""


showLogin :
    { a | middleSection : MiddleSection, screenSize : ScreenSize }
    -> Bool
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
                , attribute "aria-label" "Log In"
                , id "login-container"
                , onClick LogIn
                , style (Styles.loginContainer isPaused)
                ]
                [ Html.div
                    [ style Styles.loginItem
                    , id "login-item"
                    ]
                    [ Html.a
                        [ href "/sky/login" ]
                        [ Html.text "login" ]
                    ]
                ]
            ]

        UserStateLoggedIn user ->
            [ Html.div
                [ id "login-container"
                , onClick ToggleUserMenu
                , style (Styles.loginContainer isPaused)
                ]
                [ Html.div [ id "user-id", style Styles.loginItem ]
                    ([ Html.div
                        [ style Styles.loginText ]
                        [ Html.text (userDisplayName user) ]
                     ]
                        ++ (if isUserMenuExpanded then
                                [ Html.div
                                    [ id "logout-button"
                                    , style Styles.logoutButton
                                    , onClick LogOut
                                    ]
                                    [ Html.text "logout" ]
                                ]

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
            List.filter
                (not << String.isEmpty)
                [ user.userName, user.name, user.email ]
