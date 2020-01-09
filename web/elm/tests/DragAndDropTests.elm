module DragAndDropTests exposing (all)

import Application.Application as Application
import Common exposing (given, then_, when)
import Dict exposing (Dict)
import Expect exposing (Expectation)
import Http
import Json.Encode as Encode
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Message
import Message.Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as TopLevelMessage exposing (TopLevelMessage)
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (class, style, text)
import Time
import Url


all : Test
all =
    describe "dragging and dropping pipeline cards"
        [ test "pipeline card has dragstart listener" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> when iAmLookingAtTheFirstPipelineCard
                >> then_ itListensForDragStart
        , test "pipeline card disappears when dragging starts" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> when iAmLookingAtTheFirstPipelineCard
                >> then_ itIsInvisible
        , test "initial drop area grows when dragging starts" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> when iAmLookingAtTheInitialDropArea
                >> then_ itIsWide
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
        , test "initial drop area shrinks when dragging over final drop area" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheSecondDropArea
                >> when iAmLookingAtTheInitialDropArea
                >> then_ itIsNarrow
        , test "pipeline card has dragend listener" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> when iAmLookingAtTheFirstPipelineCard
                >> then_ itListensForDragEnd
        , test "initial drop area shrinks when pipeline card is dropped" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> given iDropThePipelineCard
                >> when iAmLookingAtTheInitialDropArea
                >> then_ itIsNarrow
        , test "pipeline card becomes visible when it is dropped" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedOnePipeline
                >> given iAmDraggingTheFirstPipelineCard
                >> given iDropThePipelineCard
                >> when iAmLookingAtTheFirstPipelineCard
                >> then_ itIsVisible
        , test "dropping first pipeline card on final drop area rearranges cards" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when iAmLookingAtTheFirstPipelineCard
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
        , test "fetches team's pipelines when order pipelines call fails" <|
            given iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when orderPipelinesFails
                >> then_ myBrowserMakesTheFetchPipelinesAPICall
        , test "failed to fetch team's pipelines displays turbulence view" <|
            given
                iVisitedTheDashboard
                >> given myBrowserFetchedTwoPipelines
                >> given iAmDraggingTheFirstPipelineCard
                >> given iAmDraggingOverTheThirdDropArea
                >> given iDropThePipelineCard
                >> when dashboardFailsToRefreshPipelines
                >> then_ iSeeTheTurbulenceView
        ]


iVisitedTheDashboard _ =
    Common.init "/"


myBrowserFetchedOnePipeline =
    Application.handleCallback
        (Callback.AllPipelinesFetched <|
            Ok
                [ { id = 0
                  , name = "pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                ]
        )


myBrowserFetchedTwoPipelines =
    Application.handleCallback
        (Callback.AllPipelinesFetched <|
            Ok
                [ { id = 0
                  , name = "pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                , { id = 1
                  , name = "other-pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                ]
        )


myBrowserFetchedPipelinesFromMultipleTeams =
    Application.handleCallback
        (Callback.AllPipelinesFetched <|
            Ok
                [ { id = 0
                  , name = "pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                , { id = 1
                  , name = "other-pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                , { id = 2
                  , name = "third-pipeline"
                  , paused = False
                  , public = True
                  , teamName = "other-team"
                  , groups = []
                  }
                ]
        )


iAmLookingAtTheFirstPipelineCard =
    Tuple.first
        >> Common.queryView
        >> Query.findAll [ class "card" ]
        >> Query.first


itListensForDragStart : Query.Single TopLevelMessage -> Expectation
itListensForDragStart =
    Event.simulate (Event.custom "dragstart" (Encode.object []))
        >> Event.expect
            (TopLevelMessage.Update <| Message.DragStart "team" 0)


iAmDraggingTheFirstPipelineCard =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragStart "team" 0)


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


iAmLookingAtTheInitialDropArea =
    Tuple.first
        >> Common.queryView
        >> Query.findAll [ class "drop-area" ]
        >> Query.first


itIsWide =
    Query.has [ style "padding" "0 198.5px" ]


itIsNarrow =
    Query.has [ style "padding" "0 50px" ]


iAmLookingAtTheFinalDropArea =
    Tuple.first
        >> Common.queryView
        >> Query.findAll [ class "drop-area" ]
        >> Query.index -1


itListensForDragEnter =
    Event.simulate (Event.custom "dragenter" (Encode.object []))
        >> Event.expect
            (TopLevelMessage.Update <| Message.DragOver "team" 1)



-- https://github.com/elm-explorations/test/pull/80 has been merged, but has
-- not yet been released. Until then we can only test that a dragover listener
-- is registered, but not that it actually has preventDefault: true.
-- TODO: once a new minor version of elm-exploration/test is released, change
--       `expect` to `expectPreventDefault` below.


itListensForDragOverPreventingDefault =
    Event.simulate (Event.custom "dragover" (Encode.object []))
        >> Event.expect
            (TopLevelMessage.Update <| Message.DragOver "team" 1)


iAmDraggingOverTheSecondDropArea =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragOver "team" 1)


iAmDraggingOverTheThirdDropArea =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragOver "team" 2)


iAmLookingAtTheTeamHeader =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "dashboard-team-header" ]


iSeeASpinner =
    Query.has
        [ style "animation"
            "container-rotate 1568ms linear infinite"
        ]


iSeeTheTurbulenceView =
    Tuple.first
        >> Common.queryView
        >> Query.has [ text "experiencing turbulence" ]


iDoNotSeeASpinner =
    Query.hasNot
        [ style "animation"
            "container-rotate 1568ms linear infinite"
        ]


itListensForDragEnd =
    Event.simulate (Event.custom "dragend" (Encode.object []))
        >> Event.expect
            (TopLevelMessage.Update <| Message.DragEnd)


iDropThePipelineCard =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.DragEnd)


itIsTheOtherPipelineCard =
    Query.has [ text "other-pipeline" ]


myBrowserMakesTheOrderPipelinesAPICall =
    Tuple.second
        >> Common.contains
            (Effects.SendOrderPipelinesRequest "team"
                [ "other-pipeline", "pipeline" ]
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
            (Callback.PipelinesOrdered "team" <|
                Err
                    (Http.BadStatus
                        { url = "http://localhost:8080"
                        , status = { code = 500, message = "could not find pipeline" }
                        , headers = Dict.empty
                        , body = ""
                        }
                    )
            )


dashboardRefreshPipelines =
    Tuple.first
        >> Application.handleCallback
            (Callback.PipelinesFetched <|
                Ok
                    [ { id = 0
                      , name = "pipeline"
                      , paused = False
                      , public = True
                      , teamName = "team"
                      , groups = []
                      }
                    , { id = 1
                      , name = "other-pipeline"
                      , paused = False
                      , public = True
                      , teamName = "team"
                      , groups = []
                      }
                    ]
            )


dashboardFailsToRefreshPipelines =
    Tuple.first
        >> Application.handleCallback
            (Callback.PipelinesFetched <|
                Err
                    (Http.BadStatus
                        { url = "http://localhost:8080"
                        , status = { code = 500, message = "could not find pipeline" }
                        , headers = Dict.empty
                        , body = ""
                        }
                    )
            )


fiveSecondsPasses =
    Tuple.first
        >> Application.handleDelivery
            (ClockTicked FiveSeconds <| Time.millisToPosix 0)


myBrowserDoesNotRequestPipelineData =
    Tuple.second >> Common.notContains Effects.FetchAllPipelines
