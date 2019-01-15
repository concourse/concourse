module PipelineTests exposing (all, givenMultiplePinnedResources, givenPinnedResource, init, it, jobBreadcrumbSelector, pinBadgeSelector, pipelineBreadcrumbSelector, resourceBreadcrumbSelector, rspecStyleDescribe)

import Char
import DashboardTests exposing (iconSelector)
import Expect exposing (..)
import Html.Attributes as Attr
import Json.Encode
import Layout
import Pipeline exposing (Msg(..), update)
import QueryString
import Routes
import SubPage
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector as Selector exposing (attribute, class, containing, id, style, tag, text)
import Time exposing (Time)
import TopBar


rspecStyleDescribe : String -> model -> List (model -> Test) -> Test
rspecStyleDescribe description beforeEach subTests =
    Test.describe description
        (subTests |> List.map (\f -> f beforeEach))


it : String -> (model -> Expectation) -> model -> Test
it desc expectationFunc model =
    Test.test desc <|
        \_ -> expectationFunc model


all : Test
all =
    describe "Pipeline"
        [ describe "update" <|
            let
                resetFocus =
                    \_ -> Cmd.map (\_ -> Noop) Cmd.none

                defaultModel : Pipeline.Model
                defaultModel =
                    Pipeline.init
                        { render = \( _, _ ) -> Cmd.none, title = \_ -> Cmd.none }
                        { teamName = "some-team"
                        , pipelineName = "some-pipeline"
                        , turbulenceImgSrc = "some-turbulence-img-src"
                        , route = { logical = Routes.Pipeline "" "", queries = QueryString.empty, page = Nothing, hash = "" }
                        }
                        |> Tuple.first
            in
            [ test "CLI icons at bottom right" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ class "cli-downloads" ]
                        |> Query.children []
                        |> Expect.all
                            [ Query.index 0
                                >> Query.has
                                    [ style
                                        [ ( "background-image", "url(/public/images/apple-logo.svg)" )
                                        , ( "background-position", "50% 50%" )
                                        , ( "background-repeat", "no-repeat" )
                                        , ( "width", "12px" )
                                        , ( "height", "12px" )
                                        , ( "display", "inline-block" )
                                        ]
                                    ]
                            , Query.index 1
                                >> Query.has
                                    [ style
                                        [ ( "background-image", "url(/public/images/windows-logo.svg)" )
                                        , ( "background-position", "50% 50%" )
                                        , ( "background-repeat", "no-repeat" )
                                        , ( "width", "12px" )
                                        , ( "height", "12px" )
                                        , ( "display", "inline-block" )
                                        ]
                                    ]
                            , Query.index 2
                                >> Query.has
                                    [ style
                                        [ ( "background-image", "url(/public/images/linux-logo.svg)" )
                                        , ( "background-position", "50% 50%" )
                                        , ( "background-repeat", "no-repeat" )
                                        , ( "width", "12px" )
                                        , ( "height", "12px" )
                                        , ( "display", "inline-block" )
                                        ]
                                    ]
                            ]
            , test "HideLegendTimerTicked" <|
                \_ ->
                    defaultModel
                        |> update (HideLegendTimerTicked 0)
                        |> Tuple.first
                        |> .hideLegendCounter
                        |> Expect.equal (1 * Time.second)
            , test "HideLegendTimeTicked reaches timeout" <|
                \_ ->
                    { defaultModel | hideLegendCounter = 10 * Time.second }
                        |> update (HideLegendTimerTicked 0)
                        |> Tuple.first
                        |> .hideLegend
                        |> Expect.equal True
            , test "ShowLegend" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ class "legend" ]
                        |> Query.children []
                        |> Expect.all
                            [ Query.count (Expect.equal 20)
                            , Query.index 1 >> Query.has [ text "succeeded" ]
                            , Query.index 3 >> Query.has [ text "errored" ]
                            , Query.index 5 >> Query.has [ text "aborted" ]
                            , Query.index 7 >> Query.has [ text "paused" ]
                            , Query.index 8 >> Query.has [ style [ ( "background-color", "#5C3BD1" ) ] ]
                            , Query.index 9 >> Query.has [ text "pinned" ]
                            , Query.index 11 >> Query.has [ text "failed" ]
                            , Query.index 13 >> Query.has [ text "pending" ]
                            , Query.index 15 >> Query.has [ text "started" ]
                            , Query.index 17 >> Query.has [ text "dependency" ]
                            , Query.index 19 >> Query.has [ text "dependency (trigger)" ]
                            ]
            , test "Legend has definition for pinned resource color" <|
                \_ ->
                    { defaultModel | hideLegend = True, hideLegendCounter = 3 * Time.second }
                        |> update ShowLegend
                        |> Tuple.first
                        |> Expect.all
                            [ \m -> m.hideLegend |> Expect.equal False
                            , \m -> m.hideLegendCounter |> Expect.equal 0
                            ]
            , test "KeyPressed" <|
                \_ ->
                    defaultModel
                        |> update (KeyPressed (Char.toCode 'a'))
                        |> Expect.equal ( defaultModel, Cmd.none )
            , test "KeyPressed f" <|
                \_ ->
                    defaultModel
                        |> update (KeyPressed (Char.toCode 'f'))
                        |> Expect.notEqual ( defaultModel, Cmd.none )
            , rspecStyleDescribe "when on pipeline page"
                (init "/teams/team/pipelines/pipeline")
                [ it "shows a pin icon on top bar" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ id "pin-icon" ]
                , it "top bar is 56px tall with dark grey background" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style [ ( "background-color", "#1e1d1d" ), ( "height", "56px" ) ] ]
                , it "top bar lays out contents horizontally" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style [ ( "display", "flex" ) ] ]
                , it "top bar centers contents vertically" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style [ ( "align-items", "center" ) ] ]
                , it "top bar maximizes spacing between the left and right navs" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has [ style [ ( "justify-content", "space-between" ) ] ]
                , it "both navs are laid out horizontally" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.children []
                        >> Query.each
                            (Query.has [ style [ ( "display", "flex" ) ] ])
                , it "top bar has a square concourse logo on the left" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.children []
                        >> Query.first
                        >> Query.has
                            [ style
                                [ ( "background-image", "url(/public/images/concourse-logo-white.svg)" )
                                , ( "background-position", "50% 50%" )
                                , ( "background-repeat", "no-repeat" )
                                , ( "background-size", "42px 42px" )
                                , ( "width", "54px" )
                                , ( "height", "54px" )
                                ]
                            ]
                , it "concourse logo on the left is a link to homepage" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.children []
                        >> Query.first
                        >> Query.has [ tag "a", attribute <| Attr.href "/" ]
                , it "pin icon has a pin background" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style [ ( "background-image", "url(/public/images/pin-ic-grey.svg)" ) ] ]
                , it "mousing over pin icon does nothing if there are no pinned resources" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children []
                        >> Query.first
                        >> Event.simulate Event.mouseEnter
                        >> Event.toResult
                        >> Expect.err
                , it "there is some space between the pin icon and the user menu" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style [ ( "margin-right", "15px" ) ] ]
                , it "pin icon has relative positioning" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style [ ( "position", "relative" ) ] ]
                , it "pin icon does not have circular background" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.findAll
                            [ id "pin-icon"
                            , style
                                [ ( "border-radius", "50%" )
                                ]
                            ]
                        >> Query.count (Expect.equal 0)
                , it "pin icon has white color when pipeline has pinned resources" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has [ style [ ( "background-image", "url(/public/images/pin-ic-white.svg)" ) ] ]
                , it "pin icon has pin badge when pipeline has pinned resources" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has pinBadgeSelector
                , it "pin badge is purple" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style [ ( "background-color", "#5C3BD1" ) ] ]
                , it "pin badge is circular" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style
                                [ ( "border-radius", "50%" )
                                , ( "width", "15px" )
                                , ( "height", "15px" )
                                ]
                            ]
                , it "pin badge is near the top right of the pin icon" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style
                                [ ( "position", "absolute" )
                                , ( "top", "3px" )
                                , ( "right", "3px" )
                                ]
                            ]
                , it "content inside pin badge is centered horizontally and vertically" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "align-items", "center" )
                                , ( "justify-content", "center" )
                                ]
                            ]
                , it "pin badge shows count of pinned resources, centered" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.findAll [ tag "div", containing [ text "1" ] ]
                        >> Query.count (Expect.equal 1)
                , it "pin badge has no other children" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.children []
                        >> Query.count (Expect.equal 1)
                , it "pin counter works with multiple pinned resources" <|
                    givenMultiplePinnedResources
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.findAll [ tag "div", containing [ text "2" ] ]
                        >> Query.count (Expect.equal 1)
                , it "before TogglePinIconDropdown msg no list of pinned resources is visible" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.hasNot [ tag "ul" ]
                , it "mousing over pin icon sends TogglePinIconDropdown msg" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children []
                        >> Query.first
                        >> Event.simulate Event.mouseEnter
                        >> Event.expect (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                , it "TogglePinIconDropdown msg causes pin icon to have light grey circular background" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "pin-icon" ]
                        >> Query.has
                            [ style
                                [ ( "background-color", "#3d3c3c" )
                                , ( "border-radius", "50%" )
                                ]
                            ]
                , it "TogglePinIconDropdown msg causes dropdown list of pinned resources to appear" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children [ tag "ul" ]
                        >> Query.count (Expect.equal 1)
                , it "on TogglePinIconDropdown, pin badge has no other children" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find pinBadgeSelector
                        >> Query.children []
                        >> Query.count (Expect.equal 1)
                , it "dropdown list of pinned resources contains resource name" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ tag "li", containing [ text "resource" ] ]
                , it "dropdown list of pinned resources shows resource names in bold" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.find [ tag "li", containing [ text "resource" ] ]
                        >> Query.findAll [ tag "div", containing [ text "resource" ], style [ ( "font-weight", "700" ) ] ]
                        >> Query.count (Expect.equal 1)
                , it "dropdown list of pinned resources shows pinned version of each resource" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.find [ tag "li", containing [ text "resource" ] ]
                        >> Query.has [ tag "table", containing [ text "v1" ] ]
                , it "dropdown list of pinned resources has white background" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style [ ( "background-color", "#fff" ) ] ]
                , it "dropdown list of pinned resources is drawn over other elements on the page" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style [ ( "z-index", "1" ) ] ]
                , it "dropdown list of pinned resources has dark grey text" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has [ style [ ( "color", "#1e1d1d" ) ] ]
                , it "dropdown list has upward-pointing arrow" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children
                            [ style
                                [ ( "border-width", "5px" )
                                , ( "border-style", "solid" )
                                , ( "border-color", "transparent transparent #fff transparent" )
                                ]
                            ]
                        >> Query.count (Expect.equal 1)
                , it "dropdown list of pinned resources is offset below and left of the pin icon" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has
                            [ style
                                [ ( "position", "absolute" )
                                , ( "top", "100%" )
                                , ( "right", "0" )
                                , ( "margin-top", "0" )
                                ]
                            ]
                , it "dropdown list of pinned resources stretches horizontally to fit content" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has
                            [ style
                                [ ( "white-space", "nowrap" ) ]
                            ]
                , it "dropdown list of pinned resources has no bullet points" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has
                            [ style
                                [ ( "list-style-type", "none" ) ]
                            ]
                , it "dropdown list has comfortable padding" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Query.has
                            [ style
                                [ ( "padding", "10px" ) ]
                            ]
                , it "dropdown list arrow is centered below the pin icon above the list" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children
                            [ style
                                [ ( "border-width", "5px" )
                                , ( "border-style", "solid" )
                                , ( "border-color", "transparent transparent #fff transparent" )
                                ]
                            ]
                        >> Query.first
                        >> Query.has
                            [ style
                                [ ( "top", "100%" )
                                , ( "right", "50%" )
                                , ( "margin-right", "-5px" )
                                , ( "margin-top", "-10px" )
                                , ( "position", "absolute" )
                                ]
                            ]
                , it "mousing off the pin icon sends TogglePinIconDropdown msg" <|
                    givenPinnedResource
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.children []
                        >> Query.first
                        >> Event.simulate Event.mouseLeave
                        >> Event.expect (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                , it "clicking a pinned resource sends a Navigation Msg" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "li" ]
                        >> Event.simulate Event.click
                        >> Event.expect (Layout.TopMsg 1 (TopBar.GoToPinnedResource "resource"))
                , it "TogglePinIconDropdown msg causes dropdown list of pinned resources to disappear" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.find [ id "pin-icon" ]
                        >> Query.hasNot [ tag "ul" ]
                , it "pinned resources in the dropdown should have a pointer cursor" <|
                    givenPinnedResource
                        >> Layout.update
                            (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                        >> Tuple.first
                        >> Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "pin-icon" ]
                        >> Query.find [ tag "ul" ]
                        >> Expect.all
                            [ Query.findAll [ tag "li" ]
                                >> Query.each (Query.has [ style [ ( "cursor", "pointer" ) ] ])
                            , Query.findAll [ style [ ( "cursor", "pointer" ) ] ]
                                >> Query.each (Query.has [ tag "li" ])
                            ]
                ]
            , test "top bar lays out contents horizontally" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style [ ( "display", "inline-block" ) ] ]
            , test "top bar centers contents vertically" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style [ ( "align-items", "center" ) ] ]
            , test "top bar maximizes spacing between the left and right navs" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style [ ( "justify-content", "space-between" ), ( "left", "0" ), ( "right", "0" ) ] ]
            , test "top bar is sticky" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has [ style [ ( "z-index", "100" ), ( "position", "fixed" ) ] ]
            , test "breadcrumb items are laid out horizontally" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.find [ tag "ul" ]
                        |> Query.children []
                        |> Query.each
                            (Query.has [ style [ ( "display", "inline-block" ) ] ])
            , rspecStyleDescribe "when on job page"
                (init "/teams/team/pipeline/pipeline/jobs/job/builds/1")
                [ it "shows no pin icon on top bar when viewing build page" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.hasNot [ id "pin-icon" ]
                ]
            , test "top nav bar is blue when pipeline is paused" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.update
                            (Layout.TopMsg 1
                                (TopBar.PipelineFetched
                                    (Ok
                                        { id = 0
                                        , name = "pipeline"
                                        , paused = True
                                        , public = True
                                        , teamName = "team"
                                        , groups = []
                                        }
                                    )
                                )
                            )
                        |> Tuple.first
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style
                                [ ( "background-color", "#3498db" ) ]
                            ]
            , test "breadcrumb list is laid out horizontally" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "ul" ]
                        |> Query.first
                        |> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , test "pipeline breadcrumb is laid out horizontally" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
                        |> Query.has [ style [ ( "display", "inline-block" ) ] ]
            , test "top bar has pipeline breadcrumb with icon rendered first" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has pipelineBreadcrumbSelector
            , test "top bar has pipeline name after pipeline icon" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.find [ attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has
                            [ text "pipeline" ]
            , test "pipeline breadcrumb should have a link to the pipeline page" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.find [ tag "ul" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , test "pipeline breadcrumb should have a link to the pipeline page when viewing build details" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/jobs/build/builds/1"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , test "pipeline breadcrumb should have a link to the pipeline page when viewing resource details" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/resources/resource"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , test "there should be a / between pipeline and job in breadcrumb" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/jobs/build/builds/1"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has [ text "/" ]
            , test "job breadcrumb is laid out horizontally with appropriate spacing" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/jobs/build/builds/1"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.find [ tag "ul" ]
                        |> Query.children []
                        |> Query.index 2
                        |> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , test "top bar has job breadcrumb with job icon rendered first" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.find [ tag "ul" ]
                        |> Query.children []
                        |> Query.index 2
                        |> Query.has jobBreadcrumbSelector
            , test "top bar has build name after job icon" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.find [ tag "ul" ]
                        |> Query.children []
                        |> Query.index 2
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has
                            [ text "job" ]
            , test "there should be a / between pipeline and resource in breadcrumb" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/resources/resource"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has [ text "/" ]
            , test "resource breadcrumb is laid out horizontally with appropriate spacing" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/resources/resource"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.find [ tag "ul" ]
                        |> Query.children []
                        |> Query.index 2
                        |> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , test "top bar has resource breadcrumb with resource icon rendered first" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/resources/resource"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.find [ tag "ul" ]
                        |> Query.children []
                        |> Query.index 2
                        |> Query.has resourceBreadcrumbSelector
            , test "top bar has resource name after resource icon" <|
                \_ ->
                    init "/teams/team/pipelines/pipeline/resources/resource"
                        |> Layout.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.findAll [ tag "nav" ]
                        |> Query.first
                        |> Query.find [ tag "ul" ]
                        |> Query.children []
                        |> Query.index 2
                        |> Query.children []
                        |> Query.index 1
                        |> Query.has
                            [ text "resource" ]
            ]
        ]


pinBadgeSelector : List Selector.Selector
pinBadgeSelector =
    [ id "pin-badge" ]


pipelineBreadcrumbSelector : List Selector.Selector
pipelineBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic-breadcrumb-pipeline.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


jobBreadcrumbSelector : List Selector.Selector
jobBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic-breadcrumb-job.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


resourceBreadcrumbSelector : List Selector.Selector
resourceBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic-breadcrumb-resource.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


init : String -> Layout.Model
init path =
    Layout.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = ""
        , csrfToken = ""
        , authToken = ""
        , pipelineRunningKeyframes = ""
        }
        { href = ""
        , host = ""
        , hostname = ""
        , protocol = ""
        , origin = ""
        , port_ = ""
        , pathname = path
        , search = ""
        , hash = ""
        , username = ""
        , password = ""
        }
        |> Tuple.first


givenPinnedResource : Layout.Model -> Layout.Model
givenPinnedResource =
    Layout.update
        (Layout.SubMsg -1 <|
            SubPage.PipelineMsg <|
                Pipeline.ResourcesFetched <|
                    Ok <|
                        Json.Encode.list
                            [ Json.Encode.object
                                [ ( "team_name", Json.Encode.string "team" )
                                , ( "pipeline_name", Json.Encode.string "pipeline" )
                                , ( "name", Json.Encode.string "resource" )
                                , ( "pinned_version", Json.Encode.object [ ( "version", Json.Encode.string "v1" ) ] )
                                ]
                            ]
        )
        >> Tuple.first


givenMultiplePinnedResources : Layout.Model -> Layout.Model
givenMultiplePinnedResources =
    Layout.update
        (Layout.SubMsg -1 <|
            SubPage.PipelineMsg <|
                Pipeline.ResourcesFetched <|
                    Ok <|
                        Json.Encode.list
                            [ Json.Encode.object
                                [ ( "team_name", Json.Encode.string "team" )
                                , ( "pipeline_name", Json.Encode.string "pipeline" )
                                , ( "name", Json.Encode.string "resource" )
                                , ( "pinned_version", Json.Encode.object [ ( "version", Json.Encode.string "v1" ) ] )
                                ]
                            , Json.Encode.object
                                [ ( "team_name", Json.Encode.string "team" )
                                , ( "pipeline_name", Json.Encode.string "pipeline" )
                                , ( "name", Json.Encode.string "other-resource" )
                                , ( "pinned_version", Json.Encode.object [ ( "version", Json.Encode.string "v2" ) ] )
                                ]
                            ]
        )
        >> Tuple.first
