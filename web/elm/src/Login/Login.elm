module Login.Login exposing (Model, tooltip, update, userDisplayName, view)

import Concourse
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (attribute, href, id)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Login.Styles as Styles
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Tooltip
import UserState exposing (UserState(..))


type alias Model r =
    { r | isUserMenuExpanded : Bool }


update : Message -> ET (Model r)
update msg ( model, effects ) =
    case msg of
        Click LoginButton ->
            ( model, effects ++ [ RedirectToLogin ] )

        Click LogoutButton ->
            ( model, effects ++ [ SendLogOutRequest ] )

        Click UserMenu ->
            ( { model | isUserMenuExpanded = not model.isUserMenuExpanded }
            , effects
            )

        _ ->
            ( model, effects )


tooltip : String -> Maybe Tooltip.Tooltip
tooltip username =
    Just
        { body = Html.text username
        , attachPosition =
            { direction = Tooltip.Bottom
            , alignment = Tooltip.End
            }
        , arrow = Just 5
        , containerAttrs = Nothing
        }


view : UserState -> Model r -> Html Message
view userState model =
    Html.div
        (id "login-component" :: Styles.loginComponent)
        (viewLoginState userState model.isUserMenuExpanded)


viewLoginState : UserState -> Bool -> List (Html Message)
viewLoginState userState isUserMenuExpanded =
    case userState of
        UserStateUnknown ->
            []

        UserStateLoggedOut ->
            [ Html.div
                ([ href "/sky/login"
                 , attribute "aria-label" "Log In"
                 , id "login-container"
                 , onClick <| Click LoginButton
                 ]
                    ++ Styles.loginContainer
                )
                [ Html.div
                    (id "login-item" :: Styles.loginItem)
                    [ Html.a
                        [ href "/sky/login" ]
                        [ Html.text "login" ]
                    ]
                ]
            ]

        UserStateLoggedIn user ->
            let
                displayName =
                    userDisplayName user
            in
            [ Html.div
                ([ id "login-container"
                 , onClick <| Click UserMenu
                 ]
                    ++ Styles.loginContainer
                )
                [ Html.div
                    (id "user-id"
                        :: Styles.loginItem
                        ++ [ onMouseEnter <| Hover <| Just (UserDisplayName displayName)
                           , onMouseLeave <| Hover Nothing
                           ]
                    )
                    (Html.text displayName
                        :: (if isUserMenuExpanded then
                                [ Html.div
                                    ([ id "logout-button"
                                     , onClick <| Click LogoutButton
                                     ]
                                        ++ Styles.logoutButton
                                    )
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
                [ user.displayUserId, user.userName, user.name, user.email ]
