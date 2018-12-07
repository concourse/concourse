port module Layout
    exposing
        ( Flags
        , Model
        , Msg(..)
        , init
        , locationMsg
        , subscriptions
        , update
        , view
        )

import Concourse
import Favicon
import FlySuccess
import Html exposing (Html)
import Html.Attributes as Attributes exposing (class, id)
import Json.Decode
import Navigation
import Pipeline
import Routes
import SubPage
import Task exposing (Task)
import TopBar


port newUrl : (String -> msg) -> Sub msg


type alias Flags =
    { turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : String
    , pipelineRunningKeyframes : String
    }


type alias NavIndex =
    Int


anyNavIndex : NavIndex
anyNavIndex =
    -1


port saveToken : String -> Cmd msg


port tokenReceived : (Maybe String -> msg) -> Sub msg


port loadToken : () -> Cmd msg


type alias Model =
    { navIndex : NavIndex
    , subModel : SubPage.Model
    , topModel : TopBar.Model
    , topBarType : TopBarType
    , turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : String
    , pipelineRunningKeyframes : String
    , route : Routes.ConcourseRoute
    }


type TopBarType
    = Dashboard
    | Normal


type Msg
    = Noop
    | RouteChanged Routes.ConcourseRoute
    | SubMsg NavIndex SubPage.Msg
    | TopMsg NavIndex TopBar.Msg
    | NewUrl String
    | ModifyUrl String
    | SaveToken String
    | LoadToken
    | TokenReceived (Maybe String)


init : Flags -> Navigation.Location -> ( Model, Cmd Msg )
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

        ( subModel, subCmd ) =
            SubPage.init
                { turbulencePath = flags.turbulenceImgSrc
                , csrfToken = flags.csrfToken
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                }
                route

        ( topModel, topCmd ) =
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
            , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
            , route = route
            }

        handleTokenCmd =
            -- We've refreshed on the page and we're not
            -- getting it from query params
            if flags.csrfToken == "" then
                loadToken ()
            else
                saveToken flags.csrfToken

        stripCSRFTokenParamCmd =
            if flags.csrfToken == "" then
                Cmd.none
            else
                Navigation.modifyUrl (Routes.customToString route)
    in
        ( model
        , Cmd.batch
            [ handleTokenCmd
            , stripCSRFTokenParamCmd
            , Cmd.map (SubMsg navIndex) subCmd
            , Cmd.map (TopMsg navIndex) topCmd
            ]
        )


locationMsg : Navigation.Location -> Msg
locationMsg =
    RouteChanged << Routes.parsePath


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        NewUrl url ->
            ( model, Navigation.newUrl url )

        ModifyUrl url ->
            ( model, Navigation.modifyUrl url )

        RouteChanged route ->
            urlUpdate route model

        SaveToken tokenValue ->
            ( model, saveToken tokenValue )

        LoadToken ->
            ( model, loadToken () )

        TokenReceived Nothing ->
            ( model, Cmd.none )

        TokenReceived (Just tokenValue) ->
            let
                ( newSubModel, subCmd ) =
                    SubPage.update model.turbulenceImgSrc model.notFoundImgSrc tokenValue (SubPage.NewCSRFToken tokenValue) model.subModel
            in
                ( { model
                    | csrfToken = tokenValue
                    , subModel = newSubModel
                  }
                , Cmd.batch
                    [ Cmd.map (SubMsg anyNavIndex) subCmd
                    ]
                )

        SubMsg navIndex (SubPage.PipelineMsg (Pipeline.ResourcesFetched (Ok fetchedResources))) ->
            let
                resources : Result String (List Concourse.Resource)
                resources =
                    Json.Decode.decodeValue (Json.Decode.list Concourse.decodeResource) fetchedResources

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
                        ( subModel, subCmd ) =
                            SubPage.update
                                model.turbulenceImgSrc
                                model.notFoundImgSrc
                                model.csrfToken
                                (SubPage.PipelineMsg (Pipeline.ResourcesFetched (Ok fetchedResources)))
                                model.subModel
                    in
                        ( { model
                            | subModel = subModel
                            , topModel = { topBar | pinnedResources = pinnedResources }
                          }
                        , Cmd.map (SubMsg navIndex) subCmd
                        )
                else
                    ( model, Cmd.none )

        SubMsg navIndex (SubPage.FlySuccessMsg (FlySuccess.CopyTokenButtonHover bool)) ->
            let
                newSubModel =
                    case model.subModel of
                        SubPage.FlySuccess m ->
                            SubPage.FlySuccess (FlySuccess.hover bool m)

                        _ ->
                            model.subModel
            in
                ( { model | subModel = newSubModel }, Cmd.none )

        SubMsg navIndex (SubPage.FlySuccessMsg FlySuccess.CopyToken) ->
            let
                newSubModel =
                    case model.subModel of
                        SubPage.FlySuccess m ->
                            SubPage.FlySuccess (FlySuccess.copied m)

                        _ ->
                            model.subModel
            in
                ( { model | subModel = newSubModel }, Cmd.none )

        -- otherwise, pass down
        SubMsg navIndex m ->
            if validNavIndex model.navIndex navIndex then
                let
                    ( subModel, subCmd ) =
                        SubPage.update model.turbulenceImgSrc model.notFoundImgSrc model.csrfToken m model.subModel
                in
                    ( { model | subModel = subModel }, Cmd.map (SubMsg navIndex) subCmd )
            else
                ( model, Cmd.none )

        TopMsg navIndex m ->
            if validNavIndex model.navIndex navIndex then
                let
                    ( topModel, topCmd ) =
                        TopBar.update m model.topModel
                in
                    ( { model | topModel = topModel }, Cmd.map (TopMsg navIndex) topCmd )
            else
                ( model, Cmd.none )

        Noop ->
            ( model, Cmd.none )


validNavIndex : NavIndex -> NavIndex -> Bool
validNavIndex modelNavIndex navIndex =
    if navIndex == anyNavIndex then
        True
    else
        navIndex == modelNavIndex


urlUpdate : Routes.ConcourseRoute -> Model -> ( Model, Cmd Msg )
urlUpdate route model =
    let
        navIndex =
            if route == model.route then
                model.navIndex
            else
                model.navIndex + 1

        ( newSubmodel, cmd ) =
            if route == model.route then
                ( model.subModel, Cmd.none )
            else if routeMatchesModel route model then
                SubPage.urlUpdate route model.subModel
            else
                SubPage.init
                    { turbulencePath = model.turbulenceImgSrc
                    , csrfToken = model.csrfToken
                    , pipelineRunningKeyframes = model.pipelineRunningKeyframes
                    }
                    route

        ( newTopModel, tCmd ) =
            if route == model.route then
                ( model.topModel, Cmd.none )
            else
                TopBar.urlUpdate route model.topModel
    in
        ( { model
            | navIndex = navIndex
            , subModel = newSubmodel
            , topModel = newTopModel
            , route = route
          }
        , Cmd.batch
            [ Cmd.map (SubMsg navIndex) cmd
            , Cmd.map (TopMsg navIndex) tCmd
            , resetFavicon
            ]
        )


resetFavicon : Cmd Msg
resetFavicon =
    Task.perform (always Noop) <|
        Favicon.set "/public/images/favicon.png"


view : Model -> Html Msg
view model =
    case model.subModel of
        SubPage.DashboardModel _ ->
            Html.map (SubMsg model.navIndex) (SubPage.view model.subModel)

        _ ->
            Html.div [ class "content-frame" ]
                [ Html.map (TopMsg model.navIndex) (TopBar.view model.topModel)
                , Html.div [ class "bottom" ]
                    [ Html.div [ id "content" ]
                        [ Html.div [ id "subpage" ]
                            [ Html.map (SubMsg model.navIndex) (SubPage.view model.subModel) ]
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
                , Sub.map (SubMsg model.navIndex) <| SubPage.subscriptions model.subModel
                ]

        Normal ->
            Sub.batch
                [ newUrl NewUrl
                , tokenReceived TokenReceived
                , Sub.map (TopMsg model.navIndex) <| TopBar.subscriptions model.topModel
                , Sub.map (SubMsg model.navIndex) <| SubPage.subscriptions model.subModel
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
