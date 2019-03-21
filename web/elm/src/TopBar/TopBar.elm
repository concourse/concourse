module TopBar.TopBar exposing
    ( handleCallback
    , init
    , queryStringFromSearch
    , update
    , viewBreadcrumbs
    , viewConcourseLogo
    )

import Concourse
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
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Hoverable(..), Message(..))
import QueryString
import Routes
import ScreenSize exposing (ScreenSize(..))
import TopBar.Model exposing (Model)
import TopBar.Styles as Styles
import UserState exposing (UserState(..))


init : ( Model {}, List Effect )
init =
    ( { isUserMenuExpanded = False
      , groups = []
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
        LoggedOut (Err err) ->
            flip always (Debug.log "failed to log out" err) <|
                ( model, effects )

        _ ->
            ( model, effects )


update : Message -> ET (Model r)
update msg ( model, effects ) =
    case msg of
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
