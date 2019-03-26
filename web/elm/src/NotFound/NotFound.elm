module NotFound.NotFound exposing (init, view)

import Html exposing (Html)
import Html.Attributes exposing (class, href, id, src, style)
import Login.Login as Login
import Message.Effects as Effects exposing (Effect)
import Message.Message exposing (Message(..))
import NotFound.Model exposing (Model)
import Routes
import UserState exposing (UserState)
import Views.Styles
import Views.TopBar as TopBar


type alias Flags =
    { route : Routes.Route
    , notFoundImgSrc : String
    }


init : Flags -> ( Model, List Effect )
init flags =
    ( { notFoundImgSrc = flags.notFoundImgSrc
      , route = flags.route
      , isUserMenuExpanded = False
      }
    , [ Effects.SetTitle "Not Found " ]
    )


view : UserState -> Model -> Html Message
view userState model =
    Html.div []
        [ Html.div
            [ style Views.Styles.pageIncludingTopBar
            , id "page-including-top-bar"
            ]
            [ Html.div
                [ id "top-bar-app"
                , style <| Views.Styles.topBar False
                ]
                [ TopBar.concourseLogo
                , TopBar.breadcrumbs model.route
                , Login.view userState model False
                ]
            , Html.div
                [ id "page-below-top-bar"
                , style <| Views.Styles.pageBelowTopBar model.route
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
