module DashboardSearchTests exposing (all)

import Application.Application as Application
import Application.Msgs as Msgs
import Dashboard.Msgs
import Expect exposing (Expectation)
import TopBar.Msgs
import SubPage.Msgs
import Test exposing (Test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (id, text)


describe : String -> model -> List (model -> Test) -> Test
describe description beforeEach subTests =
    Test.describe description
        (subTests |> List.map (\f -> f beforeEach))


context : String -> (a -> b) -> List (b -> Test) -> (a -> Test)
context description setup subTests beforeEach =
    Test.describe description
        (subTests |> List.map (\f -> f <| setup beforeEach))


it : String -> (model -> Expectation) -> model -> Test
it desc expectationFunc model =
    Test.test desc <|
        \_ -> expectationFunc model


all : Test
all =
    describe "dashboard search"
        (Application.init
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
        )
        [ context "after focusing the search bar"
            (Application.update
                (Msgs.SubMsg 1 <|
                    SubPage.Msgs.DashboardMsg <|
                        Dashboard.Msgs.FromTopBar
                            TopBar.Msgs.FocusMsg
                )
                >> Tuple.first
            )
            [ it "dropdown appears with a 'status:' option" <|
                Application.view
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.has [ text "status:" ]
            , context "after clicking 'status:' in the dropdown"
                (Application.update
                    (Msgs.SubMsg 1 <|
                        SubPage.Msgs.DashboardMsg <|
                            Dashboard.Msgs.FromTopBar <|
                                TopBar.Msgs.FilterMsg "status:"
                    )
                    >> Tuple.first
                )
                [ it "a 'status: paused' option appears" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "search-dropdown" ]
                        >> Query.has [ text "status: paused" ]
                , context "after clicking 'status: paused'"
                    (Application.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar <|
                                    TopBar.Msgs.FilterMsg "status: paused"
                        )
                        >> Tuple.first
                    )
                    [ it "the dropdown is gone" <|
                        Application.view
                            >> Query.fromHtml
                            >> Query.find [ id "search-dropdown" ]
                            >> Query.children []
                            >> Query.count (Expect.equal 0)
                    ]
                ]
            ]
        ]
