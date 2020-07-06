module DashboardArchiveTests exposing (all)

import Application.Application as Application
import Assets
import Colors
import Common
import Data
import Html.Attributes as Attr
import Message.Callback as Callback
import Routes
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, class, containing, style, tag, text)
import Url


all : Test
all =
    describe "DashboardArchive"
        [ describe "toggle switch" <|
            let
                toggleSwitch =
                    [ tag "a"
                    , containing [ text "show archived" ]
                    ]

                setupQuery path query =
                    init path query
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <| Ok [ Data.pipeline "team" 1 ])
                        |> Tuple.first
                        |> Common.queryView

                setup path =
                    setupQuery path Nothing
            in
            [ test "exists on the normal view" <|
                \_ ->
                    setup "/"
                        |> Query.has toggleSwitch
            , test "exists on the hd view" <|
                \_ ->
                    setup "/hd"
                        |> Query.has toggleSwitch
            , test "does not exist when there are no pipelines" <|
                \_ ->
                    Common.init "/"
                        |> Common.queryView
                        |> Query.hasNot toggleSwitch
            , test "renders label to the left of the button" <|
                \_ ->
                    setup "/"
                        |> Query.find toggleSwitch
                        |> Query.has [ style "flex-direction" "row-reverse" ]
            , test "has a margin between the button and the label" <|
                \_ ->
                    setup "/"
                        |> Query.find toggleSwitch
                        |> Query.children []
                        |> Query.index 0
                        |> Query.has [ style "margin-left" "10px" ]
            , test "has a margin to the right of the toggle" <|
                \_ ->
                    setup "/"
                        |> Query.find toggleSwitch
                        |> Query.has [ style "margin-right" "10px" ]
            , test "has an offset left border" <|
                \_ ->
                    setup "/"
                        |> Query.find toggleSwitch
                        |> Query.has
                            [ style "border-left" <| "1px solid " ++ Colors.background
                            , style "padding-left" "10px"
                            ]
            , describe "when not enabled" <|
                [ test "links to 'view all pipelines' route" <|
                    \_ ->
                        setup "/"
                            |> Query.find toggleSwitch
                            |> Query.has
                                [ routeHref <|
                                    Routes.Dashboard
                                        { searchType = Routes.Normal ""
                                        , dashboardView = Routes.ViewAllPipelines
                                        }
                                ]
                , test "displays the off state" <|
                    \_ ->
                        setup "/"
                            |> Query.find toggleSwitch
                            |> Query.has
                                [ style "background-image" <|
                                    Assets.backgroundImage <|
                                        Just (Assets.ToggleSwitch False)
                                ]
                ]
            , describe "when enabled" <|
                [ test "links to 'view non-archived pipelines' route" <|
                    \_ ->
                        setupQuery "/" (Just "view=all")
                            |> Query.find toggleSwitch
                            |> Query.has
                                [ routeHref <|
                                    Routes.Dashboard
                                        { searchType = Routes.Normal ""
                                        , dashboardView = Routes.ViewNonArchivedPipelines
                                        }
                                ]
                , test "displays the on state" <|
                    \_ ->
                        setupQuery "/" (Just "view=all")
                            |> Query.find toggleSwitch
                            |> Query.has
                                [ style "background-image" <|
                                    Assets.backgroundImage <|
                                        Just (Assets.ToggleSwitch True)
                                ]
                ]
            , describe "when a search query is entered" <|
                [ test "does not clear the query" <|
                    \_ ->
                        setupQuery "/" (Just "search=test")
                            |> Query.find toggleSwitch
                            |> Query.has
                                [ routeHref <|
                                    Routes.Dashboard
                                        { searchType = Routes.Normal "test"
                                        , dashboardView = Routes.ViewAllPipelines
                                        }
                                ]
                ]
            , describe "on the HD view" <|
                [ test "stays in the HD view" <|
                    \_ ->
                        setup "/hd"
                            |> Query.find toggleSwitch
                            |> Query.has
                                [ routeHref <|
                                    Routes.Dashboard
                                        { searchType = Routes.HighDensity
                                        , dashboardView = Routes.ViewAllPipelines
                                        }
                                ]
                ]
            ]
        , describe "when viewing only non-archived pipelines"
            [ test "archived pipelines are not rendered" <|
                \_ ->
                    init "/" Nothing
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 1
                                        |> Data.withName "archived-pipeline"
                                        |> Data.withArchived True
                                    ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.hasNot [ class "pipeline-wrapper", containing [ text "archived-pipeline" ] ]
            ]
        , describe "when viewing all pipelines"
            [ test "archived pipelines are rendered" <|
                \_ ->
                    init "/" (Just "view=all")
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 1
                                        |> Data.withName "archived-pipeline"
                                        |> Data.withArchived True
                                    ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.has [ class "pipeline-wrapper", containing [ text "archived-pipeline" ] ]
            ]
        ]


init : String -> Maybe String -> Application.Model
init path query =
    Application.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = "notfound.svg"
        , csrfToken = "csrf_token"
        , authToken = ""
        , pipelineRunningKeyframes = "pipeline-running"
        }
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = path
        , query = query
        , fragment = Nothing
        }
        |> Tuple.first


routeHref : Routes.Route -> Test.Html.Selector.Selector
routeHref =
    Routes.toString >> Attr.href >> attribute
