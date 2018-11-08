module PipelineTests exposing (..)

import Char
import Expect exposing (..)
import Html.Attributes as Attr
import Json.Encode
import Layout
import Pipeline exposing (update, Msg(..))
import QueryString
import Routes
import SubPage
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector as Selector exposing (attribute, containing, id, style, tag, text)
import Time exposing (Time)
import TopBar


all : Test
all =
    describe "Pipeline"
        [ describe "update" <|
            let
                resetFocus =
                    (\_ -> Cmd.map (\_ -> Noop) Cmd.none)

                defaultModel : Pipeline.Model
                defaultModel =
                    Pipeline.init
                        { render = (\( _, _ ) -> Cmd.none), title = (\_ -> Cmd.none) }
                        { teamName = "some-team"
                        , pipelineName = "some-pipeline"
                        , turbulenceImgSrc = "some-turbulence-img-src"
                        , route = { logical = Routes.Pipeline "" "", queries = QueryString.empty, page = Nothing, hash = "" }
                        }
                        |> Tuple.first
            in
                [ test "HideLegendTimerTicked" <|
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
                        { defaultModel | hideLegend = True, hideLegendCounter = 3 * Time.second }
                            |> update (ShowLegend)
                            |> Tuple.first
                            |> Expect.all
                                [ (\m -> m.hideLegend |> Expect.equal False)
                                , (\m -> m.hideLegendCounter |> Expect.equal 0)
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
                , test "shows a pin icon on top bar" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.has [ id "pin-icon" ]
                , test "top bar is 54px tall with dark grey background" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.has [ style [ ( "background-color", "#1e1d1d" ), ( "height", "54px" ) ] ]
                , test "top bar lays out contents horizontally" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.has [ style [ ( "display", "flex" ) ] ]
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
                            |> Query.has [ style [ ( "justify-content", "space-between" ) ] ]
                , test "both navs are laid out horizontally" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.children []
                            |> Query.each
                                (Query.has [ style [ ( "display", "flex" ) ] ])
                , test "top bar has a square concourse logo on the left" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.children []
                            |> Query.first
                            |> Query.has
                                [ style
                                    [ ( "background-image", "url(/public/images/concourse_logo_white.svg)" )
                                    , ( "background-position", "50% 50%" )
                                    , ( "background-repeat", "no-repeat" )
                                    , ( "background-size", "42px 42px" )
                                    , ( "width", "54px" )
                                    , ( "height", "54px" )
                                    ]
                                ]
                , test "concourse logo on the left is a link to homepage" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.children []
                            |> Query.first
                            |> Query.find
                                [ style
                                    [ ( "background-image", "url(/public/images/concourse_logo_white.svg)" )
                                    , ( "background-position", "50% 50%" )
                                    , ( "background-repeat", "no-repeat" )
                                    , ( "background-size", "42px 42px" )
                                    , ( "width", "54px" )
                                    , ( "height", "54px" )
                                    ]
                                ]
                            |> Query.has [ attribute <| Attr.href "/" ]
                , test "pin icon has a pin background" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.has [ style [ ( "background-image", "url(/public/images/pin_ic_grey.svg)" ) ] ]
                , test "mousing over pin icon does nothing if there are no pinned resources" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.children []
                            |> Query.first
                            |> Event.simulate Event.mouseEnter
                            |> Event.toResult
                            |> Expect.err
                , test "there is some space between the pin icon and the user menu" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.has [ style [ ( "margin-right", "15px" ) ] ]
                , test "pin icon has relative positioning" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.has [ style [ ( "position", "relative" ) ] ]
                , test "pin icon does not have circular background" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.findAll
                                [ id "pin-icon"
                                , style
                                    [ ( "border-radius", "50%" )
                                    ]
                                ]
                            |> Query.count (Expect.equal 0)
                , test "pin icon has white color when pipeline has pinned resources" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.has [ style [ ( "background-image", "url(/public/images/pin_ic_white.svg)" ) ] ]
                , test "pin icon has pin badge when pipeline has pinned resources" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.has pinBadgeSelector
                , test "pin badge is purple" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find pinBadgeSelector
                            |> Query.has
                                [ style [ ( "background-color", "#5C3BD1" ) ] ]
                , test "pin badge is circular" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find pinBadgeSelector
                            |> Query.has
                                [ style
                                    [ ( "border-radius", "50%" )
                                    , ( "width", "15px" )
                                    , ( "height", "15px" )
                                    ]
                                ]
                , test "pin badge is near the top right of the pin icon" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find pinBadgeSelector
                            |> Query.has
                                [ style
                                    [ ( "position", "absolute" )
                                    , ( "top", "3px" )
                                    , ( "right", "3px" )
                                    ]
                                ]
                , test "content inside pin badge is centered horizontally and vertically" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find pinBadgeSelector
                            |> Query.has
                                [ style
                                    [ ( "display", "flex" )
                                    , ( "align-items", "center" )
                                    , ( "justify-content", "center" )
                                    ]
                                ]
                , test "pin badge shows count of pinned resources, centered" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find pinBadgeSelector
                            |> Query.findAll [ tag "div", containing [ text "1" ] ]
                            |> Query.count (Expect.equal 1)
                , test "pin badge has no other children" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find pinBadgeSelector
                            |> Query.children []
                            |> Query.count (Expect.equal 1)
                , test "pin counter works with multiple pinned resources" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenMultiplePinnedResources
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find pinBadgeSelector
                            |> Query.findAll [ tag "div", containing [ text "2" ] ]
                            |> Query.count (Expect.equal 1)
                , test "before TogglePinIconDropdown msg no list of pinned resources is visible" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.hasNot [ tag "ul" ]
                , test "mousing over pin icon sends TogglePinIconDropdown msg" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.children []
                            |> Query.first
                            |> Event.simulate Event.mouseEnter
                            |> Event.expect (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                , test "TogglePinIconDropdown msg causes pin icon to have light grey circular background" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "pin-icon" ]
                            |> Query.has
                                [ style
                                    [ ( "background-color", "#3d3c3c" )
                                    , ( "border-radius", "50%" )
                                    ]
                                ]
                , test "TogglePinIconDropdown msg causes dropdown list of pinned resources to appear" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "pin-icon" ]
                            |> Query.children [ tag "ul" ]
                            |> Query.count (Expect.equal 1)
                , test "on TogglePinIconDropdown, pin badge has no other children" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find pinBadgeSelector
                            |> Query.children []
                            |> Query.count (Expect.equal 1)
                , test "dropdown list of pinned resources contains resource name" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.has [ tag "li", containing [ text "resource" ] ]
                , test "dropdown list of pinned resources shows resource names in bold" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.find [ tag "li", containing [ text "resource" ] ]
                            |> Query.findAll [ tag "div", containing [ text "resource" ], style [ ( "font-weight", "700" ) ] ]
                            |> Query.count (Expect.equal 1)
                , test "dropdown list of pinned resources shows pinned version of each resource" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.find [ tag "li", containing [ text "resource" ] ]
                            |> Query.has [ tag "table", containing [ text "v1" ] ]
                , test "dropdown list of pinned resources has white background" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.has [ style [ ( "background-color", "#fff" ) ] ]
                , test "dropdown list of pinned resources is drawn over other elements on the page" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.has [ style [ ( "z-index", "1" ) ] ]
                , test "dropdown list of pinned resources has dark grey text" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.has [ style [ ( "color", "#1e1d1d" ) ] ]
                , test "dropdown list has upward-pointing arrow" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.children
                                [ style
                                    [ ( "border-width", "5px" )
                                    , ( "border-style", "solid" )
                                    , ( "border-color", "transparent transparent #fff transparent" )
                                    ]
                                ]
                            |> Query.count (Expect.equal 1)
                , test "dropdown list of pinned resources is offset below and left of the pin icon" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.has
                                [ style
                                    [ ( "position", "absolute" )
                                    , ( "top", "100%" )
                                    , ( "right", "0" )
                                    , ( "margin-top", "0" )
                                    ]
                                ]
                , test "dropdown list of pinned resources stretches horizontally to fit content" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.has
                                [ style
                                    [ ( "white-space", "nowrap" ) ]
                                ]
                , test "dropdown list of pinned resources has no bullet points" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.has
                                [ style
                                    [ ( "list-style-type", "none" ) ]
                                ]
                , test "dropdown list has comfortable padding" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Query.has
                                [ style
                                    [ ( "padding", "10px" ) ]
                                ]
                , test "dropdown list arrow is centered below the pin icon above the list" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.children
                                [ style
                                    [ ( "border-width", "5px" )
                                    , ( "border-style", "solid" )
                                    , ( "border-color", "transparent transparent #fff transparent" )
                                    ]
                                ]
                            |> Query.first
                            |> Query.has
                                [ style
                                    [ ( "top", "100%" )
                                    , ( "right", "50%" )
                                    , ( "margin-right", "-5px" )
                                    , ( "margin-top", "-10px" )
                                    , ( "position", "absolute" )
                                    ]
                                ]
                , test "mousing off the pin icon sends TogglePinIconDropdown msg" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.children []
                            |> Query.first
                            |> Event.simulate Event.mouseLeave
                            |> Event.expect (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                , test "clicking a pinned resource sends a Navigation Msg" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "li" ]
                            |> Event.simulate Event.click
                            |> Event.expect (Layout.TopMsg 1 (TopBar.GoToPinnedResource "resource"))
                , test "TogglePinIconDropdown msg causes dropdown list of pinned resources to disappear" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.find [ id "pin-icon" ]
                            |> Query.hasNot [ tag "ul" ]
                , test "pinned resources in the dropdown should have a pointer cursor" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline"
                            |> givenPinnedResource
                            |> Layout.update
                                (Layout.TopMsg 1 TopBar.TogglePinIconDropdown)
                            |> Tuple.first
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "pin-icon" ]
                            |> Query.find [ tag "ul" ]
                            |> Expect.all
                                [ Query.findAll [ tag "li" ]
                                    >> Query.each (Query.has [ style [ ( "cursor", "pointer" ) ] ])
                                , Query.findAll [ style [ ( "cursor", "pointer" ) ] ]
                                    >> Query.each (Query.has [ tag "li" ])
                                ]
                , test "shows no pin icon on top bar when viewing build page" <|
                    \_ ->
                        init "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                            |> Layout.view
                            |> Query.fromHtml
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.hasNot [ id "pin-icon" ]
                ]
        ]


pinBadgeSelector : List Selector.Selector
pinBadgeSelector =
    [ id "pin-badge" ]


init : String -> Layout.Model
init path =
    Layout.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = ""
        , csrfToken = ""
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
