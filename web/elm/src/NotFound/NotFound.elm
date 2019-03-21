module NotFound.NotFound exposing (handleCallback, init, update, view)

import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (class, href, id, src, style)
import Message.Callback exposing (Callback)
import Message.Effects as Effects exposing (Effect)
import Message.Message exposing (Message(..))
import NotFound.Model exposing (Model)
import Routes
import TopBar.Styles
import TopBar.TopBar as TopBar
import UserState exposing (UserState)
import Views.Login as Login


type alias Flags =
    { route : Routes.Route
    , notFoundImgSrc : String
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        ( topBar, topBarEffects ) =
            TopBar.init
    in
    ( { notFoundImgSrc = flags.notFoundImgSrc
      , route = flags.route
      , isUserMenuExpanded = topBar.isUserMenuExpanded
      , groups = topBar.groups
      , screenSize = topBar.screenSize
      , shiftDown = topBar.shiftDown
      }
    , topBarEffects ++ [ Effects.SetTitle "Not Found " ]
    )


update : Message -> ET Model
update msg ( model, effects ) =
    TopBar.update msg ( model, effects )


handleCallback : Callback -> ET Model
handleCallback msg ( model, effects ) =
    TopBar.handleCallback msg ( model, effects )


view : UserState -> Model -> Html Message
view userState model =
    Html.div []
        [ Html.div
            [ style TopBar.Styles.pageIncludingTopBar
            , id "page-including-top-bar"
            ]
            [ Html.div
                [ id "top-bar-app"
                , style <| TopBar.Styles.topBar False
                ]
                [ TopBar.viewConcourseLogo
                , TopBar.viewBreadcrumbs model.route
                , Login.view userState model False
                ]
            , Html.div
                [ id "page-below-top-bar"
                , style TopBar.Styles.pageBelowTopBar
                ]
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
