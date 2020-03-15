module AssetsTests exposing (styleTests, toStringTests)

import Assets
    exposing
        ( Asset(..)
        , ImageAsset(..)
        , backgroundImageStyle
        , toString
        )
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Cli exposing (Cli(..))
import Expect
import Html
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


toStringTests : Test
toStringTests =
    describe "Assets"
        [ describe "ImageAssets"
            [ describe "CliIcon"
                [ test "OSX" <|
                    \_ ->
                        CliIcon OSX
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/apple-logo.svg"
                , test "Windows" <|
                    \_ ->
                        CliIcon Windows
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/windows-logo.svg"
                , test "Linux" <|
                    \_ ->
                        CliIcon Linux
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/linux-logo.svg"
                ]
            , test "ChevronLeft" <|
                \_ ->
                    ChevronLeft
                        |> ImageAsset
                        |> toString
                        |> Expect.equal "/public/images/baseline-chevron-left-24px.svg"
            , test "ChevronRight" <|
                \_ ->
                    ChevronRight
                        |> ImageAsset
                        |> toString
                        |> Expect.equal "/public/images/baseline-chevron-right-24px.svg"
            , describe "HighDensityIcon"
                [ test "On" <|
                    \_ ->
                        HighDensityIcon True
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/ic-hd-on.svg"
                , test "Off" <|
                    \_ ->
                        HighDensityIcon False
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/ic-hd-off.svg"
                ]
            , describe "VisibilityToggleIcon"
                [ test "Visible" <|
                    \_ ->
                        VisibilityToggleIcon True
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/baseline-visibility-24px.svg"
                , test "Not Visible" <|
                    \_ ->
                        VisibilityToggleIcon False
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/baseline-visibility-off-24px.svg"
                ]
            , describe "BuildFavicon"
                [ test "Nothing" <|
                    \_ ->
                        BuildFavicon Nothing
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/favicon.png"
                , test "Pending" <|
                    \_ ->
                        BuildFavicon (Just BuildStatusPending)
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/favicon-pending.png"
                , test "Started" <|
                    \_ ->
                        BuildFavicon (Just BuildStatusStarted)
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/favicon-started.png"
                , test "Succeeded" <|
                    \_ ->
                        BuildFavicon (Just BuildStatusSucceeded)
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/favicon-succeeded.png"
                , test "Failed" <|
                    \_ ->
                        BuildFavicon (Just BuildStatusFailed)
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/favicon-failed.png"
                , test "Errored" <|
                    \_ ->
                        BuildFavicon (Just BuildStatusErrored)
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/favicon-errored.png"
                , test "Aborted" <|
                    \_ ->
                        BuildFavicon (Just BuildStatusAborted)
                            |> ImageAsset
                            |> toString
                            |> Expect.equal "/public/images/favicon-aborted.png"
                ]
            ]
        ]


styleTests : Test
styleTests =
    describe "Style Tests"
        [ test "backgroundImageStyle" <|
            \_ ->
                Html.div
                    [ CliIcon OSX
                        |> ImageAsset
                        |> backgroundImageStyle
                    ]
                    []
                    |> Query.fromHtml
                    |> Query.has
                        [ style "background-image"
                            "url(/public/images/apple-logo.svg)"
                        ]
        ]
