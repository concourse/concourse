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
import Dict
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
    describe "build steps"
        [ describe "get step metadata"
            [ test "has a table that left aligns text in cells" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheGetStepInTheBuildOutput
                    >> when iAmLookingAtTheMetadataTableCells
                    >> then_ iSeeTheyLeftAlignText
            , test "has a table that top aligns text in cells" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheGetStepInTheBuildOutput
                    >> when iAmLookingAtTheMetadataTableCells
                    >> then_ iSeeTheyTopAlignText
            , test "has a table that padds in cells" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheGetStepInTheBuildOutput
                    >> when iAmLookingAtTheMetadataTableCells
                    >> then_ iSeeTheyHavePaddingAllAround
            , test "has a table that has a bottom margin to let content underneath breathe" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheGetStepInTheBuildOutput
                    >> when iAmLookingAtTheMetadataTable
                    >> then_ iSeeABottomMargin
            , test "has a table that has cells with bottom borders" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheGetStepInTheBuildOutput
                    >> when iAmLookingAtTheMetadataTableCells
                    >> then_ iSeeLightGrayBottomBorder
            , test "has a table with cells that don't have a shared border" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheGetStepInTheBuildOutput
                    >> when iAmLookingAtTheMetadataTable
                    >> then_ iSeeATableWithBorderCollapse
            , test "has a table that colors key cells light gray" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheGetStepInTheBuildOutput
                    >> when iAmLookingAtTheMetadataTableKeyCell
                    >> then_ iSeeLightGrayCellBackground
            , test "has a table that colors value cells dark gray" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheGetStepInTheBuildOutput
                    >> when iAmLookingAtTheMetadataTableValueCell
                    >> then_ iSeeDarkGrayCellBackground
            ]
        , describe
            "retry step"
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
        ]


iVisitABuildWithARetryStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsARetryStep


iVisitABuildWithAGetStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsAGetStep
        >> theGetStepReturnsMetadata


theGetStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader "getStepId")


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


thePlanContainsAGetStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = "getStepId"
                      , step =
                            Concourse.BuildStepGet
                                "the-git-resource"
                                (Just (Dict.fromList [ ( "ref", "abc123" ) ]))
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


theGetStepReturnsMetadata =
    Tuple.first
        >> Application.update
            (DeliveryReceived <|
                EventsReceived <|
                    Ok <|
                        [ { url = "http://localhost:8080/api/v1/builds/1/events"
                          , data =
                                FinishGet
                                    { source = "stdout"
                                    , id = "getStepId"
                                    }
                                    1
                                    (Dict.fromList [ ( "ref", "abc123" ) ])
                                    [ { name = "metadata-field"
                                      , value = "metadata-value"
                                      }
                                    ]
                                    Nothing
                          }
                        ]
            )


iAmLookingAtTheRetryStepInTheBuildOutput =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "retry" ]


iAmLookingAtTheGetStepInTheBuildOutput =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "build-step" ]


iSeeTwoChildren =
    Query.children [] >> Query.count (Expect.equal 2)


iAmLookingAtTheMetadataTable =
    Query.find [ class "step-body" ]
        >> Query.find [ tag "table" ]


iAmLookingAtTheMetadataTableCells =
    Query.find [ class "step-body" ]
        >> Query.find [ tag "table" ]
        >> Query.findAll [ tag "td" ]


iAmLookingAtTheMetadataTableKeyCell =
    Query.find [ class "step-body" ]
        >> Query.find [ tag "table" ]
        >> Query.findAll [ tag "td" ]
        >> Query.first


iAmLookingAtTheMetadataTableValueCell =
    Query.find [ class "step-body" ]
        >> Query.find [ tag "table" ]
        >> Query.findAll [ tag "td" ]
        >> Query.index 1


iSeeTheyLeftAlignText =
    Query.each (Query.has [ style "text-align" "left" ])


iSeeTheyTopAlignText =
    Query.each (Query.has [ style "vertical-align" "top" ])


iSeeLightGrayCellBackground =
    Query.has [ style "background-color" "rgb(45,45,45)" ]


iSeeDarkGrayCellBackground =
    Query.has [ style "background-color" "rgb(35,35,35)" ]


iSeeATableWithBorderCollapse =
    Query.has [ style "border-collapse" "collapse" ]


iSeeABottomMargin =
    Query.has [ style "margin-bottom" "5px" ]


iSeeLightGrayBottomBorder =
    Query.each (Query.has [ style "border-bottom" "5px solid rgb(45,45,45)" ])


iSeeTheyHavePaddingAllAround =
    Query.each (Query.has [ style "padding" "5px" ])


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
                                Concourse.BuildStatusAborted
                                (Time.millisToPosix 0)
                      , url = "http://localhost:8080/api/v1/builds/1/events"
                      }
                    ]
            )


iSeeItIsOpaque =
    Query.has [ style "opacity" "1" ]


itHasHorizontalSpacing =
    Query.has [ style "padding" "0 5px" ]
