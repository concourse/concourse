module NotFound.NotFound exposing (documentTitle, init, view)

import Browser
import Html exposing (Html)
import Html.Attributes exposing (class, href, id, src)
import Login.Login as Login
import Message.Effects as Effects exposing (Effect)
import Message.Message exposing (Message(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
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
    , []
    )


documentTitle : Model -> String
documentTitle model =
    "Not Found"


view : UserState -> Model -> Html Message
view userState model =
    Html.div []
        [ Html.div
            ([ id "page-including-top-bar" ] ++ Views.Styles.pageIncludingTopBar)
            [ Html.div
                ([ id "top-bar-app" ] ++ Views.Styles.topBar False)
                [ TopBar.concourseLogo
                , TopBar.breadcrumbs model.route
                , Login.view userState model False
                ]
            , Html.div
                ([ id "page-below-top-bar" ] ++ Views.Styles.pageBelowTopBar)
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
