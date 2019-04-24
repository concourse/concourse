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

import Browser
import Concourse
import EffectTransformer exposing (ET)
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
import Routes
import Set exposing (Set)
import SubPage.SubPage as SubPage
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
    , turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : String
    , authToken : String
    , pipelineRunningKeyframes : String
    , route : Routes.Route
    , userState : UserState
    , pipelines : List Concourse.Pipeline
    , expandedTeams : Set String
    , isSideBarOpen : Bool
    }


init : Flags -> Url.Url -> ( Model, List Effect )
init flags url =
    let
        route =
            Routes.parsePath url
                |> Maybe.withDefault (Routes.Dashboard (Routes.Normal Nothing))

        ( subModel, subEffects ) =
            SubPage.init
                { turbulencePath = flags.turbulenceImgSrc
                , authToken = flags.authToken
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                }
                route

        model =
            { subModel = subModel
            , turbulenceImgSrc = flags.turbulenceImgSrc
            , notFoundImgSrc = flags.notFoundImgSrc
            , csrfToken = flags.csrfToken
            , authToken = flags.authToken
            , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
            , route = route
            , userState = UserStateUnknown
            , isSideBarOpen = False
            , pipelines = []
            , expandedTeams = Set.empty
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
    ( model, FetchUser :: handleTokenEffect ++ subEffects )


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
            subpageHandleCallback model callback
                |> redirectToLoginIfNecessary err

        VisibilityChanged _ _ (Err err) ->
            subpageHandleCallback model callback
                |> redirectToLoginIfNecessary err

        LoggedOut (Ok ()) ->
            subpageHandleCallback { model | userState = UserStateLoggedOut } callback

        APIDataFetched (Ok ( _, data )) ->
            subpageHandleCallback
                { model | userState = data.user |> Maybe.map UserStateLoggedIn |> Maybe.withDefault UserStateLoggedOut }
                callback

        APIDataFetched (Err _) ->
            subpageHandleCallback { model | userState = UserStateLoggedOut } callback

        UserFetched (Ok user) ->
            subpageHandleCallback { model | userState = UserStateLoggedIn user } callback

        UserFetched (Err _) ->
            subpageHandleCallback { model | userState = UserStateLoggedOut } callback

        PipelinesFetched (Ok pipelines) ->
            ( { model
                | pipelines = pipelines
                , expandedTeams =

                    case ( model.pipelines, model.route ) of
                        ( [], Routes.Pipeline { id } ) ->
                            Set.insert id.teamName model.expandedTeams

                        _ ->
                            model.expandedTeams
              }
            , []
            )

        -- otherwise, pass down
        _ ->
            subpageHandleCallback model callback


subpageHandleCallback : Model -> Callback -> ( Model, List Effect )
subpageHandleCallback model callback =
    let
        ( subModel, effects ) =
            ( model.subModel, [] )
                |> SubPage.handleCallback callback
                |> SubPage.handleNotFound model.notFoundImgSrc model.route
    in
    ( { model | subModel = subModel }, effects )


update : TopLevelMessage -> Model -> ( Model, List Effect )
update msg model =
    case msg of
        Update (Message.Click Message.HamburgerMenu) ->
            ( { model | isSideBarOpen = not model.isSideBarOpen }, [] )

        Update (Message.Click (Message.SideBarTeam teamName)) ->
            ( { model
                | expandedTeams =
                    if Set.member teamName model.expandedTeams then
                        Set.remove teamName model.expandedTeams

                    else
                        Set.insert teamName model.expandedTeams
              }
            , []
            )

        Update m ->
            let
                ( subModel, subEffects ) =
                    ( model.subModel, [] )
                        |> SubPage.update m
                        |> SubPage.handleNotFound model.notFoundImgSrc model.route
            in
            ( { model | subModel = subModel }, subEffects )

        Callback callback ->
            handleCallback callback model

        DeliveryReceived delivery ->
            handleDelivery delivery model


handleDelivery : Delivery -> Model -> ( Model, List Effect )
handleDelivery delivery model =
    let
        ( newSubmodel, subPageEffects ) =
            ( model.subModel, [] )
                |> SubPage.handleDelivery delivery
                |> SubPage.handleNotFound model.notFoundImgSrc model.route

        ( newModel, applicationEffects ) =
            handleDeliveryForApplication
                delivery
                { model | subModel = newSubmodel }
    in
    ( newModel, subPageEffects ++ applicationEffects )


handleDeliveryForApplication : Delivery -> Model -> ( Model, List Effect )
handleDeliveryForApplication delivery model =
    case delivery of
        NonHrefLinkClicked route ->
            ( model, [ LoadExternal route ] )

        TokenReceived (Just tokenValue) ->
            ( { model | csrfToken = tokenValue }, [] )

        RouteChanged route ->
            urlUpdate route model

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
                SubPage.init
                    { turbulencePath = model.turbulenceImgSrc
                    , authToken = model.authToken
                    , pipelineRunningKeyframes = model.pipelineRunningKeyframes
                    }
                    route
    in
    ( { model | subModel = newSubmodel, route = route }
    , subEffects ++ [ SetFavIcon Nothing ]
    )


view : Model -> Browser.Document TopLevelMessage
view model =
    SubPage.view model model.subModel


subscriptions : Model -> List Subscription
subscriptions model =
    [ OnNonHrefLinkClicked
    , OnTokenReceived
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
