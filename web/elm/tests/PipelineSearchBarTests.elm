module PipelineSearchBarTests exposing (all)

import Application.Application as Application
import Common
    exposing
        ( queryView
        , whenOnDesktop
        , whenOnMobile
        )
import Concourse.BuildStatus exposing (BuildStatus(..))
import Data
import Expect
import Html.Attributes as Attr
import Message.Message exposing (DomID(..), Message(..))
import Message.TopLevelMessage as Msgs
import Pipeline.Filter as Filter
import Pipeline.SearchBar as SearchBar
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector as Selector
    exposing
        ( attribute
        , containing
        , id
        , style
        , text
        )


all : Test
all =
    describe "pipeline search & filter" <|
        [ describe "Pipeline.Filter" <|
            [ describe "filterJobs" <|
                let
                    jobNamed name =
                        let
                            base =
                                Data.job 0
                        in
                        { base | name = name }

                    jobWithFinishedStatus name status =
                        Data.job 0
                            |> (\j -> { j | name = name })
                            |> Data.withFinishedBuild (Just (Data.jobBuild status))

                    pausedJob name =
                        Data.job 0
                            |> (\j -> { j | name = name, paused = True })

                    jobs =
                        [ jobNamed "alpha"
                        , pausedJob "paused-job"
                        , jobWithFinishedStatus "failed-job" BuildStatusFailed
                        , jobWithFinishedStatus "running-job" BuildStatusStarted
                        ]
                in
                [ test "empty query keeps all jobs" <|
                    \_ ->
                        Filter.filterJobs "" jobs
                            |> List.map .name
                            |> Expect.equal [ "alpha", "paused-job", "failed-job", "running-job" ]
                , test "filters by fuzzy name" <|
                    \_ ->
                        Filter.filterJobs "paus" jobs
                            |> List.map .name
                            |> Expect.equal [ "paused-job" ]
                , test "filters by exact quoted name" <|
                    \_ ->
                        Filter.filterJobs "\"paused-job\"" jobs
                            |> List.map .name
                            |> Expect.equal [ "paused-job" ]
                , test "filters by starts-with when quote is not closed" <|
                    \_ ->
                        Filter.filterJobs "\"pa" jobs
                            |> List.map .name
                            |> Expect.equal [ "paused-job" ]
                , test "filters by status:paused" <|
                    \_ ->
                        Filter.filterJobs "status:paused" jobs
                            |> List.map .name
                            |> Expect.equal [ "paused-job" ]
                , test "filters by status:failed" <|
                    \_ ->
                        Filter.filterJobs "status:failed" jobs
                            |> List.map .name
                            |> Expect.equal [ "failed-job" ]
                , test "filters by status:running" <|
                    \_ ->
                        Filter.filterJobs "status:running" jobs
                            |> List.map .name
                            |> Expect.equal [ "running-job" ]
                , test "negation works (-status:failed)" <|
                    \_ ->
                        Filter.filterJobs "-status:failed" jobs
                            |> List.map .name
                            |> Expect.equal [ "alpha", "paused-job", "running-job" ]
                ]
            , describe "suggestions" <|
                [ test "suggests status: when typing filter type" <|
                    \_ ->
                        Filter.suggestions "sta"
                            |> List.map .cur
                            |> Expect.equal [ "status:" ]
                , test "suggests paused and pending status values" <|
                    \_ ->
                        Filter.suggestions "status:p"
                            |> List.map .cur
                            |> Expect.equal [ "status:paused", "status:pending" ]
                ]
            ]
        , describe "pipeline search bar" <|
            [ describe "on desktop" <|
                [ test "displays input field in the top bar" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> whenOnDesktop
                            |> queryView
                            |> Query.find [ id "top-bar-app" ]
                            |> Query.has [ id SearchBar.searchInputId ]
                , test "typing a query updates the value" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> whenOnDesktop
                            |> Expect.all
                                [ queryView
                                    >> Query.find [ id SearchBar.searchInputId ]
                                    >> Event.simulate (Event.input "status:paused")
                                    >> Event.expect (Msgs.Update <| FilterMsg "status:paused")
                                , Application.update (Msgs.Update (FilterMsg "status:paused"))
                                    >> Tuple.first
                                    >> queryView
                                    >> Query.find [ id SearchBar.searchInputId ]
                                    >> Query.has [ value "status:paused" ]
                                ]
                , test "dropdown appears when focused" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> whenOnDesktop
                            |> Application.update (Msgs.Update FocusMsg)
                            |> Tuple.first
                            |> queryView
                            |> Query.has [ id "search-dropdown" ]
                , test "dropdown is absolutely positioned" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> whenOnDesktop
                            |> Application.update (Msgs.Update FocusMsg)
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "search-dropdown" ]
                            |> Query.has
                                [ style "position" "absolute"
                                , style "z-index" "1000"
                                ]
                , test "dropdown shows status filter option" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> whenOnDesktop
                            |> Application.update (Msgs.Update FocusMsg)
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "search-dropdown" ]
                            |> Query.findAll [ text "status:" ]
                            |> Query.count (Expect.equal 1)
                , test "dropdown suggests status values" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> whenOnDesktop
                            |> Application.update (Msgs.Update (FilterMsg "status:p"))
                            |> Tuple.first
                            |> Application.update (Msgs.Update FocusMsg)
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "search-dropdown" ]
                            |> Expect.all
                                [ Query.has [ containing [ text "status:paused" ] ]
                                , Query.has [ containing [ text "status:pending" ] ]
                                ]
                ]
            , describe "on mobile" <|
                [ test "shows expand button and hides the input by default" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> whenOnMobile
                            |> queryView
                            |> Query.find [ id "top-bar-app" ]
                            |> Expect.all
                                [ Query.has [ id "show-search-button" ]
                                , Query.hasNot [ id SearchBar.searchInputId ]
                                ]
                , test "clicking expand button shows the input" <|
                    \_ ->
                        Common.init "/teams/team/pipelines/pipeline"
                            |> whenOnMobile
                            |> Expect.all
                                [ queryView
                                    >> Query.find [ id "show-search-button" ]
                                    >> Event.simulate Event.click
                                    >> Event.expect (Msgs.Update (Click ShowSearchButton))
                                , Application.update (Msgs.Update (Click ShowSearchButton))
                                    >> Tuple.first
                                    >> queryView
                                    >> Query.has [ id SearchBar.searchInputId ]
                                ]
                ]
            ]
        ]


value : String -> Selector.Selector
value v =
    attribute (Attr.value v)
