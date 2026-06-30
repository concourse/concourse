module HideUITests exposing (all)

import Application.Application as Application
import Browser
import Common exposing (queryView)
import Data
import Dict
import Expect
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Subscription exposing (Delivery(..))
import Routes
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, id)
import Url


all : Test
all =
    describe "hide_ui query parameter"
        [ describe "Routes.parseHideUI"
            [ test "is True when hide_ui=true is present" <|
                \_ ->
                    "http://example.com/teams/t/pipelines/p?hide_ui=true"
                        |> Url.fromString
                        |> Maybe.map Routes.parseHideUI
                        |> Expect.equal (Just True)
            , test "is True alongside other query params" <|
                \_ ->
                    "http://example.com/teams/t/pipelines/p?group=a&hide_ui=true"
                        |> Url.fromString
                        |> Maybe.map Routes.parseHideUI
                        |> Expect.equal (Just True)
            , test "is False when the param is absent" <|
                \_ ->
                    "http://example.com/teams/t/pipelines/p"
                        |> Url.fromString
                        |> Maybe.map Routes.parseHideUI
                        |> Expect.equal (Just False)
            , test "is False for hide_ui=false" <|
                \_ ->
                    "http://example.com/teams/t/pipelines/p?hide_ui=false"
                        |> Url.fromString
                        |> Maybe.map Routes.parseHideUI
                        |> Expect.equal (Just False)
            ]
        , describe "Routes.withHideUIParam"
            [ test "appends with ? when there is no existing query" <|
                \_ ->
                    Routes.withHideUIParam True "/teams/t/pipelines/p"
                        |> Expect.equal "/teams/t/pipelines/p?hide_ui=true"
            , test "appends with & when a query already exists" <|
                \_ ->
                    Routes.withHideUIParam True "/teams/t/pipelines/p?group=a"
                        |> Expect.equal "/teams/t/pipelines/p?group=a&hide_ui=true"
            , test "inserts the param before a fragment" <|
                \_ ->
                    Routes.withHideUIParam True "/teams/t/pipelines/p/jobs/j/builds/b#L1:2"
                        |> Expect.equal "/teams/t/pipelines/p/jobs/j/builds/b?hide_ui=true#L1:2"
            , test "is a no-op when hideUI is False" <|
                \_ ->
                    Routes.withHideUIParam False "/teams/t/pipelines/p"
                        |> Expect.equal "/teams/t/pipelines/p"
            ]
        , describe "hiding the top bar"
            [ test "top bar is hidden on the pipeline page when hide_ui=true" <|
                \_ ->
                    hideUIAt "/teams/t/pipelines/p"
                        |> queryView
                        |> Query.findAll [ id "top-bar-app" ]
                        |> Query.count (Expect.equal 0)
            , test "top bar is hidden on the resource page when hide_ui=true" <|
                \_ ->
                    hideUIAt "/teams/t/pipelines/p/resources/r"
                        |> queryView
                        |> Query.findAll [ id "top-bar-app" ]
                        |> Query.count (Expect.equal 0)
            , test "top bar is hidden on the build page when hide_ui=true" <|
                \_ ->
                    hideUIAt "/teams/t/pipelines/p/jobs/j/builds/b"
                        |> queryView
                        |> Query.findAll [ id "top-bar-app" ]
                        |> Query.count (Expect.equal 0)
            , test "top bar is hidden on the dashboard when hide_ui=true" <|
                \_ ->
                    hideUIAt "/"
                        |> queryView
                        |> Query.findAll [ id "top-bar-app" ]
                        |> Query.count (Expect.equal 0)
            , test "top bar is shown when hide_ui is absent" <|
                \_ ->
                    Common.init "/teams/t/pipelines/p"
                        |> queryView
                        |> Query.has [ id "top-bar-app" ]
            ]
        , describe "hiding the side bar"
            [ test "side bar is hidden when hide_ui=true" <|
                \_ ->
                    hideUIAt "/teams/team/pipelines/pipeline"
                        |> withVisibleSideBar
                        |> queryView
                        |> Query.hasNot [ id "side-bar" ]
            , test "side bar is shown when hide_ui is absent (control)" <|
                \_ ->
                    Common.init "/teams/team/pipelines/pipeline"
                        |> withVisibleSideBar
                        |> queryView
                        |> Query.has [ id "side-bar" ]
            ]
        , describe "pipeline legend and lower-right info"
            [ test "legend is hidden when hide_ui=true" <|
                \_ ->
                    hideUIAt "/teams/t/pipelines/p"
                        |> queryView
                        |> Query.hasNot [ id "legend" ]
            , test "lower-right info is hidden when hide_ui=true" <|
                \_ ->
                    hideUIAt "/teams/t/pipelines/p"
                        |> queryView
                        |> Query.hasNot [ class "lower-right-info" ]
            , test "legend and lower-right info are shown when hide_ui is absent" <|
                \_ ->
                    Common.init "/teams/t/pipelines/p"
                        |> queryView
                        |> Expect.all
                            [ Query.has [ id "legend" ]
                            , Query.has [ class "lower-right-info" ]
                            ]
            ]
        , describe "persistence across navigation"
            [ test "navigating keeps hideUI active in the session" <|
                \_ ->
                    hideUIAt "/teams/t/pipelines/p"
                        |> Application.handleDelivery (RouteChanged buildRoute)
                        |> Tuple.first
                        |> .session
                        |> .hideUI
                        |> Expect.equal True
            , test "clicking an internal link keeps ?hide_ui=true in the URL" <|
                \_ ->
                    hideUIAt "/teams/t/pipelines/p"
                        |> Application.handleDelivery
                            (clickInternalLink "/teams/t/pipelines/p/jobs/j")
                        |> Tuple.second
                        |> Common.contains
                            (Effects.NavigateTo "/teams/t/pipelines/p/jobs/j?hide_ui=true")
            , test "clicking an internal link does not add the param when hideUI is inactive" <|
                \_ ->
                    Common.init "/teams/t/pipelines/p"
                        |> Application.handleDelivery
                            (clickInternalLink "/teams/t/pipelines/p/jobs/j")
                        |> Tuple.second
                        |> Common.contains
                            (Effects.NavigateTo "/teams/t/pipelines/p/jobs/j")
            , test "initial load keeps ?hide_ui=true in the URL" <|
                \_ ->
                    Application.init Data.flags
                        { protocol = Url.Http
                        , host = "test.com"
                        , port_ = Nothing
                        , path = "/teams/t/pipelines/p"
                        , query = Just "hide_ui=true"
                        , fragment = Nothing
                        }
                        |> Tuple.second
                        |> Common.contains
                            (Effects.ModifyUrl "/teams/t/pipelines/p?hide_ui=true")
            ]
        ]


clickInternalLink : String -> Delivery
clickInternalLink path =
    UrlRequest <|
        Browser.Internal
            { protocol = Url.Http
            , host = "test.com"
            , port_ = Nothing
            , path = path
            , query = Nothing
            , fragment = Nothing
            }


hideUIAt : String -> Application.Model
hideUIAt path =
    Common.initCustom { initCustomOpts | query = Just "hide_ui=true" } path


initCustomOpts =
    Common.initCustomOpts


withVisibleSideBar : Application.Model -> Application.Model
withVisibleSideBar =
    Common.whenOnDesktop
        >> Application.handleCallback
            (Callback.AllPipelinesFetched <|
                Ok [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
            )
        >> Tuple.first
        >> Application.handleDelivery
            (SideBarStateReceived (Ok { isOpen = True, width = 275 }))
        >> Tuple.first


buildRoute : Routes.Route
buildRoute =
    Routes.Build
        { id =
            { teamName = "t"
            , pipelineName = "p"
            , pipelineInstanceVars = Dict.empty
            , jobName = "j"
            , buildName = "b"
            }
        , highlight = Routes.HighlightNothing
        , groups = []
        }
