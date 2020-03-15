module Assets exposing
    ( Asset(..)
    , ImageAsset(..)
    , backgroundImageStyle
    , toString
    )

import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Cli exposing (Cli(..))
import Html
import Html.Attributes exposing (style)
import Url.Builder


type Asset
    = ImageAsset ImageAsset


type ImageAsset
    = CliIcon Cli
    | ChevronLeft
    | ChevronRight
    | HighDensityIcon Bool
    | VisibilityToggleIcon Bool
    | BuildFavicon (Maybe BuildStatus)


toString : Asset -> String
toString asset =
    Url.Builder.absolute (toPath asset) []


backgroundImageStyle : Asset -> Html.Attribute msg
backgroundImageStyle asset =
    style "background-image" <| "url(" ++ toString asset ++ ")"


toPath : Asset -> List String
toPath asset =
    case asset of
        ImageAsset imgAsset ->
            imageAssetToPath imgAsset


imageAssetToPath : ImageAsset -> List String
imageAssetToPath asset =
    let
        basePath =
            [ "public", "images" ]
    in
    case asset of
        CliIcon cli ->
            let
                imageName =
                    case cli of
                        OSX ->
                            "apple"

                        Windows ->
                            "windows"

                        Linux ->
                            "linux"
            in
            basePath ++ [ imageName ++ "-logo.svg" ]

        ChevronLeft ->
            basePath ++ [ "baseline-chevron-left-24px.svg" ]

        ChevronRight ->
            basePath ++ [ "baseline-chevron-right-24px.svg" ]

        HighDensityIcon on ->
            let
                imageName =
                    if on then
                        "on"

                    else
                        "off"
            in
            basePath ++ [ "ic-hd-" ++ imageName ++ ".svg" ]

        VisibilityToggleIcon visible ->
            let
                imageName =
                    if visible then
                        ""

                    else
                        "off-"
            in
            basePath ++ [ "baseline-visibility-" ++ imageName ++ "24px.svg" ]

        BuildFavicon maybeStatus ->
            basePath
                ++ (case maybeStatus of
                        Just status ->
                            let
                                imageName =
                                    Concourse.BuildStatus.show status
                            in
                            [ "favicon-" ++ imageName ++ ".png" ]

                        Nothing ->
                            [ "favicon.png" ]
                   )
