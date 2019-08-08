module BuildStepTests exposing (all)

import Application.Application as Application
import Array
import Build.StepTree.Models exposing (BuildEvent(..))
import Colors
import Common
    exposing
        ( defineHoverBehaviour
        , given
        , iOpenTheBuildPage
        , myBrowserFetchedTheBuild
        , then_
        , when
        )
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Expect
import Message.Callback as Callback
import Message.Message as Message exposing (DomID(..))
import Message.Subscription exposing (Delivery(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, style, tag)
import Time


all : Test
all =
    describe "retry step"
        [ test "has tab list above" <|
            given iVisitABuildWithARetryStep
                >> when iAmLookingAtTheRetryStepInTheBuildOutput
                >> then_ iSeeTwoChildren
        , describe "tab list"
            [ test "is a list" <|
                given iVisitABuildWithARetryStep
                    >> when iAmLookingAtTheTabList
                    >> then_ iSeeItIsAList
            , test "does not have the default vertical margins" <|
                given iVisitABuildWithARetryStep
                    >> when iAmLookingAtTheTabList
                    >> then_ iSeeNoMargin
            , test "has large font" <|
                given iVisitABuildWithARetryStep
                    >> when iAmLookingAtTheTabList
                    >> then_ iSeeLargeFont
            , test "has tall lines" <|
                given iVisitABuildWithARetryStep
                    >> when iAmLookingAtTheTabList
                    >> then_ iSeeTallLines
            , test "has grey background" <|
                given iVisitABuildWithARetryStep
                    >> when iAmLookingAtTheTabList
                    >> then_ iSeeAGreyBackground
            , test "has as many tabs as retries" <|
                given iVisitABuildWithARetryStep
                    >> when iAmLookingAtTheTabList
                    >> then_ iSeeTwoChildren
            , describe "tabs"
                [ test "lay out horizontally" <|
                    given iVisitABuildWithARetryStep
                        >> when iAmLookingAtTheTabList
                        >> then_ iSeeItLaysOutHorizontally
                , test "have bold font" <|
                    given iVisitABuildWithARetryStep
                        >> when iAmLookingAtTheFirstTab
                        >> then_ iSeeBoldFont
                , test "have pointer cursor" <|
                    given iVisitABuildWithARetryStep
                        >> when iAmLookingAtTheFirstTab
                        >> then_ iSeePointerCursor
                , test "have light grey text" <|
                    given iVisitABuildWithARetryStep
                        >> when iAmLookingAtTheFirstTab
                        >> then_ iSeeLightGreyText
                , defineHoverBehaviour
                    { name = "build tab"
                    , setup = iVisitABuildWithARetryStep () |> Tuple.first
                    , query = (\m -> ( m, [] )) >> iAmLookingAtTheSecondTab
                    , unhoveredSelector =
                        { description = "grey background"
                        , selector =
                            [ style "background-color" Colors.background ]
                        }
                    , hoverable = StepTab "retryStepId" 2
                    , hoveredSelector =
                        { description = "lighter grey background"
                        , selector =
                            [ style "background-color" Colors.paginationHover ]
                        }
                    }
                , test "have click handlers" <|
                    given iVisitABuildWithARetryStep
                        >> when iAmLookingAtTheFirstTab
                        >> then_ (itIsClickable <| StepTab "retryStepId" 1)
                , test "have horizontal spacing" <|
                    given iVisitABuildWithARetryStep
                        >> when iAmLookingAtTheFirstTab
                        >> then_ itHasHorizontalSpacing
                , describe "pending selected attempt"
                    [ test "has lighter grey background" <|
                        given iVisitABuildWithARetryStep
                            >> when iAmLookingAtTheFirstTab
                            >> then_ iSeeALighterGreyBackground
                    , test "is transparent" <|
                        given iVisitABuildWithARetryStep
                            >> when iAmLookingAtTheFirstTab
                            >> then_ iSeeItIsTransparent
                    ]
                , describe "started selected attempt"
                    [ test "has lighter grey background" <|
                        given iVisitABuildWithARetryStep
                            >> given theFirstAttemptInitialized
                            >> when iAmLookingAtTheFirstTab
                            >> then_ iSeeALighterGreyBackground
                    , test "is opaque" <|
                        given iVisitABuildWithARetryStep
                            >> given theFirstAttemptInitialized
                            >> when iAmLookingAtTheFirstTab
                            >> then_ iSeeItIsOpaque
                    ]
                , describe "pending unselected attempt"
                    [ test "has grey background" <|
                        given iVisitABuildWithARetryStep
                            >> when iAmLookingAtTheSecondTab
                            >> then_ iSeeAGreyBackground
                    , test "is transparent" <|
                        given iVisitABuildWithARetryStep
                            >> when iAmLookingAtTheSecondTab
                            >> then_ iSeeItIsTransparent
                    ]
                , describe "started unselected attempt"
                    [ test "has lighter grey background" <|
                        given iVisitABuildWithARetryStep
                            >> given theSecondAttemptInitialized
                            >> when iAmLookingAtTheFirstTab
                            >> then_ iSeeAGreyBackground
                    , test "is opaque" <|
                        given iVisitABuildWithARetryStep
                            >> given theSecondAttemptInitialized
                            >> when iAmLookingAtTheSecondTab
                            >> then_ iSeeItIsOpaque
                    ]
                , describe "cancelled unselected attempt" <|
                    [ test "is transparent" <|
                        given iVisitABuildWithARetryStep
                            >> given theBuildFinished
                            >> when iAmLookingAtTheSecondTab
                            >> then_ iSeeItIsTransparent
                    ]
                ]
            ]
        ]


iVisitABuildWithARetryStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsARetryStep


thePlanContainsARetryStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = "retryStepId"
                      , step =
                            Concourse.BuildStepRetry
                                (Array.fromList
                                    [ { id = "attempt1Id"
                                      , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                      }
                                    , { id = "attempt2Id"
                                      , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                      }
                                    ]
                                )
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


iAmLookingAtTheRetryStepInTheBuildOutput =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "retry" ]


iSeeTwoChildren =
    Query.children [] >> Query.count (Expect.equal 2)


iAmLookingAtTheTabList =
    iAmLookingAtTheRetryStepInTheBuildOutput
        >> Query.children []
        >> Query.first


iSeeItIsAList =
    Query.has [ tag "ul" ]


iSeeNoMargin =
    Query.has [ style "margin" "0" ]


iSeeLargeFont =
    Query.has [ style "font-size" "16px" ]


iSeeTallLines =
    Query.has [ style "line-height" "26px" ]


iSeeAGreyBackground =
    Query.has [ style "background-color" Colors.background ]


iSeeItLaysOutHorizontally =
    Query.children []
        >> Query.each (Query.has [ style "display" "inline-block" ])


iAmLookingAtTheFirstTab =
    iAmLookingAtTheTabList
        >> Query.children []
        >> Query.first


iSeeBoldFont =
    Query.has [ style "font-weight" "700" ]


iSeePointerCursor =
    Query.has [ style "cursor" "pointer" ]


iSeeLightGreyText =
    Query.has [ style "color" "#f5f5f5" ]


itIsClickable domID =
    Event.simulate Event.click
        >> Event.expect (Update <| Message.Click domID)


iSeeALighterGreyBackground =
    Query.has [ style "background-color" Colors.paginationHover ]


iSeeItIsTransparent =
    Query.has [ style "opacity" "0.5" ]


iAmLookingAtTheSecondTab =
    iAmLookingAtTheTabList >> Query.children [] >> Query.index 1


theFirstAttemptInitialized =
    taskInitialized "attempt1Id"


theSecondAttemptInitialized =
    taskInitialized "attempt2Id"


taskInitialized stepId =
    Tuple.first
        >> Application.handleDelivery
            (EventsReceived <|
                Ok
                    [ { data =
                            InitializeTask
                                { source = "stdout"
                                , id = stepId
                                }
                                (Time.millisToPosix 0)
                      , url = "http://localhost:8080/api/v1/builds/1/events"
                      }
                    ]
            )


theBuildFinished =
    Tuple.first
        >> Application.handleDelivery
            (EventsReceived <|
                Ok
                    [ { data =
                            BuildStatus
                                BuildStatusAborted
                                (Time.millisToPosix 0)
                      , url = "http://localhost:8080/api/v1/builds/1/events"
                      }
                    ]
            )


iSeeItIsOpaque =
    Query.has [ style "opacity" "1" ]


itHasHorizontalSpacing =
    Query.has [ style "padding" "0 5px" ]
