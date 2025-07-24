module DownloadFly.DownloadFly exposing
    ( documentTitle
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Api.Endpoints as Endpoints
import Application.Models exposing (Session)
import Assets exposing (Asset(..))
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
import Url.Builder
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
            --TODO: styles here is weird if you refresh the page
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
                        installSteps model.selectedPlatform

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


installSteps : Platform -> Html msg
installSteps platform =
    case platform of
        LinuxAmd64 ->
            linuxSteps "amd64"

        LinuxArm64 ->
            linuxSteps "arm64"

        MacosAmd64 ->
            macOSSteps "amd64"

        MacosArm64 ->
            macOSSteps "arm64"

        WindowsAmd64 ->
            windowsSteps "amd64"

        None ->
            Html.div [] []


linuxSteps : String -> Html msg
linuxSteps arch =
    let
        url =
            downloadUrlBuilder "linux" arch
    in
    Html.div
        [ class "install-steps" ]
        [ Html.div [] [ Html.text "Follow these steps to install fly:" ]
        , Html.code
            []
            [ Html.pre []
                [ Html.text <|
                    """curl '"""
                        ++ url
                        ++ """' -o fly
chmod +x ./fly
mv ./fly /usr/local/bin/"""
                ]
            ]
        ]


macOSSteps : String -> Html msg
macOSSteps arch =
    let
        url =
            downloadUrlBuilder "darwin" arch
    in
    Html.div
        [ class "install-steps" ]
        [ Html.div [] [ Html.text "Follow these steps to install fly:" ]
        , Html.code
            []
            [ Html.pre []
                [ Html.text <|
                    """curl '"""
                        ++ url
                        ++ """' -o fly
chmod +x ./fly
mv ./fly /usr/local/bin/"""
                ]
            ]
        ]


windowsSteps : String -> Html msg
windowsSteps arch =
    let
        url =
            downloadUrlBuilder "windows" arch
    in
    Html.div
        [ class "install-steps" ]
        [ Html.div [] [ Html.text "Follow these steps to install fly using PowerShell:" ]
        , Html.code
            []
            [ Html.pre []
                [ Html.text <|
                    -- TODO: verify windows steps work
                    """$concoursePath = 'C:\\concourse\\'
mkdir $concoursePath
[Environment]::SetEnvironmentVariable('PATH', "$ENV:PATH;${concoursePath}", 'USER')
$concourseURL = '"""
                        ++ url
                        ++ """'
Invoke-WebRequest $concourseURL -OutFile "${concoursePath}\\fly.exe\\\""""
                ]
            ]
        ]


downloadUrlBuilder : String -> String -> String
downloadUrlBuilder os arch =
    Endpoints.Cli
        |> Endpoints.toString
            [ Url.Builder.string "arch" arch
            , Url.Builder.string "platform" os
            ]
