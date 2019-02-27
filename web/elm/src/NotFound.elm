module NotFound exposing (Model, Msg, handleCallback, init, update, view)

import Callback exposing (Callback)
import Effects exposing (Effect)
import Html exposing (Html)
import Html.Attributes exposing (class, href, id, src, style)
import Html.Styled as HS
import Routes
import TopBar.Model
import TopBar.Msgs
import TopBar.Styles
import TopBar.TopBar as TopBar
import UserState exposing (UserState)


type alias Model =
    { notFoundImgSrc : String
    , topBar : TopBar.Model.Model {}
    }


type alias Flags =
    { route : Routes.Route
    , notFoundImgSrc : String
    }


type Msg
    = FromTopBar TopBar.Msgs.Msg


init : Flags -> ( Model, List Effect )
init flags =
    let
        ( topBar, topBarEffects ) =
            TopBar.init { route = flags.route }
    in
    ( { notFoundImgSrc = flags.notFoundImgSrc
      , topBar = topBar
      }
    , topBarEffects ++ [ Effects.SetTitle "Not Found " ]
    )


update : Msg -> Model -> ( Model, List Effect )
update msg model =
    case msg of
        FromTopBar m ->
            let
                ( newTopBar, topBarEffects ) =
                    TopBar.update m ( model.topBar, [] )
            in
            ( { model | topBar = newTopBar }, topBarEffects )


handleCallback : Callback -> Model -> ( Model, List Effect )
handleCallback msg model =
    let
        ( newTopBar, topBarEffects ) =
            TopBar.handleCallback msg ( model.topBar, [] )
    in
    ( { model | topBar = newTopBar }, topBarEffects )


view : UserState -> Model -> Html Msg
view userState model =
    Html.div []
        [ Html.div
            [ style TopBar.Styles.pageIncludingTopBar
            , id "page-including-top-bar"
            ]
            [ TopBar.view userState TopBar.Model.None model.topBar |> HS.toUnstyled |> Html.map FromTopBar
            , Html.div [ id "page-below-top-bar", style TopBar.Styles.pageBelowTopBar ]
                [ Html.div [ class "notfound" ]
                    [ Html.div [ class "title" ] [ Html.text "404" ]
                    , Html.div [ class "reason" ] [ Html.text "this page was not found" ]
                    , Html.img [ src model.notFoundImgSrc ] []
                    , Html.div [ class "help-message" ]
                        [ Html.text "Not to worry, you can head"
                        , Html.br [] []
                        , Html.text "back to the "
                        , Html.a [ href "/" ] [ Html.text "home page" ]
                        ]
                    ]
                ]
            ]
        ]
