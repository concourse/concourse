port module BetaLayout exposing (Flags, Model, Msg, init, locationMsg, subscriptions, update, view)

import BetaRoutes
import BetaSubPage
import BetaTopBar
import Favicon
import Html exposing (Html)
import Html.Attributes as Attributes exposing (class, id)
import BetaLogin exposing (Msg(..))
import Navigation
import BetaSideBar
import Task exposing (Task)


port newUrl : (String -> msg) -> Sub msg


type alias Flags =
    { turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : String
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
    , subModel : BetaSubPage.Model
    , topModel : BetaTopBar.Model
    , sideModel : BetaSideBar.Model
    , sidebarVisible : Bool
    , turbulenceImgSrc : String
    , notFoundImgSrc : String
    , csrfToken : String
    , route : BetaRoutes.ConcourseRoute
    }


type Msg
    = Noop
    | RouteChanged BetaRoutes.ConcourseRoute
    | BetaSubMsg NavIndex BetaSubPage.Msg
    | BetaTopMsg NavIndex BetaTopBar.Msg
    | BetaSideMsg NavIndex BetaSideBar.Msg
    | NewUrl String
    | ModifyUrl String
    | SaveToken String
    | LoadToken
    | TokenReceived (Maybe String)


init : Flags -> Navigation.Location -> ( Model, Cmd Msg )
init flags location =
    let
        route =
            BetaRoutes.parsePath location

        ( subModel, subCmd ) =
            BetaSubPage.init flags.turbulenceImgSrc route

        ( topModel, topCmd ) =
            BetaTopBar.init route

        ( sideModel, sideCmd ) =
            BetaSideBar.init { csrfToken = flags.csrfToken }

        navIndex =
            1

        model =
            { navIndex = navIndex
            , subModel = subModel
            , topModel = topModel
            , sideModel = sideModel
            , sidebarVisible = False
            , turbulenceImgSrc = flags.turbulenceImgSrc
            , notFoundImgSrc = flags.notFoundImgSrc
            , route = route
            , csrfToken = flags.csrfToken
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
                Navigation.modifyUrl (BetaRoutes.customToString route)
    in
        ( model
        , Cmd.batch
            [ handleTokenCmd
            , stripCSRFTokenParamCmd
            , Cmd.map (BetaSubMsg navIndex) subCmd
            , Cmd.map (BetaTopMsg navIndex) topCmd
            , Cmd.map (BetaSideMsg navIndex) sideCmd
            ]
        )


locationMsg : Navigation.Location -> Msg
locationMsg =
    RouteChanged << BetaRoutes.parsePath


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        NewUrl url ->
            ( model, Navigation.newUrl url )

        ModifyUrl url ->
            ( model, Navigation.modifyUrl url )

        RouteChanged route ->
            urlUpdate route model

        BetaTopMsg _ BetaTopBar.ToggleSidebar ->
            ( { model
                | sidebarVisible = not model.sidebarVisible
              }
            , Cmd.none
            )

        SaveToken tokenValue ->
            ( model, saveToken tokenValue )

        LoadToken ->
            ( model, loadToken () )

        TokenReceived Nothing ->
            ( model, Cmd.none )

        TokenReceived (Just tokenValue) ->
            let
                ( newSubModel, subCmd ) =
                    BetaSubPage.update model.turbulenceImgSrc model.notFoundImgSrc tokenValue (BetaSubPage.NewCSRFToken tokenValue) model.subModel

                ( newSideModel, sideCmd ) =
                    BetaSideBar.update (BetaSideBar.NewCSRFToken tokenValue) model.sideModel
            in
                ( { model
                    | csrfToken = tokenValue
                    , subModel = newSubModel
                    , sideModel = newSideModel
                  }
                , Cmd.batch
                    [ Cmd.map (BetaSubMsg anyNavIndex) subCmd
                    , Cmd.map (BetaSideMsg anyNavIndex) sideCmd
                    ]
                )

        BetaSubMsg navIndex (BetaSubPage.BetaLoginMsg (BetaLogin.AuthSessionReceived (Ok val))) ->
            let
                ( layoutModel, layoutCmd ) =
                    update (SaveToken val.csrfToken) model

                ( subModel, subCmd ) =
                    BetaSubPage.update model.turbulenceImgSrc model.notFoundImgSrc val.csrfToken (BetaSubPage.BetaLoginMsg (BetaLogin.AuthSessionReceived (Ok val))) model.subModel

                ( sideModel, sideCmd ) =
                    BetaSideBar.update (BetaSideBar.NewCSRFToken val.csrfToken) model.sideModel
            in
                ( { model
                    | subModel = subModel
                    , sideModel = sideModel
                    , csrfToken = val.csrfToken
                  }
                , Cmd.batch
                    [ layoutCmd
                    , Cmd.map (BetaSideMsg anyNavIndex) sideCmd
                    , Cmd.map (BetaTopMsg anyNavIndex) BetaTopBar.fetchUser
                    , Cmd.map (BetaSideMsg anyNavIndex) BetaSideBar.fetchPipelines
                    , Cmd.map (BetaSubMsg navIndex) subCmd
                    ]
                )

        BetaSubMsg navIndex (BetaSubPage.PipelinesFetched (Ok pipelines)) ->
            let
                pipeline =
                    List.head pipelines

                ( subModel, subCmd ) =
                    BetaSubPage.update
                        model.turbulenceImgSrc
                        model.notFoundImgSrc
                        model.csrfToken
                        (BetaSubPage.DefaultPipelineFetched pipeline)
                        model.subModel
            in
                case pipeline of
                    Nothing ->
                        ( { model
                            | subModel = subModel
                          }
                        , Cmd.map (BetaSubMsg navIndex) subCmd
                        )

                    Just p ->
                        let
                            ( topModel, topCmd ) =
                                BetaTopBar.update
                                    (BetaTopBar.FetchPipeline { teamName = p.teamName, pipelineName = p.name })
                                    model.topModel
                        in
                            ( { model
                                | subModel = subModel
                                , topModel = topModel
                              }
                            , Cmd.batch
                                [ Cmd.map (BetaSubMsg navIndex) subCmd
                                , Cmd.map (BetaTopMsg navIndex) topCmd
                                ]
                            )

        -- otherwise, pass down
        BetaSubMsg navIndex m ->
            if validNavIndex model.navIndex navIndex then
                let
                    ( subModel, subCmd ) =
                        BetaSubPage.update model.turbulenceImgSrc model.notFoundImgSrc model.csrfToken m model.subModel
                in
                    ( { model | subModel = subModel }, Cmd.map (BetaSubMsg navIndex) subCmd )
            else
                ( model, Cmd.none )

        BetaTopMsg navIndex m ->
            if validNavIndex model.navIndex navIndex then
                let
                    ( topModel, topCmd ) =
                        BetaTopBar.update m model.topModel
                in
                    ( { model | topModel = topModel }, Cmd.map (BetaTopMsg navIndex) topCmd )
            else
                ( model, Cmd.none )

        BetaSideMsg navIndex m ->
            if validNavIndex model.navIndex navIndex then
                let
                    ( sideModel, sideCmd ) =
                        BetaSideBar.update m model.sideModel
                in
                    ( { model | sideModel = sideModel }, Cmd.map (BetaSideMsg navIndex) sideCmd )
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


urlUpdate : BetaRoutes.ConcourseRoute -> Model -> ( Model, Cmd Msg )
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
                BetaSubPage.urlUpdate route model.subModel
            else
                BetaSubPage.init model.turbulenceImgSrc route

        ( newTopModel, tCmd ) =
            if route == model.route then
                ( model.topModel, Cmd.none )
            else
                BetaTopBar.urlUpdate route model.topModel
    in
        ( { model
            | navIndex = navIndex
            , subModel = newSubmodel
            , topModel = newTopModel
            , route = route
          }
        , Cmd.batch
            [ Cmd.map (BetaSubMsg navIndex) cmd
            , Cmd.map (BetaTopMsg navIndex) tCmd
            , resetFavicon
            ]
        )


resetFavicon : Cmd Msg
resetFavicon =
    Task.perform (always Noop) <|
        Favicon.set "/public/images/favicon.png"


view : Model -> Html Msg
view model =
    let
        sidebarVisibileAppendage =
            case model.sidebarVisible of
                True ->
                    " visible"

                False ->
                    ""
    in
        case model.subModel of
            _ ->
                Html.div [ class "content-frame" ]
                    [ Html.div [ id "top-bar-app" ]
                        [ Html.map (BetaTopMsg model.navIndex) (BetaTopBar.view model.topModel) ]
                    , Html.div [ class "bottom" ]
                        [ Html.div
                            [ id "pipelines-nav-app"
                            , class <| "sidebar test" ++ sidebarVisibileAppendage
                            ]
                            [ Html.map (BetaSideMsg model.navIndex) (BetaSideBar.view model.sideModel) ]
                        , Html.div [ id "content" ]
                            [ Html.div [ id "BetaSubPage" ]
                                [ Html.map (BetaSubMsg model.navIndex) (BetaSubPage.view model.subModel) ]
                            ]
                        ]
                    ]


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ newUrl NewUrl
        , tokenReceived TokenReceived
        , Sub.map (BetaTopMsg model.navIndex) <| BetaTopBar.subscriptions model.topModel
        , Sub.map (BetaSideMsg model.navIndex) <| BetaSideBar.subscriptions model.sideModel
        , Sub.map (BetaSubMsg model.navIndex) <| BetaSubPage.subscriptions model.subModel
        ]


routeMatchesModel : BetaRoutes.ConcourseRoute -> Model -> Bool
routeMatchesModel route model =
    case ( route.logical, model.subModel ) of
        ( BetaRoutes.BetaSelectTeam, BetaSubPage.BetaSelectTeamModel _ ) ->
            True

        ( BetaRoutes.BetaTeamLogin _, BetaSubPage.BetaLoginModel _ ) ->
            True

        ( BetaRoutes.BetaPipeline _ _, BetaSubPage.BetaPipelineModel _ ) ->
            True

        ( BetaRoutes.BetaResource _ _ _, BetaSubPage.BetaResourceModel _ ) ->
            True

        ( BetaRoutes.BetaBuild _ _ _ _, BetaSubPage.BetaBuildModel _ ) ->
            True

        ( BetaRoutes.BetaJob _ _ _, BetaSubPage.BetaJobModel _ ) ->
            True

        _ ->
            False
