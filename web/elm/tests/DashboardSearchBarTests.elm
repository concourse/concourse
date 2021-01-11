module DashboardSearchBarTests exposing (all)

import Application.Application as Application
import Assets
import ColorValues
import Common exposing (whenOnDesktop, whenOnMobile)
import Concourse
import Dashboard.Filter as Filter
import Dashboard.SearchBar as SearchBar
import Data
import Expect
import FetchResult
import Html.Attributes as Attr
import Keyboard
import Message.Callback as Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message as Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..))
import Message.TopLevelMessage as Msgs
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
                            , Query.index 0 >> Query.has [ text "status: " ]
                            , Query.index 1 >> Query.has [ text "team: " ]
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
                            >> withFilter "team:"
                            >> down
                            >> press Keyboard.Enter
                            >> Expect.all
                                [ Tuple.first
                                    >> Common.queryView
                                    >> Query.find [ id SearchBar.searchInputId ]
                                    >> Query.has [ value "team: team1" ]
                                , Tuple.second >> Common.contains (Effects.ModifyUrl "/?search=team%3A%20team1")
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
                , describe "dropdown suggestions" <|
                    let
                        suggestionsTest teams query expectation =
                            test ("test query \"" ++ query ++ "\" with teams " ++ String.join ", " teams) <|
                                \_ ->
                                    Filter.suggestions
                                        { query = query
                                        , teams = FetchResult.Fetched <| List.indexedMap Concourse.Team teams
                                        , pipelines = Nothing
                                        }
                                        |> expectation

                        simpleSuggestionsTest query expected =
                            suggestionsTest [ "team", "other-team", "yet-another-team" ]
                                query
                                (Expect.equal expected)

                        manyTeams =
                            List.range 1 11 |> List.map (\i -> "team" ++ String.fromInt i)
                    in
                    [ -- available filters
                      simpleSuggestionsTest "" [ "status: ", "team: " ]
                    , simpleSuggestionsTest "-" [ "status: ", "team: " ]
                    , simpleSuggestionsTest " " [ "status: ", "team: " ]

                    -- status
                    , simpleSuggestionsTest "st" [ "status: " ]
                    , simpleSuggestionsTest "-st" [ "status: " ]
                    , simpleSuggestionsTest "status" [ "status: " ]
                    , simpleSuggestionsTest "status: "
                        [ "status: paused"
                        , "status: pending"
                        , "status: failed"
                        , "status: errored"
                        , "status: aborted"
                        , "status: running"
                        , "status: succeeded"
                        ]
                    , simpleSuggestionsTest " status: p" [ "status: paused", "status: pending" ]
                    , simpleSuggestionsTest " -status: p" [ "status: paused", "status: pending" ]
                    , simpleSuggestionsTest "status:p" [ "status: paused", "status: pending" ]
                    , simpleSuggestionsTest "status: pause" [ "status: paused" ]
                    , simpleSuggestionsTest "status: paused" []

                    -- team
                    , simpleSuggestionsTest "t" [ "team: " ]
                    , simpleSuggestionsTest "-t" [ "team: " ]
                    , simpleSuggestionsTest "team" [ "team: " ]
                    , simpleSuggestionsTest "team:" [ "team: other-team", "team: team", "team: yet-another-team" ]
                    , simpleSuggestionsTest "team: oth" [ "team: other-team" ]
                    , simpleSuggestionsTest "team:oth" [ "team: other-team" ]
                    , simpleSuggestionsTest "team: other-team" []
                    , suggestionsTest manyTeams "team:" (List.length >> Expect.equal 10)

                    -- fuzzy pipeline
                    , simpleSuggestionsTest "foo" []

                    -- takes last filter
                    , simpleSuggestionsTest "team: other-team " [ "status: ", "team: " ]
                    , simpleSuggestionsTest "team: other-team s" [ "status: " ]
                    , simpleSuggestionsTest "team: other-team -status:a" [ "status: aborted" ]
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
        ]


loadDashboard : () -> Application.Model
loadDashboard =
    \_ -> Common.init "/" |> loadInitialData


loadDashboardHD : () -> Application.Model
loadDashboardHD =
    \_ -> Common.init "/hd" |> loadInitialData


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
