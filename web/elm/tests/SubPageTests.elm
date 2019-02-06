module SubPageTests exposing (all)

import Callback exposing (Callback(..))
import Dict exposing (Dict)
import Effects
import Expect
import Http
import Layout
import NewTopBar.Model
import RemoteData
import Routes
import ScreenSize
import SubPage exposing (..)
import Test exposing (..)


notFoundResult : Result Http.Error a
notFoundResult =
    Err <|
        Http.BadStatus
            { url = ""
            , status = { code = 404, message = "" }
            , headers = Dict.empty
            , body = ""
            }


all : Test
all =
    describe "SubPage"
        [ describe "not found" <|
            let
                init : String -> () -> Layout.Model
                init path _ =
                    Layout.init
                        { turbulenceImgSrc = ""
                        , notFoundImgSrc = "notfound.svg"
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
            in
            [ test "JobNotFound" <|
                init "/teams/t/pipelines/p/jobs/j"
                    >> Layout.handleCallback
                        (Effects.SubPage 1)
                        (JobFetched notFoundResult)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel { notFoundImgSrc = "notfound.svg", topBar = topBar (Routes.Job "t" "p" "j" Nothing) })
            , test "Resource not found" <|
                init "/teams/t/pipelines/p/resources/r"
                    >> Layout.handleCallback
                        (Effects.SubPage 1)
                        (ResourceFetched notFoundResult)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel { notFoundImgSrc = "notfound.svg", topBar = topBar (Routes.Resource "t" "p" "r" Nothing) })
            , test "Build not found" <|
                init "/builds/1"
                    >> Layout.handleCallback
                        (Effects.SubPage 0)
                        (BuildFetched notFoundResult)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel { notFoundImgSrc = "notfound.svg", topBar = topBar (Routes.OneOffBuild "1" Routes.HighlightNothing) })
            , test "Pipeline not found" <|
                init "/teams/t/pipelines/p"
                    >> Layout.handleCallback
                        (Effects.SubPage 1)
                        (PipelineFetched notFoundResult)
                    >> Tuple.first
                    >> .subModel
                    >> Expect.equal
                        (NotFoundModel { notFoundImgSrc = "notfound.svg", topBar = topBar (Routes.Pipeline "t" "p" []) })
            ]
        ]


topBar : Routes.Route -> NewTopBar.Model.Model
topBar route =
    { isUserMenuExpanded = False
    , middleSection = NewTopBar.Model.Breadcrumbs route
    , teams = RemoteData.Loading
    , screenSize = ScreenSize.Desktop
    , highDensity = False
    }
