module NewTopBarTests exposing (all)

import Dict
import Expect
import Html.Attributes as Attributes
import Html.Styled as HS
import Http
import Navigation
import NewTopBar
import RemoteData
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector as THS
    exposing
        ( attribute
        , containing
        , id
        , tag
        , text
        )


init : { highDensity : Bool, query : String } -> NewTopBar.Model {}
init { highDensity, query } =
    NewTopBar.init (not highDensity) query
        |> Tuple.first


smallScreen : NewTopBar.Model r -> NewTopBar.Model r
smallScreen =
    updateModel
        << NewTopBar.ScreenResized
    <|
        { width = 300, height = 800 }


bigScreen : NewTopBar.Model r -> NewTopBar.Model r
bigScreen =
    updateModel
        << NewTopBar.ScreenResized
    <|
        { width = 1200, height = 900 }


userName : String
userName =
    "some-user"


loggedIn : NewTopBar.Model r -> NewTopBar.Model r
loggedIn =
    updateModel
        << NewTopBar.UserFetched
        << RemoteData.Success
    <|
        { id = userName
        , userName = userName
        , name = userName
        , email = userName
        , teams = Dict.empty
        }


loggedOut : NewTopBar.Model r -> NewTopBar.Model r
loggedOut =
    updateModel
        << NewTopBar.UserFetched
        << RemoteData.Failure
        << Http.BadStatus
    <|
        { url = ""
        , status =
            { code = 401
            , message = "Unauthorized"
            }
        , headers = Dict.empty
        , body = ""
        }


queryView : NewTopBar.Model r -> Query.Single NewTopBar.Msg
queryView =
    NewTopBar.view
        >> HS.toUnstyled
        >> Query.fromHtml


updateModel : NewTopBar.Msg -> NewTopBar.Model r -> NewTopBar.Model r
updateModel msg =
    NewTopBar.update msg >> Tuple.first


all : Test
all =
    describe "NewTopBarSearchInput"
        [ describe "autocompletion"
            [ test "initially status and team" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> updateModel NewTopBar.FocusMsg
                        |> queryView
                        |> Query.findAll [ tag "li" ]
                        |> Expect.all
                            [ Query.count (Expect.equal 2)
                            , Query.index 0
                                >> Query.has [ text "status:" ]
                            , Query.index 1
                                >> Query.has [ text "team:" ]
                            ]
            , test "typing status: shows all statuses" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> updateModel NewTopBar.FocusMsg
                        |> updateModel (NewTopBar.FilterMsg "status:")
                        |> queryView
                        |> Query.findAll [ tag "li" ]
                        |> Expect.all
                            [ Query.count (Expect.equal 7)
                            , Query.index 0
                                >> Query.has [ text "status: paused" ]
                            , Query.index 1
                                >> Query.has [ text "status: pending" ]
                            , Query.index 2
                                >> Query.has [ text "status: failed" ]
                            , Query.index 3
                                >> Query.has [ text "status: errored" ]
                            , Query.index 4
                                >> Query.has [ text "status: aborted" ]
                            , Query.index 5
                                >> Query.has [ text "status: running" ]
                            , Query.index 6
                                >> Query.has [ text "status: succeeded" ]
                            ]
            , test "mousing over an option sends SelectMsg" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> updateModel NewTopBar.FocusMsg
                        |> updateModel (NewTopBar.FilterMsg "status:")
                        |> queryView
                        |> Query.find
                            [ tag "li", containing [ text "status: pending" ] ]
                        |> Event.simulate Event.mouseOver
                        |> Event.expect (NewTopBar.SelectMsg 1)
            , test "SelectMsg changes selection" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> updateModel NewTopBar.FocusMsg
                        |> updateModel (NewTopBar.FilterMsg "status:")
                        |> updateModel (NewTopBar.SelectMsg 1)
                        |> updateModel (NewTopBar.KeyDown 13)
                        |> queryView
                        |> Query.find [ tag "input" ]
                        |> Query.has [ attribute (Attributes.value "status: pending") ]
            , test "typing team: shows all teams" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> updateModel
                            (NewTopBar.TeamsFetched
                                (RemoteData.Success
                                    [ { id = 0
                                      , name = "some-team"
                                      }
                                    ]
                                )
                            )
                        |> updateModel NewTopBar.FocusMsg
                        |> updateModel (NewTopBar.FilterMsg "team:")
                        |> queryView
                        |> Query.findAll [ tag "li" ]
                        |> Expect.all
                            [ Query.count (Expect.equal 1)
                            , Query.index 0
                                >> Query.has [ text "team: some-team" ]
                            ]
            , test "typing anything else shows no autocomplete" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> updateModel NewTopBar.FocusMsg
                        |> updateModel (NewTopBar.FilterMsg "anything-else")
                        |> queryView
                        |> Query.findAll [ tag "li" ]
                        |> Query.count (Expect.equal 0)
            ]
        , describe "on small screens"
            [ test "shows the search icon" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> smallScreen
                        |> queryView
                        |> Query.findAll [ id "search-button" ]
                        |> Query.count (Expect.equal 1)
            , test "shows no search bar on high density" <|
                \_ ->
                    init { highDensity = True, query = "" }
                        |> smallScreen
                        |> queryView
                        |> Query.findAll [ tag "input" ]
                        |> Query.count (Expect.equal 0)
            , describe "logging in"
                [ test "shows login button" <|
                    \_ ->
                        init { highDensity = True, query = "" }
                            |> smallScreen
                            |> loggedOut
                            |> queryView
                            |> Query.findAll [ id "login-button" ]
                            |> Query.count (Expect.equal 1)
                , test "clicking login sends LogIn message" <|
                    \_ ->
                        init { highDensity = True, query = "" }
                            |> smallScreen
                            |> loggedOut
                            |> queryView
                            |> Query.find [ id "login-button" ]
                            |> Event.simulate Event.click
                            |> Event.expect NewTopBar.LogIn
                ]
            , describe "when logged in"
                [ test "shows the user's name" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> loggedIn
                            |> queryView
                            |> Query.has [ text userName ]
                , test "does not show logout button" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> loggedIn
                            |> queryView
                            |> Query.findAll [ text "logout" ]
                            |> Query.count (Expect.equal 0)
                , test "clicking username sends ToggleUserMenu message" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> loggedIn
                            |> queryView
                            |> Query.find
                                [ id "user-id"
                                , containing [ text userName ]
                                ]
                            |> Event.simulate Event.click
                            |> Event.expect NewTopBar.ToggleUserMenu
                , test "ToggleUserMenu message shows logout button" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> loggedIn
                            |> updateModel NewTopBar.ToggleUserMenu
                            |> queryView
                            |> Query.findAll [ text "logout" ]
                            |> Query.count (Expect.equal 1)
                , test "clicking logout button sends LogOut message" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> loggedIn
                            |> updateModel NewTopBar.ToggleUserMenu
                            |> queryView
                            |> Query.find [ id "logout-button" ]
                            |> Event.simulate Event.click
                            |> Event.expect NewTopBar.LogOut
                , describe "logging out"
                    [ test "redirects to dashboard on normal dashboard" <|
                        \_ ->
                            init { highDensity = False, query = "" }
                                |> NewTopBar.update
                                    (NewTopBar.LoggedOut (Ok ()))
                                |> Tuple.second
                                |> Expect.equal (Navigation.newUrl "/")
                    , test "redirects to high-density view on high-density view" <|
                        \_ ->
                            init { highDensity = True, query = "" }
                                |> NewTopBar.update
                                    (NewTopBar.LoggedOut (Ok ()))
                                |> Tuple.second
                                |> Expect.equal (Navigation.newUrl "/hd")
                    ]
                ]
            , test "shows no search input" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> smallScreen
                        |> queryView
                        |> Query.findAll [ tag "input" ]
                        |> Query.count (Expect.equal 0)
            , test "shows search input when resizing" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> smallScreen
                        |> updateModel
                            (NewTopBar.ScreenResized
                                { width = 1200, height = 900 }
                            )
                        |> queryView
                        |> Query.findAll [ tag "input" ]
                        |> Query.count (Expect.equal 1)
            , test "sends a ShowSearchInput message when the search button is clicked" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> smallScreen
                        |> queryView
                        |> Query.find [ id "search-button" ]
                        |> Event.simulate Event.click
                        |> Event.expect NewTopBar.ShowSearchInput
            , describe "on ShowSearchInput"
                [ test "hides the search button" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> updateModel NewTopBar.ShowSearchInput
                            |> queryView
                            |> Query.findAll [ id "search-button" ]
                            |> Query.count (Expect.equal 0)
                , test "shows the search bar" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> updateModel NewTopBar.ShowSearchInput
                            |> queryView
                            |> Query.findAll [ tag "input" ]
                            |> Query.count (Expect.equal 1)
                , test "hides the user info/logout button" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> loggedIn
                            |> updateModel NewTopBar.ShowSearchInput
                            |> queryView
                            |> Query.findAll [ text userName ]
                            |> Query.count (Expect.equal 0)
                , test "sends a BlurMsg message when the search input is blurred" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> updateModel NewTopBar.ShowSearchInput
                            |> queryView
                            |> Query.find [ tag "input" ]
                            |> Event.simulate Event.blur
                            |> Event.expect NewTopBar.BlurMsg
                ]
            , describe "on BlurMsg"
                [ test "hides the search bar when there is no query" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> updateModel NewTopBar.ShowSearchInput
                            |> updateModel NewTopBar.BlurMsg
                            |> queryView
                            |> Query.findAll [ tag "input" ]
                            |> Query.count (Expect.equal 0)
                , test "hides the autocomplete when there is a query" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> updateModel NewTopBar.ShowSearchInput
                            |> updateModel (NewTopBar.FilterMsg "status:")
                            |> updateModel NewTopBar.BlurMsg
                            |> queryView
                            |> Expect.all
                                [ Query.findAll [ tag "input" ]
                                    >> Query.count (Expect.equal 1)
                                , Query.findAll [ tag "ul" ]
                                    >> Query.count (Expect.equal 0)
                                ]
                , test "shows the search button" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> updateModel NewTopBar.ShowSearchInput
                            |> updateModel NewTopBar.BlurMsg
                            |> queryView
                            |> Query.findAll [ id "search-button" ]
                            |> Query.count (Expect.equal 1)
                , test "shows the user info/logout button" <|
                    \_ ->
                        init { highDensity = False, query = "" }
                            |> smallScreen
                            |> loggedIn
                            |> updateModel NewTopBar.ShowSearchInput
                            |> updateModel NewTopBar.BlurMsg
                            |> queryView
                            |> Query.has [ text userName ]
                ]
            , describe "starting with a query"
                [ test "shows the search input on small screens" <|
                    \_ ->
                        init { highDensity = False, query = "some-query" }
                            |> smallScreen
                            |> queryView
                            |> Query.findAll [ tag "input" ]
                            |> Query.count (Expect.equal 1)
                ]
            ]
        , describe "on large screens"
            [ test "shows the entire search input on large screens" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> bigScreen
                        |> queryView
                        |> Query.find [ tag "input" ]
                        |> Query.has
                            [ attribute (Attributes.placeholder "search") ]
            , test "hides the search input on changing to a small screen" <|
                \_ ->
                    init { highDensity = False, query = "" }
                        |> bigScreen
                        |> smallScreen
                        |> queryView
                        |> Query.findAll [ tag "input" ]
                        |> Query.count (Expect.equal 0)
            , test "shows no search bar on high density" <|
                \_ ->
                    init { highDensity = True, query = "" }
                        |> bigScreen
                        |> queryView
                        |> Query.findAll [ tag "input" ]
                        |> Query.count (Expect.equal 0)
            ]
        ]
