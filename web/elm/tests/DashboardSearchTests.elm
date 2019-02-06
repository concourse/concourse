module DashboardSearchTests exposing (all)

import Dashboard.Msgs
import Expect exposing (Expectation)
import Layout
import Msgs
import NewTopBar.Msgs
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
        (Layout.init
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
            (Layout.update
                (Msgs.SubMsg 1 <|
                    SubPage.Msgs.DashboardMsg <|
                        Dashboard.Msgs.FromTopBar
                            NewTopBar.Msgs.FocusMsg
                )
                >> Tuple.first
            )
            [ it "dropdown appears with a 'status:' option" <|
                Layout.view
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.has [ text "status:" ]
            , context "after clicking 'status:' in the dropdown"
                (Layout.update
                    (Msgs.SubMsg 1 <|
                        SubPage.Msgs.DashboardMsg <|
                            Dashboard.Msgs.FromTopBar <|
                                NewTopBar.Msgs.FilterMsg "status:"
                    )
                    >> Tuple.first
                )
                [ it "a 'status: paused' option appears" <|
                    Layout.view
                        >> Query.fromHtml
                        >> Query.find [ id "search-dropdown" ]
                        >> Query.has [ text "status: paused" ]
                , context "after clicking 'status: paused'"
                    (Layout.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar <|
                                    NewTopBar.Msgs.FilterMsg "status: paused"
                        )
                        >> Tuple.first
                    )
                    [ it "the dropdown is gone" <|
                        Layout.view
                            >> Query.fromHtml
                            >> Query.find [ id "search-dropdown" ]
                            >> Query.children []
                            >> Query.count (Expect.equal 0)
                    ]
                ]
            ]
        ]
