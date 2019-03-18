module DashboardSearchTests exposing (all)

import Application.Application as Application
import Expect exposing (Expectation)
import Message.ApplicationMsgs as Msgs
import Message.Callback as Callback
import Message.DashboardMsgs
import Message.Effects as Effects
import Message.Message
import Message.SubPageMsgs
import Test exposing (Test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, id, style, text)


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
                    Message.SubPageMsgs.DashboardMsg <|
                        Message.DashboardMsgs.FromTopBar
                            Message.Message.FocusMsg
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
                        Message.SubPageMsgs.DashboardMsg <|
                            Message.DashboardMsgs.FromTopBar <|
                                Message.Message.FilterMsg "status:"
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
                            Message.SubPageMsgs.DashboardMsg <|
                                Message.DashboardMsgs.FromTopBar <|
                                    Message.Message.FilterMsg "status: paused"
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
        , it "centers 'no results' message when typing a string with no hits" <|
            Application.handleCallback
                (Effects.SubPage 1)
                (Callback.APIDataFetched
                    (Ok
                        ( 0
                        , { teams = [ { name = "team", id = 0 } ]
                          , pipelines =
                                [ { id = 0
                                  , name = "pipeline"
                                  , paused = False
                                  , public = True
                                  , teamName = "team"
                                  , groups = []
                                  }
                                ]
                          , jobs = []
                          , resources = []
                          , user = Nothing
                          , version = "0.0.0-dev"
                          }
                        )
                    )
                )
                >> Tuple.first
                >> Application.update
                    (Msgs.SubMsg 1 <|
                        Message.SubPageMsgs.DashboardMsg <|
                            Message.DashboardMsgs.FromTopBar <|
                                Message.Message.FilterMsg "asdf"
                    )
                >> Tuple.first
                >> Application.view
                >> Query.fromHtml
                >> Query.find [ class "no-results" ]
                >> Query.has
                    [ style
                        [ ( "text-align", "center" )
                        , ( "font-size", "13px" )
                        , ( "margin-top", "20px" )
                        ]
                    ]
        ]
