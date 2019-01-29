module NewestTopBarTests exposing (all)

import Callback exposing (Callback(..))
import Concourse
import Dict
import Effects
import Expect exposing (..)
import Html.Attributes as Attr
import Html.Styled exposing (toUnstyled)
import NewTopBar.Msgs as Msgs
import NewestTopBar
import QueryString
import Routes
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector as Selector exposing (attribute, class, containing, id, style, tag, text)
import UserState exposing (UserState(..))


rspecStyleDescribe : String -> subject -> List (subject -> Test) -> Test
rspecStyleDescribe description beforeEach subTests =
    Test.describe description
        (subTests |> List.map ((|>) beforeEach))


context : String -> (setup -> subject) -> List (subject -> Test) -> (setup -> Test)
context description beforeEach subTests setup =
    Test.describe description
        (subTests |> List.map ((|>) (beforeEach setup)))


it : String -> (subject -> Expectation) -> subject -> Test
it desc expectationFunc subject =
    Test.test desc <|
        \_ -> expectationFunc subject


lineHeight : String
lineHeight =
    "56px"


borderGrey : String
borderGrey =
    "#3d3c3c"


backgroundGrey : String
backgroundGrey =
    "#1e1d1d"


topBarHeight : String
topBarHeight =
    "54px"


searchBarBorder : String
searchBarBorder =
    "1px solid " ++ searchBarGrey


searchBarGrey : String
searchBarGrey =
    "#504b4b"


dropdownBackgroundGrey : String
dropdownBackgroundGrey =
    "#2e2e2e"


searchBarHeight : String
searchBarHeight =
    "30px"


searchBarWidth : String
searchBarWidth =
    "220px"


searchBarPadding : String
searchBarPadding =
    "0 42px"


initFlags : Routes.Route -> String -> NewestTopBar.Flags
initFlags r q =
    { isHd = False, route = { logical = r, queries = QueryString.parse q, page = Nothing, hash = "" } }


all : Test
all =
    describe "NewestTopBar"
        [ rspecStyleDescribe "when on pipeline page"
            (NewestTopBar.init (initFlags (Routes.Pipeline "team" "pipeline") "")
                |> Tuple.first
            )
            [ context "when login state unknown"
                (NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                )
                [ it "shows concourse logo" <|
                    Query.children []
                        >> Query.index 1
                        >> Query.has
                            [ style
                                [ ( "background-image", "url(/public/images/concourse-logo-white.svg)" )
                                , ( "background-position", "50% 50%" )
                                , ( "background-repeat", "no-repeat" )
                                , ( "background-size", "42px 42px" )
                                , ( "width", topBarHeight )
                                , ( "height", topBarHeight )
                                ]
                            ]
                , it "shows pipeline breadcrumb" <|
                    Query.has [ id "breadcrumb-pipeline" ]
                , context "pipeline breadcrumb"
                    (Query.find [ id "breadcrumb-pipeline" ])
                    [ it "renders icon first" <|
                        Query.children []
                            >> Query.first
                            >> Query.has pipelineBreadcrumbSelector
                    , it "renders pipeline name second" <|
                        Query.children []
                            >> Query.index 1
                            >> Query.has
                                [ text "pipeline" ]
                    , it "has link to the relevant pipeline page" <|
                        Query.children []
                            >> Query.index 1
                            >> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
                    ]
                , it "is 56px tall with dark grey background" <|
                    Query.has [ style [ ( "background-color", backgroundGrey ), ( "height", lineHeight ) ] ]
                , it "lays out contents horizontally" <|
                    Query.has [ style [ ( "display", "flex" ) ] ]
                , it "centers contents vertically" <|
                    Query.has [ style [ ( "align-items", "center" ) ] ]
                , it "maximizes spacing between the left and right navs" <|
                    Query.has [ style [ ( "justify-content", "space-between" ) ] ]
                , it "renders the login component last" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.has [ id "login-component" ]
                ]
            , context "when logged in"
                (logInUser
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                )
                [ it "renders the login component last" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.has [ id "login-component" ]
                , it "renders login container with relative position" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style
                                [ ( "position", "relative" )
                                ]
                            ]
                , it "lays out login container contents vertically" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "flex-direction", "column" )
                                ]
                            ]
                , it "draws lighter grey line to the left of login container" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style
                                [ ( "border-left", "1px solid " ++ borderGrey )
                                ]
                            ]
                , it "renders login container tall enough" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style
                                [ ( "line-height", lineHeight )
                                ]
                            ]
                , it "has the login username styles" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-item" ]
                        >> Query.has
                            [ style
                                [ ( "padding", "0 30px" )
                                , ( "cursor", "pointer" )
                                , ( "display", "flex" )
                                , ( "align-items", "center" )
                                , ( "justify-content", "center" )
                                , ( "flex-grow", "1" )
                                ]
                            ]
                , it "shows the logged in username when the user is logged in" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-item" ]
                        >> Query.has [ text "test" ]
                , it "ToggleUserMenu message is received when login menu is clicked" <|
                    Query.find [ id "login-container" ]
                        >> Event.simulate Event.click
                        >> Event.expect Msgs.ToggleUserMenu
                , it "renders login menu with empty content" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-item" ]
                        >> Expect.all
                            [ Query.has [ id "login-menu" ]
                            , Query.find [ id "login-menu" ] >> Query.children [] >> Query.count (Expect.equal 0)
                            ]
                ]
            ]
        , rspecStyleDescribe "rendering user menus on clicks"
            (NewestTopBar.init (initFlags (Routes.Pipeline "team" "pipeline") "")
                |> Tuple.first
                |> logInUser
            )
            [ it "shows user menu when ToggleUserMenu msg is received" <|
                NewestTopBar.update Msgs.ToggleUserMenu
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.has [ id "logout-button" ]
            , it "renders user menu content when ToggleUserMenu msg is received and logged in" <|
                NewestTopBar.update Msgs.ToggleUserMenu
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Expect.all
                        [ Query.has [ id "logout-button" ]
                        , Query.find [ id "logout-button" ]
                            >> Query.has [ text "logout" ]
                        , Query.find [ id "logout-button" ]
                            >> Query.has
                                [ style
                                    [ ( "position", "absolute" )
                                    , ( "top", "55px" )
                                    , ( "background-color", backgroundGrey )
                                    , ( "height", topBarHeight )
                                    , ( "width", "100%" )
                                    , ( "border-top", "1px solid " ++ borderGrey )
                                    , ( "cursor", "pointer" )
                                    , ( "display", "flex" )
                                    , ( "align-items", "center" )
                                    , ( "justify-content", "center" )
                                    , ( "flex-grow", "1" )
                                    ]
                                ]
                        ]
            , it "when logout is clicked, a LogOut Msg is sent" <|
                NewestTopBar.update Msgs.ToggleUserMenu
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "logout-button" ]
                    >> Event.simulate Event.click
                    >> Event.expect Msgs.LogOut
            , it "shows 'login' when LoggedOut Msg is successful" <|
                NewestTopBar.handleCallback (Callback.LoggedOut (Ok ()))
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "login-item" ]
                    >> Query.has [ text "login" ]
            ]
        , rspecStyleDescribe "login component when user is logged out"
            (NewestTopBar.init (initFlags (Routes.Pipeline "team" "pipeline") "")
                |> Tuple.first
                |> logoutUser
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "has a link to login" <|
                Query.children []
                    >> Query.index -1
                    >> Query.find [ id "login-item" ]
                    >> Query.has [ tag "a", attribute <| Attr.href "/sky/login" ]
            , it "has the login container styles" <|
                Query.children []
                    >> Query.index -1
                    >> Query.find [ id "login-container" ]
                    >> Query.has
                        [ style
                            [ ( "position", "relative" )
                            , ( "display", "flex" )
                            , ( "flex-direction", "column" )
                            , ( "border-left", "1px solid " ++ borderGrey )
                            , ( "line-height", lineHeight )
                            ]
                        ]
            , it "has the login username styles" <|
                Query.children []
                    >> Query.index -1
                    >> Query.find [ id "login-item" ]
                    >> Query.has
                        [ style
                            [ ( "padding", "0 30px" )
                            , ( "cursor", "pointer" )
                            , ( "display", "flex" )
                            , ( "align-items", "center" )
                            , ( "justify-content", "center" )
                            , ( "flex-grow", "1" )
                            ]
                        ]
            ]
        , rspecStyleDescribe "when triggering a log in message"
            (NewestTopBar.init (initFlags (Routes.Pipeline "team" "pipeline") ""))
            [ it "redirects to login page when you click login" <|
                Tuple.first
                    >> NewestTopBar.update Msgs.LogIn
                    >> Tuple.second
                    >> Expect.equal [ Effects.RedirectToLogin ]
            ]
        , rspecStyleDescribe "rendering top bar on build page"
            (NewestTopBar.init (initFlags (Routes.Build "team" "pipeline" "job" "1") "")
                |> Tuple.first
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "pipeline breadcrumb should have a link to the pipeline page when viewing build details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , it "job breadcrumb is laid out horizontally with appropriate spacing" <|
                Query.find [ id "breadcrumb-job" ]
                    >> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , it "top bar has job breadcrumb with job icon rendered first" <|
                Query.find [ id "breadcrumb-job" ]
                    >> Query.has jobBreadcrumbSelector
            , it "top bar has build name after job icon" <|
                Query.find [ id "breadcrumb-job" ]
                    >> Query.has [ text "job" ]
            ]
        , rspecStyleDescribe "rendering top bar on resource page"
            (NewestTopBar.init (initFlags (Routes.Resource "team" "pipeline" "resource") "")
                |> Tuple.first
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "pipeline breadcrumb should have a link to the pipeline page when viewing resource details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , it "there is a / between pipeline and resource in breadcrumb" <|
                Query.findAll [ tag "li" ]
                    >> Expect.all
                        [ Query.index 1 >> Query.has [ class "breadcrumb-separator" ]
                        , Query.index 1 >> Query.has [ text "/" ]
                        , Query.index 2 >> Query.has [ id "breadcrumb-resource" ]
                        ]
            , it "resource breadcrumb is laid out horizontally with appropriate spacing" <|
                Query.find [ id "breadcrumb-resource" ]
                    >> Query.has [ style [ ( "display", "inline-block" ), ( "padding", "0 10px" ) ] ]
            , it "top bar has resource breadcrumb with resource icon rendered first" <|
                Query.find [ id "breadcrumb-resource" ]
                    >> Query.children []
                    >> Query.index 0
                    >> Query.has resourceBreadcrumbSelector
            , it "top bar has resource name after resource icon" <|
                Query.find [ id "breadcrumb-resource" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has
                        [ text "resource" ]
            ]
        , rspecStyleDescribe "rendering top bar on job page"
            (NewestTopBar.init (initFlags (Routes.Job "team" "pipeline" "job") "")
                |> Tuple.first
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "pipeline breadcrumb should have a link to the pipeline page when viewing job details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , it "there is a / between pipeline and job in breadcrumb" <|
                Query.findAll [ tag "li" ]
                    >> Expect.all
                        [ Query.index 1 >> Query.has [ class "breadcrumb-separator" ]
                        , Query.index 0 >> Query.has [ id "breadcrumb-pipeline" ]
                        , Query.index 2 >> Query.has [ id "breadcrumb-job" ]
                        ]
            ]
        , rspecStyleDescribe "when checking search bar values"
            (NewestTopBar.init (initFlags (Routes.Dashboard { isHd = False }) "search=test")
                |> Tuple.first
            )
            [ it "renders the search bar with the text in the search query" <|
                NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "search-bar" ]
                    >> Query.has [ tag "input", attribute <| Attr.value "test" ]
            , it "sends a FilterMsg when the clear search button is clicked" <|
                NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "search-container" ]
                    >> Query.find [ id "search-clear" ]
                    >> Event.simulate Event.click
                    >> Event.expect (Msgs.FilterMsg "")
            , it "clears search query when FilterMsg is received with blank" <|
                NewestTopBar.update (Msgs.FilterMsg "")
                    >> Tuple.first
                    >> NewestTopBar.query
                    >> Expect.equal ""
            , it "clear search button has full opacity when there is a query" <|
                NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "search-clear" ]
                    >> Query.has [ style [ ( "opacity", "1" ) ] ]
            ]
        , rspecStyleDescribe "rendering search bar on dashboard page"
            (NewestTopBar.init (initFlags (Routes.Dashboard { isHd = False }) "")
                |> Tuple.first
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "renders search bar" <|
                Query.has [ id "search-bar" ]
            , it "search bar is an input field" <|
                Query.find [ id "search-bar" ]
                    >> Query.has [ tag "input" ]
            , it "renders search bar with transparent background to remove white of search bar" <|
                Query.find [ id "search-bar" ]
                    >> Query.has [ style [ ( "background-color", "transparent" ) ] ]
            , it "sets magnifying glass on search bar in correct position" <|
                Query.find [ id "search-bar" ]
                    >> Query.has
                        [ style
                            [ ( "background-image", "url('public/images/ic-search-white-24px.svg')" )
                            , ( "background-position", "12px 8px" )
                            , ( "background-repeat", "no-repeat" )
                            ]
                        ]
            , it "styles search border and input text colour" <|
                Query.find [ id "search-bar" ]
                    >> Query.has
                        [ style
                            [ ( "border", searchBarBorder )
                            , ( "color", "#fff" )
                            , ( "font-size", "1.15em" )
                            , ( "font-family", "Inconsolata, monospace" )
                            ]
                        ]
            , it "renders search with appropriate size and padding" <|
                Query.find [ id "search-bar" ]
                    >> Query.has [ style [ ( "height", searchBarHeight ), ( "width", searchBarWidth ), ( "padding", searchBarPadding ) ] ]
            , it "does not have an outline when focused" <|
                Query.find [ id "search-bar" ]
                    >> Query.has [ style [ ( "outline", "0" ) ] ]
            , it "has placeholder text" <|
                Query.find [ id "search-bar" ]
                    >> Query.has [ tag "input", attribute <| Attr.placeholder "search" ]
            , it "has a search container" <|
                Query.has [ id "search-container" ]
            , it "search container is positioned appropriately" <|
                Query.find [ id "search-container" ]
                    >> Query.has
                        [ style
                            [ ( "position", "relative" )
                            , ( "display", "flex" )
                            , ( "flex-direction", "column" )
                            , ( "align-items", "stretch" )
                            ]
                        ]
            , it "has a clear search button container" <|
                Query.find [ id "search-container" ]
                    >> Query.has [ id "search-clear" ]
            , it "positions the clear search button correctly" <|
                Query.find [ id "search-container" ]
                    >> Query.has [ id "search-clear" ]
            , it "has the appropriate background image for clear search and is in correct position" <|
                Query.find [ id "search-clear" ]
                    >> Query.has
                        [ style
                            [ ( "background-image", "url('public/images/ic-close-white-24px.svg')" )
                            , ( "background-position", "10px 10px" )
                            , ( "background-repeat", "no-repeat" )
                            ]
                        ]
            , it "clear search button has no border and renders text appropriately" <|
                Query.find [ id "search-clear" ]
                    >> Query.has
                        [ style
                            [ ( "border", "0" )
                            , ( "color", searchBarGrey )
                            ]
                        ]
            , it "clear search button is positioned appropriately" <|
                Query.find [ id "search-clear" ]
                    >> Query.has
                        [ style
                            [ ( "position", "absolute" )
                            , ( "right", "0" )
                            , ( "padding", "17px" )
                            ]
                        ]
            , it "sets opacity for the clear search button to low when there is no text" <|
                Query.find [ id "search-clear" ]
                    >> Query.has [ style [ ( "opacity", "0.2" ) ] ]
            ]
        , rspecStyleDescribe "when search query is updated"
            (NewestTopBar.init (initFlags (Routes.Dashboard { isHd = False }) "")
                |> Tuple.first
            )
            [ it "search item is modified" <|
                NewestTopBar.update (Msgs.FilterMsg "test")
                    >> Tuple.first
                    >> NewestTopBar.query
                    >> Expect.equal "test"
            , it "shows the list of statuses when `status:` is typed in the search bar" <|
                NewestTopBar.update Msgs.FocusMsg
                    >> Tuple.first
                    >> NewestTopBar.update (Msgs.FilterMsg "status:")
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.findAll [ tag "li" ]
                    >> Expect.all
                        [ Query.count (Expect.equal 7)
                        , Query.index 0 >> Query.has [ text "status: paused" ]
                        , Query.index 1 >> Query.has [ text "status: pending" ]
                        , Query.index 2 >> Query.has [ text "status: failed" ]
                        , Query.index 3 >> Query.has [ text "status: errored" ]
                        , Query.index 4 >> Query.has [ text "status: aborted" ]
                        , Query.index 5 >> Query.has [ text "status: running" ]
                        , Query.index 6 >> Query.has [ text "status: succeeded" ]
                        ]
            , it "after typing `status: pending` the dropdown is empty" <|
                NewestTopBar.update Msgs.FocusMsg
                    >> Tuple.first
                    >> NewestTopBar.update (Msgs.FilterMsg "status:")
                    >> Tuple.first
                    >> NewestTopBar.update (Msgs.FilterMsg "status: pending")
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.findAll [ id "search-dropdown" ]
                    >> Query.first
                    >> Query.children []
                    >> Query.count (Expect.equal 0)
            ]
        , rspecStyleDescribe "when search query is `status:`"
            (NewestTopBar.init (initFlags (Routes.Dashboard { isHd = False }) "search=status:")
                |> Tuple.first
            )
            [ it "should display a dropdown of status options when the search bar is focused" <|
                NewestTopBar.update Msgs.FocusMsg
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.findAll [ tag "li" ]
                    >> Expect.all
                        [ Query.count (Expect.equal 7)
                        , Query.index 0 >> Query.has [ text "status: paused" ]
                        , Query.index 1 >> Query.has [ text "status: pending" ]
                        , Query.index 2 >> Query.has [ text "status: failed" ]
                        , Query.index 3 >> Query.has [ text "status: errored" ]
                        , Query.index 4 >> Query.has [ text "status: aborted" ]
                        , Query.index 5 >> Query.has [ text "status: running" ]
                        , Query.index 6 >> Query.has [ text "status: succeeded" ]
                        ]
            ]
        , rspecStyleDescribe "when the search query is `team:`"
            (NewestTopBar.init (initFlags (Routes.Dashboard { isHd = False }) "search=team:")
                |> Tuple.first
            )
            [ it "when the user is not logged in the dropdown is empty" <|
                NewestTopBar.update Msgs.FocusMsg
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.children []
                    >> Query.count (Expect.equal 0)
            , it "when the user is logged in, and there are teams, the dropdown displays them" <|
                NewestTopBar.update Msgs.FocusMsg
                    >> Tuple.first
                    >> NewestTopBar.handleCallback (Callback.TeamsFetched (Ok [ Concourse.Team 1 "team1", Concourse.Team 2 "team2" ]))
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.children []
                    >> Expect.all
                        [ Query.count (Expect.equal 2)
                        , Query.first >> Query.has [ tag "li", text "team1" ]
                        , Query.index 1 >> Query.has [ tag "li", text "team2" ]
                        ]
            , it "when there are teams, the dropdown only displays the first 10" <|
                NewestTopBar.update Msgs.FocusMsg
                    >> Tuple.first
                    >> NewestTopBar.handleCallback
                        (Callback.TeamsFetched
                            (Ok
                                [ Concourse.Team 1 "team1"
                                , Concourse.Team 2 "team2"
                                , Concourse.Team 3 "team3"
                                , Concourse.Team 4 "team4"
                                , Concourse.Team 5 "team5"
                                , Concourse.Team 6 "team6"
                                , Concourse.Team 7 "team7"
                                , Concourse.Team 8 "team8"
                                , Concourse.Team 9 "team9"
                                , Concourse.Team 10 "team10"
                                , Concourse.Team 11 "team11"
                                ]
                            )
                        )
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.children []
                    >> Query.count (Expect.equal 10)
            ]
        , rspecStyleDescribe "dropdown stuff"
            (NewestTopBar.init (initFlags (Routes.Dashboard { isHd = False }) "")
                |> Tuple.first
            )
            [ context "before receiving FocusMsg"
                (NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                )
                [ it "has no dropdown" <|
                    Query.findAll [ id "search-dropdown" ]
                        >> Query.count (Expect.equal 0)
                , it "sends FocusMsg when focusing on search bar" <|
                    Query.find [ id "search-bar" ]
                        >> Event.simulate Event.focus
                        >> Event.expect Msgs.FocusMsg
                ]
            , context "after receiving FocusMsg"
                (NewestTopBar.update Msgs.FocusMsg
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                )
                [ it "has a dropdown when search bar is focused" <|
                    Query.find [ id "search-container" ]
                        >> Query.has [ id "search-dropdown" ]
                , it "should trigger a FilterMsg when typing in the search bar" <|
                    Query.find [ id "search-bar" ]
                        >> Event.simulate (Event.input "test")
                        >> Event.expect (Msgs.FilterMsg "test")
                , context "dropdown elements"
                    (Query.findAll [ tag "li" ])
                    [ it "have the same width and padding as search bar" <|
                        eachHasStyle
                            [ ( "width", searchBarWidth )
                            , ( "padding", searchBarPadding )
                            ]
                    , it "have grey background" <|
                        eachHasStyle
                            [ ( "background-color", dropdownBackgroundGrey )
                            ]
                    , it "have the same height as the search bar" <|
                        eachHasStyle
                            [ ( "line-height", searchBarHeight )
                            ]
                    , it "have no bullet points" <|
                        eachHasStyle
                            [ ( "list-style-type", "none" )
                            ]
                    , it "have the same border style as the search bar" <|
                        eachHasStyle
                            [ ( "border", searchBarBorder )
                            ]
                    , it "are vertically aligned flush to each other" <|
                        eachHasStyle
                            [ ( "margin-top", "-1px" )
                            ]
                    , it "have light grey text" <|
                        eachHasStyle
                            [ ( "color", "#9b9b9b" )
                            ]
                    , it "have slightly larger font" <|
                        eachHasStyle
                            [ ( "font-size", "1.15em" )
                            ]
                    , it "have a pointer cursor" <|
                        eachHasStyle
                            [ ( "cursor", "pointer" ) ]
                    ]
                , it "the search dropdown is positioned below the search bar" <|
                    Query.find [ id "search-dropdown" ]
                        >> Query.has
                            [ style
                                [ ( "position", "absolute" )
                                , ( "top", "100%" )
                                , ( "margin-top", "0" )
                                ]
                            ]
                , it "the search dropdown has 2 elements" <|
                    Query.find [ id "search-dropdown" ]
                        >> Expect.all
                            [ Query.findAll [ tag "li" ] >> Query.count (Expect.equal 2)
                            , Query.has [ text "status:" ]
                            , Query.has [ text "team:" ]
                            ]
                , it "when team is clicked, it should trigger a FilterMsg for team" <|
                    Query.find [ id "search-dropdown" ]
                        >> Query.find [ tag "li", containing [ text "team:" ] ]
                        >> Event.simulate Event.mouseDown
                        >> Event.expect (Msgs.FilterMsg "team:")
                , it "when status is clicked, it should trigger a FilterMsg for status" <|
                    Query.find [ id "search-dropdown" ]
                        >> Query.find [ tag "li", containing [ text "status:" ] ]
                        >> Event.simulate Event.mouseDown
                        >> Event.expect (Msgs.FilterMsg "status:")
                , it "sends BlurMsg when blurring the search bar" <|
                    Query.find [ id "search-bar" ]
                        >> Event.simulate Event.blur
                        >> Event.expect Msgs.BlurMsg
                ]
            , context "after receiving FocusMsg and then BlurMsg"
                (NewestTopBar.update Msgs.FocusMsg
                    >> Tuple.first
                    >> NewestTopBar.update Msgs.BlurMsg
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                )
                [ it "hides the dropdown" <|
                    Query.findAll [ id "search-dropdown" ]
                        >> Query.count (Expect.equal 0)
                ]
            ]
        ]


eachHasStyle : List ( String, String ) -> Query.Multiple msg -> Expectation
eachHasStyle styles =
    Query.each <| Query.has [ style styles ]


logoutUser : NewestTopBar.Model -> NewestTopBar.Model
logoutUser model =
    { model | userState = UserState.UserStateLoggedOut }


logInUser : NewestTopBar.Model -> NewestTopBar.Model
logInUser model =
    { model | userState = UserState.UserStateLoggedIn { id = "1", userName = "test", name = "Bob", email = "bob@bob.com", teams = Dict.empty } }


pipelineBreadcrumbSelector : List Selector.Selector
pipelineBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic-breadcrumb-pipeline.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


jobBreadcrumbSelector : List Selector.Selector
jobBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic-breadcrumb-job.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


resourceBreadcrumbSelector : List Selector.Selector
resourceBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic-breadcrumb-resource.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]
