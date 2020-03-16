module AssetsTests exposing (backgroundImageStyleTests, toStringTests)

import Assets
    exposing
        ( Asset(..)
        , ComponentType(..)
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
        [ describe "CliIcon"
            [ test "OSX" <|
                \_ ->
                    CliIcon OSX
                        |> toString
                        |> Expect.equal "/public/images/apple-logo.svg"
            , test "Windows" <|
                \_ ->
                    CliIcon Windows
                        |> toString
                        |> Expect.equal "/public/images/windows-logo.svg"
            , test "Linux" <|
                \_ ->
                    CliIcon Linux
                        |> toString
                        |> Expect.equal "/public/images/linux-logo.svg"
            ]
        , test "ChevronLeft" <|
            \_ ->
                ChevronLeft
                    |> toString
                    |> Expect.equal "/public/images/baseline-chevron-left-24px.svg"
        , test "ChevronRight" <|
            \_ ->
                ChevronRight
                    |> toString
                    |> Expect.equal "/public/images/baseline-chevron-right-24px.svg"
        , describe "HighDensityIcon"
            [ test "On" <|
                \_ ->
                    HighDensityIcon True
                        |> toString
                        |> Expect.equal "/public/images/ic-hd-on.svg"
            , test "Off" <|
                \_ ->
                    HighDensityIcon False
                        |> toString
                        |> Expect.equal "/public/images/ic-hd-off.svg"
            ]
        , describe "VisibilityToggleIcon"
            [ test "Visible" <|
                \_ ->
                    VisibilityToggleIcon True
                        |> toString
                        |> Expect.equal "/public/images/baseline-visibility-24px.svg"
            , test "Not Visible" <|
                \_ ->
                    VisibilityToggleIcon False
                        |> toString
                        |> Expect.equal "/public/images/baseline-visibility-off-24px.svg"
            ]
        , describe "BuildFavicon"
            [ test "Nothing" <|
                \_ ->
                    BuildFavicon Nothing
                        |> toString
                        |> Expect.equal "/public/images/favicon.png"
            , test "Pending" <|
                \_ ->
                    BuildFavicon (Just BuildStatusPending)
                        |> toString
                        |> Expect.equal "/public/images/favicon-pending.png"
            , test "Started" <|
                \_ ->
                    BuildFavicon (Just BuildStatusStarted)
                        |> toString
                        |> Expect.equal "/public/images/favicon-started.png"
            , test "Succeeded" <|
                \_ ->
                    BuildFavicon (Just BuildStatusSucceeded)
                        |> toString
                        |> Expect.equal "/public/images/favicon-succeeded.png"
            , test "Failed" <|
                \_ ->
                    BuildFavicon (Just BuildStatusFailed)
                        |> toString
                        |> Expect.equal "/public/images/favicon-failed.png"
            , test "Errored" <|
                \_ ->
                    BuildFavicon (Just BuildStatusErrored)
                        |> toString
                        |> Expect.equal "/public/images/favicon-errored.png"
            , test "Aborted" <|
                \_ ->
                    BuildFavicon (Just BuildStatusAborted)
                        |> toString
                        |> Expect.equal "/public/images/favicon-aborted.png"
            ]
        , test "PinIconWhite" <|
            \_ ->
                PinIconWhite
                    |> toString
                    |> Expect.equal "/public/images/pin-ic-white.svg"
        , test "CheckmarkIcon" <|
            \_ ->
                CheckmarkIcon
                    |> toString
                    |> Expect.equal "/public/images/checkmark-ic.svg"
        , describe "BreadcrumbIcon"
            [ test "Pipeline" <|
                \_ ->
                    BreadcrumbIcon PipelineComponent
                        |> toString
                        |> Expect.equal "/public/images/ic-breadcrumb-pipeline.svg"
            , test "Job" <|
                \_ ->
                    BreadcrumbIcon JobComponent
                        |> toString
                        |> Expect.equal "/public/images/ic-breadcrumb-job.svg"
            , test "Resource" <|
                \_ ->
                    BreadcrumbIcon ResourceComponent
                        |> toString
                        |> Expect.equal "/public/images/ic-breadcrumb-resource.svg"
            ]
        , test "PassportOfficerIcon" <|
            \_ ->
                PassportOfficerIcon
                    |> toString
                    |> Expect.equal "/public/images/passport-officer-ic.svg"
        , test "ConcourseLogoWhite" <|
            \_ ->
                ConcourseLogoWhite
                    |> toString
                    |> Expect.equal "/public/images/concourse-logo-white.svg"
        ]


backgroundImageStyleTests : Test
backgroundImageStyleTests =
    describe "backgroundImageStyle"
        [ test "Just" <|
            \_ ->
                Html.div
                    [ CliIcon OSX
                        |> Just
                        |> backgroundImageStyle
                    ]
                    []
                    |> Query.fromHtml
                    |> Query.has
                        [ style "background-image"
                            "url(/public/images/apple-logo.svg)"
                        ]
        , test "Nothing" <|
            \_ ->
                Html.div [ Nothing |> backgroundImageStyle ] []
                    |> Query.fromHtml
                    |> Query.has [ style "background-image" "none" ]
        ]
