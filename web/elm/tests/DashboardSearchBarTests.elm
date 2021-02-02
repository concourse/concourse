module DashboardSearchBarTests exposing (all)

import Application.Application as Application
import Assets
import ColorValues
import Common
    exposing
        ( gotPipelines
        , whenOnDesktop
        , whenOnMobile
        )
import Concourse exposing (JsonValue(..), hyphenNotation)
import Concourse.BuildStatus exposing (BuildStatus(..))
import Dashboard.Filter as Filter
import Dashboard.SearchBar as SearchBar
import DashboardInstanceGroupTests exposing (pipelineInstance, pipelineInstanceWithVars)
import DashboardTests exposing (job, running, whenOnDashboard)
import Data
import Dict
import Expect
import FetchResult
import Html.Attributes as Attr
import Keyboard
import Message.Callback as Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message as Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import Message.TopLevelMessage as Msgs
import Set
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector as Selector
    exposing
        ( attribute
        , class
        , containing
        , id
        , style
        , tag
        , text
        )


all : Test
all =
    describe "dashboard search bar" <|
        [ describe "on desktop" <|
            [ test "displays input field in the top bar" <|
                loadDashboard
                    >> whenOnDesktop
                    >> Common.queryView
                    >> Query.find [ id "top-bar-app" ]
                    >> Query.has [ id SearchBar.searchInputId ]
            , test "has magnifying glass icon" <|
                loadDashboard
                    >> whenOnDesktop
                    >> Common.queryView
                    >> Query.find [ id SearchBar.searchInputId ]
                    >> Query.has
                        [ style "background-image" <|
                            Assets.backgroundImage <|
                                Just Assets.SearchIconGrey
                        ]
            , test "has fixed width" <|
                loadDashboard
                    >> whenOnDesktop
                    >> Common.queryView
                    >> Query.find [ id SearchBar.searchInputId ]
                    >> Query.has [ style "width" "251px" ]
            , describe "HD view" <|
                [ test "does not display search bar" <|
                    loadDashboardHD
                        >> whenOnDesktop
                        >> Common.queryView
                        >> Query.hasNot [ id SearchBar.searchInputId ]
                , test "renders an empty top bar content that fills width" <|
                    loadDashboardHD
                        >> whenOnDesktop
                        >> Common.queryView
                        >> Query.has
                            [ id "top-bar-content"
                            , style "flex-grow" "1"
                            ]
                ]
            ]
        , describe "on mobile" <|
            [ test "does not display the search bar" <|
                loadDashboard
                    >> whenOnMobile
                    >> Common.queryView
                    >> Query.hasNot [ id "search-container" ]
            , test "displays the search bar if there is a query" <|
                loadDashboard
                    >> whenOnMobile
                    >> withFilter "foo"
                    >> Common.queryView
                    >> Query.has
                        [ id "search-container"
                        , containing [ value "foo" ]
                        ]
            , describe "expand button" <|
                [ test "displays a magnifying glass icon" <|
                    loadDashboard
                        >> whenOnMobile
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Query.has
                            [ id "show-search-button"
                            , containing
                                [ style "background-image" <|
                                    Assets.backgroundImage <|
                                        Just Assets.SearchIconGrey
                                ]
                            ]
                , test "clicking the button expands the search bar" <|
                    loadDashboard
                        >> whenOnMobile
                        >> Expect.all
                            [ Common.queryView
                                >> Query.find [ id "show-search-button" ]
                                >> Event.simulate Event.click
                                >> Event.expect (Msgs.Update (Click ShowSearchButton))
                            , Application.update (Msgs.Update (Click ShowSearchButton))
                                >> Tuple.first
                                >> Common.queryView
                                >> Query.has [ id SearchBar.searchInputId ]
                            ]
                , test "clicking the button focuses the search bar" <|
                    loadDashboard
                        >> whenOnMobile
                        >> Application.update (Msgs.Update (Click ShowSearchButton))
                        >> Tuple.second
                        >> Common.contains (Effects.Focus SearchBar.searchInputId)
                , test "clicking the button hides the login component and show archived toggle" <|
                    loadDashboard
                        >> whenOnMobile
                        >> Application.update (Msgs.Update (Click ShowSearchButton))
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.find [ id "top-bar-app" ]
                        >> Expect.all
                            [ Query.hasNot [ id "login-component" ]
                            , Query.hasNot [ text "show archived" ]
                            ]
                ]
            ]
        , describe "typing a query" <|
            [ test "updates displayed value on input" <|
                loadDashboard
                    >> Expect.all
                        [ Common.queryView
                            >> Query.find [ id SearchBar.searchInputId ]
                            >> Event.simulate (Event.input "foo")
                            >> Event.expect (Msgs.Update <| FilterMsg "foo")
                        , Application.update (Msgs.Update (FilterMsg "foo"))
                            >> Tuple.first
                            >> Common.queryView
                            >> Query.find [ id SearchBar.searchInputId ]
                            >> Query.has [ value "foo" ]
                        ]
            , test "shows clear search button" <|
                loadDashboard
                    >> withFilter "foo"
                    >> Common.queryView
                    >> Query.has [ id "search-clear" ]
            , test "clicking clear search button clears the input" <|
                loadDashboard
                    >> withFilter "foo"
                    >> Expect.all
                        [ Common.queryView
                            >> Query.find [ id "search-clear" ]
                            >> Event.simulate Event.click
                            >> Event.expect (Msgs.Update (Click ClearSearchButton))
                        , Application.update (Msgs.Update (Click ClearSearchButton))
                            >> Tuple.first
                            >> Common.queryView
                            >> Query.find [ id SearchBar.searchInputId ]
                            >> Query.has [ value "" ]
                        ]
            ]
        , describe "when focused on input" <|
            let
                focusSearchBar =
                    loadDashboard
                        >> Application.update (Msgs.Update FocusMsg)
                        >> Tuple.first
            in
            [ describe "search dropdown" <|
                let
                    findDropdown =
                        Common.queryView
                            >> Query.find [ id "search-dropdown" ]
                in
                [ test "does not appear when search bar isn't focused" <|
                    loadDashboard
                        >> whenOnDesktop
                        >> Common.queryView
                        >> Query.hasNot [ id "search-dropdown" ]
                , test "appears when focused on desktop" <|
                    focusSearchBar
                        >> whenOnDesktop
                        >> Common.queryView
                        >> Query.has [ id "search-dropdown" ]
                , test "appears when focused on mobile" <|
                    focusSearchBar
                        >> whenOnMobile
                        >> Common.queryView
                        >> Query.has [ id "search-dropdown" ]
                , test "is hidden when the search bar is blurred" <|
                    focusSearchBar
                        >> Expect.all
                            [ Common.queryView
                                >> Query.find [ id SearchBar.searchInputId ]
                                >> Event.simulate Event.blur
                                >> Event.expect (Msgs.Update BlurMsg)
                            , Application.update (Msgs.Update BlurMsg)
                                >> Tuple.first
                                >> Common.queryView
                                >> Query.hasNot [ id "search-dropdown" ]
                            ]
                , test "is positioned below the search bar" <|
                    focusSearchBar
                        >> whenOnDesktop
                        >> findDropdown
                        >> Query.has
                            [ style "top" "100%"
                            , style "margin" "0"
                            , style "position" "absolute"
                            ]
                , test "is not absolutely positioned on mobile" <|
                    focusSearchBar
                        >> whenOnMobile
                        >> findDropdown
                        >> Expect.all
                            [ Query.has
                                [ style "top" "100%"
                                , style "margin" "0"
                                ]

                            -- I can't figure out the reason for this differing behaviour
                            , Query.hasNot [ style "position" "absolute" ]
                            ]
                , test "displays available filter options when focused" <|
                    focusSearchBar
                        >> findDropdown
                        >> Query.findAll [ tag "li" ]
                        >> Expect.all
                            [ Query.count (Expect.equal 2)
                            , Query.index 0 >> Query.has [ text "status:" ]
                            , Query.index 1 >> Query.has [ text "team:" ]
                            ]
                , describe "navigating dropdown items" <|
                    let
                        isHighlighted i =
                            Query.index i
                                >> Query.has
                                    [ style "background-color" ColorValues.grey90
                                    , style "color" ColorValues.grey30
                                    ]

                        isUnhighlighted i =
                            Query.index i
                                >> Query.has
                                    [ style "background-color" ColorValues.grey80
                                    , style "color" ColorValues.grey40
                                    ]

                        expectCursorOn i =
                            let
                                assert j =
                                    if i == Just j then
                                        isHighlighted j

                                    else
                                        isUnhighlighted j
                            in
                            findDropdown
                                >> Query.findAll [ tag "li" ]
                                >> Expect.all [ assert 0, assert 1 ]

                        press key =
                            Application.handleDelivery
                                (KeyDown
                                    { ctrlKey = False
                                    , shiftKey = False
                                    , metaKey = False
                                    , code = key
                                    }
                                )

                        down =
                            press Keyboard.ArrowDown >> Tuple.first

                        up =
                            press Keyboard.ArrowUp >> Tuple.first
                    in
                    [ test "unhighlighted by default" <|
                        focusSearchBar
                            >> withFilter "team:"
                            >> expectCursorOn Nothing
                    , test "down arrow moves cursor to first item" <|
                        focusSearchBar
                            >> withFilter "team:"
                            >> down
                            >> expectCursorOn (Just 0)
                    , test "up arrow moves cursor to last item" <|
                        focusSearchBar
                            >> withFilter "team:"
                            >> up
                            >> expectCursorOn (Just 1)
                    , test "down arrow twice moves cursor to second item" <|
                        focusSearchBar
                            >> withFilter "team:"
                            >> down
                            >> down
                            >> expectCursorOn (Just 1)
                    , test "cursor loops around the bottom" <|
                        focusSearchBar
                            >> withFilter "team:"
                            >> down
                            >> down
                            >> down
                            >> expectCursorOn (Just 0)
                    , test "cursor loops around the top" <|
                        focusSearchBar
                            >> withFilter "team:"
                            >> down
                            >> up
                            >> expectCursorOn (Just 1)
                    , test "enter selects an item" <|
                        focusSearchBar
                            >> withFilter "status: pending team:"
                            >> down
                            >> press Keyboard.Enter
                            >> Expect.all
                                [ Tuple.first
                                    >> Common.queryView
                                    >> Query.find [ id SearchBar.searchInputId ]
                                    >> Query.has [ value "status: pending team:\"team1\"" ]
                                , Tuple.second >> Common.contains (Effects.ModifyUrl "/?search=status%3A%20pending%20team%3A%22team1%22")
                                ]
                    , test "doesn't try to loop if the dropdown is empty (down)" <|
                        focusSearchBar
                            >> withFilter "asdfgh"
                            >> down
                            >> down
                            >> findDropdown
                            >> Query.hasNot [ tag "li" ]
                    , test "doesn't try to loop if the dropdown is empty (up)" <|
                        focusSearchBar
                            >> withFilter "asdfgh"
                            >> up
                            >> up
                            >> findDropdown
                            >> Query.hasNot [ tag "li" ]
                    ]
                , test "clicking a suggestion applies the filter" <|
                    let
                        mouseDownOn i =
                            Query.findAll [ tag "li" ]
                                >> Query.index i
                                >> Event.simulate Event.mouseDown
                    in
                    focusSearchBar
                        >> withFilter "status:paused t"
                        >> findDropdown
                        >> mouseDownOn 0
                        >> Event.expect (Msgs.Update <| FilterMsg "status:paused team:")
                , describe "dropdown suggestions" <|
                    let
                        manyTeams =
                            List.range 1 11
                                |> List.map
                                    (\i ->
                                        pipeline BuildStatusPending i "p"
                                            |> withTeam ("team" ++ String.fromInt i)
                                    )

                        defaultPipelines =
                            [ pipeline BuildStatusSucceeded 1 "p1" |> withTeam "team1"
                            , pipeline BuildStatusFailed 2 "p2" |> withTeam "team2"
                            , pipelineInstance BuildStatusPending False 3 |> withTeam "team1"
                            , pipelineInstance BuildStatusPending False 4 |> withTeam "team1"
                            , pipelineInstance BuildStatusPending False 5 |> withTeam "other-team"
                            , pipelineInstance BuildStatusPending False 6 |> withName "other-group" |> withTeam "other-team"
                            ]

                        findSuggestions =
                            findDropdown >> Query.findAll [ tag "li" ]

                        expectSuggestions expected =
                            findSuggestions
                                >> Expect.all
                                    (Query.count (Expect.equal <| List.length expected)
                                        :: List.indexedMap
                                            (\i e ->
                                                Query.index i
                                                    >> Expect.all
                                                        [ Query.has [ text e.cur ]
                                                        , Event.simulate Event.mouseDown
                                                            >> Event.expect (Msgs.Update <| FilterMsg <| e.prev ++ e.cur)
                                                        ]
                                            )
                                            expected
                                    )

                        suggestionsTest pipelines query expectation =
                            let
                                pipelinesStr =
                                    List.map
                                        (\( p, _ ) ->
                                            p.teamName ++ "/" ++ p.name ++ "/" ++ hyphenNotation p.instanceVars
                                        )
                                        pipelines
                                        |> Debug.toString
                            in
                            test ("test query \"" ++ query ++ "\" with pipelines " ++ pipelinesStr) <|
                                loadDashboard
                                    >> gotPipelines pipelines
                                    >> withFilter query
                                    >> Application.update (Msgs.Update FocusMsg)
                                    >> Tuple.first
                                    >> expectation

                        prefixedSuggestionsTest query prefix expectedText =
                            let
                                expected =
                                    List.map (\s -> { prev = prefix, cur = s }) expectedText
                            in
                            suggestionsTest defaultPipelines query (expectSuggestions expected)

                        simpleSuggestionsTest query expected =
                            prefixedSuggestionsTest query "" expected
                    in
                    [ -- available filters
                      simpleSuggestionsTest "" [ "status:", "team:" ]
                    , simpleSuggestionsTest "-" [ "-status:", "-team:" ]
                    , simpleSuggestionsTest " " [ "status:", "team:" ]

                    -- status
                    , simpleSuggestionsTest "st" [ "status:" ]
                    , simpleSuggestionsTest "-st" [ "-status:" ]
                    , simpleSuggestionsTest "\"st" []
                    , simpleSuggestionsTest "status" [ "status:" ]
                    , simpleSuggestionsTest "status:"
                        [ "status:paused"
                        , "status:pending"
                        , "status:failed"
                        , "status:errored"
                        , "status:aborted"
                        , "status:running"
                        , "status:succeeded"
                        ]
                    , simpleSuggestionsTest " status: p" [ "status:paused", "status:pending" ]
                    , simpleSuggestionsTest " -status:p" [ "-status:paused", "-status:pending" ]
                    , simpleSuggestionsTest "status:p" [ "status:paused", "status:pending" ]
                    , simpleSuggestionsTest "status:pause" [ "status:paused" ]
                    , simpleSuggestionsTest "status:paused" []

                    -- team
                    , simpleSuggestionsTest "t" [ "team:" ]
                    , simpleSuggestionsTest "-t" [ "-team:" ]
                    , simpleSuggestionsTest "team" [ "team:" ]
                    , simpleSuggestionsTest "team:" [ "team:\"other-team\"", "team:\"team1\"", "team:\"team2\"" ]
                    , simpleSuggestionsTest "team:t1" [ "team:\"team1\"" ]
                    , simpleSuggestionsTest "team:\"t" [ "team:\"team1\"", "team:\"team2\"" ]
                    , simpleSuggestionsTest "team:\"other-team\"" []
                    , suggestionsTest manyTeams "team:" (findSuggestions >> Query.count (Expect.equal 10))

                    -- group
                    , simpleSuggestionsTest "group:" [ "group:\"group\"", "group:\"other-group\"" ]
                    , simpleSuggestionsTest "group:oth" [ "group:\"other-group\"" ]
                    , simpleSuggestionsTest "group:\"group\"" []

                    -- fuzzy pipeline
                    , simpleSuggestionsTest "foo" []

                    -- takes last filter
                    , prefixedSuggestionsTest "team:other-team " "team:other-team " [ "status:", "team:" ]
                    , prefixedSuggestionsTest "team:other-team s" "team:other-team " [ "status:" ]
                    , prefixedSuggestionsTest "team:other-team -status:a" "team:other-team " [ "-status:aborted" ]

                    -- applies previous filters to suggestion
                    , prefixedSuggestionsTest "status:succeeded team:" "status:succeeded " [ "team:\"team1\"" ]
                    , prefixedSuggestionsTest "status:failed team:" "status:failed " [ "team:\"team2\"" ]
                    , prefixedSuggestionsTest "group:\"group\" team:" "group:\"group\" " [ "team:\"other-team\"", "team:\"team1\"" ]
                    ]
                ]
            ]
        , describe "keyboard focus" <|
            [ test "hitting '/' focuses search input" <|
                loadDashboard
                    >> Application.handleDelivery
                        (KeyDown
                            { ctrlKey = False
                            , shiftKey = False
                            , metaKey = False
                            , code = Keyboard.Slash
                            }
                        )
                    >> Tuple.second
                    >> Common.contains (Effects.Focus SearchBar.searchInputId)
            , test "hitting shift + '/' (= '?') does not focus search input" <|
                loadDashboard
                    >> Application.handleDelivery
                        (KeyDown
                            { ctrlKey = False
                            , shiftKey = True
                            , metaKey = False
                            , code = Keyboard.Slash
                            }
                        )
                    >> Tuple.second
                    >> Common.notContains (Effects.Focus SearchBar.searchInputId)
            , test "hitting other keys does not focus search input" <|
                loadDashboard
                    >> Application.handleDelivery
                        (KeyDown
                            { ctrlKey = False
                            , shiftKey = True
                            , metaKey = False
                            , code = Keyboard.A
                            }
                        )
                    >> Tuple.second
                    >> Common.notContains (Effects.Focus SearchBar.searchInputId)
            , test "ESC blurs search input" <|
                loadDashboard
                    >> Application.handleDelivery
                        (KeyDown
                            { ctrlKey = False
                            , shiftKey = False
                            , metaKey = False
                            , code = Keyboard.Escape
                            }
                        )
                    >> Tuple.second
                    >> Common.contains (Effects.Blur SearchBar.searchInputId)
            ]
        , describe "filtering" <|
            let
                expectCardsIn findContainer expected =
                    Common.queryView
                        >> findContainer
                        >> Query.findAll [ class "card" ]
                        >> Expect.all
                            (Query.count (Expect.equal <| List.length expected)
                                :: List.indexedMap
                                    (\i name -> Query.index i >> Query.has [ text name ])
                                    expected
                            )

                expectCards =
                    expectCardsIn identity

                assertInstanceGroup name expectation =
                    Common.queryView
                        >> Query.find [ class "instance-group-card", containing [ text name ] ]
                        >> expectation

                containsInstances n =
                    Query.find [ class "card-body" ]
                        >> Query.findAll [ tag "a" ]
                        >> Query.count (Expect.equal n)

                teamSection team =
                    Query.find [ id team ]

                favoritesSection =
                    Query.find [ id "dashboard-favorite-pipelines" ]

                expectNoTeamSection team =
                    Common.queryView >> Query.hasNot [ id team ]

                expectNoFavoritesSection =
                    Common.queryView >> Query.hasNot [ id "dashboard-favorite-pipelines" ]

                simpleFilterTest setup filter expectation =
                    test filter <|
                        setup
                            >> withFilter filter
                            >> expectation
            in
            [ describe "status filter" <|
                let
                    statusFilterTest =
                        simpleFilterTest <|
                            loadDashboard
                                >> gotPipelines
                                    [ pipeline BuildStatusPending 1 "pending"
                                    , pipeline BuildStatusSucceeded 2 "running-succeeded" |> withJobsRunning
                                    , pipeline BuildStatusFailed 3 "running-failed" |> withJobsRunning
                                    ]
                in
                [ statusFilterTest "status:pending" (expectCards [ "pending" ])
                , statusFilterTest "status:succeeded" (expectCards [ "running-succeeded" ])
                , statusFilterTest "status:failed" (expectCards [ "running-failed" ])
                , statusFilterTest "status:running" (expectCards [ "running-succeeded", "running-failed" ])
                , statusFilterTest "-status:running" (expectCards [ "pending" ])
                , statusFilterTest "status:errored" (expectCards [])
                ]
            , describe "team filter" <|
                let
                    teamFilterTest =
                        simpleFilterTest <|
                            loadDashboard
                                >> gotPipelines
                                    [ pipeline BuildStatusPending 1 "p1" |> withTeam "team"
                                    , pipeline BuildStatusPending 2 "p1" |> withTeam "other-team"
                                    , pipeline BuildStatusPending 3 "p2" |> withTeam "another-team"
                                    , pipeline BuildStatusPending 4 "p3" |> withTeam "something-else"
                                    ]
                in
                [ teamFilterTest "team:team"
                    (Expect.all
                        [ expectCardsIn (teamSection "team") [ "p1" ]
                        , expectCardsIn (teamSection "other-team") [ "p1" ]
                        , expectCardsIn (teamSection "another-team") [ "p2" ]
                        , expectNoTeamSection "something-else"
                        ]
                    )
                , teamFilterTest "-team:team"
                    (Expect.all
                        [ expectNoTeamSection "team"
                        , expectNoTeamSection "other-team"
                        , expectNoTeamSection "another-team"
                        , expectCardsIn (teamSection "something-else") [ "p3" ]
                        ]
                    )
                , teamFilterTest "team:other-team"
                    (Expect.all
                        [ expectCardsIn (teamSection "other-team") [ "p1" ]
                        , expectCardsIn (teamSection "another-team") [ "p2" ]
                        , expectNoTeamSection "team"
                        ]
                    )
                , teamFilterTest "team:something"
                    (expectCardsIn (teamSection "something-else") [ "p3" ])
                , teamFilterTest "team:invalid"
                    (expectCards [])
                ]
            , describe "pipeline text filter" <|
                let
                    pipelineFilterTest =
                        simpleFilterTest <|
                            loadDashboard
                                >> gotPipelines
                                    [ pipeline BuildStatusPending 1 "p1"
                                    , pipeline BuildStatusPending 2 "p1" |> withTeam "team2"
                                    , pipeline BuildStatusPending 3 "pipeline1"
                                    , pipeline BuildStatusPending 4 "other-pipeline"
                                    , pipelineInstanceWithVars 5
                                        [ ( "a"
                                          , JsonObject
                                                [ ( "b", JsonString "foo" )
                                                , ( "c", JsonString "bar" )
                                                ]
                                          )
                                        , ( "d", JsonNumber 1.0 )
                                        ]
                                    , pipelineInstanceWithVars 6
                                        [ ( "a", JsonString "fir" )
                                        , ( "b", JsonString "bap" )
                                        ]
                                    ]
                in
                [ pipelineFilterTest "p" (expectCards [ "p1", "pipeline1", "other-pipeline", "group", "p1" ])
                , pipelineFilterTest "p1" (expectCards [ "p1", "pipeline1", "p1" ])
                , pipelineFilterTest "blah" (expectCards [])

                -- by instance var values
                , pipelineFilterTest "foo 1 bar"
                    (Expect.all
                        [ expectCards [ "group" ]
                        , assertInstanceGroup "group" (containsInstances 1)
                        ]
                    )
                , pipelineFilterTest "f ba"
                    (Expect.all
                        [ expectCards [ "group" ]
                        , assertInstanceGroup "group" (containsInstances 2)
                        ]
                    )
                ]
            , describe "multiple filters" <|
                let
                    multiFilterTest =
                        simpleFilterTest <|
                            loadDashboard
                                >> gotPipelines
                                    [ pipeline BuildStatusPending 1 "p1"
                                    , pipeline BuildStatusPending 2 "p1" |> withTeam "team2"
                                    , pipeline BuildStatusPending 3 "pipeline1"
                                    , pipeline BuildStatusPending 4 "other-pipeline"
                                    ]
                in
                [ multiFilterTest "p1 team:team2"
                    (Expect.all
                        [ expectCardsIn (teamSection "team2") [ "p1" ]
                        , expectNoTeamSection "team"
                        ]
                    )
                , multiFilterTest "status:running p" (expectCards [])
                ]
            , describe "favorites section" <|
                let
                    favoritesFilterTest =
                        simpleFilterTest <|
                            loadDashboard
                                >> gotPipelines
                                    [ pipeline BuildStatusPending 1 "p1"
                                    , pipeline BuildStatusPending 2 "p1" |> withTeam "team2"
                                    , pipeline BuildStatusPending 3 "pipeline1"
                                    , pipeline BuildStatusPending 4 "other-pipeline"
                                    ]
                                >> Application.handleDelivery
                                    (FavoritedPipelinesReceived <|
                                        Ok <|
                                            Set.fromList [ 1, 4 ]
                                    )
                                >> Tuple.first
                in
                [ favoritesFilterTest "p1"
                    (Expect.all
                        [ expectCardsIn favoritesSection [ "p1" ]
                        , expectCardsIn (teamSection "team") [ "p1", "pipeline1" ]
                        ]
                    )
                , favoritesFilterTest "asdfgh" expectNoFavoritesSection
                ]
            , describe "no results" <|
                [ test "displays no results message when a filter has no matching pipelines" <|
                    loadDashboard
                        >> withFilter "asdfghjk"
                        >> Common.queryView
                        >> Query.has [ text "No results" ]
                , test "does not render 'no results' when awaiting pipelines" <|
                    (\_ -> whenOnDashboard { highDensity = False })
                        >> Application.handleCallback (Callback.AllJobsFetched <| Ok [])
                        >> Tuple.first
                        >> Common.queryView
                        >> Query.hasNot [ text "No results" ]
                ]
            ]
        ]


loadDashboard : () -> Application.Model
loadDashboard =
    \_ -> whenOnDashboard { highDensity = False } |> loadInitialData


loadDashboardHD : () -> Application.Model
loadDashboardHD =
    \_ -> whenOnDashboard { highDensity = True } |> loadInitialData


loadInitialData : Application.Model -> Application.Model
loadInitialData =
    Application.handleCallback
        (Callback.AllTeamsFetched <|
            Ok
                [ Concourse.Team 1 "team1"
                , Concourse.Team 2 "team2"
                ]
        )
        >> Tuple.first
        >> Application.handleCallback
            (Callback.AllPipelinesFetched <|
                Ok
                    [ Data.pipeline "team1" 0 |> Data.withName "pipeline" ]
            )
        >> Tuple.first


withFilter : String -> Application.Model -> Application.Model
withFilter query =
    Application.update (Msgs.Update (FilterMsg query)) >> Tuple.first


value v =
    attribute <| Attr.value v


pipeline status id name =
    ( Data.pipeline "team" id |> Data.withName name
    , [ job status |> Data.withPipelineId id ]
    )


withJobsRunning =
    Tuple.mapSecond (List.map running)


withName n =
    Tuple.mapFirst (Data.withName n)


withTeam t =
    Tuple.mapFirst (Data.withTeamName t)
