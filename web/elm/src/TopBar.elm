port module TopBar exposing (Model, Msg(..), fetchUser, init, subscriptions, update, urlUpdate, view, userDisplayName)

import Concourse
import Concourse.Pipeline
import Concourse.User
import Colors
import Dict
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, disabled, href, id, style)
import Html.Events exposing (onClick, onMouseOver, onMouseOut, onMouseEnter, onMouseLeave)
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
    , pinnedResources : List ( String, Concourse.Version )
    , showPinIconDropDown : Bool
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
    | LogOut
    | LogIn
    | ResetToPipeline String
    | LoggedOut (Result Http.Error ())
    | ToggleUserMenu
    | TogglePinIconDropdown
    | GoToPinnedResource String


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
          , pinnedResources = []
          , showPinIconDropDown = False
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

        TogglePinIconDropdown ->
            ( { model | showPinIconDropDown = not model.showPinIconDropDown }, Cmd.none )

        GoToPinnedResource resourceName ->
            let
                url =
                    Routes.toString model.route.logical
            in
                ( model, Navigation.newUrl (url ++ "/resources/" ++ resourceName) )


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
    Html.div
        [ id "top-bar-app"
        , style
            [ ( "height", "56px" )
            , ( "background-color"
              , if isPaused model.pipeline then
                    "#3498db"
                else
                    "#1e1d1d"
              )
            , ( "display", "flex" )
            , ( "align-items", "center" )
            , ( "justify-content", "space-between" )
            , ( "z-index", "100" )
            , ( "left", "0" )
            , ( "right", "0" )
            , ( "position", "fixed" )
            ]
        , id "top-bar-app"
        ]
        [ Html.nav
            [ style [ ( "display", "flex" ) ] ]
            [ Html.a [ href "/" ]
                [ Html.div
                    [ style
                        [ ( "background-image", "url(/public/images/concourse_logo_white.svg)" )
                        , ( "background-position", "50% 50%" )
                        , ( "background-repeat", "no-repeat" )
                        , ( "background-size", "42px 42px" )
                        , ( "width", "54px" )
                        , ( "height", "54px" )
                        ]
                    ]
                    []
                ]
            , Html.ul [ class "groups" ] <| viewBreadcrumbs model
            ]
        , Html.nav
            [ style [ ( "display", "flex" ) ] ]
            ((case model.route.logical of
                Routes.Pipeline _ _ ->
                    [ Html.div
                        ([ style [ ( "margin-right", "15px" ) ]
                         , id "pin-icon"
                         ]
                            ++ (if model.showPinIconDropDown then
                                    [ style
                                        [ ( "background-color", "#3d3c3c" )
                                        , ( "border-radius", "50%" )
                                        ]
                                    ]
                                else
                                    []
                               )
                        )
                        [ Html.div
                            ([ style
                                ([ ( "background-image"
                                   , if List.length model.pinnedResources > 0 then
                                        "url(/public/images/pin_ic_white.svg)"
                                     else
                                        "url(/public/images/pin_ic_grey.svg)"
                                   )
                                 , ( "width", "40px" )
                                 , ( "height", "40px" )
                                 , ( "background-repeat", "no-repeat" )
                                 , ( "background-position", "50% 50%" )
                                 , ( "position", "relative" )
                                 , ( "top", "10px" )
                                 ]
                                )
                             ]
                                ++ (if List.length model.pinnedResources > 0 then
                                        [ onMouseEnter TogglePinIconDropdown
                                        , onMouseLeave TogglePinIconDropdown
                                        ]
                                    else
                                        []
                                   )
                            )
                            (if List.length model.pinnedResources > 0 then
                                ([ Html.div
                                    [ style
                                        [ ( "background-color", Colors.pinned )
                                        , ( "border-radius", "50%" )
                                        , ( "width", "15px" )
                                        , ( "height", "15px" )
                                        , ( "position", "absolute" )
                                        , ( "top", "3px" )
                                        , ( "right", "3px" )
                                        , ( "display", "flex" )
                                        , ( "align-items", "center" )
                                        , ( "justify-content", "center" )
                                        ]
                                    , id "pin-badge"
                                    ]
                                    [ Html.div [] [ Html.text <| toString <| List.length model.pinnedResources ]
                                    ]
                                 ]
                                    ++ (if model.showPinIconDropDown then
                                            [ Html.ul
                                                [ style
                                                    [ ( "background-color", "#fff" )
                                                    , ( "color", "#1e1d1d" )
                                                    , ( "position", "absolute" )
                                                    , ( "top", "100%" )
                                                    , ( "right", "0" )
                                                    , ( "white-space", "nowrap" )
                                                    , ( "list-style-type", "none" )
                                                    , ( "padding", "10px" )
                                                    , ( "margin-top", "0" )
                                                    , ( "z-index", "1" )
                                                    ]
                                                ]
                                                (model.pinnedResources
                                                    |> List.map
                                                        (\( resourceName, pinnedVersion ) ->
                                                            Html.li
                                                                [ onClick (GoToPinnedResource resourceName)
                                                                , style
                                                                    [ ( "cursor", "pointer" )
                                                                    ]
                                                                ]
                                                                [ Html.div
                                                                    [ style [ ( "font-weight", "700" ) ] ]
                                                                    [ Html.text resourceName ]
                                                                , Html.table []
                                                                    (pinnedVersion
                                                                        |> Dict.toList
                                                                        |> List.map
                                                                            (\( k, v ) ->
                                                                                Html.tr []
                                                                                    [ Html.td [] [ Html.text k ]
                                                                                    , Html.td [] [ Html.text v ]
                                                                                    ]
                                                                            )
                                                                    )
                                                                ]
                                                        )
                                                )
                                            , Html.div
                                                [ style
                                                    [ ( "border-width", "5px" )
                                                    , ( "border-style", "solid" )
                                                    , ( "border-color", "transparent transparent #fff transparent" )
                                                    , ( "position", "absolute" )
                                                    , ( "top", "100%" )
                                                    , ( "right", "50%" )
                                                    , ( "margin-right", "-5px" )
                                                    , ( "margin-top", "-10px" )
                                                    ]
                                                ]
                                                []
                                            ]
                                        else
                                            []
                                       )
                                )
                             else
                                []
                            )
                        ]
                    ]

                _ ->
                    []
             )
                ++ [ viewUserState model.userState model.userMenuVisible
                   ]
            )
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
    Html.li [ style cssBreadcrumbContainer ] [ Html.text "/" ]


viewBreadcrumbsComponent : String -> String -> List (Html Msg)
viewBreadcrumbsComponent componentType name =
    [ Html.div
        [ style <|
            [ ( "background-image", "url(/public/images/ic_breadcrumb_" ++ componentType ++ ".svg)" )
            , ( "background-repeat", "no-repeat" )
            , ( "background-size", "contain" )
            , ( "display", "inline-block" )
            , ( "vertical-align", "middle" )
            , ( "height", "16px" )
            , ( "width", "32px" )
            , ( "margin-right", "10px" )
            ]
        ]
        []
    , Html.text <| decodeName name
    ]


cssBreadcrumbContainer : List ( String, String )
cssBreadcrumbContainer =
    [ ( "display", "inline-block" ), ( "vertical-align", "middle" ), ( "font-size", "18px" ), ( "padding", "0 10px" ), ( "line-height", "54px" ) ]


viewBreadcrumbPipeline : String -> Routes.Route -> Html Msg
viewBreadcrumbPipeline pipelineName route =
    let
        url =
            Routes.toString route
    in
        Html.li [ style cssBreadcrumbContainer ]
            [ Html.a
                [ StrictEvents.onLeftClick <| ResetToPipeline url
                , href url
                ]
              <|
                viewBreadcrumbsComponent "pipeline" pipelineName
            ]


viewBreadcrumbJob : String -> Html Msg
viewBreadcrumbJob name =
    Html.li [ style cssBreadcrumbContainer ]
        [ Html.div [] <|
            viewBreadcrumbsComponent "job" name
        ]


viewBreadcrumbResource : String -> Html Msg
viewBreadcrumbResource name =
    Html.li [ style cssBreadcrumbContainer ]
        [ Html.div [] <|
            viewBreadcrumbsComponent "resource" name
        ]


decodeName : String -> String
decodeName name =
    Maybe.withDefault name (Http.decodeUri name)


isPaused : Maybe Concourse.Pipeline -> Bool
isPaused =
    Maybe.withDefault False << Maybe.map .paused


cssUserContainer : List ( String, String )
cssUserContainer =
    [ ( "position", "relative" )
    , ( "display", "flex" )

    -- , ( "max-width", "20%" )
    , ( "flex-direction", "column" )
    , ( "border-left", "1px solid #3d3c3c" )
    , ( "line-height", "56px" )
    ]


cssUserName : List ( String, String )
cssUserName =
    [ ( "padding", "0 30px" )
    , ( "cursor", "pointer" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    , ( "flex-grow", "1" )
    ]


viewUserState : UserState -> Bool -> Html Msg
viewUserState userState userMenuVisible =
    case userState of
        UserStateUnknown ->
            Html.text ""

        UserStateLoggedOut ->
            Html.div
                [ onClick LogIn
                , style cssUserContainer
                ]
                [ Html.a
                    [ href "/sky/login"
                    , Html.Attributes.attribute "aria-label" "Log In"
                    , style cssUserName
                    ]
                    [ Html.text "login"
                    ]
                ]

        UserStateLoggedIn user ->
            Html.div
                [ style cssUserContainer ]
                [ Html.div
                    [ style cssUserName
                    , onClick ToggleUserMenu
                    ]
                    [ Html.text (userDisplayName user)
                    , (if userMenuVisible then
                        Html.div
                            [ attribute "aria-label" "Log Out"
                            , style
                                [ ( "position", "absolute" )
                                , ( "top", "55px" )
                                , ( "background-color", "#1e1d1d" )
                                , ( "height", "54px" )
                                , ( "width", "100%" )
                                , ( "border-top", "1px solid #3d3c3c" )
                                , ( "cursor", "pointer" )
                                , ( "display", "flex" )
                                , ( "align-items", "center" )
                                , ( "justify-content", "center" )
                                , ( "flex-grow", "1" )
                                ]
                            , onClick LogOut
                            , id "logout-button"
                            ]
                            [ Html.div [] [ Html.text "logout" ] ]
                       else
                        Html.text ""
                      )
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
