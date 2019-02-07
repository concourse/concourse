module NotFound exposing (Model, Msg, handleCallback, init, update, view)

import Callback exposing (Callback)
import Effects exposing (Effect)
import Html exposing (Html)
import Html.Attributes exposing (class, href, src, style)
import Html.Styled as HS
import NewTopBar.Model
import NewTopBar.Msgs
import NewestTopBar
import Routes
import UserState exposing (UserState)


type alias Model =
    { notFoundImgSrc : String
    , topBar : NewTopBar.Model.Model
    }


type alias Flags =
    { route : Routes.Route
    , notFoundImgSrc : String
    }


type Msg
    = FromTopBar NewTopBar.Msgs.Msg


init : Flags -> ( Model, List Effect )
init flags =
    let
        ( topBar, topBarEffects ) =
            NewestTopBar.init { route = flags.route }
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
                    NewestTopBar.update m model.topBar
            in
            ( { model | topBar = newTopBar }, topBarEffects )


handleCallback : Callback -> Model -> ( Model, List Effect )
handleCallback msg model =
    let
        ( newTopBar, topBarEffects ) =
            NewestTopBar.handleCallback msg model.topBar
    in
    ( { model | topBar = newTopBar }, topBarEffects )


view : UserState -> Model -> Html Msg
view userState model =
    Html.div
        [ class "page"
        , style
            [ ( "-webkit-font-smoothing", "antialiased" )
            , ( "font-weight", "700" )
            ]
        ]
        [ NewestTopBar.view userState NewTopBar.Model.None model.topBar |> HS.toUnstyled |> Html.map FromTopBar
        , Html.div [ class "notfound" ]
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
