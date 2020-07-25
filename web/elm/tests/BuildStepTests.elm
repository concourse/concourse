module BuildStepTests exposing (all)

import Application.Application as Application
import Array
import Assets
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
import Concourse exposing (JsonValue(..))
import Concourse.BuildStatus exposing (BuildStatus(..))
import DashboardTests exposing (iconSelector)
import Dict
import Expect
import Json.Encode
import Message.Callback as Callback
import Message.Message as Message exposing (DomID(..))
import Message.Subscription exposing (Delivery(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import Routes
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, containing, style, tag, text)
import Time
import Views.Styles


all : Test
all =
    describe "build steps"
        [ describe "get step metadata"
            [ test "has a table that left aligns text in cells" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> when iAmLookingAtTheMetadataTableCells
                    >> then_ iSeeTheyLeftAlignText
            , test "has a table that top aligns text in cells" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> when iAmLookingAtTheMetadataTableCells
                    >> then_ iSeeTheyTopAlignText
            , test "has a table that padds in cells" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> when iAmLookingAtTheMetadataTableCells
                    >> then_ iSeeTheyHavePaddingAllAround
            , test "has a table that has a bottom margin to let content (logs) underneath breathe" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> when iAmLookingAtTheMetadataTable
                    >> then_ iSeeABottomMargin
            , test "has a table that has cells with bottom borders" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> when iAmLookingAtTheMetadataTableCells
                    >> then_ iSeeLightGrayBottomBorder
            , test "has a table with cells that don't have a shared border" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> when iAmLookingAtTheMetadataTable
                    >> then_ iSeeATableWithBorderCollapse
            , test "has a table that colors key cells light gray" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> when iAmLookingAtTheMetadataTableKeyCell
                    >> then_ iSeeLightGrayCellBackground
            , test "has a table that colors value cells dark gray" <|
                given iVisitABuildWithAGetStep
                    >> given theGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> when iAmLookingAtTheMetadataTableValueCell
                    >> then_ iSeeDarkGrayCellBackground
            ]
        , describe "retry step"
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
                    , test "have default font weight" <|
                        given iVisitABuildWithARetryStep
                            >> when iAmLookingAtTheFirstTab
                            >> then_ iSeeDefaultFontWeight
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
                        , hoverable = StepTab "retryStepId" 1
                        , hoveredSelector =
                            { description = "lighter grey background"
                            , selector =
                                [ style "background-color" Colors.paginationHover ]
                            }
                        }
                    , test "have click handlers" <|
                        given iVisitABuildWithARetryStep
                            >> when iAmLookingAtTheFirstTab
                            >> then_ (itIsClickable <| StepTab "retryStepId" 0)
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
        , describe "across step"
            [ test "shows var names" <|
                given iVisitABuildWithAnAcrossStep
                    >> when iAmLookingAtTheAcrossStepInTheBuildOutput
                    >> then_ iSeeTheVarNames
            , describe "sub headers"
                [ test "has as many sub headers as sub steps" <|
                    given iVisitABuildWithAnAcrossStep
                        >> given theAcrossStepIsExpanded
                        >> when iAmLookingAtTheAcrossSubHeaders
                        >> then_ iSeeFourOfThem
                , test "have key-value pairs for the vars" <|
                    given iVisitABuildWithAnAcrossStep
                        >> given theAcrossStepIsExpanded
                        >> when iAmLookingAtTheAcrossSubHeaders
                        >> then_ iSeeTheKeyValuePairs
                , test "display subtree when expanded" <|
                    given iVisitABuildWithAnAcrossStep
                        >> given theAcrossStepIsExpanded
                        >> given theFirstAcrossSubHeaderIsExpanded
                        >> when iAmLookingAtTheAcrossStepInTheBuildOutput
                        >> then_ iSeeATaskHeader
                , describe "shows status of subtree"
                    [ test "pending" <|
                        given iVisitABuildWithAnAcrossStep
                            >> given theAcrossStepIsExpanded
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.PendingIcon)
                    , test "running" <|
                        given iVisitABuildWithAnAcrossStep
                            >> given theAcrossStepIsExpanded
                            >> given theFirstTaskInitialized
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ iSeeASpinner
                    , test "succeeded" <|
                        given iVisitABuildWithAnAcrossStep
                            >> given theAcrossStepIsExpanded
                            >> given theFirstTaskSucceeded
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.SuccessCheckIcon)
                    , test "failed" <|
                        given iVisitABuildWithAnAcrossStep
                            >> given theAcrossStepIsExpanded
                            >> given theFirstTaskFailed
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.FailureTimesIcon)
                    , test "errored" <|
                        given iVisitABuildWithAnAcrossStep
                            >> given theAcrossStepIsExpanded
                            >> given theFirstTaskErrored
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.ExclamationTriangleIcon)
                    , test "interrupted" <|
                        given iVisitABuildWithAnAcrossStep
                            >> given theAcrossStepIsExpanded
                            >> given theFirstTaskIsInterrupted
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.InterruptedIcon)
                    , test "cancelled" <|
                        given iVisitABuildWithAnAcrossStep
                            >> given theAcrossStepIsExpanded
                            >> given theFirstTaskIsCancelled
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.CancelledIcon)
                    , test "does not consider unreachable steps in retry" <|
                        given iVisitABuildWithAnAcrossStepWrappingARetryStep
                            >> given theAcrossStepIsExpanded
                            >> given theFirstAttemptSucceeded
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.SuccessCheckIcon)
                    , test "does not consider unreachable hook in on_failure" <|
                        given (iVisitABuildWithAnAcrossStepWrappingAHook Concourse.BuildStepOnFailure)
                            >> given theAcrossStepIsExpanded
                            >> given theStepSucceeded
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.SuccessCheckIcon)
                    , test "does not consider unreachable hook in on_error" <|
                        given (iVisitABuildWithAnAcrossStepWrappingAHook Concourse.BuildStepOnError)
                            >> given theAcrossStepIsExpanded
                            >> given theStepSucceeded
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.SuccessCheckIcon)
                    , test "does not consider unreachable hook in on_abort" <|
                        given (iVisitABuildWithAnAcrossStepWrappingAHook Concourse.BuildStepOnAbort)
                            >> given theAcrossStepIsExpanded
                            >> given theStepSucceeded
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ (iSeeStatusIcon Assets.SuccessCheckIcon)
                    ]
                , describe "complex values"
                    [ test "displays object fields with dot notation" <|
                        given iVisitABuildWithAnAcrossStepWithComplexValues
                            >> given theAcrossStepIsExpanded
                            >> when iAmLookingAtTheFirstAcrossSubHeader
                            >> then_ iSeeTheObjectKeyValuePairs
                    , test "displays array fields with index notation" <|
                        given iVisitABuildWithAnAcrossStepWithComplexValues
                            >> given theAcrossStepIsExpanded
                            >> when iAmLookingAtTheSecondAcrossSubHeader
                            >> then_ iSeeTheArrayKeyValuePairs
                    ]
                ]
            ]
        , describe "task step"
            [ test "logs show timestamps" <|
                given iVisitABuildWithATaskStep
                    >> given (thereIsALog taskStepId)
                    >> given theTaskStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeATimestamp
            ]
        , describe "set-pipeline step"
            [ test "should show pipeline name" <|
                given iVisitABuildWithASetPipelineStep
                    >> given theSetPipelineStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeThePipelineName
            ]
        , describe "load_var step"
            [ test "should show var name" <|
                given iVisitABuildWithALoadVarStep
                    >> given theLoadVarStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLoadVarName
            ]
        ]


iVisitABuildWithARetryStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsARetryStep


iVisitABuildWithAnAcrossStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsAnAcrossStep


iVisitABuildWithAnAcrossStepWrappingARetryStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsAnAcrossStepWithSubplan
            { id = "retryStepId"
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


iVisitABuildWithAnAcrossStepWrappingAHook hook =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsAnAcrossStepWithSubplan
            { id = "retryStepId"
            , step =
                hook
                    { step =
                        { id = "stepId"
                        , step =
                            Concourse.BuildStepTask
                                "taskName"
                        }
                    , hook =
                        { id = "hookId"
                        , step =
                            Concourse.BuildStepTask
                                "taskName"
                        }
                    }
            }


iVisitABuildWithAnAcrossStepWithComplexValues =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsAnAcrossStepWithComplexValues


iVisitABuildWithATaskStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsATaskStep


iVisitABuildWithAGetStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsAGetStep
        >> theGetStepReturnsMetadata


iVisitABuildWithASetPipelineStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsASetPipelineStep


iVisitABuildWithALoadVarStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsALoadVarStep


theGetStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader "getStepId")


theTaskStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader taskStepId)


theSetPipelineStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader setPipelineStepId)


theLoadVarStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader setLoadVarStepId)


theAcrossStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader acrossStepId)


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


thePlanContainsAnAcrossStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = acrossStepId
                      , step =
                            Concourse.BuildStepAcross
                                { vars = [ "var1", "var2" ]
                                , steps =
                                    [ ( [ JsonString "a1", JsonString "b1" ]
                                      , { id = "task1Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                      )
                                    , ( [ JsonString "a1", JsonString "b2" ]
                                      , { id = "task2Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                      )
                                    , ( [ JsonString "a2", JsonString "b1" ]
                                      , { id = "task3Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                      )
                                    , ( [ JsonString "a2", JsonString "b2" ]
                                      , { id = "task4Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                      )
                                    ]
                                }
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


thePlanContainsAnAcrossStepWithSubplan plan =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = acrossStepId
                      , step =
                            Concourse.BuildStepAcross
                                { vars = [ "var1" ]
                                , steps = [ ( [ JsonString "a1" ], plan ) ]
                                }
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


thePlanContainsAnAcrossStepWithComplexValues =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = acrossStepId
                      , step =
                            Concourse.BuildStepAcross
                                { vars = [ "var1" ]
                                , steps =
                                    [ ( [ JsonObject
                                            [ ( "f1", JsonString "v1" )
                                            , ( "f2", JsonNumber 1 )
                                            , ( "f3"
                                              , JsonRaw
                                                    (Json.Encode.object
                                                        [ ( "abc", Json.Encode.int 123 ) ]
                                                    )
                                              )
                                            ]
                                        ]
                                      , { id = "task1Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                      )
                                    , ( [ JsonArray
                                            [ JsonString "v1"
                                            , JsonNumber 1
                                            ]
                                        ]
                                      , { id = "task2Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                      )
                                    ]
                                }
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


thePlanContainsATaskStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = taskStepId
                      , step = Concourse.BuildStepTask "task-name"
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


taskStepId =
    "taskStepId"


thePlanContainsASetPipelineStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = setPipelineStepId
                      , step = Concourse.BuildStepSetPipeline "pipeline-name"
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


setPipelineStepId =
    "setPipelineStep"


thePlanContainsALoadVarStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = setLoadVarStepId
                      , step = Concourse.BuildStepLoadVar "var-name"
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


setLoadVarStepId =
    "loadVarStep"


acrossStepId =
    "acrossStep"


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


iAmLookingAtTheAcrossStepInTheBuildOutput =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "build-step", containing [ text "across:" ] ]


iAmLookingAtTheStepBody =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "build-step" ]


iSeeTwoChildren =
    Query.children [] >> Query.count (Expect.equal 2)


iSeeFourOfThem =
    Query.count (Expect.equal 4)


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
    Query.has [ style "background-color" "rgb(30,30,30)" ]


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


iAmLookingAtTheAcrossSubHeaders =
    iAmLookingAtTheAcrossStepInTheBuildOutput
        >> Query.findAll [ class "sub-header" ]


kvPair k v =
    [ containing [ text k ], containing [ text v ] ]


iSeeTheKeyValuePairs =
    Expect.all
        [ Query.index 0 >> Query.has (kvPair "var1" "a1" ++ kvPair "var2" "b1")
        , Query.index 1 >> Query.has (kvPair "var1" "a1" ++ kvPair "var2" "b2")
        , Query.index 2 >> Query.has (kvPair "var1" "a2" ++ kvPair "var2" "b1")
        , Query.index 3 >> Query.has (kvPair "var1" "a2" ++ kvPair "var2" "b2")
        ]


iSeeTheObjectKeyValuePairs =
    Query.has
        (kvPair "var1.f1" "v1"
            ++ kvPair "var1.f2" "1"
            ++ kvPair "var1.f3" "{\"abc\":123}"
        )


iSeeTheArrayKeyValuePairs =
    Query.has
        (kvPair "var1[0]" "v1"
            ++ kvPair "var1[1]" "1"
        )


iAmLookingAtTheFirstAcrossSubHeader =
    iAmLookingAtTheAcrossSubHeaders >> Query.first


iAmLookingAtTheSecondAcrossSubHeader =
    iAmLookingAtTheAcrossSubHeaders >> Query.index 1


theFirstAcrossSubHeaderIsExpanded =
    Tuple.first
        >> Application.update
            (Update <|
                Message.Click <|
                    StepSubHeader acrossStepId 0
            )


iSeeATaskHeader =
    Query.has [ class "header", containing [ text "task" ] ]


iSeeASpinner =
    Query.has
        [ style "animation" "container-rotate 1568ms linear infinite"
        , style "height" "14px"
        , style "width" "14px"
        ]


iSeeStatusIcon asset =
    Query.has
        (iconSelector
            { size = "28px"
            , image = asset
            }
        )


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


iSeeDefaultFontWeight =
    Query.has [ style "font-weight" Views.Styles.fontWeightDefault ]


iSeePointerCursor =
    Query.has [ style "cursor" "pointer" ]


iSeeLightGreyText =
    Query.has [ style "color" "#f5f5f5" ]


itIsClickable domID =
    Event.simulate Event.click
        >> Event.expect (Update <| Message.Click domID)


iSeeALighterGreyBackground =
    Query.has [ style "background-color" Colors.paginationHover ]


iSeeItIsSelected =
    iSeeALighterGreyBackground


iSeeTheSuccessColor =
    Query.has [ style "background-color" Colors.success ]


iSeeTheFailureColor =
    Query.has [ style "background-color" Colors.failure ]


iSeeTheStartedColor =
    Query.has [ style "background-color" Colors.started ]


iSeeTheErrorColor =
    Query.has [ style "background-color" Colors.error ]


iSeeItIsTransparent =
    Query.has [ style "opacity" "0.5" ]


iSeeATimestamp =
    Query.has [ text "00:00:01" ]


iSeeThePipelineName =
    Query.has [ text "pipeline-name" ]


iSeeTheLoadVarName =
    Query.has [ text "var-name" ]


iSeeTheVarNames =
    Query.has [ text "var1, var2" ]


iAmLookingAtTheSecondTab =
    iAmLookingAtTheTabList >> Query.children [] >> Query.index 1


theFirstAttemptInitialized =
    taskInitialized "attempt1Id"


theFirstAttemptSucceeded =
    taskFinished "attempt1Id" 0


theSecondAttemptInitialized =
    taskInitialized "attempt2Id"


theFirstTaskInitialized =
    taskInitialized "task1Id"


theSecondTaskInitialized =
    taskInitialized "task2Id"


theFirstTaskSucceeded =
    taskFinished "task1Id" 0


theFirstTaskFailed =
    taskFinished "task1Id" 1


theFirstTaskErrored =
    taskErrored "task1Id"


theStepSucceeded =
    taskFinished "stepId" 0


theFirstTaskIsInterrupted =
    thereIsALog "task1Id" >> buildAborted


theFirstTaskIsCancelled =
    buildAborted


buildAborted =
    taskEvent <|
        BuildStatus BuildStatusAborted (Time.millisToPosix 0)


taskEvent event =
    Tuple.first
        >> Application.handleDelivery
            (EventsReceived <|
                Ok
                    [ { data = event
                      , url = "http://localhost:8080/api/v1/builds/1/events"
                      }
                    ]
            )


taskInitialized stepId =
    taskEvent <|
        InitializeTask
            { source = "stdout"
            , id = stepId
            }
            (Time.millisToPosix 0)


taskFinished stepId exitCode =
    taskEvent <|
        FinishTask
            { source = "stdout"
            , id = stepId
            }
            exitCode
            (Time.millisToPosix 0)


taskErrored stepId =
    taskEvent <|
        Error
            { source = "stdout"
            , id = stepId
            }
            "errored"
            (Time.millisToPosix 0)


thereIsALog stepId =
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
                    , { data =
                            StartTask
                                { source = "stdout"
                                , id = stepId
                                }
                                (Time.millisToPosix 0)
                      , url = "http://localhost:8080/api/v1/builds/1/events"
                      }
                    , { data =
                            Log
                                { source = "stdout"
                                , id = stepId
                                }
                                "the log output"
                                (Just <| Time.millisToPosix 1000)
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
