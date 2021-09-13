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
        , iOpenTheBuildPageHighlighting
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
import Message.Effects as Effects
import Message.Message as Message exposing (DomID(..))
import Message.Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
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
                , test "appears behind top-level headers" <|
                    given iVisitABuildWithAnAcrossStep
                        >> given theAcrossStepIsExpanded
                        >> when iAmLookingAtTheFirstAcrossSubHeader
                        >> then_ iSeeItAppearsBehindTopLevelHeaders
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
                , test "auto-expands when a substep is highlighted" <|
                    given (iOpenTheBuildPageHighlighting "task1Id")
                        >> given myBrowserFetchedTheBuild
                        >> given thePlanContainsAnAcrossStep
                        >> given (thereIsALog "task1Id")
                        >> when iAmLookingAtTheStepBody
                        >> then_ iSeeATimestamp
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
                    ]
                , describe "substep plans sent in build event" <|
                    [ test "renders a header before substeps are sent" <|
                        given iVisitABuildWithAnAcrossStepWithoutSubsteps
                            >> when iAmLookingAtTheAcrossStepInTheBuildOutput
                            >> then_ iSeeTheVarNames
                    , test "receiving the substeps renders them" <|
                        given iVisitABuildWithAnAcrossStepWithoutSubsteps
                            >> given theAcrossStepIsExpanded
                            >> given iReceiveAcrossSubsteps
                            >> when iAmLookingAtTheAcrossSubHeaders
                            >> then_ iSeeFourOfThem
                    , test "receiving the substeps doesn't clear existing logs" <|
                        given iVisitABuildWithAnAcrossStepWithoutSubsteps
                            >> given theAcrossStepIsExpanded
                            >> given (thereIsALog acrossStepId)
                            >> given iReceiveAcrossSubsteps
                            >> when iAmLookingAtTheStepBody
                            >> then_ iSeeATimestamp
                    , test "auto-expands when a received substep is highlighted" <|
                        given (iOpenTheBuildPageHighlighting "task1Id")
                            >> given myBrowserFetchedTheBuild
                            >> given thePlanContainsAnAcrossStepWithoutSubsteps
                            >> given iReceiveAcrossSubsteps
                            >> given (thereIsALog "task1Id")
                            >> when iAmLookingAtTheStepBody
                            >> then_ iSeeATimestamp
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
            , test "shows image check sub-step" <|
                given iVisitABuildWithATaskStep
                    >> given (thereIsAnImageCheckStep taskStepId)
                    >> given (thereIsALog imageCheckStepId)
                    >> given (theStepInitializationIsExpanded taskStepId)
                    >> given theImageCheckStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLogOutput
            , test "shows image get sub-step" <|
                given iVisitABuildWithATaskStep
                    >> given (thereIsAnImageGetStep taskStepId)
                    >> given (thereIsALog imageGetStepId)
                    >> given (theStepInitializationIsExpanded taskStepId)
                    >> given theImageGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLogOutput
            , test "initialization toggle gets viewport for tooltip" <|
                given iVisitABuildWithATaskStep
                    >> given (thereIsAnImageGetStep taskStepId)
                    >> given iHoverOverInitializationToggle
                    >> given timeElapses
                    >> then_ (itGetsViewportOf initializationToggleID)
            , test "initialization toggle shows tooltip" <|
                given iVisitABuildWithATaskStep
                    >> given (thereIsAnImageGetStep taskStepId)
                    >> given iHoverOverInitializationToggle
                    >> given (gotViewportAndElementOf initializationToggleID)
                    >> then_ (iSeeText "image fetching")
            ]
        , describe "check step"
            [ test "should show resource name" <|
                given iVisitABuildWithACheckStep
                    >> given theCheckStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheResourceName
            , test "shows image check sub-step" <|
                given iVisitABuildWithACheckStep
                    >> given (theStepInitializationIsExpanded checkStepId)
                    >> given (thereIsALog imageCheckStepId)
                    >> given theImageCheckStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLogOutput
            , test "shows image get sub-step" <|
                given iVisitABuildWithACheckStep
                    >> given (theStepInitializationIsExpanded checkStepId)
                    >> given (thereIsALog imageGetStepId)
                    >> given theImageGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLogOutput
            ]
        , describe "get step"
            [ test "shows image check sub-step" <|
                given iVisitABuildWithAGetStep
                    >> given (theStepInitializationIsExpanded getStepId)
                    >> given (thereIsALog imageCheckStepId)
                    >> given theImageCheckStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLogOutput
            , test "shows image get sub-step" <|
                given iVisitABuildWithAGetStep
                    >> given (theStepInitializationIsExpanded getStepId)
                    >> given (thereIsALog imageGetStepId)
                    >> given theImageGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLogOutput
            ]
        , describe "put step"
            [ test "shows image check sub-step" <|
                given iVisitABuildWithAPutStep
                    >> given (theStepInitializationIsExpanded putStepId)
                    >> given (thereIsALog imageCheckStepId)
                    >> given theImageCheckStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLogOutput
            , test "shows image get sub-step" <|
                given iVisitABuildWithAPutStep
                    >> given (theStepInitializationIsExpanded putStepId)
                    >> given (thereIsALog imageGetStepId)
                    >> given theImageGetStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLogOutput
            ]
        , describe "set-pipeline step"
            [ test "should show pipeline name" <|
                given iVisitABuildWithASetPipelineStep
                    >> given theSetPipelineStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeThePipelineName
            , test "should show instance vars when they exist" <|
                given iVisitABuildWithASetPipelineStepWithInstanceVars
                    >> given theSetPipelineStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheInstanceVars
            , test "should show a separator when there are instance vars" <|
                given iVisitABuildWithASetPipelineStepWithInstanceVars
                    >> given theSetPipelineStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeASeparator
            , test "should not show a separator when there are no instance vars" <|
                given iVisitABuildWithASetPipelineStep
                    >> given theSetPipelineStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iDoNotSeeASeparator
            ]
        , describe "load_var step"
            [ test "should show var name" <|
                given iVisitABuildWithALoadVarStep
                    >> given theLoadVarStepIsExpanded
                    >> when iAmLookingAtTheStepBody
                    >> then_ iSeeTheLoadVarName
            ]
        ]


gotViewportAndElementOf domID =
    Tuple.first
        >> Application.handleCallback
            (Callback.GotViewport domID <|
                Ok
                    { scene =
                        { width = 1
                        , height = 0
                        }
                    , viewport =
                        { width = 1
                        , height = 0
                        , x = 0
                        , y = 0
                        }
                    }
            )
        >> Tuple.first
        >> Application.handleCallback
            (Callback.GotElement <|
                Ok
                    { scene =
                        { width = 0
                        , height = 0
                        }
                    , viewport =
                        { width = 0
                        , height = 0
                        , x = 0
                        , y = 0
                        }
                    , element =
                        { x = 0
                        , y = 0
                        , width = 1
                        , height = 1
                        }
                    }
            )


iHoverOverInitializationToggle =
    Tuple.first
        >> Application.update
            (Update <| Message.Hover <| Just initializationToggleID)


timeElapses =
    Tuple.first
        >> Application.handleDelivery
            (ClockTicked OneSecond <|
                Time.millisToPosix 0
            )


itGetsViewportOf domID =
    Tuple.second
        >> Common.contains (Effects.GetViewportOf domID)


initializationToggleID =
    Message.StepInitialization "foo"


iVisitABuildWithARetryStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsARetryStep


iVisitABuildWithAnAcrossStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsAnAcrossStep


iVisitABuildWithAnAcrossStepWithoutSubsteps =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsAnAcrossStepWithoutSubsteps


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


iVisitABuildWithAPutStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsAPutStep


iVisitABuildWithASetPipelineStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsASetPipelineStep


iVisitABuildWithASetPipelineStepWithInstanceVars =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsASetPipelineStepWithInstanceVars


iVisitABuildWithACheckStep =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild
        >> thePlanContainsACheckStep


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


theStepInitializationIsExpanded stepId =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepInitialization stepId)


theImageCheckStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader imageCheckStepId)


theImageGetStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader imageGetStepId)


theSetPipelineStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader setPipelineStepId)


theCheckStepIsExpanded =
    Tuple.first
        >> Application.update (Update <| Message.Click <| StepHeader checkStepId)


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
                                    [ { values = [ JsonString "a1", JsonString "b1" ]
                                      , step =
                                            { id = "task1Id"
                                            , step =
                                                Concourse.BuildStepTask
                                                    "taskName"
                                            }
                                      }
                                    , { values = [ JsonString "a1", JsonString "b2" ]
                                      , step =
                                            { id = "task2Id"
                                            , step =
                                                Concourse.BuildStepTask
                                                    "taskName"
                                            }
                                      }
                                    , { values = [ JsonString "a2", JsonString "b1" ]
                                      , step =
                                            { id = "task3Id"
                                            , step =
                                                Concourse.BuildStepTask
                                                    "taskName"
                                            }
                                      }
                                    , { values = [ JsonString "a2", JsonString "b2" ]
                                      , step =
                                            { id = "task4Id"
                                            , step =
                                                Concourse.BuildStepTask
                                                    "taskName"
                                            }
                                      }
                                    ]
                                }
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


thePlanContainsAnAcrossStepWithoutSubsteps =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = acrossStepId
                      , step =
                            Concourse.BuildStepAcross
                                { vars = [ "var1", "var2" ]
                                , steps = []
                                }
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


iReceiveAcrossSubsteps =
    Tuple.first
        >> Application.handleDelivery
            (EventsReceived <|
                Ok
                    [ { data =
                            AcrossSubsteps
                                { source = ""
                                , id = acrossStepId
                                }
                                [ { values = [ JsonString "a1", JsonString "b1" ]
                                  , step =
                                        { id = "task1Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                  }
                                , { values = [ JsonString "a1", JsonString "b2" ]
                                  , step =
                                        { id = "task2Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                  }
                                , { values = [ JsonString "a2", JsonString "b1" ]
                                  , step =
                                        { id = "task3Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                  }
                                , { values = [ JsonString "a2", JsonString "b2" ]
                                  , step =
                                        { id = "task4Id"
                                        , step =
                                            Concourse.BuildStepTask
                                                "taskName"
                                        }
                                  }
                                ]
                      , url = "http://localhost:8080/api/v1/builds/1/events"
                      }
                    ]
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
                                , steps =
                                    [ { values = [ JsonString "a1" ]
                                      , step = plan
                                      }
                                    ]
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
                                    [ { values =
                                            [ JsonObject
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
                                      , step =
                                            { id = "task1Id"
                                            , step =
                                                Concourse.BuildStepTask
                                                    "taskName"
                                            }
                                      }
                                    , { values = [ JsonString "test" ]
                                      , step =
                                            { id = "task2Id"
                                            , step =
                                                Concourse.BuildStepTask
                                                    "taskName"
                                            }
                                      }
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


imageCheckStepId =
    "imageCheckStepId"


imageGetStepId =
    "imageGetStepId"


thePlanContainsASetPipelineStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = setPipelineStepId
                      , step = Concourse.BuildStepSetPipeline "pipeline-name" Dict.empty
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


thePlanContainsASetPipelineStepWithInstanceVars =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = setPipelineStepId
                      , step =
                            Concourse.BuildStepSetPipeline "pipeline-name" <|
                                Dict.fromList
                                    [ ( "foo", JsonString "bar" ), ( "a", JsonObject [ ( "b", JsonNumber 1 ) ] ) ]
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


setPipelineStepId =
    "setPipelineStep"


thePlanContainsACheckStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = checkStepId
                      , step =
                            Concourse.BuildStepCheck "resource-name"
                                (Just
                                    { check =
                                        { id = imageCheckStepId
                                        , step = Concourse.BuildStepCheck "some-check" Nothing
                                        }
                                    , get =
                                        { id = imageGetStepId
                                        , step = Concourse.BuildStepGet "some-get" Nothing Nothing Nothing
                                        }
                                    }
                                )
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


checkStepId =
    "checkStep"


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


getStepId =
    "getStepId"


thePlanContainsAGetStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = getStepId
                      , step =
                            Concourse.BuildStepGet
                                "the-git-resource"
                                (Just "the-git-resource")
                                (Just (Dict.fromList [ ( "ref", "abc123" ) ]))
                                (Just
                                    { check =
                                        { id = imageCheckStepId
                                        , step = Concourse.BuildStepCheck "some-check" Nothing
                                        }
                                    , get =
                                        { id = imageGetStepId
                                        , step = Concourse.BuildStepGet "some-get" Nothing Nothing Nothing
                                        }
                                    }
                                )
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


putStepId =
    "putStepId"


thePlanContainsAPutStep =
    Tuple.first
        >> Application.handleCallback
            (Callback.PlanAndResourcesFetched 1 <|
                Ok
                    ( { id = putStepId
                      , step =
                            Concourse.BuildStepPut
                                "the-git-resource"
                                (Just "the-git-resource")
                                (Just
                                    { check =
                                        { id = imageCheckStepId
                                        , step = Concourse.BuildStepCheck "some-check" Nothing
                                        }
                                    , get =
                                        { id = imageGetStepId
                                        , step = Concourse.BuildStepGet "some-get" Nothing Nothing Nothing
                                        }
                                    }
                                )
                      }
                    , { inputs = []
                      , outputs = []
                      }
                    )
            )


iSeeText str =
    Tuple.first
        >> Common.queryView
        >> Query.findAll [ text str ]
        >> Query.count (Expect.equal 1)


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
        >> Query.findAll [ class "build-step" ]
        >> Query.first


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
    Query.has [ style "background-color" "#4D4D4D" ]


iSeeDarkGrayCellBackground =
    Query.has [ style "background-color" "#262626" ]


iSeeATableWithBorderCollapse =
    Query.has [ style "border-collapse" "collapse" ]


iSeeABottomMargin =
    Query.has [ style "margin-bottom" "5px" ]


iSeeLightGrayBottomBorder =
    Query.first
        >> Query.has [ style "border-bottom" "8px solid #4D4D4D" ]


iSeeTheyHavePaddingAllAround =
    Query.each (Query.has [ style "padding" "8px" ])


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


iAmLookingAtTheFirstAcrossSubHeader =
    iAmLookingAtTheAcrossSubHeaders >> Query.first


iSeeItAppearsBehindTopLevelHeaders =
    Query.has [ style "z-index" "9" ]


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
            { size = "14px"
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


iSeeTheInstanceVars =
    Expect.all
        [ Query.has [ text "foo" ]
        , Query.has [ text "bar" ]
        , Query.has [ text "a.b" ]
        , Query.has [ text "1" ]
        ]


iSeeASeparator =
    Query.has [ text "/" ]


iDoNotSeeASeparator =
    Query.hasNot [ text "/" ]


iSeeTheResourceName =
    Query.has [ text "resource-name" ]


iSeeTheLogOutput =
    Query.has [ text "the log output" ]


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


eventsUrl =
    "http://localhost:8080/api/v1/builds/1/events"


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
                      , url = eventsUrl
                      }
                    , { data =
                            StartTask
                                { source = "stdout"
                                , id = stepId
                                }
                                (Time.millisToPosix 0)
                      , url = eventsUrl
                      }
                    , { data =
                            Log
                                { source = "stdout"
                                , id = stepId
                                }
                                "the log output"
                                (Just <| Time.millisToPosix 1000)
                      , url = eventsUrl
                      }
                    ]
            )


thereIsAnImageCheckStep stepId =
    Tuple.first
        >> Application.handleDelivery
            (EventsReceived <|
                Ok
                    [ { data =
                            ImageCheck
                                { source = ""
                                , id = stepId
                                }
                                (Concourse.BuildPlan imageCheckStepId (Concourse.BuildStepCheck "image" Nothing))
                      , url = "http://localhost:8080/api/v1/builds/1/events"
                      }
                    ]
            )


thereIsAnImageGetStep stepId =
    Tuple.first
        >> Application.handleDelivery
            (EventsReceived <|
                Ok
                    [ { data =
                            ImageGet
                                { source = ""
                                , id = stepId
                                }
                                (Concourse.BuildPlan imageGetStepId (Concourse.BuildStepGet "image" Nothing Nothing Nothing))
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
