module Application.Application exposing
    ( Flags
    , Model
    , handleCallback
    , init
    , locationMsg
    , subscriptions
    , update
    , view
    )

import Application.Msgs as Msgs exposing (Msg(..), NavIndex)
import Callback exposing (Callback(..))
import Effects exposing (Effect(..), LayoutDispatch(..))
import Html exposing (Html)
import Http
import Navigation
import Routes
import SubPage.SubPage as SubPage
import Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import UserState exposing (UserState(..))


type alias Flags =
    { turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : String
    , authToken : String
    , pipelineRunningKeyframes : String
    }


anyNavIndex : NavIndex
anyNavIndex =
    -1


type alias Model =
    { navIndex : NavIndex
    , subModel : SubPage.Model
    , turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : String
    , authToken : String
    , pipelineRunningKeyframes : String
    , route : Routes.Route
    , userState : UserState
    }


init : Flags -> Navigation.Location -> ( Model, List ( LayoutDispatch, Effect ) )
init flags location =
    let
        route =
            Routes.parsePath location

        ( subModel, subEffects ) =
            SubPage.init
                { turbulencePath = flags.turbulenceImgSrc
                , csrfToken = flags.csrfToken
                , authToken = flags.authToken
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                }
                route

        navIndex =
            1

        model =
            { navIndex = navIndex
            , subModel = subModel
            , turbulenceImgSrc = flags.turbulenceImgSrc
            , notFoundImgSrc = flags.notFoundImgSrc
            , csrfToken = flags.csrfToken
            , authToken = flags.authToken
            , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
            , route = route
            , userState = UserStateUnknown
            }

        handleTokenEffect =
            -- We've refreshed on the page and we're not
            -- getting it from query params
            if flags.csrfToken == "" then
                ( Layout, LoadToken )

            else
                ( Layout, SaveToken flags.csrfToken )

        stripCSRFTokenParamCmd =
            if flags.csrfToken == "" then
                []

            else
                [ ( Layout, Effects.ModifyUrl <| Routes.toString route ) ]
    in
    ( model
    , [ ( Layout, FetchUser ), handleTokenEffect ]
        ++ stripCSRFTokenParamCmd
        ++ List.map (\ef -> ( SubPage navIndex, ef )) subEffects
    )


locationMsg : Navigation.Location -> Msg
locationMsg =
    RouteChanged << Routes.parsePath


handleCallback :
    LayoutDispatch
    -> Callback
    -> Model
    -> ( Model, List ( LayoutDispatch, Effect ) )
handleCallback disp callback model =
    case disp of
        SubPage navIndex ->
            case callback of
                ResourcesFetched (Ok fetchedResources) ->
                    if validNavIndex model.navIndex navIndex then
                        subpageHandleCallback model callback navIndex

                    else
                        ( model, [] )

                BuildTriggered (Err err) ->
                    ( model, redirectToLoginIfNecessary err navIndex )

                BuildAborted (Err err) ->
                    ( model, redirectToLoginIfNecessary err navIndex )

                PausedToggled (Err err) ->
                    ( model, redirectToLoginIfNecessary err navIndex )

                JobBuildsFetched (Err err) ->
                    ( model, redirectToLoginIfNecessary err navIndex )

                InputToFetched (Err err) ->
                    ( model, redirectToLoginIfNecessary err navIndex )

                OutputOfFetched (Err err) ->
                    ( model, redirectToLoginIfNecessary err navIndex )

                LoggedOut (Ok ()) ->
                    subpageHandleCallback { model | userState = UserStateLoggedOut } callback navIndex

                APIDataFetched (Ok ( time, data )) ->
                    subpageHandleCallback
                        { model | userState = data.user |> Maybe.map UserStateLoggedIn |> Maybe.withDefault UserStateLoggedOut }
                        callback
                        navIndex

                APIDataFetched (Err err) ->
                    subpageHandleCallback { model | userState = UserStateLoggedOut } callback navIndex

                -- otherwise, pass down
                _ ->
                    subpageHandleCallback model callback navIndex

        Layout ->
            case callback of
                UserFetched (Ok user) ->
                    subpageHandleCallback { model | userState = UserStateLoggedIn user } callback model.navIndex

                UserFetched (Err _) ->
                    subpageHandleCallback { model | userState = UserStateLoggedOut } callback model.navIndex

                _ ->
                    ( model, [] )


subpageHandleCallback : Model -> Callback -> Int -> ( Model, List ( LayoutDispatch, Effect ) )
subpageHandleCallback model callback navIndex =
    let
        ( subModel, effects ) =
            SubPage.handleCallback model.csrfToken callback model.subModel
                |> SubPage.handleNotFound model.notFoundImgSrc model.route
    in
    ( { model | subModel = subModel }
    , List.map (\ef -> ( SubPage navIndex, ef )) effects
    )


update : Msg -> Model -> ( Model, List ( LayoutDispatch, Effect ) )
update msg model =
    case msg of
        Msgs.ModifyUrl route ->
            ( model, [ ( Layout, Effects.ModifyUrl <| Routes.toString route ) ] )

        RouteChanged route ->
            urlUpdate route model

        SubMsg navIndex m ->
            if validNavIndex model.navIndex navIndex then
                let
                    ( subModel, subEffects ) =
                        SubPage.update
                            model.turbulenceImgSrc
                            model.notFoundImgSrc
                            model.csrfToken
                            model.route
                            m
                            model.subModel
                in
                ( { model | subModel = subModel }
                , List.map (\ef -> ( SubPage navIndex, ef )) subEffects
                )

            else
                ( model, [] )

        Callback dispatch callback ->
            handleCallback dispatch callback model

        DeliveryReceived delivery ->
            handleDelivery delivery model


handleDelivery : Delivery -> Model -> ( Model, List ( LayoutDispatch, Effect ) )
handleDelivery delivery model =
    let
        ( newSubmodel, subPageEffects ) =
            SubPage.handleDelivery
                model.notFoundImgSrc
                model.route
                delivery
                model.subModel

        ( newModel, applicationEffects ) =
            handleDeliveryForApplication delivery model
    in
    ( { newModel | subModel = newSubmodel }
    , List.map (\ef -> ( SubPage model.navIndex, ef )) subPageEffects ++ applicationEffects
    )


handleDeliveryForApplication : Delivery -> Model -> ( Model, List ( LayoutDispatch, Effect ) )
handleDeliveryForApplication delivery model =
    case delivery of
        NonHrefLinkClicked route ->
            ( model, [ ( Layout, NavigateTo route ) ] )

        TokenReceived (Just tokenValue) ->
            ( { model | csrfToken = tokenValue }, [] )

        _ ->
            ( model, [] )


redirectToLoginIfNecessary : Http.Error -> NavIndex -> List ( LayoutDispatch, Effect )
redirectToLoginIfNecessary err navIndex =
    case err of
        Http.BadStatus { status } ->
            if status.code == 401 then
                [ ( SubPage navIndex, RedirectToLogin ) ]

            else
                []

        _ ->
            []


validNavIndex : NavIndex -> NavIndex -> Bool
validNavIndex modelNavIndex navIndex =
    if navIndex == anyNavIndex then
        True

    else
        navIndex == modelNavIndex


urlUpdate : Routes.Route -> Model -> ( Model, List ( LayoutDispatch, Effect ) )
urlUpdate route model =
    let
        navIndex =
            if route == model.route then
                model.navIndex

            else
                model.navIndex + 1

        ( newSubmodel, subEffects ) =
            if route == model.route then
                ( model.subModel, [] )

            else if routeMatchesModel route model then
                SubPage.urlUpdate route model.subModel

            else
                SubPage.init
                    { turbulencePath = model.turbulenceImgSrc
                    , csrfToken = model.csrfToken
                    , authToken = model.authToken
                    , pipelineRunningKeyframes = model.pipelineRunningKeyframes
                    }
                    route
    in
    ( { model
        | navIndex = navIndex
        , subModel = newSubmodel
        , route = route
      }
    , List.map (\ef -> ( SubPage navIndex, ef )) subEffects
        ++ [ ( Layout, SetFavIcon Nothing ) ]
    )


view : Model -> Html Msg
view model =
    Html.map (SubMsg model.navIndex) (SubPage.view model.userState model.subModel)


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
