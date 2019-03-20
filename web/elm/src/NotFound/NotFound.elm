module NotFound.NotFound exposing (handleCallback, init, update, view)

import Html exposing (Html)
import Html.Attributes exposing (class, href, id, src, style)
import Message.Callback exposing (Callback)
import Message.Effects as Effects exposing (Effect)
import Message.Message exposing (Message(..))
import NotFound.Model exposing (Model)
import Routes
import TopBar.Model
import TopBar.Styles
import TopBar.TopBar as TopBar
import UserState exposing (UserState)


type alias Flags =
    { route : Routes.Route
    , notFoundImgSrc : String
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        ( topBar, topBarEffects ) =
            TopBar.init { route = flags.route }
    in
    ( { notFoundImgSrc = flags.notFoundImgSrc
      , isUserMenuExpanded = topBar.isUserMenuExpanded
      , isPinMenuExpanded = topBar.isPinMenuExpanded
      , groups = topBar.groups
      , route = topBar.route
      , dropdown = topBar.dropdown
      , screenSize = topBar.screenSize
      , shiftDown = topBar.shiftDown
      }
    , topBarEffects ++ [ Effects.SetTitle "Not Found " ]
    )


update : Message -> ( Model, List Effect ) -> ( Model, List Effect )
update msg ( model, effects ) =
    TopBar.update msg ( model, effects )


handleCallback : Callback -> ( Model, List Effect ) -> ( Model, List Effect )
handleCallback msg ( model, effects ) =
    TopBar.handleCallback msg ( model, effects )


view : UserState -> Model -> Html Message
view userState model =
    Html.div []
        [ Html.div
            [ style TopBar.Styles.pageIncludingTopBar
            , id "page-including-top-bar"
            ]
            [ TopBar.view userState Nothing model
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
