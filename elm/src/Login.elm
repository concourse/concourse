module Login exposing (Model, Msg(..), init, update, view, subscriptions)

import String
import Html exposing (Html)
import Html.Attributes as Attributes exposing (id, class)
import Html.Events exposing (onInput, onSubmit)
import Http
import Navigation
import Task
import Concourse
import Concourse.AuthMethod
import Concourse.Login
import StrictEvents exposing (onLeftClick)


type alias Ports =
    { title : String -> Cmd Msg
    }


type alias BasicAuthFields =
    { username : String
    , password : String
    }


type alias Model =
    { teamName : String
    , authMethods : Maybe (List Concourse.AuthMethod)
    , hasTeamSelectionInBrowserHistory : Bool
    , redirect : Maybe String
    , basicAuthInput : Maybe BasicAuthFields
    , loginFailed : Bool
    }


type Msg
    = Noop
    | AuthFetched (Result Http.Error (List Concourse.AuthMethod))
    | NoAuthSubmit
    | BasicAuthUsernameChanged String
    | BasicAuthPasswordChanged String
    | BasicAuthSubmit
    | AuthSessionReceived (Result Http.Error Concourse.AuthSession)
    | GoBack


init : Ports -> String -> Maybe String -> ( Model, Cmd Msg )
init ports teamName redirect =
    ( { teamName = teamName
      , authMethods = Nothing
      , hasTeamSelectionInBrowserHistory = False
      , redirect = redirect
      , basicAuthInput = Nothing
      , loginFailed = False
      }
    , Cmd.batch
        [ Task.attempt AuthFetched <|
            Concourse.AuthMethod.fetchAll teamName
        , ports.title "Login - "
        ]
    )


update : Msg -> Model -> ( Model, Cmd Msg )
update action model =
    case action of
        Noop ->
            ( model, Cmd.none )

        AuthFetched (Ok authMethods) ->
            let
                newInputFields =
                    if List.member Concourse.AuthMethodBasic authMethods then
                        Just <|
                            { username = ""
                            , password = ""
                            }
                    else
                        Nothing
            in
                ( { model
                    | authMethods = Just authMethods
                    , basicAuthInput = newInputFields
                  }
                , Cmd.none
                )

        AuthFetched (Err err) ->
            flip always (Debug.log ("failed to fetch auth methods") (err)) <|
                ( model, Cmd.none )

        NoAuthSubmit ->
            ( model, noAuthSubmit model.teamName )

        AuthSessionReceived (Ok authsession) ->
            ( model
            , Navigation.newUrl (redirectUrl model.redirect)
            )

        AuthSessionReceived (Err err) ->
            flip always (Debug.log ("login failed") (err)) <|
                ( { model
                    | loginFailed = True
                  }
                , Cmd.none
                )

        BasicAuthUsernameChanged un ->
            ( case model.basicAuthInput of
                Nothing ->
                    flip always (Debug.log ("input to nonexistent UN field") ()) <|
                        model

                Just fields ->
                    { model
                        | basicAuthInput =
                            Just
                                { fields
                                    | username = un
                                }
                    }
            , Cmd.none
            )

        BasicAuthPasswordChanged pw ->
            ( case model.basicAuthInput of
                Nothing ->
                    flip always (Debug.log ("input to nonexistent PW field") ()) <|
                        model

                Just fields ->
                    { model
                        | basicAuthInput =
                            Just
                                { fields
                                    | password = pw
                                }
                    }
            , Cmd.none
            )

        BasicAuthSubmit ->
            ( model
            , case model.basicAuthInput of
                Nothing ->
                    Debug.log "tried to submit illegal basic auth"
                        Cmd.none

                Just fields ->
                    basicAuthSubmit model.teamName fields
            )

        GoBack ->
            case model.hasTeamSelectionInBrowserHistory of
                -- TODO this goes away?
                True ->
                    ( model, Navigation.back 1 )

                False ->
                    ( model, Navigation.newUrl <| teamSelectionRoute model.redirect )


redirectUrl : Maybe String -> String
redirectUrl redirectParam =
    Maybe.withDefault "/" redirectParam


teamSelectionRoute : Maybe String -> String
teamSelectionRoute redirectParam =
    -- TODO: Replace this back with Erl...if we ever go back (#134461889)
    case redirectParam of
        Nothing ->
            "/login"

        Just r ->
            "/login?redirect=" ++ r


routeWithRedirect : Maybe String -> String -> String
routeWithRedirect redirectParam route =
    let
        actualRedirect =
            case redirectParam of
                Nothing ->
                    indexPageUrl

                Just r ->
                    r
    in
        -- TODO: Replace this back with Erl...if we ever go back (#134461889)
        if List.length (String.split "?" route) == 2 then
            route ++ "&redirect=" ++ actualRedirect
        else
            route ++ "?redirect=" ++ actualRedirect


indexPageUrl : String
indexPageUrl =
    "/"


view : Model -> Html Msg
view model =
    Html.div [ class "login-page" ]
        [ Html.div
            [ class "small-title" ]
            [ Html.a
                [ onLeftClick GoBack
                , Attributes.href <| teamSelectionRoute model.redirect
                ]
                [ Html.i [ class "fa fa-fw fa-chevron-left" ] []
                , Html.text "back to team selection"
                ]
            ]
        , Html.div
            [ class "login-box auth-methods" ]
          <|
            [ Html.div
                [ class "auth-methods-title" ]
                [ Html.text "logging in to "
                , Html.span
                    [ class "bright-text" ]
                    [ Html.text model.teamName ]
                ]
            ]
                ++ loginMethods model
        ]


loginMethods : Model -> List (Html Msg)
loginMethods model =
    case model.authMethods of
        Nothing ->
            [ viewLoading ]

        Just methods ->
            case ( viewBasicAuthForm methods model.loginFailed, viewOAuthButtons model.redirect methods ) of
                ( Just basicForm, Just buttons ) ->
                    [ buttons, viewOrBar, basicForm ]

                ( Just basicForm, Nothing ) ->
                    [ basicForm ]

                ( Nothing, Just buttons ) ->
                    [ buttons ]

                ( Nothing, Nothing ) ->
                    [ viewNoAuthButton ]


viewLoading : Html Msg
viewLoading =
    Html.div [ class "loading" ]
        [ Html.i [ class "fa fa-fw fa-spin fa-circle-o-notch" ] []
        ]


loginErrMessage : Bool -> Html Msg
loginErrMessage loginFailed =
    if loginFailed then
        Html.div [ class "login-error" ] [ Html.text "login error: not authorized" ]
    else
        Html.div [] []


viewOrBar : Html Msg
viewOrBar =
    Html.div
        [ class "or-bar" ]
        [ Html.div [] []
        , Html.span [] [ Html.text "or" ]
        ]


viewNoAuthButton : Html Msg
viewNoAuthButton =
    Html.form
        [ class "auth-method login-button"
        ]
        [ Html.button
            [ onLeftClick NoAuthSubmit ]
            [ Html.text "login" ]
        ]


viewBasicAuthForm : List Concourse.AuthMethod -> Bool -> Maybe (Html Msg)
viewBasicAuthForm methods loginFailed =
    if List.member Concourse.AuthMethodBasic methods then
        Just <|
            Html.form
                [ class "auth-method basic-auth"
                ]
                [ Html.label
                    [ Attributes.for "basic-auth-username-input" ]
                    [ Html.text "username" ]
                , Html.div
                    [ class "input-holder" ]
                    [ Html.input
                        [ id "basic-auth-username-input"
                        , Attributes.type_ "text"
                        , Attributes.name "username"
                        , onInput BasicAuthUsernameChanged
                        , onSubmit BasicAuthSubmit
                        ]
                        []
                    ]
                , Html.label
                    [ Attributes.for "basic-auth-password-input" ]
                    [ Html.text "password" ]
                , Html.div [ class "input-holder" ]
                    -- for LastPass web UI
                    [ Html.input
                        [ id "basic-auth-password-input"
                        , Attributes.type_ "password"
                        , Attributes.name "password"
                        , onInput BasicAuthPasswordChanged
                        , onSubmit BasicAuthSubmit
                        ]
                        []
                    ]
                , loginErrMessage loginFailed
                , Html.div
                    [ class "login-button" ]
                    [ Html.button
                        [ onLeftClick BasicAuthSubmit ]
                        [ Html.text "login" ]
                    ]
                ]
    else
        Nothing


viewOAuthButtons : Maybe String -> List Concourse.AuthMethod -> Maybe (Html Msg)
viewOAuthButtons redirectParam methods =
    case List.filterMap (viewOAuthButton redirectParam) methods of
        [] ->
            Nothing

        buttons ->
            Just <|
                Html.div [ class "oauth-buttons" ] buttons


viewOAuthButton : Maybe String -> Concourse.AuthMethod -> Maybe (Html Msg)
viewOAuthButton redirect method =
    case method of
        Concourse.AuthMethodBasic ->
            Nothing

        Concourse.AuthMethodOAuth oAuthMethod ->
            Just <|
                Html.div [ class "auth-method login-button" ]
                    [ Html.a
                        [ Attributes.href <| routeWithRedirect redirect oAuthMethod.authUrl ]
                        [ Html.text <| "login with " ++ oAuthMethod.displayName ]
                    ]


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none


noAuthSubmit : String -> Cmd Msg
noAuthSubmit teamName =
    Task.attempt AuthSessionReceived <|
        Concourse.Login.noAuth teamName


basicAuthSubmit : String -> BasicAuthFields -> Cmd Msg
basicAuthSubmit teamName fields =
    Task.attempt AuthSessionReceived <|
        Concourse.Login.basicAuth teamName fields.username fields.password
