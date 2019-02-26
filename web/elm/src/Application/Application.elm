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
import Build.Msgs
import Build.Output
import Callback exposing (Callback(..))
import Dashboard.Msgs
import Effects exposing (Effect(..), LayoutDispatch(..))
import Html exposing (Html)
import Http
import Job.Msgs
import Navigation
import Pipeline.Msgs
import Resource.Msgs
import Routes
import SubPage.Msgs
import SubPage.SubPage as SubPage
import Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import TopBar.Msgs
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
    case delivery of
        KeyDown keycode ->
            case model.subModel of
                SubPage.DashboardModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar <|
                                    TopBar.Msgs.KeyDown keycode
                        )
                        model

                SubPage.ResourceModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.ResourceMsg <|
                                Resource.Msgs.KeyDowns keycode
                        )
                        model

                SubPage.PipelineModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.PipelineMsg <|
                                Pipeline.Msgs.KeyPressed keycode
                        )
                        model

                SubPage.BuildModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.BuildMsg <|
                                Build.Msgs.KeyPressed keycode
                        )
                        model

                _ ->
                    ( model, [] )

        KeyUp keycode ->
            case model.subModel of
                SubPage.BuildModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.BuildMsg <|
                                Build.Msgs.KeyUped keycode
                        )
                        model

                SubPage.ResourceModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.ResourceMsg <|
                                Resource.Msgs.KeyUps keycode
                        )
                        model

                _ ->
                    ( model, [] )

        MouseMoved ->
            case model.subModel of
                SubPage.DashboardModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.ShowFooter
                        )
                        model

                SubPage.PipelineModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.PipelineMsg <|
                                Pipeline.Msgs.ShowLegend
                        )
                        model

                _ ->
                    ( model, [] )

        MouseClicked ->
            case model.subModel of
                SubPage.DashboardModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.ShowFooter
                        )
                        model

                SubPage.PipelineModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.PipelineMsg <|
                                Pipeline.Msgs.ShowLegend
                        )
                        model

                _ ->
                    ( model, [] )

        ClockTicked interval time ->
            case ( interval, model.subModel ) of
                ( OneSecond, SubPage.ResourceModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.ResourceMsg <|
                                Resource.Msgs.ClockTick time
                        )
                        model

                ( OneSecond, SubPage.PipelineModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.PipelineMsg <|
                                Pipeline.Msgs.HideLegendTimerTicked time
                        )
                        model

                ( OneSecond, SubPage.JobModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.JobMsg <|
                                Job.Msgs.ClockTick time
                        )
                        model

                ( OneSecond, SubPage.DashboardModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.ClockTick time
                        )
                        model

                ( OneSecond, SubPage.BuildModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.BuildMsg <|
                                Build.Msgs.ClockTick time
                        )
                        model

                ( OneSecond, _ ) ->
                    ( model, [] )

                ( FiveSeconds, SubPage.ResourceModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.ResourceMsg <|
                                Resource.Msgs.AutoupdateTimerTicked
                        )
                        model

                ( FiveSeconds, SubPage.PipelineModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.PipelineMsg <|
                                Pipeline.Msgs.AutoupdateTimerTicked
                        )
                        model

                ( FiveSeconds, SubPage.JobModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.JobMsg <|
                                Job.Msgs.SubscriptionTick
                        )
                        model

                ( FiveSeconds, SubPage.DashboardModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.AutoRefresh
                        )
                        model

                ( FiveSeconds, _ ) ->
                    ( model, [] )

                ( OneMinute, SubPage.PipelineModel _ ) ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.PipelineMsg <|
                                Pipeline.Msgs.AutoupdateVersionTicked
                        )
                        model

                ( OneMinute, _ ) ->
                    ( model, [] )

        AnimationFrameAdvanced ->
            case model.subModel of
                SubPage.BuildModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.BuildMsg <|
                                Build.Msgs.ScrollDown
                        )
                        model

                _ ->
                    ( model, [] )

        ScrolledFromWindowBottom distance ->
            case model.subModel of
                SubPage.BuildModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.BuildMsg <|
                                Build.Msgs.WindowScrolled distance
                        )
                        model

                _ ->
                    ( model, [] )

        WindowResized screenSize ->
            case model.subModel of
                SubPage.DashboardModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.WindowResized screenSize
                        )
                        model

                _ ->
                    ( model, [] )

        NonHrefLinkClicked route ->
            ( model, [ ( Layout, NavigateTo route ) ] )

        TokenReceived Nothing ->
            ( model, [] )

        TokenReceived (Just tokenValue) ->
            let
                ( newSubModel, subCmd ) =
                    SubPage.update
                        model.turbulenceImgSrc
                        model.notFoundImgSrc
                        tokenValue
                        model.route
                        (SubPage.Msgs.NewCSRFToken tokenValue)
                        model.subModel
            in
            ( { model
                | csrfToken = tokenValue
                , subModel = newSubModel
              }
            , List.map (\ef -> ( SubPage anyNavIndex, ef )) subCmd
            )

        EventReceived eventSourceMsg ->
            case model.subModel of
                SubPage.BuildModel _ ->
                    update
                        (SubMsg model.navIndex <|
                            SubPage.Msgs.BuildMsg <|
                                Build.Msgs.BuildEventsMsg <|
                                    Build.Output.parseMsg eventSourceMsg
                        )
                        model

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
