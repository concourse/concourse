module DownloadFly.Model exposing
    ( Model
    , Platform(..)
    , platformText
    , platformValue
    )

import Html
import Html.Attributes exposing (value)
import Login.Login as Login
import Routes


type alias Model =
    Login.Model
        { route : Routes.Route }


type Platform
    = None
    | LinuxAmd64
    | LinuxArm64
    | MacosAmd64
    | MacosArm64
    | WindowsAmd64


platformValue : Platform -> Html.Attribute msg
platformValue platform =
    case platform of
        LinuxAmd64 ->
            value "linux-amd64"

        LinuxArm64 ->
            value "linux-arm64"

        MacosAmd64 ->
            value "macos-amd64"

        MacosArm64 ->
            value "macos-arm64"

        WindowsAmd64 ->
            value "windows-amd64"

        _ ->
            value ""


platformText : Platform -> Html.Html msg
platformText platform =
    case platform of
        LinuxAmd64 ->
            Html.text "Linux (x86_64)"

        LinuxArm64 ->
            Html.text "Linux (arm64)"

        MacosAmd64 ->
            Html.text "macOS (x86_64)"

        MacosArm64 ->
            Html.text "macOS (arm64)"

        WindowsAmd64 ->
            Html.text "Windows (x86_64)"

        _ ->
            Html.text "Select a platform..."
