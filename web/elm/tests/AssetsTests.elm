module AssetsTests exposing (styleTests, toStringTests)

import Assets
    exposing
        ( Asset(..)
        , ImageAsset(..)
        , backgroundImageStyle
        , toString
        )
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
