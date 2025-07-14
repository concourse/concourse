module DownloadFly.DownloadFly exposing
    ( documentTitle
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , view
    )

import Application.Models exposing (Session)
import DownloadFly.Model exposing (Model)
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (class, href, id, src, style)
import Login.Login as Login
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Message(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Routes
import SideBar.SideBar as SideBar
import Tooltip
import Views.Styles
import Views.TopBar as TopBar


type alias Flags =
    { route : Routes.Route }


init : Flags -> ( Model, List Effect )
init flags =
    ( { route = flags.route
      , isUserMenuExpanded = False
      }
    , []
    )


documentTitle : String
documentTitle =
    "Download fly cli"


view : Session -> Model -> Html Message
view session model =
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            [ Html.div
                [ style "display" "flex", style "align-items" "center" ]
                (SideBar.sideBarIcon session
                    :: TopBar.breadcrumbs session model.route
                )
            , Html.div []
                [ Login.view session.userState model ]
            ]
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar model.route)
            [ SideBar.view session Nothing
            , Html.div [ class "notfound" ]
                [ Html.div [ class "title" ] [ Html.text "Download Fly" ]
                , Html.div [ class "reason" ] [ Html.text "DOWNLOAD FLY CLI" ]
                ]
            ]
        ]


tooltip : Model -> a -> Maybe Tooltip.Tooltip
tooltip _ _ =
    Nothing


subscriptions : List Subscription
subscriptions =
    []


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        _ ->
            ( model, effects )
