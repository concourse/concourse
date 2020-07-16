module TopBarTests exposing (all)

import Application.Application as Application
import Assets
import Char
import Common exposing (defineHoverBehaviour, queryView)
import Concourse
import Dashboard.SearchBar as SearchBar
import DashboardTests exposing (iconSelector)
import Data
import Dict
import Expect exposing (..)
import Html.Attributes as Attr
import Http
import Keyboard
import Login.Login as Login
import Message.Callback as Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message as Msgs
import Message.Subscription exposing (Delivery(..))
import Message.TopLevelMessage as ApplicationMsgs
import Routes
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
import Time
import Url
import Views.Styles


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


update : Msgs.Message -> Login.Model {} -> ( Login.Model {}, List Effects.Effect )
update msg =
    (\a -> ( a, [] )) >> Login.update msg


lineHeight : String
lineHeight =
    "54px"


borderGrey : String
borderGrey =
    "#3d3c3c"


backgroundGrey : String
backgroundGrey =
    "#1e1d1d"


pausedBlue : String
pausedBlue =
    "#3498db"


almostWhite : String
almostWhite =
    "rgba(255, 255, 255, 0.5)"


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


flags : Application.Flags
flags =
    { turbulenceImgSrc = ""
    , notFoundImgSrc = ""
    , csrfToken = ""
    , authToken = ""
    , pipelineRunningKeyframes = ""
    }


all : Test
all =
    describe "TopBar"
        [ rspecStyleDescribe "when on pipeline page"
            (Common.init "/teams/team/pipelines/pipeline")
            [ context "when login state unknown"
                queryView
                [ it "shows concourse logo" <|
                    Query.has
                        [ style "background-image" <|
                            Assets.backgroundImage <|
                                Just Assets.ConcourseLogoWhite
                        , style "background-position" "50% 50%"
                        , style "background-repeat" "no-repeat"
                        , style "background-size" "42px 42px"
                        , style "width" topBarHeight
                        , style "height" topBarHeight
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
                    , it "has pointer cursor" <|
                        Query.has [ style "cursor" "pointer" ]
                    , it "is a link to the relevant pipeline page" <|
                        Query.has
                            [ tag "a"
                            , attribute <|
                                Attr.href
                                    "/teams/team/pipelines/pipeline"
                            ]
                    ]
                , it "has dark grey background" <|
                    Query.has [ style "background-color" backgroundGrey ]
                , it "lays out contents horizontally" <|
                    Query.has [ style "display" "flex" ]
                , it "maximizes spacing between the left and right navs" <|
                    Query.has [ style "justify-content" "space-between" ]
                , it "renders the login component last" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.has [ id "login-component" ]
                ]
            , context "when logged out"
                (Application.handleCallback
                    (Callback.UserFetched <| Data.httpUnauthorized)
                    >> Tuple.first
                    >> queryView
                )
                [ it "renders the login component last" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.has [ id "login-component" ]
                , it "has a link to login" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-item" ]
                        >> Query.has [ tag "a", attribute <| Attr.href "/sky/login" ]
                ]
            , context "when logged in"
                (Application.handleCallback
                    (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> queryView
                )
                [ it "renders the login component last" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.has [ id "login-component" ]
                , it "renders login component with a maximum width" <|
                    Query.find [ id "login-component" ]
                        >> Query.has [ style "max-width" "20%" ]
                , it "renders login container with relative position" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style "position" "relative" ]
                , it "lays out login container contents vertically" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style "display" "flex"
                            , style "flex-direction" "column"
                            ]
                , it "draws lighter grey line to the left of login container" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style "border-left" <| "1px solid " ++ borderGrey ]
                , it "renders login container tall enough" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style "line-height" lineHeight ]
                , it "has the login username styles" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "user-id" ]
                        >> Expect.all
                            [ Query.has
                                [ style "padding" "0 30px"
                                , style "cursor" "pointer"
                                , style "display" "flex"
                                , style "align-items" "center"
                                , style "justify-content" "center"
                                , style "flex-grow" "1"
                                ]
                            , Query.children []
                                >> Query.index 0
                                >> Query.has
                                    [ style "overflow" "hidden"
                                    , style "text-overflow" "ellipsis"
                                    ]
                            ]
                , it "shows the logged in username when the user is logged in" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "user-id" ]
                        >> Query.has [ text "test" ]
                , it "Click UserMenu message is received when login menu is clicked" <|
                    Query.find [ id "login-container" ]
                        >> Event.simulate Event.click
                        >> Event.expect
                            (ApplicationMsgs.Update <| Msgs.Click Msgs.UserMenu)
                , it "does not render the logout button" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "user-id" ]
                        >> Query.hasNot [ id "logout-button" ]
                , it "renders pause pipeline button" <|
                    Query.find [ id "top-bar-pause-toggle" ]
                        >> Query.children []
                        >> Query.first
                        >> Query.has
                            [ style "background-image" <|
                                Assets.backgroundImage <|
                                    Just Assets.PauseIcon
                            ]
                , it "draws lighter grey line to the left of pause pipeline button" <|
                    Query.find [ id "top-bar-pause-toggle" ]
                        >> Query.has
                            [ style "border-left" <| "1px solid " ++ borderGrey ]
                ]
            , it "clicking a pinned resource navigates to the pinned resource page" <|
                Application.update
                    (ApplicationMsgs.Update <|
                        Msgs.GoToRoute
                            (Routes.Resource
                                { id =
                                    { teamName = "t"
                                    , pipelineName = "p"
                                    , resourceName = "r"
                                    }
                                , page = Nothing
                                }
                            )
                    )
                    >> Tuple.second
                    >> Expect.equal
                        [ Effects.NavigateTo <|
                            Routes.toString <|
                                Routes.Resource
                                    { id =
                                        { teamName = "t"
                                        , pipelineName = "p"
                                        , resourceName = "r"
                                        }
                                    , page = Nothing
                                    }
                        ]
            , context "when pipeline is paused"
                (Application.handleCallback
                    (Callback.PipelineFetched <|
                        Ok <|
                            (Data.pipeline "t" 0
                                |> Data.withName "p"
                                |> Data.withPaused True
                            )
                    )
                    >> Tuple.first
                    >> Application.handleCallback
                        (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> queryView
                )
                [ it "has blue background" <|
                    Query.has [ style "background-color" pausedBlue ]
                , it "draws almost-white line to the left of login container" <|
                    Query.find [ id "login-container" ]
                        >> Query.has
                            [ style "border-left" <| "1px solid " ++ almostWhite ]
                ]
            , context "when pipeline is archived"
                (Application.handleCallback
                    (Callback.PipelineFetched <|
                        Ok <|
                            (Data.pipeline "t" 0
                                |> Data.withName "p"
                                |> Data.withPaused True
                                |> Data.withArchived True
                            )
                    )
                    >> Tuple.first
                    >> Application.handleCallback
                        (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> queryView
                )
                [ it "does not render pause toggle" <|
                    Query.hasNot [ id "top-bar-pause-toggle" ]
                , it "draws uses the normal border colour for the login container" <|
                    Query.find [ id "login-container" ]
                        >> Query.has
                            [ style "border-left" <| "1px solid " ++ borderGrey ]
                ]
            ]
        , rspecStyleDescribe "rendering user menus on clicks"
            (Common.init "/teams/team/pipelines/pipeline")
            [ it "shows user menu when ToggleUserMenu msg is received" <|
                Application.handleCallback
                    (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update <| Msgs.Click Msgs.UserMenu)
                    >> Tuple.first
                    >> queryView
                    >> Query.has [ id "logout-button" ]
            , it "renders user menu content when click UserMenu msg is received and logged in" <|
                Application.handleCallback
                    (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update <| Msgs.Click Msgs.UserMenu)
                    >> Tuple.first
                    >> queryView
                    >> Expect.all
                        [ Query.has [ id "logout-button" ]
                        , Query.find [ id "logout-button" ]
                            >> Query.has [ text "logout" ]
                        , Query.find [ id "logout-button" ]
                            >> Query.has
                                [ style "position" "absolute"
                                , style "top" "55px"
                                , style "background-color" backgroundGrey
                                , style "height" topBarHeight
                                , style "width" "100%"
                                , style "border-top" <| "1px solid " ++ borderGrey
                                , style "cursor" "pointer"
                                , style "display" "flex"
                                , style "align-items" "center"
                                , style "justify-content" "center"
                                , style "flex-grow" "1"
                                ]
                        ]
            , it "when logout is clicked, a Click LogoutButton msg is sent" <|
                Application.handleCallback
                    (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update <| Msgs.Click Msgs.UserMenu)
                    >> Tuple.first
                    >> queryView
                    >> Query.find [ id "logout-button" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (ApplicationMsgs.Update <| Msgs.Click Msgs.LogoutButton)
            , it "shows 'login' when LoggedOut TopLevelMessage is successful" <|
                Application.handleCallback
                    (Callback.LoggedOut <| Ok ())
                    >> Tuple.first
                    >> queryView
                    >> Query.find [ id "login-item" ]
                    >> Query.has [ text "login" ]
            ]
        , rspecStyleDescribe "login component when user is logged out"
            (Common.init "/teams/team/pipelines/pipeline"
                |> Application.handleCallback
                    (Callback.LoggedOut (Ok ()))
                |> Tuple.first
                |> queryView
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
                        [ style "position" "relative"
                        , style "display" "flex"
                        , style "flex-direction" "column"
                        , style "border-left" <| "1px solid " ++ borderGrey
                        , style "line-height" lineHeight
                        ]
            , it "has the login username styles" <|
                Query.children []
                    >> Query.index -1
                    >> Query.find [ id "login-item" ]
                    >> Query.has
                        [ style "padding" "0 30px"
                        , style "cursor" "pointer"
                        , style "display" "flex"
                        , style "align-items" "center"
                        , style "justify-content" "center"
                        , style "flex-grow" "1"
                        ]
            ]
        , rspecStyleDescribe "when triggering a log in message"
            (Common.init "/"
                |> Application.handleCallback
                    (Callback.LoggedOut (Ok ()))
            )
            [ it "redirects to login page when you click login" <|
                Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update <| Msgs.Click Msgs.LoginButton)
                    >> Tuple.second
                    >> Expect.equal [ Effects.RedirectToLogin ]
            ]
        , rspecStyleDescribe "rendering top bar on build page"
            (Common.init "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                |> queryView
            )
            [ it "should pad the breadcrumbs to max size so they can be left-aligned" <|
                Query.find
                    [ id "breadcrumbs" ]
                    >> Query.has [ style "flex-grow" "1" ]
            , it "pipeline breadcrumb should have a link to the pipeline page when viewing build details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.has
                        [ tag "a"
                        , attribute <|
                            Attr.href
                                "/teams/team/pipelines/pipeline"
                        ]
            , context "job breadcrumb"
                (Query.find [ id "breadcrumb-job" ])
                [ it "is laid out horizontally with appropriate spacing" <|
                    Query.has
                        [ style "display" "inline-block"
                        , style "padding" "0 10px"
                        ]
                , it "has job icon rendered first" <|
                    Query.has jobBreadcrumbSelector
                , it "has build name after job icon" <|
                    Query.has [ text "job" ]
                , it "does not appear clickable" <|
                    Query.hasNot [ style "cursor" "pointer" ]
                ]
            ]
        , rspecStyleDescribe "rendering top bar on resource page"
            (Common.init "/teams/team/pipelines/pipeline/resources/resource"
                |> queryView
            )
            [ it "should pad the breadcrumbs to max size so they can be left-aligned" <|
                Query.find
                    [ id "breadcrumbs" ]
                    >> Query.has [ style "flex-grow" "1" ]
            , it "pipeline breadcrumb should have a link to the pipeline page when viewing resource details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.has
                        [ tag "a"
                        , attribute <|
                            Attr.href
                                "/teams/team/pipelines/pipeline"
                        ]
            , it "there is a / between pipeline and resource in breadcrumb" <|
                Query.find [ id "breadcrumbs" ]
                    >> Query.children []
                    >> Expect.all
                        [ Query.index 1
                            >> Query.has [ class "breadcrumb-separator" ]
                        , Query.index 1 >> Query.has [ text "/" ]
                        , Query.index 2 >> Query.has [ id "breadcrumb-resource" ]
                        ]
            , it "resource breadcrumb is laid out horizontally with appropriate spacing" <|
                Query.find [ id "breadcrumb-resource" ]
                    >> Query.has
                        [ style "display" "inline-block"
                        , style "padding" "0 10px"
                        ]
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
            (Common.init "/teams/team/pipelines/pipeline/jobs/job"
                |> queryView
            )
            [ it "should pad the breadcrumbs to max size so they can be left-aligned" <|
                Query.find
                    [ id "breadcrumbs" ]
                    >> Query.has [ style "flex-grow" "1" ]
            , it "pipeline breadcrumb should have a link to the pipeline page when viewing job details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.has
                        [ tag "a"
                        , attribute <|
                            Attr.href
                                "/teams/team/pipelines/pipeline"
                        ]
            , it "there is a / between pipeline and job in breadcrumb" <|
                Query.find [ id "breadcrumbs" ]
                    >> Query.children []
                    >> Expect.all
                        [ Query.index 1
                            >> Query.has [ class "breadcrumb-separator" ]
                        , Query.index 0 >> Query.has [ id "breadcrumb-pipeline" ]
                        , Query.index 2 >> Query.has [ id "breadcrumb-job" ]
                        ]
            ]
        , rspecStyleDescribe "when checking search bar values"
            (Application.init
                flags
                { protocol = Url.Http
                , host = ""
                , port_ = Nothing
                , path = "/"
                , query = Just "search=test"
                , fragment = Nothing
                }
                |> Tuple.first
                |> Application.handleCallback
                    (Callback.AllTeamsFetched <|
                        Ok
                            [ Concourse.Team 1 "team1"
                            , Concourse.Team 2 "team2"
                            ]
                    )
                |> Tuple.first
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok
                            [ Data.pipeline "team1" 0 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
            )
            [ it "renders the search bar with the text in the search query" <|
                queryView
                    >> Query.find [ id SearchBar.searchInputId ]
                    >> Query.has [ tag "input", attribute <| Attr.value "test" ]
            , it "sends a click msg when the clear search button is clicked" <|
                queryView
                    >> Query.find [ id "search-container" ]
                    >> Query.find [ id "search-clear" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (ApplicationMsgs.Update <|
                            Msgs.Click Msgs.ClearSearchButton
                        )
            , it "click msg clears the search input" <|
                Application.update
                    (ApplicationMsgs.Update <|
                        Msgs.Click Msgs.ClearSearchButton
                    )
                    >> Tuple.first
                    >> queryView
                    >> Query.find [ id "search-input-field" ]
                    >> Query.has [ attribute <| Attr.value "" ]
            , it "clear search button has full opacity when there is a query" <|
                queryView
                    >> Query.find [ id "search-clear" ]
                    >> Query.has [ style "opacity" "1" ]
            ]
        , rspecStyleDescribe "rendering search bar on dashboard page"
            (Common.init "/"
                |> Application.handleCallback
                    (Callback.AllTeamsFetched <|
                        Ok
                            [ Concourse.Team 1 "team1"
                            , Concourse.Team 2 "team2"
                            ]
                    )
                |> Tuple.first
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok
                            [ Data.pipeline "team1" 0 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
            )
            [ context "when desktop sized"
                (Application.handleCallback
                    (ScreenResized
                        { scene = { width = 0, height = 0 }
                        , viewport = { x = 0, y = 0, width = 1500, height = 900 }
                        }
                    )
                    >> Tuple.first
                    >> queryView
                )
                [ it "renders search bar" <|
                    Query.has [ id SearchBar.searchInputId ]
                , it "search bar is an input field" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has [ tag "input" ]
                , it "renders search bar with transparent background to remove white of search bar" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has [ style "background-color" "transparent" ]
                , it "search bar does not use browser's built-in autocomplete" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has
                            [ attribute <| Attr.attribute "autocomplete" "off" ]
                , it "sets magnifying glass on search bar in correct position" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has
                            [ style "background-image" <|
                                Assets.backgroundImage <|
                                    Just Assets.SearchIcon
                            , style "background-position" "12px 8px"
                            , style "background-repeat" "no-repeat"
                            ]
                , it "styles search border and input text colour" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has
                            [ style "border" searchBarBorder
                            , style "color" "#ffffff"
                            , style "font-size" "1.15em"
                            , style "font-family" Views.Styles.fontFamilyDefault
                            ]
                , it "renders search with appropriate size and padding" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has
                            [ style "height" searchBarHeight
                            , style "width" searchBarWidth
                            , style "padding" searchBarPadding
                            ]
                , it "does not have an outline when focused" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has [ style "outline" "0" ]
                , it "has placeholder text" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has [ tag "input", attribute <| Attr.placeholder "search" ]
                , it "has a wrapper for top bar content" <|
                    Query.has
                        [ id "top-bar-content"
                        , containing [ id "search-container" ]
                        ]
                , it "top bar content wrapper fills available space" <|
                    Query.find [ id "top-bar-content" ]
                        >> Query.has [ style "flex-grow" "1" ]
                , it "top bar content wrapper centers its content" <|
                    Query.find [ id "top-bar-content" ]
                        >> Query.has
                            [ style "display" "flex"
                            , style "justify-content" "center"
                            ]
                , it "search container is positioned appropriately" <|
                    Query.find [ id "search-container" ]
                        >> Expect.all
                            [ Query.has
                                [ style "position" "relative"
                                , style "display" "flex"
                                , style "flex-direction" "column"
                                , style "align-items" "stretch"
                                ]
                            , Query.hasNot [ style "flex-grow" "1" ]
                            ]
                , it "search container is sized correctly" <|
                    Query.find [ id "search-container" ]
                        >> Expect.all
                            [ Query.has [ style "margin" "12px" ]
                            , Query.hasNot [ style "height" "56px" ]
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
                            [ style "background-image" <|
                                Assets.backgroundImage <|
                                    Just Assets.CloseIcon
                            , style "background-position" "10px 10px"
                            , style "background-repeat" "no-repeat"
                            ]
                , it "clear search button has no border and renders text appropriately" <|
                    Query.find [ id "search-clear" ]
                        >> Query.has
                            [ style "border" "0"
                            , style "color" searchBarGrey
                            ]
                , it "clear search button is positioned appropriately" <|
                    Query.find [ id "search-clear" ]
                        >> Query.has
                            [ style "position" "absolute"
                            , style "right" "0"
                            , style "padding" "17px"
                            ]
                , it "sets opacity for the clear search button to low when there is no text" <|
                    Query.find [ id "search-clear" ]
                        >> Query.has [ style "opacity" "0.2" ]
                ]
            , context "when mobile sized"
                (Application.handleCallback
                    (ScreenResized
                        { scene = { width = 0, height = 0 }
                        , viewport = { x = 0, y = 0, width = 400, height = 900 }
                        }
                    )
                    >> Tuple.first
                )
                [ it "should not have a search bar" <|
                    queryView
                        >> Query.hasNot
                            [ id SearchBar.searchInputId ]
                , it "should have a magnifying glass icon" <|
                    queryView
                        >> Query.find [ id "show-search-button" ]
                        >> Query.has
                            [ style "background-image" <|
                                Assets.backgroundImage <|
                                    Just Assets.SearchIcon
                            , style "background-position" "12px 8px"
                            , style "background-repeat" "no-repeat"
                            ]
                , it "shows the login component" <|
                    queryView
                        >> Query.has [ id "login-component" ]
                , context "after clicking the search icon"
                    (Application.update
                        (ApplicationMsgs.Update <|
                            Msgs.Click Msgs.ShowSearchButton
                        )
                    )
                    [ it "tells the ui to focus on the search bar" <|
                        Tuple.second
                            >> Expect.equal
                                [ Effects.Focus SearchBar.searchInputId ]
                    , context "the ui"
                        (Tuple.first
                            >> queryView
                        )
                        [ it "renders search bar" <|
                            Query.has [ id SearchBar.searchInputId ]
                        , it "search bar is an input field" <|
                            Query.find [ id SearchBar.searchInputId ]
                                >> Query.has [ tag "input" ]
                        , it "has placeholder text" <|
                            Query.find [ id SearchBar.searchInputId ]
                                >> Query.has [ tag "input", attribute <| Attr.placeholder "search" ]
                        , it "has a search container" <|
                            Query.has [ id "search-container" ]
                        , it "positions the search container appropriately" <|
                            Query.find [ id "search-container" ]
                                >> Query.has
                                    [ style "position" "relative"
                                    , style "display" "flex"
                                    , style "flex-direction" "column"
                                    , style "align-items" "stretch"
                                    , style "flex-grow" "1"
                                    ]
                        , it "search container is sized correctly" <|
                            Query.find [ id "search-container" ]
                                >> Expect.all
                                    [ Query.has [ style "margin" "12px" ]
                                    , Query.hasNot [ style "height" "56px" ]
                                    ]
                        , it "has a clear search button container" <|
                            Query.find [ id "search-container" ]
                                >> Query.has [ id "search-clear" ]
                        , it "has the appropriate background image for clear search and is in correct position" <|
                            Query.find [ id "search-clear" ]
                                >> Query.has
                                    [ style "background-image" <|
                                        Assets.backgroundImage <|
                                            Just Assets.CloseIcon
                                    , style "background-position" "10px 10px"
                                    , style "background-repeat" "no-repeat"
                                    ]
                        , it "hides the login component" <|
                            Query.hasNot [ id "login-component" ]
                        ]
                    , context "after the focus returns"
                        (Tuple.first
                            >> Application.update
                                (ApplicationMsgs.Update Msgs.FocusMsg)
                            >> Tuple.first
                        )
                        [ it "should display a dropdown of options" <|
                            queryView
                                >> Query.find [ id "search-dropdown" ]
                                >> Query.findAll [ tag "li" ]
                                >> Expect.all
                                    [ Query.count (Expect.equal 2)
                                    , Query.index 0 >> Query.has [ text "status: " ]
                                    , Query.index 1 >> Query.has [ text "team: " ]
                                    ]
                        , it "the search dropdown is positioned below the search bar" <|
                            queryView
                                >> Query.find [ id "search-dropdown" ]
                                >> Expect.all
                                    [ Query.has
                                        [ style "top" "100%"
                                        , style "margin" "0"
                                        ]
                                    , Query.hasNot [ style "position" "absolute" ]
                                    ]
                        , it "the search dropdown is the same width as search bar" <|
                            queryView
                                >> Query.find [ id "search-dropdown" ]
                                >> Query.has [ style "width" "100%" ]
                        , context "after the search is blurred"
                            (Application.update
                                (ApplicationMsgs.Update Msgs.BlurMsg)
                                >> Tuple.first
                                >> queryView
                            )
                            [ it "should not have a search bar" <|
                                Query.hasNot
                                    [ id SearchBar.searchInputId ]
                            , it "should have a magnifying glass icon" <|
                                Query.find [ id "show-search-button" ]
                                    >> Query.has
                                        [ style "background-image" <|
                                            Assets.backgroundImage <|
                                                Just Assets.SearchIcon
                                        , style "background-position" "12px 8px"
                                        , style "background-repeat" "no-repeat"
                                        ]
                            , it "shows the login component" <|
                                Query.has [ id "login-component" ]
                            ]
                        , context "after the search is blurred with a search query"
                            (Application.update
                                (ApplicationMsgs.Update <|
                                    Msgs.FilterMsg "query"
                                )
                                >> Tuple.first
                                >> Application.update
                                    (ApplicationMsgs.Update <| Msgs.BlurMsg)
                                >> Tuple.first
                                >> queryView
                            )
                            [ it "should have a search bar" <|
                                Query.has [ id SearchBar.searchInputId ]
                            , it "should not have a magnifying glass icon" <|
                                Query.hasNot [ id "show-search-button" ]
                            , it "should not show the login component" <|
                                Query.hasNot [ id "login-component" ]
                            , it "should not display a dropdown of options" <|
                                Query.hasNot [ id "search-dropdown" ]
                            ]
                        ]
                    ]
                ]
            ]
        , rspecStyleDescribe "when search query is updated"
            (Common.init "/"
                |> Application.handleCallback
                    (Callback.AllTeamsFetched <|
                        Ok
                            [ Concourse.Team 1 "team1"
                            , Concourse.Team 2 "team2"
                            ]
                    )
                |> Tuple.first
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok
                            [ Data.pipeline "team1" 0 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
            )
            [ it
                ("shows the list of statuses when "
                    ++ "`status:` is typed in the search bar"
                )
              <|
                Application.update
                    (ApplicationMsgs.Update Msgs.FocusMsg)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update <|
                            Msgs.FilterMsg "status:"
                        )
                    >> Tuple.first
                    >> queryView
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
                Application.update
                    (ApplicationMsgs.Update Msgs.FocusMsg)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update <| Msgs.FilterMsg "status:")
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update <|
                            Msgs.FilterMsg "status: pending"
                        )
                    >> Tuple.first
                    >> queryView
                    >> Query.findAll [ id "search-dropdown" ]
                    >> Query.first
                    >> Query.children []
                    >> Query.count (Expect.equal 0)
            ]
        , rspecStyleDescribe "when search query is `status:`"
            (Application.init
                flags
                { protocol = Url.Http
                , host = ""
                , port_ = Nothing
                , path = "/"
                , query = Just "search=status:"
                , fragment = Nothing
                }
                |> Tuple.first
                |> Application.handleCallback
                    (Callback.AllTeamsFetched <|
                        Ok
                            [ Concourse.Team 1 "team1"
                            , Concourse.Team 2 "team2"
                            ]
                    )
                |> Tuple.first
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok
                            [ Data.pipeline "team1" 0 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
            )
            [ it "should display a dropdown of status options when the search bar is focused" <|
                Application.update
                    (ApplicationMsgs.Update Msgs.FocusMsg)
                    >> Tuple.first
                    >> queryView
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
            (Application.init
                flags
                { protocol = Url.Http
                , host = ""
                , port_ = Nothing
                , path = "/"
                , query = Just "search=team:"
                , fragment = Nothing
                }
                |> Tuple.first
            )
            [ it "when there are teams the dropdown displays them" <|
                Application.handleCallback
                    (Callback.AllTeamsFetched <|
                        Ok
                            [ Concourse.Team 1 "team1", Concourse.Team 2 "team2" ]
                    )
                    >> Tuple.first
                    >> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team1" 0 |> Data.withName "pipeline" ]
                        )
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update Msgs.FocusMsg)
                    >> Tuple.first
                    >> queryView
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.children []
                    >> Expect.all
                        [ Query.count (Expect.equal 2)
                        , Query.first >> Query.has [ tag "li", text "team1" ]
                        , Query.index 1 >> Query.has [ tag "li", text "team2" ]
                        ]
            , it "when there are many teams, the dropdown only displays the first 10" <|
                Application.handleCallback
                    (Callback.AllTeamsFetched <|
                        Ok
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
                    >> Tuple.first
                    >> Application.handleCallback
                        (Callback.AllPipelinesFetched <|
                            Ok
                                [ Data.pipeline "team1" 0 |> Data.withName "pipeline" ]
                        )
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update Msgs.FocusMsg)
                    >> Tuple.first
                    >> queryView
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.children []
                    >> Query.count (Expect.equal 10)
            ]
        , rspecStyleDescribe "dropdown stuff"
            (Common.init "/"
                |> Application.handleCallback
                    (Callback.AllTeamsFetched <|
                        Ok
                            [ { id = 0, name = "team" } ]
                    )
                |> Tuple.first
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok
                            [ Data.pipeline "team" 0 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
            )
            [ context "before receiving FocusMsg"
                queryView
                [ it "has no dropdown" <|
                    Query.findAll [ id "search-dropdown" ]
                        >> Query.count (Expect.equal 0)
                , it "sends FocusMsg when focusing on search bar" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Event.simulate Event.focus
                        >> Event.expect (ApplicationMsgs.Update Msgs.FocusMsg)
                ]
            , it "hitting '/' focuses search input" <|
                Application.update
                    (ApplicationMsgs.DeliveryReceived <|
                        KeyDown
                            { ctrlKey = False
                            , shiftKey = False
                            , metaKey = False
                            , code = Keyboard.Slash
                            }
                    )
                    >> Tuple.second
                    >> Expect.equal [ Effects.Focus SearchBar.searchInputId ]
            , it "hitting shift + '/' (= '?') does not focus search input" <|
                Application.update
                    (ApplicationMsgs.DeliveryReceived <|
                        KeyDown
                            { ctrlKey = False
                            , shiftKey = True
                            , metaKey = False
                            , code = Keyboard.Slash
                            }
                    )
                    >> Tuple.second
                    >> Expect.equal []
            , it "hitting other keys does not cause dropdown to expand" <|
                Application.update
                    (ApplicationMsgs.DeliveryReceived <|
                        KeyDown
                            { ctrlKey = False
                            , shiftKey = False
                            , metaKey = False
                            , code = Keyboard.A
                            }
                    )
                    >> Tuple.first
                    >> queryView
                    >> Query.findAll [ id "search-dropdown" ]
                    >> Query.count (Expect.equal 0)
            , context "after receiving FocusMsg"
                (Application.update (ApplicationMsgs.Update Msgs.FocusMsg))
                ([ testDropdown [] [ 0, 1 ] ]
                    ++ [ context "after down arrow keypress"
                            (Tuple.first
                                >> Application.update
                                    (ApplicationMsgs.DeliveryReceived <|
                                        KeyDown
                                            { ctrlKey = False
                                            , shiftKey = False
                                            , metaKey = False
                                            , code = Keyboard.ArrowDown
                                            }
                                    )
                            )
                            ([ testDropdown [ 0 ] [ 1 ] ]
                                ++ [ context "after second down arrow keypress"
                                        (Tuple.first
                                            >> Application.update
                                                (ApplicationMsgs.DeliveryReceived <|
                                                    KeyDown
                                                        { ctrlKey = False
                                                        , shiftKey = False
                                                        , metaKey = False
                                                        , code = Keyboard.ArrowDown
                                                        }
                                                )
                                        )
                                        ([ testDropdown [ 1 ] [ 0 ] ]
                                            ++ [ context "after loop around down arrow keypress"
                                                    (Tuple.first
                                                        >> Application.update
                                                            (ApplicationMsgs.DeliveryReceived <|
                                                                KeyDown
                                                                    { ctrlKey = False
                                                                    , shiftKey = False
                                                                    , metaKey = False
                                                                    , code = Keyboard.ArrowDown
                                                                    }
                                                            )
                                                    )
                                                    [ testDropdown [ 0 ] [ 1 ] ]
                                               , context "after hitting enter"
                                                    (Tuple.first
                                                        >> Application.update
                                                            (ApplicationMsgs.DeliveryReceived <|
                                                                KeyDown
                                                                    { ctrlKey = False
                                                                    , shiftKey = False
                                                                    , metaKey = False
                                                                    , code = Keyboard.Enter
                                                                    }
                                                            )
                                                        >> viewNormally
                                                    )
                                                    [ it "updates the query" <|
                                                        Query.find [ id SearchBar.searchInputId ]
                                                            >> Query.has [ attribute <| Attr.value "team: " ]
                                                    ]
                                               ]
                                        )
                                   , context "after hitting enter"
                                        (Tuple.first
                                            >> Application.update
                                                (ApplicationMsgs.DeliveryReceived <|
                                                    KeyDown
                                                        { ctrlKey = False
                                                        , shiftKey = False
                                                        , metaKey = False
                                                        , code = Keyboard.Enter
                                                        }
                                                )
                                        )
                                        [ it "updates the query" <|
                                            Tuple.first
                                                >> queryView
                                                >> Query.find
                                                    [ id SearchBar.searchInputId ]
                                                >> Query.has
                                                    [ attribute <|
                                                        Attr.value "status: "
                                                    ]
                                        , it "updates the URL" <|
                                            Tuple.second
                                                >> Expect.equal
                                                    [ Effects.ModifyUrl
                                                        "/?search=status%3A%20"
                                                    ]
                                        ]
                                   ]
                            )
                       , context "after up arrow keypress"
                            (Tuple.first
                                >> Application.update
                                    (ApplicationMsgs.DeliveryReceived <|
                                        KeyDown
                                            { ctrlKey = False
                                            , shiftKey = False
                                            , metaKey = False
                                            , code = Keyboard.ArrowUp
                                            }
                                    )
                            )
                            ([ testDropdown [ 1 ] [ 0 ] ]
                                ++ [ context "after second up arrow keypress"
                                        (Tuple.first
                                            >> Application.update
                                                (ApplicationMsgs.DeliveryReceived <|
                                                    KeyDown
                                                        { ctrlKey = False
                                                        , shiftKey = False
                                                        , metaKey = False
                                                        , code = Keyboard.ArrowUp
                                                        }
                                                )
                                        )
                                        ([ testDropdown [ 0 ] [ 1 ] ]
                                            ++ [ context "after loop around up arrow keypress"
                                                    (Tuple.first
                                                        >> Application.update
                                                            (ApplicationMsgs.DeliveryReceived <|
                                                                KeyDown
                                                                    { ctrlKey = False
                                                                    , shiftKey = False
                                                                    , metaKey = False
                                                                    , code = Keyboard.ArrowUp
                                                                    }
                                                            )
                                                    )
                                                    [ testDropdown [ 1 ] [ 0 ] ]
                                               ]
                                        )
                                   ]
                            )
                       ]
                    ++ [ context "on ESC keypress"
                            (Tuple.first
                                >> Application.update
                                    (ApplicationMsgs.DeliveryReceived <|
                                        KeyDown
                                            { ctrlKey = False
                                            , shiftKey = False
                                            , metaKey = False
                                            , code = Keyboard.Escape
                                            }
                                    )
                            )
                            [ it "search input is blurred" <|
                                Tuple.second
                                    >> Expect.equal [ Effects.Blur SearchBar.searchInputId ]
                            ]
                       ]
                )
            , context "after receiving FocusMsg and then BlurMsg"
                (Application.update (ApplicationMsgs.Update Msgs.FocusMsg)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update Msgs.BlurMsg)
                    >> viewNormally
                )
                [ it "hides the dropdown" <|
                    Query.findAll [ id "search-dropdown" ]
                        >> Query.count (Expect.equal 0)
                ]
            ]
        , rspecStyleDescribe "HD dashboard view"
            (Common.init "/hd"
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok
                            [ Data.pipeline "team1" 0 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
            )
            [ it "renders an empty top bar content that fills width" <|
                queryView
                    >> Query.has
                        [ id "top-bar-content"
                        , style "flex-grow" "1"
                        ]
            ]
        , describe "pause toggle" <|
            let
                givenPipelinePaused =
                    Common.init "/teams/t/pipelines/p"
                        |> Application.handleCallback
                            (Callback.PipelineFetched <|
                                Ok
                                    (Data.pipeline "t" 0
                                        |> Data.withName "p"
                                        |> Data.withPaused True
                                    )
                            )
                        |> Tuple.first

                givenUserAuthorized =
                    Application.handleCallback
                        (Callback.UserFetched <|
                            Ok
                                { id = "test"
                                , userName = "test"
                                , name = "test"
                                , email = "test"
                                , isAdmin = False
                                , teams =
                                    Dict.fromList
                                        [ ( "t", [ "member" ] ) ]
                                }
                        )
                        >> Tuple.first

                givenUserUnauthorized =
                    Application.handleCallback
                        (Callback.UserFetched <|
                            Ok
                                { id = "test"
                                , userName = "test"
                                , name = "test"
                                , email = "test"
                                , isAdmin = False
                                , teams =
                                    Dict.fromList
                                        [ ( "s", [ "member" ] ) ]
                                }
                        )
                        >> Tuple.first

                pipelineIdentifier =
                    { pipelineName = "p"
                    , teamName = "t"
                    }

                toggleMsg =
                    ApplicationMsgs.Update <|
                        Msgs.Click <|
                            Msgs.PipelineButton
                                pipelineIdentifier
            in
            [ defineHoverBehaviour
                { name = "play pipeline icon when authorized"
                , setup = givenPipelinePaused |> givenUserAuthorized
                , query =
                    queryView
                        >> Query.find [ id "top-bar-pause-toggle" ]
                        >> Query.children []
                        >> Query.first
                , unhoveredSelector =
                    { description = "faded play button with light border"
                    , selector =
                        [ style "opacity" "0.5"
                        , style "margin" "17px"
                        , style "cursor" "pointer"
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = Assets.PlayIcon
                                }
                    }
                , hoveredSelector =
                    { description = "white play button with light border"
                    , selector =
                        [ style "opacity" "1"
                        , style "margin" "17px"
                        , style "cursor" "pointer"
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = Assets.PlayIcon
                                }
                    }
                , hoverable =
                    Msgs.PipelineButton { pipelineName = "p", teamName = "t" }
                }
            , defineHoverBehaviour
                { name = "play pipeline icon when unauthenticated"
                , setup = givenPipelinePaused
                , query =
                    queryView
                        >> Query.find [ id "top-bar-pause-toggle" ]
                        >> Query.children []
                        >> Query.first
                , unhoveredSelector =
                    { description = "faded play button with light border"
                    , selector =
                        [ style "opacity" "0.5"
                        , style "margin" "17px"
                        , style "cursor" "pointer"
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = Assets.PlayIcon
                                }
                    }
                , hoveredSelector =
                    { description = "white play button with light border"
                    , selector =
                        [ style "opacity" "1"
                        , style "margin" "17px"
                        , style "cursor" "pointer"
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = Assets.PlayIcon
                                }
                    }
                , hoverable =
                    Msgs.PipelineButton { pipelineName = "p", teamName = "t" }
                }
            , defineHoverBehaviour
                { name = "play pipeline icon when unauthorized"
                , setup = givenPipelinePaused |> givenUserUnauthorized
                , query =
                    queryView
                        >> Query.find [ id "top-bar-pause-toggle" ]
                        >> Query.children []
                        >> Query.first
                , unhoveredSelector =
                    { description = "faded play button with light border"
                    , selector =
                        [ style "opacity" "0.2"
                        , style "margin" "17px"
                        , style "cursor" "default"
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = Assets.PlayIcon
                                }
                    }
                , hoveredSelector =
                    { description = "faded play button with tooltip below"
                    , selector =
                        [ containing
                            ([ style "cursor" "default"
                             , style "opacity" "0.2"
                             ]
                                ++ iconSelector
                                    { size = "20px"
                                    , image = Assets.PlayIcon
                                    }
                            )
                        , containing
                            [ style "position" "absolute"
                            , style "top" "100%"
                            ]
                        , style "position" "relative"
                        , style "margin" "17px"
                        ]
                    }
                , hoverable =
                    Msgs.PipelineButton { pipelineName = "p", teamName = "t" }
                }
            , test "clicking play button sends TogglePipelinePaused msg" <|
                \_ ->
                    givenPipelinePaused
                        |> queryView
                        |> Query.find [ id "top-bar-pause-toggle" ]
                        |> Query.children []
                        |> Query.first
                        |> Event.simulate Event.click
                        |> Event.expect toggleMsg
            , test "play button unclickable for non-members" <|
                \_ ->
                    givenPipelinePaused
                        |> givenUserUnauthorized
                        |> queryView
                        |> Query.find [ id "top-bar-pause-toggle" ]
                        |> Query.children []
                        |> Query.first
                        |> Event.simulate Event.click
                        |> Event.toResult
                        |> Expect.err
            , test "play button click msg sends api call" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.update toggleMsg
                        |> Tuple.second
                        |> Expect.equal
                            [ Effects.SendTogglePipelineRequest
                                pipelineIdentifier
                                True
                            ]
            , test "play button click msg turns icon into spinner" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.update toggleMsg
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "top-bar-pause-toggle" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has
                            [ style "animation"
                                "container-rotate 1568ms linear infinite"
                            , style "height" "20px"
                            , style "width" "20px"
                            ]
            , test "successful PipelineToggled callback turns topbar dark" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.update toggleMsg
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.PipelineToggled pipelineIdentifier <| Ok ())
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style "background-color" backgroundGrey ]
            , test "successful callback turns spinner into pause button" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.update toggleMsg
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.PipelineToggled pipelineIdentifier <| Ok ())
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "top-bar-pause-toggle" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has
                            (iconSelector
                                { size = "20px"
                                , image = Assets.PauseIcon
                                }
                            )
            , test "Unauthorized PipelineToggled callback redirects to login" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.handleCallback
                            (Callback.PipelineToggled pipelineIdentifier <|
                                Data.httpUnauthorized
                            )
                        |> Tuple.second
                        |> Expect.equal
                            [ Effects.RedirectToLogin ]
            , test "erroring PipelineToggled callback leaves topbar blue" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.handleCallback
                            (Callback.PipelineToggled pipelineIdentifier <|
                                Data.httpInternalServerError
                            )
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style "background-color" pausedBlue ]
            ]
        ]


eachHasStyle : String -> String -> Query.Multiple msg -> Expectation
eachHasStyle property value =
    Query.each <| Query.has [ style property value ]


sampleUser : Concourse.User
sampleUser =
    { id = "1", userName = "test", name = "Bob", isAdmin = False, email = "bob@bob.com", teams = Dict.empty }


pipelineBreadcrumbSelector : List Selector.Selector
pipelineBreadcrumbSelector =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.BreadcrumbIcon Assets.PipelineComponent)
    , style "background-repeat" "no-repeat"
    ]


jobBreadcrumbSelector : List Selector.Selector
jobBreadcrumbSelector =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.BreadcrumbIcon Assets.JobComponent)
    , style "background-repeat" "no-repeat"
    ]


resourceBreadcrumbSelector : List Selector.Selector
resourceBreadcrumbSelector =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.BreadcrumbIcon Assets.ResourceComponent)
    , style "background-repeat" "no-repeat"
    ]


viewNormally :
    ( Application.Model, List Effects.Effect )
    -> Query.Single ApplicationMsgs.TopLevelMessage
viewNormally =
    Tuple.first >> queryView


testDropdown :
    List Int
    -> List Int
    -> ( Application.Model, List Effects.Effect )
    -> Test
testDropdown selecteds notSelecteds =
    context "ui"
        viewNormally
        [ it "has a dropdown when search bar is focused" <|
            Query.find [ id "search-container" ]
                >> Query.has [ id "search-dropdown" ]
        , it "should trigger a FilterMsg when typing in the search bar" <|
            Query.find [ id SearchBar.searchInputId ]
                >> Event.simulate (Event.input "test")
                >> Event.expect
                    (ApplicationMsgs.Update <| Msgs.FilterMsg "test")
        , context "dropdown elements"
            (Query.findAll [ tag "li" ])
            [ it "have the same width and padding as search bar" <|
                eachHasStyle "padding" searchBarPadding
            , it "have the same height as the search bar" <|
                eachHasStyle "line-height" searchBarHeight
            , it "have no bullet points" <|
                eachHasStyle "list-style-type" "none"
            , it "have the same border style as the search bar" <|
                eachHasStyle "border" searchBarBorder
            , it "are vertically aligned flush to each other" <|
                eachHasStyle "margin-top" "-1px"
            , it "have slightly larger font" <|
                eachHasStyle "font-size" "1.15em"
            , it "have a pointer cursor" <|
                eachHasStyle "cursor" "pointer"
            ]
        , it "the search dropdown is positioned below the search bar" <|
            Query.find [ id "search-dropdown" ]
                >> Query.has
                    [ style "position" "absolute"
                    , style "top" "100%"
                    , style "margin" "0"
                    ]
        , it "the search dropdown is the same width as search bar" <|
            Query.find [ id "search-dropdown" ]
                >> Query.has [ style "width" "100%" ]
        , it "the search dropdown has 2 elements" <|
            Query.find [ id "search-dropdown" ]
                >> Expect.all
                    [ Query.findAll [ tag "li" ] >> Query.count (Expect.equal 2)
                    , Query.has [ text "status: " ]
                    , Query.has [ text "team: " ]
                    ]
        , it "when team is clicked, it should trigger a FilterMsg for team" <|
            Query.find [ id "search-dropdown" ]
                >> Query.find [ tag "li", containing [ text "team: " ] ]
                >> Event.simulate Event.mouseDown
                >> Event.expect
                    (ApplicationMsgs.Update <| Msgs.FilterMsg "team: ")
        , it "when status is clicked, it should trigger a FilterMsg for status" <|
            Query.find [ id "search-dropdown" ]
                >> Query.find [ tag "li", containing [ text "status: " ] ]
                >> Event.simulate Event.mouseDown
                >> Event.expect
                    (ApplicationMsgs.Update <| Msgs.FilterMsg "status: ")
        , it "sends BlurMsg when blurring the search bar" <|
            Query.find [ id SearchBar.searchInputId ]
                >> Event.simulate Event.blur
                >> Event.expect
                    (ApplicationMsgs.Update Msgs.BlurMsg)
        , context "selected highlighting"
            (Query.findAll [ tag "li" ])
            (List.concat
                (List.map
                    (\idx ->
                        [ it ("has the first element highlighted " ++ String.fromInt idx) <|
                            Query.index idx
                                >> Query.has [ style "background-color" "#1e1d1d" ]
                        , it ("has white text " ++ String.fromInt idx) <|
                            Query.index idx
                                >> Query.has [ style "color" "#ffffff" ]
                        ]
                    )
                    selecteds
                )
                ++ [ it "always has at least one test" <| \_ -> Expect.equal 0 0 ]
            )
        , context "other highlighting"
            (Query.findAll [ tag "li" ])
            (List.concat
                (List.map
                    (\idx ->
                        [ it ("has the other elements not highlighted " ++ String.fromInt idx) <|
                            Query.index idx
                                >> Query.has [ style "background-color" dropdownBackgroundGrey ]
                        , it ("have light grey text " ++ String.fromInt idx) <|
                            Query.index idx
                                >> Query.has [ style "color" "#9b9b9b" ]
                        ]
                    )
                    notSelecteds
                )
            )
        ]
