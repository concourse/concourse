module DashboardSearchTests exposing (all)

import Application.Application as Application
import Common exposing (queryView)
import Concourse
import Expect exposing (Expectation)
import Message.Callback as Callback
import Message.Message
import Message.TopLevelMessage as Msgs
import Test exposing (Test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, id, style, text)
import Time
import Url


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
        (Common.init "/"
            |> Application.handleCallback
                (Callback.APIDataFetched
                    (Ok
                        ( Time.millisToPosix 0
                        , { teams =
                                [ Concourse.Team 1 "team1"
                                , Concourse.Team 2 "team2"
                                ]
                          , pipelines =
                                [ { id = 0
                                  , name = "pipeline"
                                  , paused = False
                                  , public = True
                                  , teamName = "team1"
                                  , groups = []
                                  }
                                ]
                          , jobs =
                                [ { pipeline =
                                        { teamName = "team1"
                                        , pipelineName = "pipeline"
                                        }
                                  , name = "job"
                                  , pipelineName = "pipeline"
                                  , teamName = "team1"
                                  , nextBuild =
                                        Just
                                            { id = 1
                                            , name = "1"
                                            , job =
                                                Just
                                                    { teamName = "team1"
                                                    , pipelineName = "pipeline"
                                                    , jobName = "job"
                                                    }
                                            , status = Concourse.BuildStatusStarted
                                            , duration =
                                                { startedAt = Nothing
                                                , finishedAt = Nothing
                                                }
                                            , reapTime = Nothing
                                            }
                                  , finishedBuild = Nothing
                                  , transitionBuild = Nothing
                                  , paused = False
                                  , disableManualTrigger = False
                                  , inputs = []
                                  , outputs = []
                                  , groups = []
                                  }
                                ]
                          , resources = []
                          , user = Nothing
                          , version = ""
                          }
                        )
                    )
                )
            |> Tuple.first
        )
        [ context "after focusing the search bar"
            (Application.update
                (Msgs.Update <|
                    Message.Message.FocusMsg
                )
                >> Tuple.first
            )
            [ it "dropdown appears with a 'status:' option" <|
                queryView
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.has [ text "status:" ]
            , context "after clicking 'status:' in the dropdown"
                (Application.update
                    (Msgs.Update <|
                        Message.Message.FilterMsg "status:"
                    )
                    >> Tuple.first
                )
                [ it "a 'status: paused' option appears" <|
                    queryView
                        >> Query.find [ id "search-dropdown" ]
                        >> Query.has [ text "status: paused" ]
                , it "a 'status: running' option appears" <|
                    queryView
                        >> Query.find [ id "search-dropdown" ]
                        >> Query.has [ text "status: running" ]
                , context "after clicking 'status: paused'"
                    (Application.update
                        (Msgs.Update <|
                            Message.Message.FilterMsg "status: paused"
                        )
                        >> Tuple.first
                    )
                    [ it "the dropdown is gone" <|
                        queryView
                            >> Query.find [ id "search-dropdown" ]
                            >> Query.children []
                            >> Query.count (Expect.equal 0)
                    ]
                , context "after clicking 'status: running'"
                    (Application.update
                        (Msgs.Update <|
                            Message.Message.FilterMsg "status: running"
                        )
                        >> Tuple.first
                    )
                    [ it "shows the running pipeline" <|
                        queryView
                            >> Query.find [ class "card" ]
                            >> Query.has [ text "pipeline" ]
                    ]
                ]
            ]
        , it "shows empty teams when only filtering on team name" <|
            Application.update
                (Msgs.Update <|
                    Message.Message.FilterMsg "team: team2"
                )
                >> Tuple.first
                >> queryView
                >> Query.find [ class "dashboard-team-group" ]
                >> Query.has [ text "team2" ]
        , it "fuzzy matches team name" <|
            Application.update
                (Msgs.Update <|
                    Message.Message.FilterMsg "team: team"
                )
                >> Tuple.first
                >> queryView
                >> Query.findAll [ class "dashboard-team-group" ]
                >> Expect.all
                    [ Query.index 0
                        >> Query.has [ text "team1" ]
                    , Query.index 1
                        >> Query.has [ text "team2" ]
                    ]
        , it "centers 'no results' message when typing a string with no hits" <|
            Application.handleCallback
                (Callback.APIDataFetched
                    (Ok
                        ( Time.millisToPosix 0
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
                    (Msgs.Update <|
                        Message.Message.FilterMsg "asdf"
                    )
                >> Tuple.first
                >> queryView
                >> Query.find [ class "no-results" ]
                >> Query.has
                    [ style "text-align" "center"
                    , style "font-size" "13px"
                    , style "margin-top" "20px"
                    ]
        ]
