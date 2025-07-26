module DownloadFlyTests exposing (all)

import Application.Application as Application
import Common exposing (initRoute, queryView)
import DownloadFly.DownloadFly as DownloadFly
import Expect exposing (..)
import Html.Attributes as Attr
import Message.Message as Message exposing (Message(..))
import Message.TopLevelMessage as Msgs
import Routes exposing (Route(..))
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector as Selector exposing (attribute, class, text)


all : Test
all =
    describe "Download Fly"
        [ test "page title is correct" <|
            \_ ->
                Common.initRoute Routes.DownloadFly
                    |> Application.view
                    |> .title
                    |> Expect.equal "Download fly cli - Concourse"
        , test "does not show install steps on init" <|
            \_ ->
                Common.initRoute Routes.DownloadFly
                    |> queryView
                    |> Query.findAll [ class "install-steps" ]
                    |> Query.count (Expect.equal 0)
        , describe "platform dropdown" <|
            let
                dropdown =
                    Common.initRoute Routes.DownloadFly
                        |> queryView
                        |> Query.find [ class "platforms" ]
                        |> Query.children []
            in
            [ test "is populated" <|
                \_ ->
                    dropdown
                        |> Query.count (Expect.equal 6)
            , test "contains 'Select a platform...' as the first option" <|
                \_ ->
                    dropdown
                        |> Query.index 0
                        |> Query.has
                            [ attribute <| Attr.attribute "value" ""
                            , text "Select a platform..."
                            ]
            , test "contains linux-amd64" <|
                \_ ->
                    dropdown
                        |> Query.index 1
                        |> Query.has
                            [ attribute <| Attr.attribute "value" "linux-amd64"
                            , text "Linux (x86_64)"
                            ]
            , test "contains linux-arm64" <|
                \_ ->
                    dropdown
                        |> Query.index 2
                        |> Query.has
                            [ attribute <| Attr.attribute "value" "linux-arm64"
                            , text "Linux (arm64)"
                            ]
            , test "contains macos-amd64" <|
                \_ ->
                    dropdown
                        |> Query.index 3
                        |> Query.has
                            [ attribute <| Attr.attribute "value" "macos-amd64"
                            , text "macOS (x86_64)"
                            ]
            , test "contains macos-arm64" <|
                \_ ->
                    dropdown
                        |> Query.index 4
                        |> Query.has
                            [ attribute <| Attr.attribute "value" "macos-arm64"
                            , text "macOS (arm64)"
                            ]
            , test "contains windows-amd64" <|
                \_ ->
                    dropdown
                        |> Query.index 5
                        |> Query.has
                            [ attribute <| Attr.attribute "value" "windows-amd64"
                            , text "Windows (x86_64)"
                            ]
            ]
        , describe "shows install steps"
            [ test "none" <|
                \_ ->
                    let
                        installSteps =
                            Common.initRoute Routes.DownloadFly
                                |> Application.update (Msgs.Update <| Message.PlatformSelected "")
                                |> Tuple.first
                                |> queryView
                    in
                    installSteps
                        |> Query.findAll [ class "install-steps" ]
                        |> Query.count (Expect.equal 0)
            , test "linux" <|
                \_ ->
                    let
                        installSteps =
                            Common.initRoute Routes.DownloadFly
                                |> Application.update (Msgs.Update <| Message.PlatformSelected "linux-amd64")
                                |> Tuple.first
                                |> queryView

                        expectedInstructions =
                            DownloadFly.linuxSteps DownloadFly.defaultHostname "amd64"
                    in
                    installSteps
                        |> Query.contains [ expectedInstructions ]
            , test "macOS" <|
                \_ ->
                    let
                        installSteps =
                            Common.initRoute Routes.DownloadFly
                                |> Application.update (Msgs.Update <| Message.PlatformSelected "macos-amd64")
                                |> Tuple.first
                                |> queryView

                        expectedInstructions =
                            DownloadFly.macosSteps DownloadFly.defaultHostname "amd64"
                    in
                    installSteps
                        |> Query.contains [ expectedInstructions ]
            , test "windows" <|
                \_ ->
                    let
                        installSteps =
                            Common.initRoute Routes.DownloadFly
                                |> Application.update (Msgs.Update <| Message.PlatformSelected "windows-amd64")
                                |> Tuple.first
                                |> queryView

                        expectedInstructions =
                            DownloadFly.windowsSteps DownloadFly.defaultHostname "amd64"
                    in
                    installSteps
                        |> Query.contains [ expectedInstructions ]
            ]
        ]
