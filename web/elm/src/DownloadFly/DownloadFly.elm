module DownloadFly.DownloadFly exposing
    ( defaultHostname
    , directDownloadLink
    , documentTitle
    , handleDelivery
    , init
    , linuxSteps
    , macosSteps
    , subscriptions
    , tooltip
    , update
    , view
    , windowsSteps
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
import Html.Attributes exposing (class, href, id, style)
import Html.Events exposing (onFocus, onInput)
import Login.Login as Login
import Message.Effects exposing (Effect(..))
import Message.Message as Message exposing (Message(..))
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


defaultHostname : String
defaultHostname =
    "https://CONCOURSE-URL"


init : Routes.Route -> ( Model, List Effect )
init route =
    ( { route = route
      , isUserMenuExpanded = False
      , selectedPlatform = None
      , hostname = defaultHostname
      }
    , []
    )


documentTitle : String
documentTitle =
    "Download fly CLI"


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
            , Login.view session.userState model
            ]
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar Routes.DownloadFly)
            [ SideBar.view session Nothing
            , Html.div [ class "download-fly-card" ]
                [ Html.p
                    [ class "title" ]
                    [ Html.text "Download fly CLI" ]
                , Html.div
                    [ class "body" ]
                    [ Html.select
                        [ class "platforms"
                        , onInput PlatformSelected
                        , onFocus Message.GetHostname
                        ]
                        [ Html.option [ platformValue None ] [ platformText None ]
                        , Html.option [ platformValue LinuxAmd64 ] [ platformText LinuxAmd64 ]
                        , Html.option [ platformValue LinuxArm64 ] [ platformText LinuxArm64 ]
                        , Html.option [ platformValue MacosAmd64 ] [ platformText MacosAmd64 ]
                        , Html.option [ platformValue MacosArm64 ] [ platformText MacosArm64 ]
                        , Html.option [ platformValue WindowsAmd64 ] [ platformText WindowsAmd64 ]
                        ]
                    , if model.selectedPlatform /= None then
                        installSteps model.selectedPlatform model.hostname

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

        Message.GetHostname ->
            ( model, Message.Effects.GetHostname :: effects )

        _ ->
            ( model, effects )


tooltip : Model -> a -> Maybe Tooltip.Tooltip
tooltip _ _ =
    Nothing


subscriptions : List Subscription
subscriptions =
    [ OnHostnameReceived ]


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        GotHostname hostname ->
            ( { model | hostname = hostname }, effects )

        _ ->
            ( model, effects )


installSteps : Platform -> String -> Html msg
installSteps platform baseUrl =
    case platform of
        LinuxAmd64 ->
            linuxSteps baseUrl "amd64"

        LinuxArm64 ->
            linuxSteps baseUrl "arm64"

        MacosAmd64 ->
            macosSteps baseUrl "amd64"

        MacosArm64 ->
            macosSteps baseUrl "arm64"

        WindowsAmd64 ->
            windowsSteps baseUrl "amd64"

        None ->
            Html.div [] []


linuxSteps : String -> String -> Html msg
linuxSteps baseUrl arch =
    let
        url =
            downloadUrlBuilder baseUrl "linux" arch
    in
    Html.div
        [ class "install-steps" ]
        [ Html.div [] [ Html.text "Run these steps in your terminal to install fly:" ]
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
        , directDownloadLink url
        ]


macosSteps : String -> String -> Html msg
macosSteps baseUrl arch =
    let
        url =
            downloadUrlBuilder baseUrl "darwin" arch
    in
    Html.div
        [ class "install-steps" ]
        [ Html.div [] [ Html.text "Run these steps in your terminal to install fly:" ]
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
        , directDownloadLink url
        ]


windowsSteps : String -> String -> Html msg
windowsSteps baseUrl arch =
    let
        url =
            downloadUrlBuilder baseUrl "windows" arch
    in
    Html.div
        [ class "install-steps" ]
        [ Html.div [] [ Html.text "Run these steps in your PowerShell terminal to install fly:" ]
        , Html.code
            []
            [ Html.pre []
                [ Html.text <|
                    """$concoursePath = 'C:\\concourse\\'
mkdir $concoursePath
[Environment]::SetEnvironmentVariable('PATH', "$ENV:PATH;${concoursePath}", 'USER')
$concourseURL = '"""
                        ++ url
                        ++ """'
Invoke-WebRequest $concourseURL -OutFile "${concoursePath}\\fly.exe\""""
                ]
            ]
        , directDownloadLink url
        ]


downloadUrlBuilder : String -> String -> String -> String
downloadUrlBuilder baseUrl os arch =
    baseUrl
        ++ (Endpoints.Cli
                |> Endpoints.toString
                    [ Url.Builder.string "arch" arch
                    , Url.Builder.string "platform" os
                    ]
           )


directDownloadLink : String -> Html msg
directDownloadLink url =
    Html.div []
        [ Html.div [] [ Html.text "Or use this link to download the binary:" ]
        , Html.a [ href url ] [ Html.text url ]
        ]
