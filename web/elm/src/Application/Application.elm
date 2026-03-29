module Application.Application exposing
    ( Flags
    , Model
    , handleCallback
    , handleDelivery
    , init
    , locationMsg
    , subscriptions
    , update
    , view
    )

import Application.Models exposing (Session)
import Application.Styles as Styles
import Assets
import Browser
import Colors
import Concourse
import EffectTransformer exposing (ET)
import HoverState
import Html
import Html.Attributes exposing (class, disabled, href, id, placeholder, rel, style, target, type_, value)
import Html.Events exposing (onClick, onInput, onMouseEnter, onMouseLeave)
import Http
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects exposing (Effect(..))
import Message.Message as Message
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Message.TopLevelMessage as Msgs exposing (TopLevelMessage(..))
import RemoteData
import Routes
import ScreenSize
import Set
import SideBar.SideBar as SideBar
import SubPage.SubPage as SubPage
import Time
import Tooltip
import Url
import UserState exposing (UserState(..))
import Views.Icon as Icon
import Views.Styles


type alias Flags =
    { turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : Concourse.CSRFToken
    , authToken : String
    , pipelineRunningKeyframes : String
    , featureFlags : Concourse.FeatureFlags
    }


type alias WallEditor =
    { isOpen : Bool
    , message : String
    }


type alias Model =
    { subModel : SubPage.Model
    , session : Session
    , wallMessage : Maybe String
    , wallEditor : WallEditor
    }


init : Flags -> Url.Url -> ( Model, List Effect )
init flags url =
    let
        route =
            Routes.parsePath url
                |> Maybe.withDefault
                    (Routes.Dashboard
                        { searchType = Routes.Normal ""
                        , dashboardView = Routes.ViewNonArchivedPipelines
                        }
                    )

        session =
            { userState = UserStateUnknown
            , hovered = HoverState.NoHover
            , clusterName = ""
            , version = ""
            , featureFlags = flags.featureFlags
            , turbulenceImgSrc = flags.turbulenceImgSrc
            , notFoundImgSrc = flags.notFoundImgSrc
            , csrfToken = flags.csrfToken
            , authToken = flags.authToken
            , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
            , expandedTeamsInAllPipelines = Set.empty
            , collapsedTeamsInFavorites = Set.empty
            , pipelines = RemoteData.NotAsked
            , sideBarState =
                { isOpen = False
                , width = 275
                }
            , draggingSideBar = False
            , screenSize = ScreenSize.Desktop
            , timeZone = Time.utc
            , favoritedPipelines = Set.empty
            , favoritedInstanceGroups = Set.empty
            , route = route
            }

        ( subModel, subEffects ) =
            SubPage.init session route

        model =
            { subModel = subModel
            , session = session
            , wallMessage = Nothing
            , wallEditor =
                { isOpen = False
                , message = ""
                }
            }

        handleTokenEffect =
            -- We've refreshed on the page and we're not
            -- getting it from query params
            if flags.csrfToken == "" then
                [ LoadToken ]

            else
                [ SaveToken flags.csrfToken
                , Effects.ModifyUrl <| Routes.toString route
                ]
    in
    ( model
    , [ FetchUser
      , GetScreenSize
      , LoadSideBarState
      , LoadFavoritedPipelines
      , LoadFavoritedInstanceGroups
      , FetchClusterInfo
      , FetchWall
      ]
        ++ handleTokenEffect
        ++ subEffects
    )


locationMsg : Url.Url -> TopLevelMessage
locationMsg url =
    case Routes.parsePath url of
        Just route ->
            DeliveryReceived <| RouteChanged route

        Nothing ->
            Msgs.Callback EmptyCallback


handleCallback : Callback -> Model -> ( Model, List Effect )
handleCallback callback model =
    case callback of
        BuildTriggered (Err err) ->
            redirectToLoginIfNecessary err ( model, [] )

        BuildAborted (Err err) ->
            redirectToLoginIfNecessary err ( model, [] )

        PausedToggled (Err err) ->
            redirectToLoginIfNecessary err ( model, [] )

        JobBuildsFetched (Err err) ->
            redirectToLoginIfNecessary err ( model, [] )

        InputToFetched (Err err) ->
            redirectToLoginIfNecessary err ( model, [] )

        OutputOfFetched (Err err) ->
            redirectToLoginIfNecessary err ( model, [] )

        PipelineToggled _ (Err err) ->
            subpageHandleCallback callback ( model, [] )
                |> redirectToLoginIfNecessary err

        VisibilityChanged _ _ (Err err) ->
            subpageHandleCallback callback ( model, [] )
                |> redirectToLoginIfNecessary err

        LoggedOut (Ok ()) ->
            let
                session =
                    model.session

                newSession =
                    { session | userState = UserStateLoggedOut }

                wallEditor =
                    model.wallEditor
            in
            subpageHandleCallback callback ( { model | session = newSession, wallEditor = { wallEditor | isOpen = False } }, [] )

        AllTeamsFetched (Err err) ->
            let
                session =
                    model.session

                newSession =
                    { session | userState = UserStateLoggedOut }

                wallEditor =
                    model.wallEditor
            in
            subpageHandleCallback callback ( { model | session = newSession, wallEditor = { wallEditor | isOpen = False } }, [] )
                |> redirectToLoginIfNecessary err

        UserFetched (Ok user) ->
            let
                session =
                    model.session

                newSession =
                    { session | userState = UserStateLoggedIn user }
            in
            subpageHandleCallback callback ( { model | session = newSession }, [] )

        UserFetched (Err _) ->
            let
                session =
                    model.session

                newSession =
                    { session | userState = UserStateLoggedOut }

                wallEditor =
                    model.wallEditor
            in
            subpageHandleCallback callback ( { model | session = newSession, wallEditor = { wallEditor | isOpen = False } }, [] )

        ClusterInfoFetched (Ok { clusterName, version }) ->
            let
                session =
                    model.session

                newSession =
                    { session | clusterName = clusterName, version = version }
            in
            subpageHandleCallback callback ( { model | session = newSession }, [] )

        ScreenResized viewport ->
            let
                session =
                    model.session

                newSession =
                    { session
                        | screenSize =
                            ScreenSize.fromWindowSize viewport.viewport.width
                    }
            in
            subpageHandleCallback
                callback
                ( { model | session = newSession }, [] )

        GotCurrentTimeZone zone ->
            let
                session =
                    model.session
            in
            ( { model | session = { session | timeZone = zone } }, [] )

        WallFetched (Ok wall) ->
            let
                newMsg =
                    let
                        trimmed =
                            String.trim wall.message
                    in
                    if trimmed == "" then
                        Nothing

                    else
                        Just trimmed
            in
            ( { model | wallMessage = newMsg }, [] )

        WallFetched (Err _) ->
            ( model, [] )

        WallSet (Ok ()) ->
            let
                wallEditor =
                    model.wallEditor
            in
            ( { model | wallEditor = { wallEditor | isOpen = False } }
            , [ FetchWall ]
            )

        WallSet (Err _) ->
            ( model, [] )

        WallCleared (Ok ()) ->
            ( { model | wallMessage = Nothing }, [] )

        WallCleared (Err _) ->
            ( model, [] )

        _ ->
            sideBarHandleCallback callback ( model, [] )
                |> subpageHandleCallback callback


sideBarHandleCallback : Callback -> ET Model
sideBarHandleCallback callback ( model, effects ) =
    let
        ( session, newEffects ) =
            ( model.session, effects )
                |> SideBar.handleCallback callback
                |> Tooltip.handleCallback callback
    in
    ( { model | session = session }, newEffects )


subpageHandleCallback : Callback -> ET Model
subpageHandleCallback callback ( model, effects ) =
    let
        ( subModel, newEffects ) =
            ( model.subModel, effects )
                |> SubPage.handleCallback callback model.session
                |> SubPage.handleNotFound model.session
    in
    ( { model | subModel = subModel }, newEffects )


update : TopLevelMessage -> Model -> ( Model, List Effect )
update msg model =
    case msg of
        Update (Message.Hover hovered) ->
            let
                session =
                    model.session

                newHovered =
                    case hovered of
                        Just h ->
                            HoverState.Hovered h

                        Nothing ->
                            HoverState.NoHover

                ( newSession, sideBarEffects ) =
                    { session | hovered = newHovered }
                        |> SideBar.update (Message.Hover hovered)

                ( subModel, subEffects ) =
                    ( model.subModel, [] )
                        |> SubPage.update model.session (Message.Hover hovered)
            in
            ( { model | subModel = subModel, session = newSession }
            , subEffects ++ sideBarEffects
            )

        Update (Message.Click Message.SetWallButton) ->
            let
                wallEditor =
                    model.wallEditor
            in
            if wallEditor.isOpen then
                ( { model | wallEditor = { wallEditor | isOpen = False } }, [] )

            else
                ( { model
                    | wallEditor =
                        { wallEditor
                            | isOpen = True
                            , message = Maybe.withDefault "" model.wallMessage
                        }
                  }
                , [ Focus (Effects.toHtmlID Message.WallEditorTextarea) ]
                )

        Update (Message.Click Message.CloseWallEditorButton) ->
            let
                wallEditor =
                    model.wallEditor
            in
            ( { model | wallEditor = { wallEditor | isOpen = False } }, [] )

        Update (Message.EditWallMessage text) ->
            let
                wallEditor =
                    model.wallEditor
            in
            ( { model | wallEditor = { wallEditor | message = text } }, [] )

        Update (Message.Click Message.SaveWallButton) ->
            let
                wallEditor =
                    model.wallEditor

                wall =
                    { message = wallEditor.message
                    , ttl = 0
                    }
            in
            ( { model | wallEditor = { wallEditor | isOpen = False } }
            , [ Effects.DoSetWall wall ]
            )

        Update (Message.Click Message.ClearWallButton) ->
            ( { model | wallEditor = { isOpen = False, message = "" } }
            , [ Effects.DoClearWall ]
            )

        Update m ->
            let
                ( subModel, subEffects ) =
                    ( model.subModel, [] )
                        |> SubPage.update model.session m
                        |> SubPage.handleNotFound model.session

                ( session, sessionEffects ) =
                    SideBar.update m model.session
            in
            ( { model | subModel = subModel, session = session }
            , subEffects ++ sessionEffects
            )

        Callback callback ->
            handleCallback callback model

        DeliveryReceived delivery ->
            handleDelivery delivery model


handleDelivery : Delivery -> Model -> ( Model, List Effect )
handleDelivery delivery model =
    let
        ( newSubmodel, subPageEffects ) =
            ( model.subModel, [] )
                |> SubPage.handleDelivery model.session delivery
                |> SubPage.handleNotFound model.session

        ( newModel, applicationEffects ) =
            handleDeliveryForApplication
                delivery
                { model | subModel = newSubmodel }

        ( newSession, sessionEffects ) =
            ( newModel.session, [] )
                |> SideBar.handleDelivery delivery
    in
    ( { newModel | session = newSession }, subPageEffects ++ applicationEffects ++ sessionEffects )


handleDeliveryForApplication : Delivery -> Model -> ( Model, List Effect )
handleDeliveryForApplication delivery model =
    case delivery of
        NonHrefLinkClicked route ->
            ( model, [ LoadExternal route ] )

        TokenReceived (Ok tokenValue) ->
            let
                session =
                    model.session

                newSession =
                    { session | csrfToken = tokenValue }
            in
            ( { model | session = newSession }, [] )

        RouteChanged route ->
            urlUpdate route model

        WindowResized width _ ->
            let
                session =
                    model.session

                newSession =
                    { session | screenSize = ScreenSize.fromWindowSize width }
            in
            ( { model | session = newSession }, [] )

        UrlRequest request ->
            case request of
                Browser.Internal url ->
                    case Routes.parsePath url of
                        Just route ->
                            ( model, [ NavigateTo <| Routes.toString route ] )

                        Nothing ->
                            ( model, [ LoadExternal <| Url.toString url ] )

                Browser.External url ->
                    ( model, [ LoadExternal url ] )

        ClockTicked Message.Subscription.FiveSeconds _ ->
            ( model, [ FetchWall ] )

        ClockTicked _ _ ->
            ( model, [] )

        _ ->
            ( model, [] )


redirectToLoginIfNecessary : Http.Error -> ET Model
redirectToLoginIfNecessary err ( model, effects ) =
    case err of
        Http.BadStatus { status } ->
            if status.code == 401 then
                ( model, effects ++ [ RedirectToLogin ] )

            else
                ( model, effects )

        _ ->
            ( model, effects )


urlUpdate : Routes.Route -> Model -> ( Model, List Effect )
urlUpdate route model =
    let
        oldRoute =
            model.session.route

        newRoute =
            case route of
                Routes.Pipeline _ ->
                    route

                _ ->
                    Routes.withGroups (Routes.getGroups oldRoute) route

        ( newSubmodel, subEffects ) =
            if newRoute == oldRoute then
                ( model.subModel, [] )

            else if routeMatchesModel newRoute model then
                SubPage.urlUpdate
                    model.session
                    { from = oldRoute
                    , to = newRoute
                    }
                    ( model.subModel, [] )

            else
                SubPage.init model.session newRoute

        oldSession =
            model.session

        newSession =
            { oldSession | route = newRoute, hovered = HoverState.NoHover }
    in
    ( { model
        | subModel = newSubmodel
        , session = newSession
        , wallEditor =
            case newRoute of
                Routes.Dashboard _ ->
                    model.wallEditor

                _ ->
                    { isOpen = False, message = "" }
      }
    , subEffects ++ [ SetFavIcon Nothing ]
    )


view : Model -> Browser.Document TopLevelMessage
view model =
    let
        ( title, body ) =
            SubPage.view model.session model.subModel
    in
    { title = title ++ " - Concourse"
    , body =
        List.map (Html.map Update)
            [ SubPage.tooltip model.subModel model.session
                |> Maybe.map (Tooltip.view model.session)
                |> Maybe.withDefault (Html.text "")
            , SideBar.tooltip model.session
                |> Maybe.map (Tooltip.view model.session)
                |> Maybe.withDefault (Html.text "")
            , case model.wallMessage of
                Just msg ->
                    if model.wallEditor.isOpen then
                        Html.text ""

                    else
                        Html.div [ id "wall-banner", style "white-space" "pre-wrap" ] <|
                            wallLinks msg

                Nothing ->
                    Html.text ""
            , if model.wallEditor.isOpen then
                wallEditorView model.wallEditor model.wallMessage model.session.hovered

              else
                Html.text ""
            , Html.div
                ([ id "page-wrapper", style "height" "100%" ]
                    ++ (if model.session.draggingSideBar then
                            Styles.disableInteraction

                        else
                            []
                       )
                )
                [ body ]
            ]
    }


subscriptions : Model -> List Subscription
subscriptions model =
    [ OnNonHrefLinkClicked
    , OnLocalStorageReceived
    , OnCacheReceived
    , OnWindowResize
    , OnClockTick Message.Subscription.FiveSeconds
    ]
        ++ (if model.session.draggingSideBar then
                [ OnMouse
                , OnMouseUp
                ]

            else
                []
           )
        ++ SubPage.subscriptions model.subModel


routeMatchesModel : Routes.Route -> Model -> Bool
routeMatchesModel route model =
    case ( route, model.subModel ) of
        ( Routes.Pipeline _, SubPage.PipelineModel _ ) ->
            True

        ( Routes.Resource _, SubPage.ResourceModel _ ) ->
            True

        ( Routes.Build _, SubPage.BuildModel _ ) ->
            True

        ( Routes.OneOffBuild _, SubPage.BuildModel _ ) ->
            True

        ( Routes.Job _, SubPage.JobModel _ ) ->
            True

        ( Routes.Dashboard _, SubPage.DashboardModel _ ) ->
            True

        _ ->
            False


wallEditorTextarea : WallEditor -> Html.Html Message.Message
wallEditorTextarea editor =
    Html.input
        ([ id (Effects.toHtmlID Message.WallEditorTextarea)
         , type_ "text"
         , value editor.message
         , onInput Message.EditWallMessage
         , placeholder "Enter broadcast message…"
         , style "background-color" Colors.sectionHeader
         ]
            ++ Views.Styles.commentBarTextarea
        )
        []


clearWallButton : HoverState.HoverState -> Html.Html Message.Message
clearWallButton hovered =
    Html.button
        ([ id (Effects.toHtmlID Message.ClearWallButton)
         , onMouseEnter (Message.Hover (Just Message.ClearWallButton))
         , onMouseLeave (Message.Hover Nothing)
         , onClick (Message.Click Message.ClearWallButton)
         , style "color" Colors.text
         , style "cursor" "pointer"
         , if HoverState.isHovered Message.ClearWallButton hovered then
            style "background-color" Colors.pinTools

           else
            style "background-color" "transparent"
         ]
            ++ Views.Styles.commentBarTextButton
        )
        [ Html.text "clear" ]


setWallButton : WallEditor -> HoverState.HoverState -> Html.Html Message.Message
setWallButton editor hovered =
    Html.button
        ([ id (Effects.toHtmlID Message.SaveWallButton)
         , onMouseEnter (Message.Hover (Just Message.SaveWallButton))
         , onMouseLeave (Message.Hover Nothing)
         , onClick (Message.Click Message.SaveWallButton)
         , disabled (String.trim editor.message == "")
         , style "color" Colors.text
         , style "cursor"
            (if String.trim editor.message == "" then
                "not-allowed"

             else
                "pointer"
            )
         , style "opacity"
            (if String.trim editor.message == "" then
                "0.5"

             else
                "1"
            )
         , if HoverState.isHovered Message.SaveWallButton hovered then
            style "background-color" Colors.pinTools

           else
            style "background-color" "transparent"
         ]
            ++ Views.Styles.commentBarTextButton
        )
        [ Html.text "set" ]


closeWallEditorButton : HoverState.HoverState -> Html.Html Message.Message
closeWallEditorButton hovered =
    Html.button
        ([ id (Effects.toHtmlID Message.CloseWallEditorButton)
         , onMouseEnter (Message.Hover (Just Message.CloseWallEditorButton))
         , onMouseLeave (Message.Hover Nothing)
         , onClick (Message.Click Message.CloseWallEditorButton)
         , style "color" Colors.text
         , style "cursor" "pointer"
         , if HoverState.isHovered Message.CloseWallEditorButton hovered then
            style "background-color" Colors.pinTools

           else
            style "background-color" "transparent"
         ]
            ++ Views.Styles.commentBarTextButton
        )
        [ Html.text "×" ]


wallEditorView : WallEditor -> Maybe String -> HoverState.HoverState -> Html.Html Message.Message
wallEditorView editor currentWallMessage hovered =
    Html.div
        [ class "wall-editor-bar"
        , style "display" "flex"
        , style "align-items" "flex-start"
        , style "color" Colors.text
        , style "background-color" Colors.sectionHeader
        , style "border" ("1px solid " ++ Colors.text)
        ]
        [ Icon.icon
            { sizePx = 16
            , image = Assets.BullhornIcon
            }
            [ style "margin" "10px"
            , style "flex-shrink" "0"
            , style "background-size" "contain"
            , style "background-origin" "content-box"
            ]
        , wallEditorTextarea editor
        , Html.div
            [ style "display" "flex"
            , style "align-items" "center"
            , style "flex-shrink" "0"
            ]
            [ if currentWallMessage /= Nothing then
                clearWallButton hovered

              else
                Html.text ""
            , setWallButton editor hovered
            , closeWallEditorButton hovered
            ]
        ]


wallLinks : String -> List (Html.Html msg)
wallLinks msg =
    let
        tokens =
            String.split " " msg

        stripTrailingPunct original =
            let
                punctuations =
                    [ '.', ',' ]

                dropWhileEnd s =
                    case String.uncons (String.reverse s) of
                        Just ( c, restRev ) ->
                            if List.member c punctuations then
                                let
                                    trimmed =
                                        String.reverse restRev

                                    ( url, trailing ) =
                                        dropWhileEnd trimmed
                                in
                                ( url, String.fromChar c ++ trailing )

                            else
                                ( s, "" )

                        Nothing ->
                            ( s, "" )
            in
            dropWhileEnd original

        render t =
            if String.startsWith "http://" t || String.startsWith "https://" t then
                let
                    ( url, trailing ) =
                        stripTrailingPunct t

                    anchor =
                        Html.a [ href url, target "_blank", rel "noopener noreferrer", style "text-decoration" "underline", style "margin-right" "0.25em" ] [ Html.text url ]
                in
                if trailing == "" then
                    [ anchor ]

                else
                    [ anchor, Html.text trailing ]

            else
                [ Html.text t ]
    in
    tokens
        |> List.map render
        |> List.intersperse [ Html.text " " ]
        |> List.concat
