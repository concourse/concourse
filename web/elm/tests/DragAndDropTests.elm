module DragAndDropTests exposing (all)

import Application.Application as Application
import Common exposing (given, then_, when)
import Concourse exposing (JsonValue(..))
import Dashboard.Group.Models exposing (Card(..))
import DashboardTests exposing (whenOnDashboard)
import Data
import Dict exposing (Dict)
import Expect exposing (Expectation)
import Http
import Json.Encode as Encode
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Message exposing (DropTarget(..), Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as TopLevelMessage exposing (TopLevelMessage(..))
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, containing, id, style, text)
import Time
import Url


all : Test
all =
    describe "dragging and dropping pipeline cards"
        [ test "pipeline card has dragstart listener" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> when iAmLookingAtTheFirstCard
                >> then_ (itListensForDragStartWithCard firstPipelineCard)
        , test "instance group card has drag start listener with id independent of the visible instances" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedPipelinesWithInstanceVars
                >> when iAmLookingAtTheInstanceGroupCard
                >> then_ (itListensForDragStartWithCard instanceGroupCard)
        , test "pipeline card disappears when dragging starts" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> when iAmLookingAtTheFirstCard
                >> then_ itIsInvisible
        , test "pipeline cards wrappers transition their transform when dragging" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> when iAmLookingAtTheFirstPipelineCardWrapper
                >> then_ itHasTransformTransition
        , test "final drop area has dragenter listener" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> when iAmLookingAtTheFinalDropArea
                >> then_ itListensForDragEnter
        , test "final drop area has dragover listener (should prevent default)" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> when iAmLookingAtTheFinalDropArea
                >> then_ itListensForDragOverPreventingDefault
        , test "pipeline card has dragend listener" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> when iAmLookingAtTheFirstCard
                >> then_ itListensForDragEnd
        , test "pipeline card becomes visible when it is dropped" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> given iDropThePipelineCard
                >> when iAmLookingAtTheFirstCard
                >> then_ itIsVisible
        , test "dropping first pipeline card on final drop area rearranges cards" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when iAmLookingAtTheFirstCard
                >> then_ itIsTheOtherPipelineCard
        , test "dropping first pipeline card on final drop area makes API call" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> when iDropThePipelineCard
                >> then_ myBrowserMakesTheOrderPipelinesAPICall
        , test "API call only orders pipelines on a single team" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedPipelinesFromMultipleTeams
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> when iDropThePipelineCard
                >> then_ myBrowserMakesTheOrderPipelinesAPICall
        , test "instance group name appears once in API call" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedPipelinesWithInstanceVars
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> when iDropThePipelineCard
                >> then_ myBrowserMakesTheOrderPipelinesAPICall
        , test "dragging an instance group card uses an identifier independent of what instances are visible" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedPipelinesWithInstanceVars
                >> given iAmDraggingTheInstanceGroupCard
                >> given iAmDraggingOverTheFirstDropArea
                >> when iDropTheCard
                >> then_ myBrowserMakesTheOrderPipelinesAPICall
        , test "instance group can be reordered" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedPipelinesWithInstanceVars
                >> given iAmDraggingTheInstanceGroupCard
                >> given iAmDraggingOverTheFirstDropArea
                >> when iDropTheCard
                >> then_ myBrowserMakesTheOrderPipelinesAPICall
        , test "instanced pipelines can be reordered" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedPipelinesWithInstanceVars
                >> given iAmViewingTheInstanceGroup
                >> given iAmDraggingTheFirstInstancedPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> when iDropTheCard
                >> then_ myBrowserMakesTheOrderPipelinesWithinGroupAPICall
        , test "instanced pipeline card has dragstart listener" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedPipelinesWithInstanceVars
                >> given iAmViewingTheInstanceGroup
                >> when iAmLookingAtTheFirstCard
                >> then_ (itListensForDragStartWithCard firstInstancedPipelineCard)
        , test "dashboard does not auto-refresh during dragging" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedPipelinesFromMultipleTeams
                >> given iAmDraggingTheFirstPipelineCard
                >> when fiveSecondsPasses
                >> then_ myBrowserDoesNotRequestPipelineData
        , test "dropping a card displays a spinner near the pipeline team name" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when iAmLookingAtTheTeamHeader
                >> then_ iSeeASpinner
        , test "dropping a card does not display a spinner near other team names" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedPipelinesFromMultipleTeams
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when iAmLookingAtTheOtherTeamHeader
                >> then_ iDoNotSeeASpinner
        , test "after dropping a card, every pipeline card of that team has opacity 0.5" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when iAmLookingAtAllPipelineCardsOfThatTeam
                >> then_ iSeeAllCardsHaveOpacity
        , test "after dropping a card, every pipeline card of that team is disabled" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when iAmLookingAtAllPipelineCardsOfThatTeam
                >> then_ theyAreNotClickable
        , test "fetches team's pipelines when order pipelines call succeeds" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when orderPipelinesSucceeds
                >> then_ myBrowserMakesTheFetchPipelinesAPICall
        , test "when dropping succeeds the spinner disappears" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> given dashboardRefreshPipelines
                >> when iAmLookingAtTheTeamHeader
                >> then_ iDoNotSeeASpinner
        , test "when dropping succeeds all pipeline cards of that team have opacity of 1" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> given dashboardRefreshPipelines
                >> when iAmLookingAtAllPipelineCardsOfThatTeam
                >> then_ iSeeAllCardsDontHaveOpacity
        , test "when dropping succeeds, every pipeline card of that team is enabled" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> given dashboardRefreshPipelines
                >> when iAmLookingAtAllPipelineCardsOfThatTeam
                >> then_ theyAreClickable
        , test "fetches team's pipelines when order pipelines call fails" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when orderPipelinesFails
                >> then_ myBrowserMakesTheFetchPipelinesAPICall
        ]


iVisitedTheDashboard _ =
    whenOnDashboard { highDensity = False }


myBrowserFetchedOnePipeline =
    Application.handleCallback
        (Callback.AllPipelinesFetched <|
            Ok
                [ Data.pipeline "team" 1 |> Data.withName "pipeline" ]
        )


firstPipelineCard =
    PipelineCard <| (Data.dashboardPipeline "team" 1 |> Data.withName "pipeline")


myBrowserFetchedTwoPipelines =
    Application.handleCallback
        (Callback.AllPipelinesFetched <|
            Ok
                [ Data.pipeline "team" 1 |> Data.withName "pipeline"
                , Data.pipeline "team" 2 |> Data.withName "other-pipeline"
                ]
        )


myBrowserFetchedPipelinesWithInstanceVars =
    Application.handleCallback
        (Callback.AllPipelinesFetched <|
            Ok
                [ Data.pipeline "team" 1 |> Data.withName "pipeline"
                , Data.pipeline "team" 2 |> Data.withName "other-pipeline" |> Data.withArchived True
                , Data.pipeline "team" 3
                    |> Data.withName "other-pipeline"
                    |> Data.withInstanceVars (Dict.fromList [ ( "hello", JsonString "world" ) ])
                , Data.pipeline "team" 4
                    |> Data.withName "other-pipeline"
                    |> Data.withInstanceVars (Dict.fromList [ ( "brach", JsonString "world-1" ) ])
                ]
        )


firstInstancedPipelineCard =
    InstancedPipelineCard <|
        (Data.dashboardPipeline "team" 3
            |> Data.withName "other-pipeline"
            |> Data.withInstanceVars (Dict.fromList [ ( "hello", JsonString "world" ) ])
        )


instanceGroupCard =
    -- pipeline 2 is not included because it's archived and we aren't viewing archived pipelines
    InstanceGroupCard
        (Data.dashboardPipeline "team" 3
            |> Data.withName "other-pipeline"
            |> Data.withInstanceVars (Dict.fromList [ ( "hello", JsonString "world" ) ])
        )
        [ Data.dashboardPipeline "team" 4
            |> Data.withName "other-pipeline"
            |> Data.withInstanceVars (Dict.fromList [ ( "brach", JsonString "world-1" ) ])
        ]


iAmViewingTheInstanceGroup =
    Tuple.first
        >> Application.update
            (Update <|
                FilterMsg "group:\"other-pipeline\""
            )


myBrowserFetchedPipelinesFromMultipleTeams =
    Application.handleCallback
        (Callback.AllPipelinesFetched <|
            Ok
                [ Data.pipeline "team" 1 |> Data.withName "pipeline"
                , Data.pipeline "team" 2 |> Data.withName "other-pipeline"
                , Data.pipeline "other-team" 3 |> Data.withName "third-pipeline"
                ]
        )


iAmLookingAtTheFirstCard =
    Tuple.first
        >> Common.queryView
        >> Query.findAll [ class "card" ]
        >> Query.first


iAmLookingAtTheInstanceGroupCard =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "card", containing [ text "other-pipeline" ] ]


iAmLookingAtTheFirstPipelineCardWrapper =
    Tuple.first
        >> Common.queryView
        >> Query.findAll [ class "card-wrapper" ]
        >> Query.first


iAmLookingAtTheInitialDropArea =
    Tuple.first
        >> Common.queryView
        >> Query.findAll [ class "drop-area" ]
        >> Query.first


iAmLookingAtAllPipelineCardsOfThatTeam =
    Tuple.first
        >> Common.queryView
        >> Query.find [ id "team" ]
        >> Query.findAll [ class "card" ]


itListensForDragStartWithCard : Card -> Query.Single TopLevelMessage -> Expectation
itListensForDragStartWithCard card =
    Event.simulate (Event.custom "dragstart" (Encode.object []))
        >> Event.expect
            (TopLevelMessage.Update <| Message.DragStart card)


iAmDraggingTheFirstPipelineCard =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragStart firstPipelineCard)


iAmDraggingTheFirstInstancedPipelineCard =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragStart firstInstancedPipelineCard)


iAmDraggingTheInstanceGroupCard =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragStart instanceGroupCard)


itIsInvisible =
    Query.has
        [ style "width" "0"
        , style "margin" "0 12.5px"
        , style "overflow" "hidden"
        ]


itIsVisible =
    Query.hasNot
        [ style "width" "0"
        , style "margin" "0 12.5px"
        , style "overflow" "hidden"
        ]


itHasTransformTransition =
    Query.has [ style "transition" "transform 0.2s ease-in-out" ]


theyAreClickable =
    Query.each (Query.hasNot [ style "pointer-events" "none" ])


theyAreNotClickable =
    Query.each (Query.has [ style "pointer-events" "none" ])


iAmLookingAtTheFinalDropArea =
    Tuple.first
        >> Common.queryView
        >> Query.findAll [ class "drop-area" ]
        >> Query.index -1


itListensForDragEnter =
    Event.simulate (Event.custom "dragenter" (Encode.object []))
        >> Event.expect
            (TopLevelMessage.Update <| Message.DragOver <| End)



-- https://github.com/elm-explorations/test/pull/80 has been merged, but has
-- not yet been released. Until then we can only test that a dragover listener
-- is registered, but not that it actually has preventDefault: true.
-- TODO: once a new minor version of elm-exploration/test is released, change
--       `expect` to `expectPreventDefault` below.


itListensForDragOverPreventingDefault =
    Event.simulate (Event.custom "dragover" (Encode.object []))
        >> Event.expect
            (TopLevelMessage.Update <| Message.DragOver <| End)


iAmDraggingOverTheFirstDropArea =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragOver <| Before firstPipelineCard)


iAmDraggingOverTheThirdDropArea =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragOver <| End)


iAmLookingAtTheTeamHeader =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "dashboard-team-header" ]


iAmLookingAtTheOtherTeamHeader =
    Tuple.first
        >> Common.queryView
        >> Query.find [ id "other-team" ]
        >> Query.find [ class "dashboard-team-header" ]


iSeeASpinner =
    Query.has
        [ style "animation"
            "container-rotate 1568ms linear infinite"
        ]


iSeeAllCardsHaveOpacity =
    Query.each (Query.has [ style "opacity" "0.5" ])


iDoNotSeeASpinner =
    Query.hasNot
        [ style "animation"
            "container-rotate 1568ms linear infinite"
        ]


iSeeAllCardsDontHaveOpacity =
    Query.each (Query.has [ style "opacity" "1" ])


itListensForDragEnd =
    Event.simulate (Event.custom "dragend" (Encode.object []))
        >> Event.expect
            (TopLevelMessage.Update <| Message.DragEnd)


iDropThePipelineCard =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragEnd)


iDropTheCard =
    iDropThePipelineCard


itIsTheOtherPipelineCard =
    Query.has [ text "other-pipeline" ]


myBrowserMakesTheOrderPipelinesAPICall =
    Tuple.second
        >> Common.contains
            (Effects.SendOrderPipelinesRequest "team"
                [ "other-pipeline", "pipeline" ]
            )


myBrowserMakesTheOrderPipelinesWithinGroupAPICall =
    Tuple.second
        >> Common.contains
            (Effects.SendOrderPipelinesWithinGroupRequest { teamName = "team", name = "other-pipeline" }
                [ Dict.empty
                , Dict.fromList [ ( "brach", JsonString "world-1" ) ]
                , Dict.fromList [ ( "hello", JsonString "world" ) ]
                ]
            )


myBrowserMakesTheFetchPipelinesAPICall =
    Tuple.second
        >> Common.contains
            (Effects.FetchPipelines "team")


orderPipelinesSucceeds =
    Tuple.first
        >> Application.handleCallback
            (Callback.PipelinesOrdered "team" <| Ok ())


orderPipelinesFails =
    Tuple.first
        >> Application.handleCallback
            (Callback.PipelinesOrdered "team" <| Data.httpInternalServerError)


dashboardRefreshPipelines =
    Tuple.first
        >> Application.handleCallback
            (Callback.PipelinesFetched <|
                Ok
                    [ Data.pipeline "team" 0 |> Data.withName "pipeline"
                    , Data.pipeline "team" 1 |> Data.withName "other-pipeline"
                    ]
            )


dashboardFailsToRefreshPipelines =
    Tuple.first
        >> Application.handleCallback
            (Callback.PipelinesFetched <| Data.httpInternalServerError)


fiveSecondsPasses =
    Tuple.first
        >> Application.handleDelivery
            (ClockTicked FiveSeconds <| Time.millisToPosix 0)


myBrowserDoesNotRequestPipelineData =
    Tuple.second >> Common.notContains Effects.FetchAllPipelines
