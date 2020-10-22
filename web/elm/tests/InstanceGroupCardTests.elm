module InstanceGroupCardTests exposing (all)

import Application.Application as Application
import Assets
import Colors
import Common exposing (defineHoverBehaviour, isColorWithStripes)
import Concourse exposing (Job, JsonValue(..), Pipeline)
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.PipelineStatus exposing (PipelineStatus(..), StatusDetails(..))
import DashboardInstanceGroupTests
    exposing
        ( archived
        , gotPipelines
        , pipelineInstance
        )
import DashboardTests
    exposing
        ( afterSeconds
        , amber
        , apiData
        , blue
        , brown
        , circularJobs
        , darkGrey
        , fadedGreen
        , givenDataAndUser
        , givenDataUnauthenticated
        , green
        , iconSelector
        , job
        , jobWithNameTransitionedAt
        , lightGrey
        , middleGrey
        , orange
        , otherJob
        , red
        , running
        , userWithRoles
        , whenOnDashboard
        , whenOnDashboardViewingAllPipelines
        , white
        )
import Data
import Dict
import Expect exposing (Expectation)
import Html.Attributes as Attr
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Msgs exposing (DomID(..), PipelinesSection(..))
import Message.Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as ApplicationMsgs
import Routes
import Set
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, class, containing, id, style, tag, text)
import Time


all : Test
all =
    describe "instance group cards" <|
        let
            findCard =
                Query.find
                    [ class "card"
                    , containing
                        [ text "group" ]
                    ]

            findHeader =
                findCard
                    >> Query.find [ class "card-header" ]

            findBody =
                findCard
                    >> Query.find [ class "card-body" ]

            findBanner =
                findCard
                    >> Query.find [ class "banner" ]

            rows =
                Query.children []

            firstRow =
                Query.first >> Query.children []

            firstCol =
                Query.first
        in
        [ test "displays an instance group card when there's a single pipeline with instance vars" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                    |> Common.queryView
                    |> Query.has [ class "card", class "instance-group-card" ]
        , test "links to instance group view" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                    |> Common.queryView
                    |> findCard
                    |> Query.has
                        [ Common.routeHref <|
                            Routes.Dashboard
                                { searchType = Routes.Normal "" <| Just { teamName = "team", name = "group" }
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                        ]
        , test "link maintains search filter and dashboard view" <|
            \_ ->
                whenOnDashboardViewingAllPipelines { highDensity = False }
                    |> Application.update (ApplicationMsgs.Update <| Msgs.FilterMsg "g")
                    |> Tuple.first
                    |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                    |> Common.queryView
                    |> findCard
                    |> Query.has
                        [ Common.routeHref <|
                            Routes.Dashboard
                                { searchType = Routes.Normal "g" <| Just { teamName = "team", name = "group" }
                                , dashboardView = Routes.ViewAllPipelines
                                }
                        ]
        , test "fills available space" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                    |> Common.queryView
                    |> Query.find
                        [ class "card"
                        , containing [ text "group" ]
                        ]
                    |> Query.has
                        [ style "width" "100%"
                        , style "height" "100%"
                        , style "display" "flex"
                        , style "flex-direction" "column"
                        ]
        , describe "header" <|
            let
                findName =
                    Query.find [ class "dashboard-group-name" ]
            in
            [ test "has dark grey background" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                        |> Common.queryView
                        |> findHeader
                        |> Query.has [ style "background-color" darkGrey ]
            , test "has larger, spaced-out white text" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                        |> Common.queryView
                        |> findHeader
                        |> Query.has
                            [ style "font-size" "1.5em"
                            , style "letter-spacing" "0.1em"
                            , style "color" white
                            , containing [ text "group" ]
                            ]
            , test "text does not overflow or wrap" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                        |> Common.queryView
                        |> findHeader
                        |> Query.has
                            [ style "width" "245px"
                            , style "white-space" "nowrap"
                            , style "overflow" "hidden"
                            , style "text-overflow" "ellipsis"
                            ]
            , test "name is hoverable" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                        |> Common.queryView
                        |> findHeader
                        |> findName
                        |> Event.simulate Event.mouseEnter
                        |> Event.expect
                            (ApplicationMsgs.Update <|
                                Msgs.Hover <|
                                    Just <|
                                        Msgs.InstanceGroupCardName AllPipelinesSection "team" "group"
                            )
            , test "name has html id" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                        |> Common.queryView
                        |> findHeader
                        |> findName
                        |> Query.has
                            [ id <|
                                Effects.toHtmlID <|
                                    Msgs.InstanceGroupCardName AllPipelinesSection "team" "group"
                            ]
            , test "displays resource error if any pipeline has an error" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines
                            [ pipelineInstance BuildStatusSucceeded False 1
                            , pipelineInstance BuildStatusSucceeded False 2
                            ]
                        |> Application.handleCallback
                            (Callback.AllResourcesFetched <|
                                Ok
                                    [ Data.resource Nothing
                                        |> Data.withBuild (Just <| Data.build Concourse.BuildStatus.BuildStatusFailed)
                                        |> Data.withPipelineId 2
                                    ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> findHeader
                        |> Query.has [ class "dashboard-resource-error" ]
            , describe "badge"
                [ test "has a badge that displays the number of pipelines" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> gotPipelines
                                [ pipelineInstance BuildStatusSucceeded False 1
                                , pipelineInstance BuildStatusSucceeded False 2
                                ]
                            |> Common.queryView
                            |> findHeader
                            |> Query.has
                                [ style "width" "20px"
                                , style "height" "20px"
                                , style "font-size" "14px"
                                , containing [ text "2" ]
                                ]
                , test "caps out at 99" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> gotPipelines
                                (List.range 1 100 |> List.map (pipelineInstance BuildStatusSucceeded False))
                            |> Common.queryView
                            |> findHeader
                            |> Query.has
                                [ style "width" "20px"
                                , style "height" "20px"
                                , style "font-size" "11px"
                                , containing [ text "99+" ]
                                ]
                ]
            ]
        , describe "banner" <|
            [ test "is 7px tall and grey" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                        |> Common.queryView
                        |> findBanner
                        |> Query.has
                            [ style "height" "7px"
                            , style "background-color" Colors.instanceGroupBanner
                            ]
            ]
        , describe "body"
            [ test "renders on multiple rows" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines
                            [ pipelineInstance BuildStatusSucceeded False 1
                            , pipelineInstance BuildStatusSucceeded False 2
                            , pipelineInstance BuildStatusSucceeded False 3
                            , pipelineInstance BuildStatusSucceeded False 4
                            ]
                        |> Common.queryView
                        |> findBody
                        |> rows
                        |> Query.count (Expect.equal 2)
            , test "pads the last row if there's not enough boxes" <|
                \_ ->
                    let
                        thirdRow =
                            Query.index 2 >> Query.children []

                        lastCol =
                            Query.index -1
                    in
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines
                            [ pipelineInstance BuildStatusSucceeded False 1
                            , pipelineInstance BuildStatusSucceeded False 2
                            , pipelineInstance BuildStatusSucceeded False 3
                            , pipelineInstance BuildStatusSucceeded False 4
                            , pipelineInstance BuildStatusSucceeded False 5
                            ]
                        |> Common.queryView
                        |> findBody
                        |> rows
                        |> thirdRow
                        |> lastCol
                        |> Expect.all
                            [ Query.has [ style "flex-grow" "1" ]
                            , Query.hasNot [ tag "a" ]
                            ]
            , describe "pipeline box" <|
                [ test "links to pipeline page" <|
                    \_ ->
                        let
                            pipeline =
                                pipelineInstance BuildStatusSucceeded False 1

                            pipelineId =
                                pipeline
                                    |> Tuple.first
                                    |> Concourse.toPipelineId
                        in
                        whenOnDashboard { highDensity = False }
                            |> gotPipelines [ pipeline ]
                            |> Common.queryView
                            |> findBody
                            |> rows
                            |> firstRow
                            |> firstCol
                            |> Query.has
                                [ style "display" "flex"
                                , containing
                                    [ tag "a"
                                    , Common.routeHref <| Routes.Pipeline { id = pipelineId, groups = [] }
                                    , style "flex-grow" "1"
                                    ]
                                ]
                , test "displays status colour" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> gotPipelines
                                [ pipelineInstance BuildStatusSucceeded False 1
                                , pipelineInstance BuildStatusFailed False 2
                                ]
                            |> Common.queryView
                            |> findBody
                            |> rows
                            |> firstRow
                            |> firstCol
                            |> Query.has [ style "background-color" Colors.success ]
                , test "displays stripes when running" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> gotPipelines
                                [ pipelineInstance BuildStatusSucceeded True 1
                                , pipelineInstance BuildStatusFailed False 2
                                ]
                            |> Common.queryView
                            |> findBody
                            |> rows
                            |> firstRow
                            |> firstCol
                            |> isColorWithStripes
                                { thin = Colors.success
                                , thick = Colors.successFaded
                                }
                , test "displays correct status color for archived pipelines" <|
                    \_ ->
                        whenOnDashboardViewingAllPipelines { highDensity = False }
                            |> gotPipelines
                                [ archived <| pipelineInstance BuildStatusSucceeded True 1
                                , pipelineInstance BuildStatusFailed False 2
                                ]
                            |> Common.queryView
                            |> findBody
                            |> rows
                            |> firstRow
                            |> firstCol
                            |> Query.has [ style "background-color" Colors.background ]
                , defineHoverBehaviour
                    { name = "pending pipeline"
                    , setup =
                        whenOnDashboard { highDensity = False }
                            |> gotPipelines [ pipelineInstance BuildStatusPending False 1 ]
                    , query =
                        Common.queryView
                            >> findBody
                            >> rows
                            >> firstRow
                            >> firstCol
                    , unhoveredSelector =
                        { description = "light grey background"
                        , selector = [ style "background-color" Colors.pending ]
                        }
                    , hoverable =
                        Msgs.PipelinePreview AllPipelinesSection 1
                    , hoveredSelector =
                        { description = "dark grey background"
                        , selector = [ style "background-color" Colors.pendingFaded ]
                        }
                    }
                , test "has html id" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                            |> Common.queryView
                            |> findBody
                            |> rows
                            |> firstRow
                            |> firstCol
                            |> Query.has
                                [ id <|
                                    Effects.toHtmlID <|
                                        Msgs.PipelinePreview AllPipelinesSection 1
                                ]

                -- TODO: reordering
                ]
            , describe "HD view" <|
                let
                    findName =
                        Query.find [ class "dashboardhd-group-name" ]
                in
                [ test "shows the badge" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> gotPipelines
                                [ pipelineInstance BuildStatusSucceeded True 1
                                ]
                            |> Common.queryView
                            |> findCard
                            |> Query.has
                                [ style "width" "20px"
                                , style "height" "20px"
                                , style "font-size" "14px"
                                , containing [ text "1" ]
                                ]
                , test "displays resource errors" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> gotPipelines
                                [ pipelineInstance BuildStatusSucceeded False 1
                                , pipelineInstance BuildStatusSucceeded False 2
                                ]
                            |> Application.handleCallback
                                (Callback.AllResourcesFetched <|
                                    Ok
                                        [ Data.resource Nothing
                                            |> Data.withBuild (Just <| Data.build Concourse.BuildStatus.BuildStatusFailed)
                                            |> Data.withPipelineId 2
                                        ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> findCard
                            |> Query.has [ style "border-top" <| "30px solid " ++ Colors.resourceError ]
                , test "links to instance group view" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                            |> Common.queryView
                            |> findCard
                            |> Query.has
                                [ Common.routeHref <|
                                    Routes.Dashboard
                                        { searchType = Routes.Normal "" <| Just { teamName = "team", name = "group" }
                                        , dashboardView = Routes.ViewNonArchivedPipelines
                                        }
                                ]
                , test "link maintains search filter and dashboard view" <|
                    \_ ->
                        whenOnDashboardViewingAllPipelines { highDensity = True }
                            |> Application.update (ApplicationMsgs.Update <| Msgs.FilterMsg "g")
                            |> Tuple.first
                            |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                            |> Common.queryView
                            |> findCard
                            |> Query.has
                                [ Common.routeHref <|
                                    Routes.Dashboard
                                        { searchType = Routes.Normal "g" <| Just { teamName = "team", name = "group" }
                                        , dashboardView = Routes.ViewAllPipelines
                                        }
                                ]
                , test "name is hoverable" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                            |> Common.queryView
                            |> findCard
                            |> findName
                            |> Event.simulate Event.mouseEnter
                            |> Event.expect
                                (ApplicationMsgs.Update <|
                                    Msgs.Hover <|
                                        Just <|
                                            Msgs.InstanceGroupCardNameHD "team" "group"
                                )
                , test "name has html id" <|
                    \_ ->
                        whenOnDashboard { highDensity = True }
                            |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                            |> Common.queryView
                            |> findCard
                            |> Query.has
                                [ id <|
                                    Effects.toHtmlID <|
                                        Msgs.InstanceGroupCardNameHD "team" "group"
                                ]
                ]
            ]
        , describe "when pipeline instances are favorited" <|
            [ test "only the favorited instances are shown in the favorites section" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> gotPipelines
                            [ pipelineInstance BuildStatusSucceeded False 1
                            , pipelineInstance BuildStatusSucceeded False 2
                            ]
                        |> Application.handleDelivery
                            (FavoritedPipelinesReceived <|
                                Ok <|
                                    Set.singleton 1
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ id "dashboard-favorite-pipelines" ]
                        |> findBody
                        |> rows
                        |> Expect.all
                            [ Query.count (Expect.equal 1)
                            , firstRow >> Query.count (Expect.equal 1)
                            ]
            ]
        ]
