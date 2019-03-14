module ApplicationTests exposing (all)

import Application.Application as Application
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
        ]
