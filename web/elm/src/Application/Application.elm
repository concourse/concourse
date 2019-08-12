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
import Browser
import Concourse
import EffectTransformer exposing (ET)
import HoverState
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
import Url
import UserState exposing (UserState(..))


type alias Flags =
    { turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : Concourse.CSRFToken
    , authToken : String
    , pipelineRunningKeyframes : String
    }


type alias Model =
    { subModel : SubPage.Model
    , route : Routes.Route
    , session : Session
    }


init : Flags -> Url.Url -> ( Model, List Effect )
init flags url =
    let
        route =
            Routes.parsePath url
                |> Maybe.withDefault (Routes.Dashboard (Routes.Normal Nothing))

        session =
            { userState = UserStateUnknown
            , hovered = HoverState.NoHover
            , clusterName = ""
            , turbulenceImgSrc = flags.turbulenceImgSrc
            , notFoundImgSrc = flags.notFoundImgSrc
            , csrfToken = flags.csrfToken
            , authToken = flags.authToken
            , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
            , expandedTeams = Set.empty
            , pipelines = RemoteData.NotAsked
            , isSideBarOpen = False
            , screenSize = ScreenSize.Desktop
            , timeZone = Time.utc
            }

        ( subModel, subEffects ) =
            SubPage.init session route

        model =
            { subModel = subModel
            , session = session
            , route = route
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
    , [ FetchUser, GetScreenSize, LoadSideBarState, FetchClusterInfo ]
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
            in
            subpageHandleCallback callback ( { model | session = newSession }, [] )

        APIDataFetched (Ok ( _, data )) ->
            let
                session =
                    model.session

                newSession =
                    { session
                        | userState =
                            data.user
                                |> Maybe.map UserStateLoggedIn
                                |> Maybe.withDefault UserStateLoggedOut
                    }
            in
            subpageHandleCallback callback ( { model | session = newSession }, [] )

        APIDataFetched (Err err) ->
            let
                session =
                    model.session

                newSession =
                    { session | userState = UserStateLoggedOut }
            in
            subpageHandleCallback callback ( { model | session = newSession }, [] )
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
            in
            subpageHandleCallback callback ( { model | session = newSession }, [] )

        ClusterInfoFetched (Ok { clusterName }) ->
            let
                session =
                    model.session

                newSession =
                    { session | clusterName = clusterName }
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

                newSession =
                    { session | timeZone = zone }
            in
            ( { model | session = newSession }, [] )

        -- otherwise, pass down
        _ ->
            sideBarHandleCallback callback ( model, [] )
                |> subpageHandleCallback callback


sideBarHandleCallback : Callback -> ET Model
sideBarHandleCallback callback ( model, effects ) =
    let
        ( session, newEffects ) =
            ( model.session, effects )
                |> (case model.subModel of
                        SubPage.ResourceModel { resourceIdentifier } ->
                            SideBar.handleCallback callback <|
                                RemoteData.Success resourceIdentifier

                        SubPage.PipelineModel { pipelineLocator } ->
                            SideBar.handleCallback callback <|
                                RemoteData.Success pipelineLocator

                        SubPage.JobModel { jobIdentifier } ->
                            SideBar.handleCallback callback <|
                                RemoteData.Success jobIdentifier

                        SubPage.BuildModel buildModel ->
                            SideBar.handleCallback callback
                                (buildModel.currentBuild
                                    |> RemoteData.map .build
                                    |> RemoteData.andThen
                                        (\b ->
                                            case b.job of
                                                Just j ->
                                                    RemoteData.Success j

                                                Nothing ->
                                                    RemoteData.NotAsked
                                        )
                                )

                        _ ->
                            SideBar.handleCallback callback <|
                                RemoteData.NotAsked
                   )
    in
    ( { model | session = session }, newEffects )


subpageHandleCallback : Callback -> ET Model
subpageHandleCallback callback ( model, effects ) =
    let
        ( subModel, newEffects ) =
            ( model.subModel, effects )
                |> SubPage.handleCallback callback model.session
                |> SubPage.handleNotFound model.session.notFoundImgSrc model.route
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

        Update m ->
            let
                ( subModel, subEffects ) =
                    ( model.subModel, [] )
                        |> SubPage.update model.session m
                        |> SubPage.handleNotFound model.session.notFoundImgSrc model.route

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
                |> SubPage.handleNotFound model.session.notFoundImgSrc model.route

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

        TokenReceived (Just tokenValue) ->
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
        ( newSubmodel, subEffects ) =
            if route == model.route then
                ( model.subModel, [] )

            else if routeMatchesModel route model then
                SubPage.urlUpdate route ( model.subModel, [] )

            else
                SubPage.init model.session route
    in
    ( { model | subModel = newSubmodel, route = route }
    , subEffects ++ [ SetFavIcon Nothing ]
    )


view : Model -> Browser.Document TopLevelMessage
view model =
    SubPage.view model.session model.subModel


subscriptions : Model -> List Subscription
subscriptions model =
    [ OnNonHrefLinkClicked
    , OnTokenReceived
    , OnSideBarStateReceived
    , OnWindowResize
    ]
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

        ( Routes.Job _, SubPage.JobModel _ ) ->
            True

        ( Routes.Dashboard _, SubPage.DashboardModel _ ) ->
            True

        _ ->
            False
