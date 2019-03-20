module Login exposing (view)

import Concourse
import Html exposing (Html)
import Html.Attributes exposing (attribute, href, id, style)
import Html.Events exposing (onClick)
import Message.Message exposing (Message(..))
import ScreenSize exposing (ScreenSize(..))
import TopBar.Model exposing (MiddleSection(..), Model, middleSection)
import TopBar.Styles as Styles
import UserState exposing (UserState(..))


view :
    UserState
    -> Model r
    -> Bool
    -> Html Message
view userState model isPaused =
    if showLogin model then
        Html.div [ id "login-component", style Styles.loginComponent ] <|
            viewLoginState userState model.isUserMenuExpanded isPaused

    else
        Html.text ""


showLogin :
    Model r
    -> Bool
showLogin model =
    case middleSection model of
        SearchBar ->
            model.screenSize /= Mobile

        _ ->
            True


viewLoginState : UserState -> Bool -> Bool -> List (Html Message)
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
