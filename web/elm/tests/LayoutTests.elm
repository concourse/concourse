module LayoutTests exposing (all)

import Layout
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


all : Test
all =
    describe "top-level layout"
        [ test "bold and antialiasing on dashboard" <|
            \_ ->
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
                    , pathname = "/"
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Layout.view
                    |> Query.fromHtml
                    |> Query.has
                        [ style
                            [ ( "-webkit-font-smoothing", "antialiased" )
                            , ( "font-weight", "700" )
                            ]
                        ]
        , test "bold and antialiasing everywhere else" <|
            \_ ->
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
                    , pathname = "/teams/team/pipelines/pipeline"
                    , search = ""
                    , hash = ""
                    , username = ""
                    , password = ""
                    }
                    |> Tuple.first
                    |> Layout.view
                    |> Query.fromHtml
                    |> Query.has
                        [ style
                            [ ( "-webkit-font-smoothing", "antialiased" )
                            , ( "font-weight", "700" )
                            ]
                        ]
        ]
