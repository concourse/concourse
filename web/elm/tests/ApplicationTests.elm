module ApplicationTests exposing (all)

import Application.Application as Application
import Application.Msgs as Msgs
import Concourse.PipelineStatus as PipelineStatus
import Dashboard.Msgs
import Effects
import Expect
import SubPage.Msgs
import Subscription exposing (Delivery(..))
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


all : Test
all =
    describe "top-level layout"
        [ test "bold and antialiasing on dashboard" <|
            \_ ->
                Application.init
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
                    , pathname = "/"
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Application.view
                    |> Query.fromHtml
                    |> Query.has
                        [ style
                            [ ( "-webkit-font-smoothing", "antialiased" )
                            , ( "font-weight", "700" )
                            ]
                        ]
        , test "bold and antialiasing on resource page" <|
            \_ ->
                Application.init
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
                    , pathname = "/teams/t/pipelines/p/resources/r"
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Application.view
                    |> Query.fromHtml
                    |> Query.has
                        [ style
                            [ ( "-webkit-font-smoothing", "antialiased" )
                            , ( "font-weight", "700" )
                            ]
                        ]
        , test "bold and antialiasing everywhere else" <|
            \_ ->
                Application.init
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
                    , pathname = "/teams/team/pipelines/pipeline"
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Application.view
                    |> Query.fromHtml
                    |> Query.has
                        [ style
                            [ ( "-webkit-font-smoothing", "antialiased" )
                            , ( "font-weight", "700" )
                            ]
                        ]
        , test "should subscribe to clicks from the not-automatically-linked boxes in the pipeline, and the token return" <|
            \_ ->
                Application.init
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
                    , pathname = "/teams/t/pipelines/p/"
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Application.subscriptions
                    |> Expect.all
                        [ List.member Subscription.OnNonHrefLinkClicked
                            >> Expect.true "not subscribed to the weird pipeline links?"
                        , List.member Subscription.OnTokenReceived
                            >> Expect.true "not subscribed to token callback?"
                        ]
        , test "clicking a not-automatically-linked box in the pipeline redirects" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = ""
                    , csrfToken = "token"
                    , authToken = ""
                    , pipelineRunningKeyframes = ""
                    }
                    { href = ""
                    , host = ""
                    , hostname = ""
                    , protocol = ""
                    , origin = ""
                    , port_ = ""
                    , pathname = "/teams/t/pipelines/p/"
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| NonHrefLinkClicked "/foo/bar")
                    |> Tuple.second
                    |> Expect.equal
                        [ ( Effects.Layout, "token", Effects.NavigateTo "/foo/bar" )
                        ]
        , test "received token is passed to all subsquent requests" <|
            \_ ->
                let
                    pipeline =
                        { id = 0
                        , name = "p"
                        , teamName = "t"
                        , public = True
                        , jobs = []
                        , resourceError = False
                        , status = PipelineStatus.PipelineStatusSucceeded PipelineStatus.Running
                        }
                in
                Application.init
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
                    , pathname = "/"
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Application.update (Msgs.DeliveryReceived <| TokenReceived <| Just "real-token")
                    |> Tuple.first
                    |> Application.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.TogglePipelinePaused pipeline
                        )
                    |> Tuple.second
                    |> Expect.equal [ ( Effects.SubPage 1, "real-token", Effects.SendTogglePipelineRequest pipeline ) ]
        ]
