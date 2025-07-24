module DownloadFly.DownloadFly exposing
    ( documentTitle
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Application.Models exposing (Session)
import DownloadFly.Model
    exposing
        ( Model
        , Platform(..)
        , platformText
        , platformValue
        , valueToPlatform
        )
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes exposing (class, href, id, src, style, value)
import Html.Events exposing (onInput)
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
      , selectedPlatform = None
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
            , Html.div [ class "download-fly-card" ]
                [ Html.p
                    [ class "title" ]
                    [ Html.text "Download fly CLI" ]
                , Html.div
                    [ class "body" ]
                    [ Html.select
                        [ onInput PlatformSelected
                        ]
                        [ Html.option [ platformValue None ] [ platformText None ]
                        , Html.option [ platformValue LinuxAmd64 ] [ platformText LinuxAmd64 ]
                        , Html.option [ platformValue LinuxArm64 ] [ platformText LinuxArm64 ]
                        , Html.option [ platformValue MacosAmd64 ] [ platformText MacosAmd64 ]
                        , Html.option [ platformValue MacosArm64 ] [ platformText MacosArm64 ]
                        , Html.option [ platformValue WindowsAmd64 ] [ platformText WindowsAmd64 ]
                        ]
                    , if model.selectedPlatform /= None then
                        Html.div
                            [ class "selected-platform" ]
                            [ Html.text "Selected platform: ", platformText model.selectedPlatform ]

                      else
                        Html.text ""
                    ]
                ]
            ]
        ]


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        PlatformSelected platform ->
            ( { model | selectedPlatform = valueToPlatform platform }
            , effects
            )

        _ ->
            ( model, effects )


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
