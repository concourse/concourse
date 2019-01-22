port module Layout exposing
    ( Flags
    , Model
    , Msg(..)
    , handleCallback
    , init
    , locationMsg
    , subscriptions
    , update
    , view
    )

import Concourse
import Effects exposing (Callback(..), Effect(..), LayoutDispatch(..))
import Html exposing (Html)
import Html.Attributes as Attributes exposing (class, id, style)
import Http
import Json.Decode
import Navigation
import Routes
import SubPage
import SubPage.Msgs
import TopBar


port newUrl : (String -> msg) -> Sub msg


port tokenReceived : (Maybe String -> msg) -> Sub msg


type alias Flags =
    { turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : String
    , authToken : String
    , pipelineRunningKeyframes : String
    }


type alias NavIndex =
    Int


anyNavIndex : NavIndex
anyNavIndex =
    -1


type alias Model =
    { navIndex : NavIndex
    , subModel : SubPage.Model
    , topModel : TopBar.Model {}
    , topBarType : TopBarType
    , turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : String
    , authToken : String
    , pipelineRunningKeyframes : String
    , route : Routes.ConcourseRoute
    }


type TopBarType
    = Dashboard
    | Normal


type Msg
    = Noop
    | RouteChanged Routes.ConcourseRoute
    | SubMsg NavIndex SubPage.Msgs.Msg
    | TopMsg NavIndex TopBar.Msg
    | NewUrl String
    | ModifyUrl String
    | TokenReceived (Maybe String)
    | Callback Effects.LayoutDispatch Effects.Callback


init : Flags -> Navigation.Location -> ( Model, List ( LayoutDispatch, Effect ) )
init flags location =
    let
        route =
            Routes.parsePath location

        topBarType =
            case route.logical of
                Routes.Dashboard ->
                    Dashboard

                Routes.DashboardHd ->
                    Dashboard

                _ ->
                    Normal

        ( subModel, subEffects ) =
            SubPage.init
                { turbulencePath = flags.turbulenceImgSrc
                , csrfToken = flags.csrfToken
                , authToken = flags.authToken
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                }
                route

        ( topModel, topEffects ) =
            TopBar.init route

        navIndex =
            1

        model =
            { navIndex = navIndex
            , subModel = subModel
            , topModel = topModel
            , topBarType = topBarType
            , turbulenceImgSrc = flags.turbulenceImgSrc
            , notFoundImgSrc = flags.notFoundImgSrc
            , csrfToken = flags.csrfToken
            , authToken = flags.authToken
            , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
            , route = route
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
                [ ( Layout, Effects.ModifyUrl (Routes.customToString route) ) ]
    in
    ( model
    , [ handleTokenEffect ]
        ++ stripCSRFTokenParamCmd
        ++ List.map (\ef -> ( SubPage navIndex, ef )) subEffects
        ++ List.map (\ef -> ( TopBar navIndex, ef )) topEffects
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
        TopBar navIndex ->
            let
                ( topModel, effects ) =
                    TopBar.handleCallback callback model.topModel
            in
            ( { model | topModel = topModel }
            , List.map (\ef -> ( TopBar navIndex, ef )) effects
            )

        SubPage navIndex ->
            case callback of
                Effects.ResourcesFetched (Ok fetchedResources) ->
                    let
                        resources : Result String (List Concourse.Resource)
                        resources =
                            Json.Decode.decodeValue
                                (Json.Decode.list Concourse.decodeResource)
                                fetchedResources

                        pinnedResources : List ( String, Concourse.Version )
                        pinnedResources =
                            case resources of
                                Ok rs ->
                                    rs
                                        |> List.filterMap
                                            (\resource ->
                                                case resource.pinnedVersion of
                                                    Just v ->
                                                        Just ( resource.name, v )

                                                    Nothing ->
                                                        Nothing
                                            )

                                Err _ ->
                                    []

                        topBar =
                            model.topModel
                    in
                    if validNavIndex model.navIndex navIndex then
                        let
                            ( subModel, subEffects ) =
                                SubPage.handleCallback
                                    model.csrfToken
                                    (Effects.ResourcesFetched (Ok fetchedResources))
                                    model.subModel
                                    |> SubPage.handleNotFound model.notFoundImgSrc
                        in
                        ( { model
                            | subModel = subModel
                            , topModel = { topBar | pinnedResources = pinnedResources }
                          }
                        , List.map (\ef -> ( SubPage navIndex, ef )) subEffects
                        )

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

                -- otherwise, pass down
                _ ->
                    let
                        ( subModel, effects ) =
                            SubPage.handleCallback
                                model.csrfToken
                                callback
                                model.subModel
                                |> SubPage.handleNotFound model.notFoundImgSrc
                    in
                    ( { model | subModel = subModel }
                    , List.map (\ef -> ( SubPage navIndex, ef )) effects
                    )

        Layout ->
            ( model, [] )


update : Msg -> Model -> ( Model, List ( LayoutDispatch, Effect ) )
update msg model =
    case msg of
        NewUrl url ->
            ( model, [ ( Layout, NavigateTo url ) ] )

        ModifyUrl url ->
            ( model, [ ( Layout, Effects.ModifyUrl url ) ] )

        RouteChanged route ->
            urlUpdate route model

        SubMsg navIndex m ->
            if validNavIndex model.navIndex navIndex then
                let
                    ( subModel, subEffects ) =
                        SubPage.update model.turbulenceImgSrc model.notFoundImgSrc model.csrfToken m model.subModel
                in
                ( { model | subModel = subModel }
                , List.map (\ef -> ( SubPage navIndex, ef )) subEffects
                )

            else
                ( model, [] )

        TopMsg navIndex m ->
            if validNavIndex model.navIndex navIndex then
                let
                    ( topModel, topEffects ) =
                        TopBar.update m model.topModel
                in
                ( { model | topModel = topModel }
                , List.map (\ef -> ( TopBar navIndex, ef )) topEffects
                )

            else
                ( model, [] )

        TokenReceived Nothing ->
            ( model, [] )

        TokenReceived (Just tokenValue) ->
            let
                ( newSubModel, subCmd ) =
                    SubPage.update model.turbulenceImgSrc model.notFoundImgSrc tokenValue (SubPage.Msgs.NewCSRFToken tokenValue) model.subModel
            in
            ( { model
                | csrfToken = tokenValue
                , subModel = newSubModel
              }
            , List.map (\ef -> ( SubPage anyNavIndex, ef )) subCmd
            )

        Callback dispatch callback ->
            handleCallback dispatch callback model

        Noop ->
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


urlUpdate : Routes.ConcourseRoute -> Model -> ( Model, List ( LayoutDispatch, Effect ) )
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

        ( newTopModel, topEffects ) =
            if route == model.route then
                ( model.topModel, [] )

            else
                TopBar.urlUpdate route model.topModel
    in
    ( { model
        | navIndex = navIndex
        , subModel = newSubmodel
        , topModel = newTopModel
        , route = route
      }
    , List.map (\ef -> ( SubPage navIndex, ef )) subEffects
        ++ List.map (\ef -> ( TopBar navIndex, ef )) topEffects
        ++ [ ( Layout, SetFavIcon Nothing ) ]
    )


view : Model -> Html Msg
view model =
    case model.subModel of
        SubPage.DashboardModel _ ->
            Html.map (SubMsg model.navIndex) (SubPage.view model.subModel)

        SubPage.ResourceModel _ ->
            Html.map (SubMsg model.navIndex) (SubPage.view model.subModel)

        _ ->
            Html.div
                [ class "content-frame"
                , style
                    [ ( "-webkit-font-smoothing", "antialiased" )
                    , ( "font-weight", "700" )
                    ]
                ]
                [ Html.map (TopMsg model.navIndex) (TopBar.view model.topModel)
                , Html.div [ class "bottom" ]
                    [ Html.div [ id "content" ]
                        [ Html.div [ id "subpage" ]
                            [ Html.map
                                (SubMsg model.navIndex)
                                (SubPage.view model.subModel)
                            ]
                        ]
                    ]
                ]


subscriptions : Model -> Sub Msg
subscriptions model =
    case model.topBarType of
        Dashboard ->
            Sub.batch
                [ newUrl NewUrl
                , tokenReceived TokenReceived
                , Sub.map (SubMsg model.navIndex) <|
                    SubPage.subscriptions model.subModel
                ]

        Normal ->
            Sub.batch
                [ newUrl NewUrl
                , tokenReceived TokenReceived
                , Sub.map (TopMsg model.navIndex) <|
                    TopBar.subscriptions model.topModel
                , Sub.map (SubMsg model.navIndex) <|
                    SubPage.subscriptions model.subModel
                ]


routeMatchesModel : Routes.ConcourseRoute -> Model -> Bool
routeMatchesModel route model =
    case ( route.logical, model.subModel ) of
        ( Routes.Pipeline _ _, SubPage.PipelineModel _ ) ->
            True

        ( Routes.Resource _ _ _, SubPage.ResourceModel _ ) ->
            True

        ( Routes.Build _ _ _ _, SubPage.BuildModel _ ) ->
            True

        ( Routes.Job _ _ _, SubPage.JobModel _ ) ->
            True

        ( Routes.Dashboard, SubPage.DashboardModel _ ) ->
            True

        _ ->
            False
