port module TopBar exposing (Model, Msg(..), fetchUser, init, subscriptions, update, urlUpdate, view, userDisplayName)

import Concourse
import Concourse.Pipeline
import Concourse.User
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, disabled, href, id, style)
import Html.Events exposing (onClick)
import Http
import LoginRedirect
import Navigation exposing (Location)
import Pipeline
import Routes
import StrictEvents exposing (onLeftClickOrShiftLeftClick)
import Task
import Time


type alias Model =
    { route : Routes.ConcourseRoute
    , pipeline : Maybe Concourse.Pipeline
    , userState : UserState
    , userMenuVisible : Bool
    }


type UserState
    = UserStateLoggedIn Concourse.User
    | UserStateLoggedOut
    | UserStateUnknown


type Msg
    = Noop
    | PipelineFetched (Result Http.Error Concourse.Pipeline)
    | UserFetched (Result Http.Error Concourse.User)
    | FetchUser Time.Time
    | FetchPipeline Concourse.PipelineIdentifier
    | ToggleSidebar
    | LogOut
    | LogIn
    | ResetToPipeline String
    | LoggedOut (Result Http.Error ())
    | ToggleUserMenu


init : Routes.ConcourseRoute -> ( Model, Cmd Msg )
init route =
    let
        pid =
            extractPidFromRoute route.logical
    in
        ( { route = route
          , pipeline = Nothing
          , userState = UserStateUnknown
          , userMenuVisible = False
          }
        , case pid of
            Nothing ->
                fetchUser

            Just pid ->
                Cmd.batch [ fetchPipeline pid, fetchUser ]
        )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Noop ->
            ( model, Cmd.none )

        FetchPipeline pid ->
            ( model, fetchPipeline pid )

        FetchUser _ ->
            ( model, fetchUser )

        UserFetched (Ok user) ->
            ( { model | userState = UserStateLoggedIn user }
            , Cmd.none
            )

        UserFetched (Err _) ->
            ( { model | userState = UserStateLoggedOut }
            , Cmd.none
            )

        PipelineFetched (Ok pipeline) ->
            ( { model
                | pipeline = Just pipeline
              }
            , Cmd.none
            )

        PipelineFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, LoginRedirect.requestLoginRedirect "" )
                    else
                        ( model, Cmd.none )

                _ ->
                    ( model, Cmd.none )

        ToggleSidebar ->
            flip always (Debug.log "sidebar-toggle-incorrectly-handled" ()) <|
                ( model, Cmd.none )

        LogIn ->
            ( { model
                | pipeline = Nothing
              }
            , LoginRedirect.requestLoginRedirect ""
            )

        LogOut ->
            ( model, logOut )

        LoggedOut (Ok _) ->
            ( { model
                | userState = UserStateLoggedOut
                , pipeline = Nothing
              }
            , Navigation.newUrl "/"
            )

        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, Cmd.none )

        ResetToPipeline url ->
            ( model, Cmd.batch [ Navigation.newUrl url, Pipeline.resetPipelineFocus () ] )

        ToggleUserMenu ->
            ( { model | userMenuVisible = not model.userMenuVisible }, Cmd.none )


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ case pipelineIdentifierFromRouteOrModel model.route model of
            Nothing ->
                Sub.none

            Just pid ->
                Time.every (5 * Time.second) (always (FetchPipeline pid))
        , Time.every (5 * Time.second) FetchUser
        ]


pipelineIdentifierFromRouteOrModel : Routes.ConcourseRoute -> Model -> Maybe Concourse.PipelineIdentifier
pipelineIdentifierFromRouteOrModel route model =
    case extractPidFromRoute route.logical of
        Nothing ->
            case model.pipeline of
                Nothing ->
                    Nothing

                Just pipeline ->
                    Just { teamName = pipeline.teamName, pipelineName = pipeline.name }

        Just pidFromRoute ->
            Just pidFromRoute


extractPidFromRoute : Routes.Route -> Maybe Concourse.PipelineIdentifier
extractPidFromRoute route =
    case route of
        Routes.Build teamName pipelineName jobName buildName ->
            Just { teamName = teamName, pipelineName = pipelineName }

        Routes.Job teamName pipelineName jobName ->
            Just { teamName = teamName, pipelineName = pipelineName }

        Routes.Resource teamName pipelineName resourceName ->
            Just { teamName = teamName, pipelineName = pipelineName }

        Routes.OneOffBuild buildId ->
            Nothing

        Routes.Pipeline teamName pipelineName ->
            Just { teamName = teamName, pipelineName = pipelineName }

        Routes.Dashboard ->
            Nothing

        Routes.DashboardHd ->
            Nothing


urlUpdate : Routes.ConcourseRoute -> Model -> ( Model, Cmd Msg )
urlUpdate route model =
    let
        pipelineIdentifier =
            pipelineIdentifierFromRouteOrModel route model
    in
        ( { model
            | route = route
          }
        , case pipelineIdentifier of
            Nothing ->
                fetchUser

            Just pid ->
                Cmd.batch [ fetchPipeline pid, fetchUser ]
        )


view : Model -> Html Msg
view model =
    Html.nav
        [ classList
            [ ( "module-topbar", True )
            , ( "top-bar", True )
            , ( "test", True )
            , ( "paused", isPaused model.pipeline )
            ]
        ]
        [ Html.div [ class "topbar-logo" ] [ Html.a [ class "logo-image-link", href "/" ] [] ]
        , Html.ul [ class "groups" ] <|
            [ Html.li [ class "main" ]
                [ Html.span
                    [ class "sidebar-toggle test btn-hamburger"
                    , onClick ToggleSidebar
                    , Html.Attributes.attribute "aria-label" "Toggle List of Pipelines"
                    ]
                    [ Html.i [ class "fa fa-bars" ] []
                    ]
                ]
            ]
                ++ viewBreadcrumbs model
        , Html.div [ class "topbar-login" ]
            [ Html.div [ class "topbar-user-info" ]
                [ viewUserState model.userState model.userMenuVisible
                ]
            ]
        ]


viewBreadcrumbs : Model -> List (Html Msg)
viewBreadcrumbs model =
    List.intersperse viewBreadcrumbSeparator <|
        case model.route.logical of
            Routes.Pipeline teamName pipelineName ->
                [ viewBreadcrumbPipeline pipelineName model.route.logical ]

            Routes.Job teamName pipelineName jobName ->
                [ viewBreadcrumbPipeline pipelineName <| Routes.Pipeline teamName pipelineName
                , viewBreadcrumbJob jobName
                ]

            Routes.Build teamName pipelineName jobName buildName ->
                [ viewBreadcrumbPipeline pipelineName <| Routes.Pipeline teamName pipelineName
                , viewBreadcrumbJob jobName
                ]

            Routes.Resource teamName pipelineName resourceName ->
                [ viewBreadcrumbPipeline pipelineName <| Routes.Pipeline teamName pipelineName
                , viewBreadcrumbResource resourceName
                ]

            _ ->
                []


viewBreadcrumbSeparator : Html Msg
viewBreadcrumbSeparator =
    Html.li [ class "nav-item" ] [ Html.text "/" ]


viewBreadcrumbPipeline : String -> Routes.Route -> Html Msg
viewBreadcrumbPipeline pipelineName route =
    let
        url =
            Routes.toString route
    in
        Html.li [ class "nav-item" ]
            [ Html.a
                [ StrictEvents.onLeftClick <| ResetToPipeline url
                , href url
                ]
                [ Html.div [ class "breadcrumb-icon breadcrumb-pipeline-icon" ] []
                , Html.text <| decodeName pipelineName
                ]
            ]


viewBreadcrumbJob : String -> Html Msg
viewBreadcrumbJob name =
    Html.li [ class "nav-item" ]
        [ Html.div [ class "breadcrumb-icon breadcrumb-job-icon" ] []
        , Html.text <| decodeName name
        ]


viewBreadcrumbResource : String -> Html Msg
viewBreadcrumbResource name =
    Html.li [ class "nav-item" ]
        [ Html.div [ class "breadcrumb-icon breadcrumb-resource-icon" ] []
        , Html.text <| decodeName name
        ]


decodeName : String -> String
decodeName name =
    Maybe.withDefault name (Http.decodeUri name)


isPaused : Maybe Concourse.Pipeline -> Bool
isPaused =
    Maybe.withDefault False << Maybe.map .paused


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


userDisplayName : Concourse.User -> String
userDisplayName user =
    Maybe.withDefault user.id <|
        List.head <|
            List.filter (not << String.isEmpty) [ user.userName, user.name, user.email ]


fetchPipeline : Concourse.PipelineIdentifier -> Cmd Msg
fetchPipeline pipelineIdentifier =
    Task.attempt PipelineFetched <|
        Concourse.Pipeline.fetchPipeline pipelineIdentifier


fetchUser : Cmd Msg
fetchUser =
    Task.attempt UserFetched Concourse.User.fetchUser


logOut : Cmd Msg
logOut =
    Task.attempt LoggedOut Concourse.User.logOut
