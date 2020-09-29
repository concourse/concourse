module DashboardInstanceGroupTests exposing
    ( all
    , archived
    , gotPipelines
    , pipelineInstance
    )

import Application.Application as Application
import Assets
import Common
    exposing
        ( defineHoverBehaviour
        , isColorWithStripes
        , pipelineRunningKeyframes
        )
import Concourse exposing (JsonValue(..))
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Cli as Cli
import Concourse.PipelineStatus exposing (PipelineStatus(..))
import DashboardTests
    exposing
        ( givenDataUnauthenticated
        , job
        , running
        , whenOnDashboard
        )
import Data
import Dict
import Expect exposing (Expectation)
import Html.Attributes as Attr
import Http
import Json.Encode
import Keyboard
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Msgs
import Message.Subscription as Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as ApplicationMsgs
import Routes exposing (DashboardView(..))
import Set
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector
    exposing
        ( Selector
        , attribute
        , class
        , containing
        , id
        , style
        , tag
        , text
        )
import Time


all : Test
all =
    describe "Dashboard Instance Group View" <|
        [ test "high density toggle is disabled" <|
            \_ ->
                whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                    |> gotPipelines [ pipelineInstance BuildStatusSucceeded False 1 ]
                    |> Common.queryView
                    |> Query.find [ id "legend" ]
                    |> Query.hasNot [ text "high-density" ]
        , test "displays a card for each instance" <|
            \_ ->
                whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                    |> gotPipelines
                        [ pipelineInstance BuildStatusSucceeded False 1
                        , pipelineInstance BuildStatusSucceeded False 2
                        ]
                    |> Common.queryView
                    |> Query.findAll [ class "card" ]
                    |> Query.count (Expect.equal 2)
        , test "does not display other pipeline cards" <|
            \_ ->
                whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                    |> gotPipelines
                        [ pipelineInstance BuildStatusSucceeded False 1
                        , ( Data.pipeline "team" 2 |> Data.withName "other-pipeline", [] )
                        ]
                    |> Common.queryView
                    |> Query.findAll [ class "card" ]
                    |> Query.count (Expect.equal 1)
        , test "applies filters to cards" <|
            \_ ->
                whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                    |> gotPipelines
                        [ pipelineInstance BuildStatusSucceeded False 1
                        , pipelineInstance BuildStatusFailed False 1
                        ]
                    |> Application.update
                        (ApplicationMsgs.Update <|
                            Msgs.FilterMsg "status: succeeded"
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.findAll [ class "card" ]
                    |> Query.count (Expect.equal 1)
        , test "respects dashboard view" <|
            \_ ->
                whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                    |> gotPipelines
                        [ pipelineInstance BuildStatusSucceeded False 1
                        , archived <| pipelineInstance BuildStatusFailed False 2
                        ]
                    |> Common.queryView
                    |> Query.findAll [ class "card" ]
                    |> Query.count (Expect.equal 1)
        , describe "breadcrumb" <|
            let
                findBreadcrumb =
                    Query.find [ id "breadcrumbs" ]
                        >> Query.children []
                        >> Query.index 0
            in
            [ test "displays instance group name" <|
                \_ ->
                    whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                        |> gotPipelines
                            [ pipelineInstance BuildStatusSucceeded False 1
                            , pipelineInstance BuildStatusSucceeded False 2
                            ]
                        |> Common.queryView
                        |> findBreadcrumb
                        |> Query.has [ text "group" ]
            , test "displays badge displaying number of pipelines" <|
                \_ ->
                    whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                        |> gotPipelines
                            [ pipelineInstance BuildStatusSucceeded False 1
                            , pipelineInstance BuildStatusSucceeded False 2
                            ]
                        |> Common.queryView
                        |> findBreadcrumb
                        |> Query.has
                            [ style "width" "20px"
                            , style "height" "20px"
                            , style "font-size" "14px"
                            , containing [ text "2" ]
                            ]
            ]
        , describe "pipeline cards" <|
            let
                findCardWrapper =
                    Query.find [ class "card-wrapper" ]

                findCard =
                    Query.find [ class "card" ]

                findCards =
                    Query.findAll [ class "card" ]

                findHeader =
                    Query.find [ class "card-header" ]

                findInstanceVars =
                    Query.findAll [ class "instance-var" ]
            in
            [ test "displays instance vars in header" <|
                \_ ->
                    whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                        |> gotPipelines
                            [ pipelineInstanceWithVars 1
                                [ ( "a"
                                  , JsonObject
                                        [ ( "b", JsonString "foo" )
                                        , ( "c", JsonString "bar" )
                                        ]
                                  )
                                , ( "d"
                                  , JsonArray
                                        [ JsonNumber 1.0
                                        , JsonRaw <| Json.Encode.bool True
                                        ]
                                  )
                                ]
                            ]
                        |> Common.queryView
                        |> findInstanceVars
                        |> Expect.all
                            [ Query.index 0 >> Query.has [ text "a.b" ]
                            , Query.index 1 >> Query.has [ text "a.c" ]
                            , Query.index 2 >> Query.has [ text "d[0]" ]
                            , Query.index 3 >> Query.has [ text "d[1]" ]
                            ]
            , test "card header expands with number of variables" <|
                \_ ->
                    whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                        |> gotPipelines
                            [ pipelineInstanceWithVars 1
                                [ ( "a", JsonString "foo" )
                                , ( "b", JsonString "bar" )
                                ]
                            ]
                        |> Common.queryView
                        |> findHeader
                        |> Query.has
                            [ style "height" "80px"
                            , style "box-sizing" "border-box"
                            ]
            , test "card wrapper expands with number of variables" <|
                \_ ->
                    whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                        |> gotPipelines
                            [ pipelineInstanceWithVars 1
                                [ ( "a", JsonString "foo" )
                                , ( "b", JsonString "bar" )
                                ]
                            ]
                        |> Common.queryView
                        |> findCardWrapper
                        |> Query.has [ style "height" "298px" ]
            , test "card header height matches largest header in row" <|
                \_ ->
                    whenOnDashboardViewingInstanceGroup { dashboardView = ViewNonArchivedPipelines }
                        |> gotPipelines
                            [ pipelineInstanceWithVars 1
                                [ ( "a", JsonString "foo" ) ]
                            , pipelineInstanceWithVars 2
                                [ ( "a", JsonString "foo" )
                                , ( "b", JsonString "bar" )
                                ]
                            ]
                        |> Common.queryView
                        |> findCards
                        |> Query.first
                        |> findHeader
                        |> Query.has
                            [ style "height" "80px"
                            , style "box-sizing" "border-box"
                            ]
            ]
        ]


whenOnDashboardViewingInstanceGroup : { dashboardView : DashboardView } -> Application.Model
whenOnDashboardViewingInstanceGroup { dashboardView } =
    whenOnDashboard { highDensity = False }
        |> Application.handleDelivery
            (RouteChanged <|
                Routes.Dashboard
                    { searchType = Routes.Normal "" <| Just { teamName = "team", name = "group" }
                    , dashboardView = dashboardView
                    }
            )
        |> Tuple.first


gotPipelines : List ( Concourse.Pipeline, List Concourse.Job ) -> Application.Model -> Application.Model
gotPipelines data =
    Application.handleCallback
        (Callback.AllJobsFetched <| Ok (data |> List.concatMap Tuple.second))
        >> Tuple.first
        >> givenDataUnauthenticated [ { id = 1, name = "team" } ]
        >> Tuple.first
        >> Application.handleCallback
            (Callback.AllPipelinesFetched <| Ok (data |> List.map Tuple.first))
        >> Tuple.first


pipelineInstance : BuildStatus -> Bool -> Int -> ( Concourse.Pipeline, List Concourse.Job )
pipelineInstance status isRunning id =
    let
        jobFunc =
            if isRunning then
                job >> running

            else
                job
    in
    ( Data.pipeline "team" id
        |> Data.withName "group"
        |> Data.withInstanceVars (Dict.fromList [ ( "version", JsonNumber <| toFloat id ) ])
    , [ status |> jobFunc |> Data.withPipelineId id ]
    )


pipelineInstanceWithVars : Int -> List ( String, JsonValue ) -> ( Concourse.Pipeline, List Concourse.Job )
pipelineInstanceWithVars id vars =
    ( Data.pipeline "team" id
        |> Data.withName "group"
        |> Data.withInstanceVars (Dict.fromList vars)
    , [ job BuildStatusSucceeded |> Data.withPipelineId id ]
    )


archived : ( Concourse.Pipeline, a ) -> ( Concourse.Pipeline, a )
archived ( p, j ) =
    ( p |> Data.withArchived True, j )
