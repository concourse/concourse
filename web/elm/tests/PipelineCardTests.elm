module PipelineCardTests exposing (all)

import Application.Application as Application
import Assets
import Colors
import Common exposing (defineHoverBehaviour, isColorWithStripes)
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.PipelineStatus exposing (PipelineStatus(..), StatusDetails(..))
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
import Expect exposing (Expectation)
import Html.Attributes as Attr
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Msgs
import Message.Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as ApplicationMsgs
import Set
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, class, containing, style, tag, text)
import Time


all : Test
all =
    describe "pipeline cards" <|
        let
            findHeader :
                Query.Single ApplicationMsgs.TopLevelMessage
                -> Query.Single ApplicationMsgs.TopLevelMessage
            findHeader =
                Query.find [ class "card-header" ]

            findBody :
                Query.Single ApplicationMsgs.TopLevelMessage
                -> Query.Single ApplicationMsgs.TopLevelMessage
            findBody =
                Query.find [ class "card-body" ]

            pipelineWithStatus :
                BuildStatus
                -> Bool
                -> Application.Model
                -> Query.Single ApplicationMsgs.TopLevelMessage
            pipelineWithStatus status isRunning =
                let
                    jobFunc =
                        if isRunning then
                            job >> running

                        else
                            job
                in
                Application.handleCallback
                    (Callback.AllJobsFetched <| Ok [ jobFunc status ])
                    >> Tuple.first
                    >> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                    >> Tuple.first
                    >> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    >> Tuple.first
                    >> Application.handleDelivery
                        (ClockTicked OneSecond <| Time.millisToPosix 0)
                    >> Tuple.first
                    >> Common.queryView
        in
        [ describe "when team has no visible pipelines" <|
            let
                noPipelinesCard : () -> Query.Single ApplicationMsgs.TopLevelMessage
                noPipelinesCard _ =
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated
                            (apiData
                                [ ( "some-team", [] )
                                , ( "other-team", [] )
                                ]
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "other-team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find
                            [ class "dashboard-team-group"
                            , attribute <|
                                Attr.attribute "data-team-name"
                                    "some-team"
                            ]
                        |> Query.find [ class "card" ]
            in
            [ describe "card" <|
                [ test "card has display flex with direction column" <|
                    noPipelinesCard
                        >> Query.has
                            [ style "display" "flex"
                            , style "flex-direction" "column"
                            ]
                , test "card has width 272px and height 268px" <|
                    noPipelinesCard
                        >> Query.has
                            [ style "width" "272px"
                            , style "height" "268px"
                            ]
                , test "card has a left margin of 25px" <|
                    noPipelinesCard
                        >> Query.has
                            [ style "margin-left" "25px" ]
                ]
            , describe "header" <|
                let
                    header : () -> Query.Single ApplicationMsgs.TopLevelMessage
                    header =
                        noPipelinesCard
                            >> findHeader
                in
                [ test "says 'no pipeline set' in white font" <|
                    header
                        >> Expect.all
                            [ Query.has [ text "no pipeline set" ]
                            , Query.has [ style "color" white ]
                            ]
                , test "has dark grey background and 12.5px padding" <|
                    header
                        >> Query.has
                            [ style "background-color" darkGrey
                            , style "padding" "12.5px"
                            ]
                , test "text is larger and wider spaced" <|
                    header
                        >> Query.has
                            [ style "font-size" "1.5em"
                            , style "letter-spacing" "0.1em"
                            ]
                , test "text is centered" <|
                    header
                        >> Query.has [ style "text-align" "center" ]
                ]
            , describe "body" <|
                let
                    body : () -> Query.Single ApplicationMsgs.TopLevelMessage
                    body =
                        noPipelinesCard
                            >> Query.find [ class "card-body" ]
                in
                [ test "has 20px 36px padding" <|
                    body
                        >> Query.has
                            [ style "padding" "20px 36px" ]
                , test "fills available height" <|
                    body
                        >> Query.has [ style "flex-grow" "1" ]
                , test "has dark grey background" <|
                    body
                        >> Query.has [ style "background-color" darkGrey ]
                , test "has 2px margins above and below" <|
                    body
                        >> Query.has [ style "margin" "2px 0" ]
                , test "has lighter grey placeholder box that fills" <|
                    body
                        >> Expect.all
                            [ Query.has [ style "display" "flex" ]
                            , Query.children []
                                >> Query.first
                                >> Query.has
                                    [ style "background-color" middleGrey
                                    , style "flex-grow" "1"
                                    ]
                            ]
                ]
            , test "footer is dark grey and 47 pixels tall" <|
                noPipelinesCard
                    >> Query.find [ class "card-footer" ]
                    >> Query.has
                        [ style "background-color" darkGrey
                        , style "height" "47px"
                        ]
            ]
        , test "has 'move' cursor when not searching and result fetched" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find
                        [ class "card"
                        , containing [ text "pipeline" ]
                        ]
                    |> Query.has [ style "cursor" "move" ]
        , test "does not have 'move' cursor when rendering based on cache" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                    |> Tuple.first
                    |> Application.handleDelivery
                        (CachedPipelinesReceived <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find
                        [ class "card"
                        , containing [ text "pipeline" ]
                        ]
                    |> Query.hasNot [ style "cursor" "move" ]
        , test "fills available space" <|
            \_ ->
                whenOnDashboard { highDensity = False }
                    |> givenDataUnauthenticated (apiData [ ( "team", [] ) ])
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                        )
                    |> Tuple.first
                    |> Common.queryView
                    |> Query.find
                        [ class "card"
                        , containing [ text "pipeline" ]
                        ]
                    |> Query.has
                        [ style "width" "100%"
                        , style "height" "100%"
                        , style "display" "flex"
                        , style "flex-direction" "column"
                        ]
        , describe "header" <|
            let
                header : () -> Query.Single ApplicationMsgs.TopLevelMessage
                header _ =
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated
                            (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find
                            [ class "card"
                            , containing [ text "pipeline" ]
                            ]
                        |> findHeader
            in
            [ test "has dark grey background" <|
                header
                    >> Query.has [ style "background-color" darkGrey ]
            , test "has larger, spaced-out white text" <|
                header
                    >> Query.has
                        [ style "font-size" "1.5em"
                        , style "letter-spacing" "0.1em"
                        , style "color" white
                        ]
            , test "has 12.5px padding" <|
                header
                    >> Query.has [ style "padding" "12.5px" ]
            , test "text does not overflow or wrap" <|
                header
                    >> Query.children []
                    >> Query.first
                    >> Query.has
                        [ style "width" "245px"
                        , style "white-space" "nowrap"
                        , style "overflow" "hidden"
                        , style "text-overflow" "ellipsis"
                        ]
            ]
        , describe "colored banner" <|
            let
                findBanner =
                    Query.find
                        [ class "card"
                        , containing [ text "pipeline" ]
                        ]
                        >> Query.children []
                        >> Query.first

                isSolid : String -> Query.Single ApplicationMsgs.TopLevelMessage -> Expectation
                isSolid color =
                    Query.has [ style "background-color" color ]
            in
            [ describe "non-HD view"
                [ test "is 7px tall" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> findBanner
                            |> Query.has [ style "height" "7px" ]
                , test "is grey when pipeline is cached" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                            |> Tuple.first
                            |> Application.handleDelivery
                                (CachedPipelinesReceived <|
                                    Ok
                                        [ Data.pipeline "team" 0
                                            |> Data.withName "pipeline"
                                            |> Data.withPaused True
                                        ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> findBanner
                            |> isSolid lightGrey
                , test "is dark grey when pipeline is archived" <|
                    \_ ->
                        whenOnDashboardViewingAllPipelines { highDensity = False }
                            |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0
                                            |> Data.withName "pipeline"
                                            |> Data.withArchived True
                                            |> Data.withPaused True
                                        ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> findBanner
                            |> isSolid Colors.backgroundDark
                , test "is blue when pipeline is paused" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0
                                            |> Data.withName "pipeline"
                                            |> Data.withPaused True
                                        ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> findBanner
                            |> isSolid blue
                , test "is green when pipeline is succeeding" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusSucceeded
                                False
                            |> findBanner
                            |> isSolid green
                , test "is green with black stripes when pipeline is succeeding and running" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusSucceeded
                                True
                            |> findBanner
                            |> isColorWithStripes { thin = green, thick = darkGrey }
                , test "is grey when pipeline is pending" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataUnauthenticated
                                (apiData [ ( "team", [] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> findBanner
                            |> isSolid lightGrey
                , test "is grey with black stripes when pipeline is pending and running" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusStarted
                                True
                            |> findBanner
                            |> isColorWithStripes { thin = lightGrey, thick = darkGrey }
                , test "is red when pipeline is failing" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusFailed
                                False
                            |> findBanner
                            |> isSolid red
                , test "is red with black stripes when pipeline is failing and running" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusFailed
                                True
                            |> findBanner
                            |> isColorWithStripes { thin = red, thick = darkGrey }
                , test "is amber when pipeline is erroring" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusErrored
                                False
                            |> findBanner
                            |> isSolid amber
                , test "is amber with black stripes when pipeline is erroring and running" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusErrored
                                True
                            |> findBanner
                            |> isColorWithStripes { thin = amber, thick = darkGrey }
                , test "is brown when pipeline is aborted" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusAborted
                                False
                            |> findBanner
                            |> isSolid brown
                , test "is brown with black stripes when pipeline is aborted and running" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusAborted
                                True
                            |> findBanner
                            |> isColorWithStripes { thin = brown, thick = darkGrey }
                , describe "status priorities" <|
                    let
                        givenTwoJobs :
                            BuildStatus
                            -> BuildStatus
                            -> Query.Single ApplicationMsgs.TopLevelMessage
                        givenTwoJobs firstStatus secondStatus =
                            whenOnDashboard { highDensity = False }
                                |> Application.handleCallback
                                    (Callback.AllJobsFetched <|
                                        Ok
                                            [ job firstStatus
                                            , otherJob secondStatus
                                            ]
                                    )
                                |> Tuple.first
                                |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                    in
                    [ test "failed is more important than errored" <|
                        \_ ->
                            givenTwoJobs
                                BuildStatusFailed
                                BuildStatusErrored
                                |> findBanner
                                |> isSolid red
                    , test "errored is more important than aborted" <|
                        \_ ->
                            givenTwoJobs
                                BuildStatusErrored
                                BuildStatusAborted
                                |> findBanner
                                |> isSolid amber
                    , test "aborted is more important than succeeding" <|
                        \_ ->
                            givenTwoJobs
                                BuildStatusAborted
                                BuildStatusSucceeded
                                |> findBanner
                                |> isSolid brown
                    , test "succeeding is more important than pending" <|
                        \_ ->
                            givenTwoJobs
                                BuildStatusSucceeded
                                BuildStatusPending
                                |> findBanner
                                |> isSolid green
                    ]
                , test "does not crash with a circular pipeline" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> Application.handleCallback (Callback.AllJobsFetched <| Ok circularJobs)
                            |> Tuple.first
                            |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> findBanner
                            |> isSolid green
                , describe "HD view"
                    [ test "is 8px wide" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (apiData [ ( "team", [] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> findBanner
                                |> Query.has [ style "width" "8px" ]
                    , test "is grey when pipeline is cached" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                                |> Tuple.first
                                |> Application.handleDelivery
                                    (CachedPipelinesReceived <|
                                        Ok
                                            [ Data.pipeline "team" 0
                                                |> Data.withName "pipeline"
                                                |> Data.withPaused True
                                            ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> findBanner
                                |> isSolid lightGrey
                    , test "is dark grey when pipeline is archived" <|
                        \_ ->
                            whenOnDashboardViewingAllPipelines { highDensity = False }
                                |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0
                                                |> Data.withName "pipeline"
                                                |> Data.withArchived True
                                                |> Data.withPaused True
                                            ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> findBanner
                                |> isSolid Colors.backgroundDark
                    , test "is blue when pipeline is paused" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0
                                                |> Data.withName "pipeline"
                                                |> Data.withPaused True
                                            ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> findBanner
                                |> isSolid blue
                    , test "is green when pipeline is succeeding" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> pipelineWithStatus
                                    BuildStatusSucceeded
                                    False
                                |> findBanner
                                |> isSolid green
                    , test "is green with black stripes when pipeline is succeeding and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> pipelineWithStatus
                                    BuildStatusSucceeded
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = green, thick = darkGrey }
                    , test "is grey when pipeline is pending" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> givenDataUnauthenticated
                                    (apiData [ ( "team", [] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> findBanner
                                |> isSolid lightGrey
                    , test "is grey with black stripes when pipeline is pending and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> pipelineWithStatus
                                    BuildStatusStarted
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = lightGrey, thick = darkGrey }
                    , test "is red when pipeline is failing" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> pipelineWithStatus
                                    BuildStatusFailed
                                    False
                                |> findBanner
                                |> isSolid red
                    , test "is red with black stripes when pipeline is failing and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> pipelineWithStatus
                                    BuildStatusFailed
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = red, thick = darkGrey }
                    , test "is amber when pipeline is erroring" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> pipelineWithStatus
                                    BuildStatusErrored
                                    False
                                |> findBanner
                                |> isSolid amber
                    , test "is amber with black stripes when pipeline is erroring and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> pipelineWithStatus
                                    BuildStatusErrored
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = amber, thick = darkGrey }
                    , test "is brown when pipeline is aborted" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> pipelineWithStatus
                                    BuildStatusAborted
                                    False
                                |> findBanner
                                |> isSolid brown
                    , test "is brown with black stripes when pipeline is aborted and running" <|
                        \_ ->
                            whenOnDashboard { highDensity = True }
                                |> pipelineWithStatus
                                    BuildStatusAborted
                                    True
                                |> findBanner
                                |> isColorWithStripes { thin = brown, thick = darkGrey }
                    , describe "status priorities" <|
                        let
                            givenTwoJobs :
                                BuildStatus
                                -> BuildStatus
                                -> Query.Single ApplicationMsgs.TopLevelMessage
                            givenTwoJobs firstStatus secondStatus =
                                whenOnDashboard { highDensity = False }
                                    |> Application.handleCallback
                                        (Callback.AllJobsFetched <|
                                            Ok
                                                [ job firstStatus
                                                , otherJob secondStatus
                                                ]
                                        )
                                    |> Tuple.first
                                    |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                                    |> Tuple.first
                                    |> Application.handleCallback
                                        (Callback.AllPipelinesFetched <|
                                            Ok
                                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                        )
                                    |> Tuple.first
                                    |> Common.queryView
                        in
                        [ test "failed is more important than errored" <|
                            \_ ->
                                givenTwoJobs
                                    BuildStatusFailed
                                    BuildStatusErrored
                                    |> findBanner
                                    |> isSolid red
                        , test "errored is more important than aborted" <|
                            \_ ->
                                givenTwoJobs
                                    BuildStatusErrored
                                    BuildStatusAborted
                                    |> findBanner
                                    |> isSolid amber
                        , test "aborted is more important than succeeding" <|
                            \_ ->
                                givenTwoJobs
                                    BuildStatusAborted
                                    BuildStatusSucceeded
                                    |> findBanner
                                    |> isSolid brown
                        , test "succeeding is more important than pending" <|
                            \_ ->
                                givenTwoJobs
                                    BuildStatusSucceeded
                                    BuildStatusPending
                                    |> findBanner
                                    |> isSolid green
                        ]
                    ]
                ]
            ]
        , describe "on HD view" <|
            let
                setup : () -> Query.Single ApplicationMsgs.TopLevelMessage
                setup _ =
                    whenOnDashboard { highDensity = True }
                        |> givenDataUnauthenticated
                            (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView

                noPipelines : () -> Query.Single ApplicationMsgs.TopLevelMessage
                noPipelines _ =
                    whenOnDashboard { highDensity = True }
                        |> givenDataUnauthenticated
                            (apiData
                                [ ( "some-team", [] )
                                , ( "other-team", [] )
                                ]
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "other-team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView

                card : Query.Single ApplicationMsgs.TopLevelMessage -> Query.Single ApplicationMsgs.TopLevelMessage
                card =
                    Query.find
                        [ class "card"
                        , containing [ text "pipeline" ]
                        ]

                cardText : Query.Single ApplicationMsgs.TopLevelMessage -> Query.Single ApplicationMsgs.TopLevelMessage
                cardText =
                    card
                        >> Query.children []
                        >> Query.index 1

                noPipelinesCard =
                    Query.find
                        [ class "card"
                        , containing [ text "no pipeline" ]
                        ]
            in
            [ test "no pipelines card has 14px font and 1px spacing" <|
                noPipelines
                    >> noPipelinesCard
                    >> Query.has
                        [ style "font-size" "14px"
                        , style "letter-spacing" "1px"
                        ]
            , test "no pipelines card text is vertically centered" <|
                noPipelines
                    >> noPipelinesCard
                    >> Query.has
                        [ style "display" "flex"
                        , style "align-items" "center"
                        ]
            , test "no pipelines card is 60px tall" <|
                noPipelines
                    >> noPipelinesCard
                    >> Query.has [ style "height" "60px" ]
            , test "no pipelines card has 60px right margin" <|
                noPipelines
                    >> noPipelinesCard
                    >> Query.has [ style "margin-right" "60px" ]
            , test "no pipelines card text has 10px padding" <|
                noPipelines
                    >> noPipelinesCard
                    >> Query.children []
                    >> Query.first
                    >> Query.has [ style "padding" "10px" ]
            , test "no pipelines card is 200px wide" <|
                noPipelines
                    >> noPipelinesCard
                    >> Query.has [ style "width" "200px" ]
            , test "no pipelines card has dark grey background" <|
                noPipelines
                    >> noPipelinesCard
                    >> Query.has [ style "background-color" darkGrey ]
            , test "card has larger tighter font" <|
                setup
                    >> card
                    >> Query.has
                        [ style "font-size" "19px"
                        , style "letter-spacing" "1px"
                        ]
            , test "card text does not overflow or wrap" <|
                setup
                    >> cardText
                    >> Query.has
                        [ style "width" "180px"
                        , style "white-space" "nowrap"
                        , style "overflow" "hidden"
                        , style "text-overflow" "ellipsis"
                        ]
            , test "card text is vertically centered" <|
                setup
                    >> cardText
                    >> Query.has
                        [ style "align-self" "center" ]
            , test "card text has 10px padding" <|
                setup
                    >> cardText
                    >> Query.has
                        [ style "padding" "10px" ]
            , test "card lays out contents horizontally" <|
                setup
                    >> card
                    >> Query.has
                        [ style "display" "flex" ]
            , test "card is 60px tall" <|
                setup
                    >> card
                    >> Query.has [ style "height" "60px" ]
            , test "card is 200px wide" <|
                setup
                    >> card
                    >> Query.has [ style "width" "200px" ]
            , test "no triangle when there is no resource error" <|
                setup
                    >> card
                    >> Query.children []
                    >> Query.count (Expect.equal 2)
            , describe "resource error triangle" <|
                let
                    givenResourceFailingToCheck : () -> Query.Single ApplicationMsgs.TopLevelMessage
                    givenResourceFailingToCheck _ =
                        whenOnDashboard { highDensity = True }
                            |> Application.handleCallback
                                (Callback.AllResourcesFetched <|
                                    Ok
                                        [ { teamName = "team"
                                          , pipelineName = "pipeline"
                                          , name = "resource"
                                          , failingToCheck = True
                                          , checkError = ""
                                          , checkSetupError = ""
                                          , lastChecked = Nothing
                                          , pinnedVersion = Nothing
                                          , pinnedInConfig = False
                                          , pinComment = Nothing
                                          , icon = Nothing
                                          }
                                        ]
                                )
                            |> Tuple.first
                            |> givenDataUnauthenticated [ { id = 0, name = "team" } ]
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView

                    resourceErrorTriangle =
                        Query.children []
                            >> Query.index -1
                in
                [ test "exists" <|
                    givenResourceFailingToCheck
                        >> card
                        >> Query.children []
                        >> Query.count (Expect.equal 3)
                , test "is at the top right of card" <|
                    givenResourceFailingToCheck
                        >> card
                        >> Expect.all
                            [ Query.has [ style "position" "relative" ]
                            , resourceErrorTriangle
                                >> Query.has
                                    [ style "position" "absolute"
                                    , style "top" "0"
                                    , style "right" "0"
                                    ]
                            ]
                , test "is an orange 'top right' triangle" <|
                    givenResourceFailingToCheck
                        >> card
                        >> resourceErrorTriangle
                        >> Query.has
                            [ style "width" "0"
                            , style "height" "0"
                            , style "border-top" <| "30px solid " ++ orange
                            , style "border-left" "30px solid transparent"
                            ]
                ]
            , test
                ("cards are spaced 4px apart vertically and "
                    ++ "60px apart horizontally"
                )
              <|
                setup
                    >> card
                    >> Query.has
                        [ style "margin" "0 60px 4px 0" ]
            , test "card is faded green when pipeline is succeeding" <|
                \_ ->
                    whenOnDashboard { highDensity = True }
                        |> pipelineWithStatus
                            BuildStatusSucceeded
                            False
                        |> card
                        |> Query.has [ style "background-color" fadedGreen ]
            , test "card is red when pipeline is failing" <|
                \_ ->
                    whenOnDashboard { highDensity = True }
                        |> pipelineWithStatus
                            BuildStatusFailed
                            False
                        |> card
                        |> Query.has [ style "background-color" red ]
            , test "card is amber when pipeline is erroring" <|
                \_ ->
                    whenOnDashboard { highDensity = True }
                        |> pipelineWithStatus
                            BuildStatusErrored
                            False
                        |> card
                        |> Query.has [ style "background-color" amber ]
            , test "card is not affected by jobs from other pipelines" <|
                \_ ->
                    whenOnDashboard { highDensity = True }
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllJobsFetched <|
                                let
                                    baseJob =
                                        job BuildStatusErrored
                                in
                                Ok
                                    [ { baseJob | pipelineName = "other-pipeline" } ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> card
                        |> Query.hasNot [ style "background-color" amber ]
            ]
        , describe "body" <|
            let
                setup : () -> Query.Single ApplicationMsgs.TopLevelMessage
                setup _ =
                    whenOnDashboard { highDensity = False }
                        |> givenDataUnauthenticated
                            (apiData [ ( "team", [] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find
                            [ class "card"
                            , containing [ text "pipeline" ]
                            ]
            in
            [ test "has dark grey background" <|
                setup
                    >> findBody
                    >> Query.has [ style "background-color" darkGrey ]
            , test "has 2px margin above and below" <|
                setup
                    >> findBody
                    >> Query.has [ style "margin" "2px 0" ]
            , test "fills available height" <|
                setup
                    >> findBody
                    >> Query.has [ style "flex-grow" "1" ]
            , test "allows pipeline-grid to fill available height" <|
                setup
                    >> findBody
                    >> Query.has [ style "display" "flex" ]
            , test "pipeline-grid fills available width" <|
                setup
                    >> findBody
                    >> Query.find [ class "pipeline-grid" ]
                    >> Query.has
                        [ style "box-sizing" "border-box"
                        , style "width" "100%"
                        ]
            ]
        , describe "footer" <|
            let
                hasStyle : String -> String -> Expectation
                hasStyle property value =
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "card-footer" ]
                        |> Query.has [ style property value ]
            in
            [ test "has dark grey background" <|
                \_ ->
                    hasStyle "background-color" darkGrey
            , test "has medium padding" <|
                \_ ->
                    hasStyle "padding" "13.5px"
            , test "lays out contents horizontally" <|
                \_ ->
                    hasStyle "display" "flex"
            , test "is divided into a left and right section, spread apart" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "card-footer" ]
                        |> Expect.all
                            [ Query.children []
                                >> Query.count (Expect.equal 2)
                            , Query.has
                                [ style "justify-content" "space-between" ]
                            ]
            , test "both sections lay out contents horizontally" <|
                \_ ->
                    whenOnDashboard { highDensity = False }
                        |> givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles [ ( "team", [ "owner" ] ) ])
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.AllPipelinesFetched <|
                                Ok
                                    [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                            )
                        |> Tuple.first
                        |> Common.queryView
                        |> Query.find [ class "card-footer" ]
                        |> Query.children []
                        |> Query.each (Query.has [ style "display" "flex" ])
            , describe "left-hand section" <|
                let
                    findStatusIcon =
                        Query.find [ class "pipeline-status" ]
                            >> Query.children []
                            >> Query.first

                    findStatusText =
                        Query.find [ class "pipeline-status" ]
                            >> Query.children []
                            >> Query.index -1
                in
                [ describe "when pipeline is paused" <|
                    let
                        setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    [ { id = 0, name = "team" } ]
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0
                                                |> Data.withName "pipeline"
                                                |> Data.withPaused True
                                            ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                    in
                    [ test "status icon is blue pause" <|
                        \_ ->
                            setup
                                |> findStatusIcon
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = PipelineStatusPaused |> Assets.PipelineStatusIcon
                                        }
                                        ++ [ style "background-size" "contain" ]
                                    )
                    , test "status text is blue" <|
                        \_ ->
                            setup
                                |> findStatusText
                                |> Query.has [ style "color" blue ]
                    , test "status text is larger and spaced more widely" <|
                        \_ ->
                            setup
                                |> findStatusText
                                |> Query.has
                                    [ style "font-size" "18px"
                                    , style "line-height" "20px"
                                    , style "letter-spacing" "0.05em"
                                    ]
                    , test "status text is offset to the right of the icon" <|
                        \_ ->
                            setup
                                |> findStatusText
                                |> Query.has [ style "margin-left" "8px" ]
                    , test "status text says 'paused'" <|
                        \_ ->
                            setup
                                |> findStatusText
                                |> Query.has [ text "paused" ]
                    ]
                , describe "when a pipeline is based on cache" <|
                    let
                        setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    [ { id = 0, name = "team" } ]
                                |> Tuple.first
                                |> Application.handleDelivery
                                    (CachedPipelinesReceived <|
                                        Ok
                                            [ Data.pipeline "team" 0
                                                |> Data.withName "pipeline"
                                                |> Data.withPaused True
                                            ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                    in
                    [ test "status icon is grey pending" <|
                        \_ ->
                            setup
                                |> findStatusIcon
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = Assets.PipelineStatusIconStale
                                        }
                                        ++ [ style "background-size" "contain" ]
                                    )
                    , test "status text is grey" <|
                        \_ ->
                            setup
                                |> findStatusText
                                |> Query.has [ style "color" lightGrey ]
                    , test "status text says 'loading...'" <|
                        \_ ->
                            setup
                                |> findStatusText
                                |> Query.has [ text "loading..." ]
                    , test "pipeline card is faded" <|
                        \_ ->
                            setup
                                |> Query.find [ class "card" ]
                                |> Query.has [ style "opacity" "0.45" ]
                    ]
                , describe "when pipeline has no jobs due to a disabled endpoint" <|
                    let
                        setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    [ { id = 0, name = "team" } ]
                                |> Tuple.first
                                |> Application.handleDelivery
                                    (CachedJobsReceived <| Ok [ Data.job 0 ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 ]
                                    )
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllJobsFetched <|
                                        Data.httpNotImplemented
                                    )

                        domID =
                            Msgs.PipelineStatusIcon
                                { teamName = "team"
                                , pipelineName = "pipeline-0"
                                }
                    in
                    [ test "status icon is faded sync" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Common.queryView
                                |> findStatusIcon
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = Assets.PipelineStatusIconJobsDisabled
                                        }
                                        ++ [ style "background-size" "contain"
                                           , style "opacity" "0.5"
                                           ]
                                    )
                    , test "status icon is hoverable" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Common.queryView
                                |> findStatusIcon
                                |> Event.simulate Event.mouseEnter
                                |> Event.expect
                                    (ApplicationMsgs.Update <|
                                        Msgs.Hover <|
                                            Just domID
                                    )
                    , test "hovering status icon sends location request" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.Hover <|
                                            Just domID
                                    )
                                |> Tuple.second
                                |> Common.contains
                                    (Effects.GetViewportOf domID)
                    , test "hovering status icon shows tooltip" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.Hover <|
                                            Just domID
                                    )
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.GotViewport domID
                                        (Ok
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
                                    )
                                |> Tuple.first
                                |> Application.handleCallback
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
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.has
                                    [ style "position" "fixed"
                                    , containing [ text "automatic job monitoring disabled" ]
                                    ]
                    , test "status text says 'no data'" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Common.queryView
                                |> findStatusText
                                |> Query.has [ text "no data" ]
                    , test "job preview is empty placeholder" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "card-body" ]
                                |> Query.has [ style "background-color" middleGrey ]
                    , test "job data is cleared" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "parallel-grid" ]
                                |> Query.hasNot [ tag "a" ]
                    , test "job data is cleared from local cache" <|
                        \_ ->
                            setup
                                |> Tuple.second
                                |> Common.contains Effects.DeleteCachedJobs
                    ]
                , describe "when pipeline is archived" <|
                    let
                        setup =
                            whenOnDashboardViewingAllPipelines { highDensity = False }
                                |> givenDataUnauthenticated
                                    [ { id = 0, name = "team" } ]
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0
                                                |> Data.withArchived True
                                            ]
                                    )
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllJobsFetched <|
                                        Ok
                                            [ Data.job 0 ]
                                    )
                    in
                    [ test "status section is empty" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "pipeline-status" ]
                                |> Query.children []
                                |> Query.count (Expect.equal 0)
                    , test "job preview is empty placeholder" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "card-body" ]
                                |> Query.has [ style "background-color" middleGrey ]
                    , test "there is no pause button" <|
                        \_ ->
                            setup
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.hasNot [ class "pause-toggle" ]
                    ]
                , describe "when pipeline is pending" <|
                    [ test "status icon is grey" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    BuildStatusPending
                                    False
                                |> findStatusIcon
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = PipelineStatusPending True |> Assets.PipelineStatusIcon
                                        }
                                        ++ [ style "background-size" "contain" ]
                                    )
                    , test "status text is grey" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    BuildStatusPending
                                    False
                                |> findStatusText
                                |> Query.has [ style "color" lightGrey ]
                    , test "status text says 'pending'" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    BuildStatusPending
                                    False
                                |> findStatusText
                                |> Query.has
                                    [ text "pending" ]
                    , test "when running, status text says 'running'" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    BuildStatusPending
                                    True
                                |> findStatusText
                                |> Query.has
                                    [ text "running" ]
                    ]
                , describe "when pipeline is succeeding"
                    [ test "status icon is a green check" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    BuildStatusSucceeded
                                    False
                                |> findStatusIcon
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = PipelineStatusSucceeded Running |> Assets.PipelineStatusIcon
                                        }
                                        ++ [ style "background-size" "contain" ]
                                    )
                    , test "status text is green" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    BuildStatusSucceeded
                                    False
                                |> findStatusText
                                |> Query.has [ style "color" green ]
                    , test "when running, status text says 'running'" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    BuildStatusSucceeded
                                    True
                                |> findStatusText
                                |> Query.has
                                    [ text "running" ]
                    , test "when not running, status text shows age" <|
                        \_ ->
                            Common.init "/"
                                |> Application.handleCallback
                                    (Callback.AllJobsFetched <|
                                        Ok
                                            [ jobWithNameTransitionedAt
                                                "job"
                                                (Just <| Time.millisToPosix 0)
                                                BuildStatusSucceeded
                                            ]
                                    )
                                |> Tuple.first
                                |> givenDataUnauthenticated
                                    [ { id = 0, name = "team" } ]
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> afterSeconds 1
                                |> Common.queryView
                                |> findStatusText
                                |> Query.has
                                    [ text "1s" ]
                    ]
                , describe "when pipeline is failing"
                    [ test "status icon is a red !" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    BuildStatusFailed
                                    False
                                |> findStatusIcon
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = PipelineStatusFailed Running |> Assets.PipelineStatusIcon
                                        }
                                        ++ [ style "background-size" "contain" ]
                                    )
                    , test "status text is red" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> pipelineWithStatus
                                    BuildStatusFailed
                                    False
                                |> findStatusText
                                |> Query.has [ style "color" red ]
                    ]
                , test "when pipeline is aborted, status icon is a brown x" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusAborted
                                False
                            |> findStatusIcon
                            |> Query.has
                                (iconSelector
                                    { size = "20px"
                                    , image = PipelineStatusAborted Running |> Assets.PipelineStatusIcon
                                    }
                                    ++ [ style "background-size" "contain" ]
                                )
                , test "when pipeline is errored, status icon is an amber triangle" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> pipelineWithStatus
                                BuildStatusErrored
                                False
                            |> findStatusIcon
                            |> Query.has
                                (iconSelector
                                    { size = "20px"
                                    , image = PipelineStatusErrored Running |> Assets.PipelineStatusIcon
                                    }
                                    ++ [ style "background-size" "contain" ]
                                )
                ]
            , describe "right-hand section"
                [ describe "visibility toggle" <|
                    let
                        pipelineId =
                            { pipelineName = "pipeline"
                            , teamName = "team"
                            }

                        visibilityToggle =
                            Common.queryView
                                >> Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.index -1
                                >> Query.children []
                                >> Query.index 2

                        openEye =
                            iconSelector
                                { size = "20px"
                                , image = Assets.VisibilityToggleIcon True
                                }
                                ++ [ style "background-size" "contain" ]

                        slashedOutEye =
                            iconSelector
                                { size = "20px"
                                , image = Assets.VisibilityToggleIcon False
                                }
                                ++ [ style "background-size" "contain" ]

                        openEyeClickable setup =
                            [ defineHoverBehaviour
                                { name = "open eye toggle"
                                , setup =
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                , query = visibilityToggle
                                , unhoveredSelector =
                                    { description = "faded 20px square"
                                    , selector =
                                        openEye
                                            ++ [ style "opacity" "0.5"
                                               , style "cursor" "pointer"
                                               ]
                                    }
                                , hoverable =
                                    Msgs.VisibilityButton pipelineId
                                , hoveredSelector =
                                    { description = "bright 20px square"
                                    , selector =
                                        openEye
                                            ++ [ style "opacity" "1"
                                               , style "cursor" "pointer"
                                               ]
                                    }
                                }
                            , test "has click handler" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> visibilityToggle
                                        |> Event.simulate Event.click
                                        |> Event.expect
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                            , test "click has HidePipeline effect" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ Effects.ChangeVisibility
                                                Msgs.Hide
                                                pipelineId
                                            ]
                            , defineHoverBehaviour
                                { name = "visibility spinner"
                                , setup =
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                                        |> Tuple.first
                                , query = visibilityToggle
                                , unhoveredSelector =
                                    { description = "20px spinner"
                                    , selector =
                                        [ style "animation"
                                            "container-rotate 1568ms linear infinite"
                                        , style "height" "20px"
                                        , style "width" "20px"
                                        ]
                                    }
                                , hoverable =
                                    Msgs.VisibilityButton pipelineId
                                , hoveredSelector =
                                    { description = "20px spinner"
                                    , selector =
                                        [ style "animation"
                                            "container-rotate 1568ms linear infinite"
                                        , style "height" "20px"
                                        , style "width" "20px"
                                        ]
                                    }
                                }
                            , test "success resolves spinner to slashed-out eye" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                                        |> Tuple.first
                                        |> Application.handleCallback
                                            (Callback.VisibilityChanged
                                                Msgs.Hide
                                                pipelineId
                                                (Ok ())
                                            )
                                        |> Tuple.first
                                        |> visibilityToggle
                                        |> Query.has slashedOutEye
                            , test "error resolves spinner to open eye" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                                        |> Tuple.first
                                        |> Application.handleCallback
                                            (Callback.VisibilityChanged
                                                Msgs.Hide
                                                pipelineId
                                                Data.httpInternalServerError
                                            )
                                        |> Tuple.first
                                        |> visibilityToggle
                                        |> Query.has openEye
                            , test "401 redirects to login" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                                        |> Tuple.first
                                        |> Application.handleCallback
                                            (Callback.VisibilityChanged
                                                Msgs.Hide
                                                pipelineId
                                                Data.httpUnauthorized
                                            )
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ Effects.RedirectToLogin ]
                            ]

                        openEyeUnclickable setup =
                            [ defineHoverBehaviour
                                { name = "open eye toggle"
                                , setup =
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                , query = visibilityToggle
                                , unhoveredSelector =
                                    { description = "faded 20px square"
                                    , selector =
                                        openEye
                                            ++ [ style "opacity" "0.5"
                                               , style "cursor" "default"
                                               ]
                                    }
                                , hoverable =
                                    Msgs.VisibilityButton pipelineId
                                , hoveredSelector =
                                    { description = "faded 20px square"
                                    , selector =
                                        openEye
                                            ++ [ style "opacity" "0.5"
                                               , style "cursor" "default"
                                               ]
                                    }
                                }
                            , test "has no click handler" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> visibilityToggle
                                        |> Event.simulate Event.click
                                        |> Event.toResult
                                        |> Expect.err
                            ]

                        slashedOutEyeClickable setup =
                            [ defineHoverBehaviour
                                { name = "slashed-out eye toggle"
                                , setup =
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                , query = visibilityToggle
                                , unhoveredSelector =
                                    { description = "faded 20px square"
                                    , selector =
                                        slashedOutEye
                                            ++ [ style "opacity" "0.5"
                                               , style "cursor" "pointer"
                                               ]
                                    }
                                , hoverable =
                                    Msgs.VisibilityButton pipelineId
                                , hoveredSelector =
                                    { description = "bright 20px square"
                                    , selector =
                                        slashedOutEye
                                            ++ [ style "opacity" "1"
                                               , style "cursor" "pointer"
                                               ]
                                    }
                                }
                            , test "has click handler" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> visibilityToggle
                                        |> Event.simulate Event.click
                                        |> Event.expect
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                            , test "click has ExposePipeline effect" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ Effects.ChangeVisibility
                                                Msgs.Expose
                                                pipelineId
                                            ]
                            , defineHoverBehaviour
                                { name = "visibility spinner"
                                , setup =
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                                        |> Tuple.first
                                , query = visibilityToggle
                                , unhoveredSelector =
                                    { description = "20px spinner"
                                    , selector =
                                        [ style "animation"
                                            "container-rotate 1568ms linear infinite"
                                        , style "height" "20px"
                                        , style "width" "20px"
                                        ]
                                    }
                                , hoverable =
                                    Msgs.VisibilityButton pipelineId
                                , hoveredSelector =
                                    { description = "20px spinner"
                                    , selector =
                                        [ style "animation"
                                            "container-rotate 1568ms linear infinite"
                                        , style "height" "20px"
                                        , style "width" "20px"
                                        ]
                                    }
                                }
                            , test "success resolves spinner to open eye" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                                        |> Tuple.first
                                        |> Application.handleCallback
                                            (Callback.VisibilityChanged
                                                Msgs.Expose
                                                pipelineId
                                                (Ok ())
                                            )
                                        |> Tuple.first
                                        |> visibilityToggle
                                        |> Query.has openEye
                            , test "error resolves spinner to slashed-out eye" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.VisibilityButton
                                                        pipelineId
                                            )
                                        |> Tuple.first
                                        |> Application.handleCallback
                                            (Callback.VisibilityChanged
                                                Msgs.Expose
                                                pipelineId
                                                Data.httpInternalServerError
                                            )
                                        |> Tuple.first
                                        |> visibilityToggle
                                        |> Query.has slashedOutEye
                            ]

                        slashedOutEyeUnclickable setup =
                            [ defineHoverBehaviour
                                { name = "slashed-out eye toggle"
                                , setup =
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                , query = visibilityToggle
                                , unhoveredSelector =
                                    { description = "faded 20px square"
                                    , selector =
                                        slashedOutEye
                                            ++ [ style "opacity" "0.5"
                                               , style "cursor" "default"
                                               ]
                                    }
                                , hoverable =
                                    Msgs.VisibilityButton pipelineId
                                , hoveredSelector =
                                    { description = "faded 20px square"
                                    , selector =
                                        slashedOutEye
                                            ++ [ style "opacity" "0.5"
                                               , style "cursor" "default"
                                               ]
                                    }
                                }
                            , test "has no click handler" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> visibilityToggle
                                        |> Event.simulate Event.click
                                        |> Event.toResult
                                        |> Expect.err
                            ]
                    in
                    [ describe "when authorized" <|
                        let
                            whenAuthorizedPublic =
                                givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles
                                        [ ( "team", [ "owner" ] ) ]
                                    )
                                    >> Tuple.first
                                    >> Application.handleCallback
                                        (Callback.AllPipelinesFetched <|
                                            Ok
                                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                        )

                            whenAuthorizedNonPublic =
                                givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles
                                        [ ( "team", [ "owner" ] ) ]
                                    )
                                    >> Tuple.first
                                    >> Application.handleCallback
                                        (Callback.AllPipelinesFetched <|
                                            Ok
                                                [ Data.pipeline "team" 0
                                                    |> Data.withName "pipeline"
                                                    |> Data.withPublic False
                                                ]
                                        )
                        in
                        [ describe "on public pipeline" <|
                            openEyeClickable whenAuthorizedPublic
                        , describe "on a non-public pipeline" <|
                            slashedOutEyeClickable whenAuthorizedNonPublic
                        ]
                    , describe "when unauthorized" <|
                        let
                            whenUnauthorizedPublic =
                                givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles
                                        [ ( "team", [ "viewer" ] ) ]
                                    )
                                    >> Tuple.first
                                    >> Application.handleCallback
                                        (Callback.AllPipelinesFetched <|
                                            Ok
                                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                        )

                            whenUnauthorizedNonPublic =
                                givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles
                                        [ ( "team", [ "viewer" ] ) ]
                                    )
                                    >> Tuple.first
                                    >> Application.handleCallback
                                        (Callback.AllPipelinesFetched <|
                                            Ok
                                                [ Data.pipeline "team" 0
                                                    |> Data.withName "pipeline"
                                                    |> Data.withPublic False
                                                ]
                                        )
                        in
                        [ describe "on public pipeline" <|
                            openEyeUnclickable whenUnauthorizedPublic
                        , describe "on a non-public pipeline" <|
                            slashedOutEyeUnclickable
                                whenUnauthorizedNonPublic
                        ]
                    , describe "when unauthenticated" <|
                        let
                            whenUnauthenticated =
                                givenDataUnauthenticated
                                    (apiData [ ( "team", [] ) ])
                                    >> Tuple.first
                                    >> Application.handleCallback
                                        (Callback.AllPipelinesFetched <|
                                            Ok
                                                [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                        )
                        in
                        [ describe "on public pipeline" <|
                            openEyeClickable whenUnauthenticated
                        ]
                    ]
                , test "there is medium spacing between the eye and the play/pause button" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataAndUser
                                (apiData [ ( "team", [] ) ])
                                (userWithRoles [ ( "team", [ "owner" ] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ class "card-footer" ]
                            |> Query.children []
                            |> Query.index -1
                            |> Query.children []
                            |> Expect.all
                                [ Query.count (Expect.equal 5)
                                , Query.index 1 >> Query.has [ style "width" "13.5px" ]
                                ]
                , test "there is medium spacing between the eye and the favorited icon" <|
                    \_ ->
                        whenOnDashboard { highDensity = False }
                            |> givenDataAndUser
                                (apiData [ ( "team", [] ) ])
                                (userWithRoles [ ( "team", [ "owner" ] ) ])
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                            |> Tuple.first
                            |> Common.queryView
                            |> Query.find [ class "card-footer" ]
                            |> Query.children []
                            |> Query.index -1
                            |> Query.children []
                            |> Expect.all
                                [ Query.count (Expect.equal 5)
                                , Query.index 3 >> Query.has [ style "width" "13.5px" ]
                                ]
                , describe "pause toggle"
                    [ test "the right section has a 20px square pause button on the left" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.children []
                                |> Query.index -1
                                |> Query.children []
                                |> Query.index 0
                                |> Query.has
                                    (iconSelector
                                        { size = "20px"
                                        , image = Assets.PauseIcon
                                        }
                                    )
                    , test "pause button has pointer cursor when authorized" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.find
                                    (iconSelector
                                        { size = "20px"
                                        , image = Assets.PauseIcon
                                        }
                                    )
                                |> Query.has [ style "cursor" "pointer" ]
                    , test "pause button is transparent" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.find
                                    (iconSelector
                                        { size = "20px"
                                        , image = Assets.PauseIcon
                                        }
                                    )
                                |> Query.has [ style "opacity" "0.5" ]
                    , defineHoverBehaviour
                        { name = "pause button"
                        , setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                        , query =
                            Common.queryView
                                >> Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.index -1
                                >> Query.children []
                                >> Query.index 0
                        , unhoveredSelector =
                            { description = "a faded 20px square pause button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = Assets.PauseIcon
                                    }
                                    ++ [ style "cursor" "pointer"
                                       , style "opacity" "0.5"
                                       ]
                            }
                        , hoverable =
                            Msgs.PipelineButton
                                { pipelineName = "pipeline"
                                , teamName = "team"
                                }
                        , hoveredSelector =
                            { description = "a bright 20px square pause button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = Assets.PauseIcon
                                    }
                                    ++ [ style "cursor" "pointer"
                                       , style "opacity" "1"
                                       ]
                            }
                        }
                    , defineHoverBehaviour
                        { name = "play button"
                        , setup =
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0
                                                |> Data.withName "pipeline"
                                                |> Data.withPaused True
                                            ]
                                    )
                                |> Tuple.first
                        , query =
                            Common.queryView
                                >> Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.index -1
                                >> Query.children []
                                >> Query.index 0
                        , unhoveredSelector =
                            { description = "a transparent 20px square play button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = Assets.PlayIcon
                                    }
                                    ++ [ style "cursor" "pointer"
                                       , style "opacity" "0.5"
                                       ]
                            }
                        , hoverable =
                            Msgs.PipelineButton
                                { pipelineName = "pipeline"
                                , teamName = "team"
                                }
                        , hoveredSelector =
                            { description = "an opaque 20px square play button with pointer cursor"
                            , selector =
                                iconSelector
                                    { size = "20px"
                                    , image = Assets.PlayIcon
                                    }
                                    ++ [ style "cursor" "pointer"
                                       , style "opacity" "1"
                                       ]
                            }
                        }
                    , test "clicking pause button sends TogglePipeline msg" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.find [ class "pause-toggle" ]
                                |> Event.simulate Event.click
                                |> Event.expect
                                    (ApplicationMsgs.Update <|
                                        Msgs.Click <|
                                            Msgs.PipelineButton
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                    )
                    , test "pause button turns into spinner on click" <|
                        \_ ->
                            let
                                animation =
                                    "container-rotate 1568ms linear infinite"
                            in
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.Click <|
                                            Msgs.PipelineButton
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                    )
                                |> Tuple.first
                                |> Common.queryView
                                |> Query.find [ class "card-footer" ]
                                |> Query.has [ style "animation" animation ]
                    , test "clicking pause button sends toggle api call" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.Click <|
                                            Msgs.PipelineButton
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                    )
                                |> Tuple.second
                                |> Expect.equal
                                    [ Effects.SendTogglePipelineRequest
                                        { pipelineName = "pipeline"
                                        , teamName = "team"
                                        }
                                        False
                                    ]
                    , test "all pipelines are refetched after ok toggle call" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataAndUser
                                    (apiData [ ( "team", [] ) ])
                                    (userWithRoles [ ( "team", [ "owner" ] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.Click <|
                                            Msgs.PipelineButton
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                    )
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.PipelineToggled
                                        { pipelineName = "pipeline"
                                        , teamName = "team"
                                        }
                                        (Ok ())
                                    )
                                |> Tuple.second
                                |> Expect.equal [ Effects.FetchAllPipelines ]
                    , test "401 toggle call redirects to login" <|
                        \_ ->
                            whenOnDashboard { highDensity = False }
                                |> givenDataUnauthenticated
                                    (apiData [ ( "team", [] ) ])
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.AllPipelinesFetched <|
                                        Ok
                                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                    )
                                |> Tuple.first
                                |> Application.update
                                    (ApplicationMsgs.Update <|
                                        Msgs.Click <|
                                            Msgs.PipelineButton
                                                { pipelineName = "pipeline"
                                                , teamName = "team"
                                                }
                                    )
                                |> Tuple.first
                                |> Application.handleCallback
                                    (Callback.PipelineToggled
                                        { pipelineName = "pipeline"
                                        , teamName = "team"
                                        }
                                        Data.httpUnauthorized
                                    )
                                |> Tuple.second
                                |> Expect.equal [ Effects.RedirectToLogin ]
                    ]
                , describe "favorited icon" <|
                    let
                        pipelineId =
                            0

                        unfilledFavoritedIcon =
                            iconSelector
                                { size = "20px"
                                , image = Assets.StarIconUnfilled
                                }
                                ++ [ style "background-size" "contain" ]

                        filledFavoritedIcon =
                            iconSelector
                                { size = "20px"
                                , image = Assets.StarIconFilled
                                }
                                ++ [ style "background-size" "contain" ]

                        favoritedToggle =
                            Common.queryView
                                >> Query.find [ class "card-footer" ]
                                >> Query.children []
                                >> Query.index -1
                                >> Query.children []
                                >> Query.index -1

                        favoritedIconClickable setup =
                            [ defineHoverBehaviour
                                { name = "favorited icon toggle"
                                , setup =
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                , query = favoritedToggle
                                , unhoveredSelector =
                                    { description = "faded 20px square"
                                    , selector =
                                        unfilledFavoritedIcon
                                            ++ [ style "opacity" "0.5"
                                               , style "cursor" "pointer"
                                               ]
                                    }
                                , hoverable =
                                    Msgs.PipelineCardFavoritedIcon pipelineId
                                , hoveredSelector =
                                    { description = "bright 20px square"
                                    , selector =
                                        unfilledFavoritedIcon
                                            ++ [ style "opacity" "1"
                                               , style "cursor" "pointer"
                                               ]
                                    }
                                }
                            , test "has click handler" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> favoritedToggle
                                        |> Event.simulate Event.click
                                        |> Event.expect
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.PipelineCardFavoritedIcon
                                                        pipelineId
                                            )
                            , test "click has FavoritedPipeline effect" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.update
                                            (ApplicationMsgs.Update <|
                                                Msgs.Click <|
                                                    Msgs.PipelineCardFavoritedIcon
                                                        pipelineId
                                            )
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ Effects.SaveFavoritedPipelines <|
                                                Set.singleton pipelineId
                                            ]
                            , test "favorited pipeline card has a bright filled star icon" <|
                                \_ ->
                                    whenOnDashboard { highDensity = False }
                                        |> setup
                                        |> Tuple.first
                                        |> Application.handleDelivery
                                            (FavoritedPipelinesReceived <|
                                                Ok <|
                                                    Set.singleton pipelineId
                                            )
                                        |> Tuple.first
                                        |> favoritedToggle
                                        |> Expect.all
                                            [ Query.has filledFavoritedIcon
                                            , Query.has [ style "opacity" "1" ]
                                            ]
                            ]
                    in
                    favoritedIconClickable
                        (givenDataAndUser
                            (apiData [ ( "team", [] ) ])
                            (userWithRoles
                                [ ( "team", [ "owner" ] ) ]
                            )
                            >> Tuple.first
                            >> Application.handleCallback
                                (Callback.AllPipelinesFetched <|
                                    Ok
                                        [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                                )
                        )
                ]
            ]
        ]
