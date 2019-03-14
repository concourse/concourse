module CliTests exposing (all)

import Concourse.Cli exposing (..)
import Expect
import Test exposing (..)


all : Test
all =
    describe "cli display functions"
        [ test "downloadUrl uses the correct architecture and platform name for each os" <|
            \_ ->
                List.map downloadUrl clis
                    |> Expect.equal
                        [ "/api/v1/cli?arch=amd64&platform=darwin"
                        , "/api/v1/cli?arch=amd64&platform=windows"
                        , "/api/v1/cli?arch=amd64&platform=linux"
                        ]
        , test "cli-icon returns the icon name for each os" <|
            \_ ->
                List.map iconUrl clis
                    |> Expect.equal
                        [ "url(/public/images/apple-logo.svg)"
                        , "url(/public/images/windows-logo.svg)"
                        , "url(/public/images/linux-logo.svg)"
                        ]
        , test "cli label returns the text for each os" <|
            \_ ->
                List.map label clis
                    |> Expect.equal
                        [ "Download OS X CLI"
                        , "Download Windows CLI"
                        , "Download Linux CLI"
                        ]
        ]
