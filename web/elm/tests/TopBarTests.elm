module TopBarTests exposing (all)

import Application.Application as Application
import Application.Msgs as Msgs
import Callback exposing (Callback(..))
import Char
import Concourse
import Concourse.PipelineStatus as PipelineStatus
import Dashboard.Msgs
import Dict
import Effects
import Expect exposing (..)
import Html.Attributes as Attr
import Html.Styled exposing (toUnstyled)
import Keycodes
import Routes
import SubPage.Msgs
import Subscription exposing (Delivery(..))
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
import TopBar.Model as Model
import TopBar.Msgs
import TopBar.TopBar as TopBar
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


fit : String -> (subject -> Expectation) -> subject -> Test
fit desc expectationFunc subject =
    Test.only <|
        Test.test desc <|
            \_ -> expectationFunc subject


update : TopBar.Msgs.Msg -> Model.Model {} -> ( Model.Model {}, List Effects.Effect )
update msg =
    flip (,) [] >> TopBar.update msg


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


all : Test
all =
    describe "TopBar"
        [ rspecStyleDescribe "on init"
            (TopBar.init
                { route =
                    Routes.Pipeline
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            }
                        , groups = []
                        }
                }
                |> Tuple.second
            )
            [ it "requests screen size" <|
                Expect.equal [ Effects.GetScreenSize ]
            ]
        , rspecStyleDescribe "when on pipeline page"
            (TopBar.init
                { route =
                    Routes.Pipeline
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            }
                        , groups = []
                        }
                }
            )
            [ context "when login state unknown"
                (Tuple.first
                    >> TopBar.view UserState.UserStateUnknown Model.None
                    >> toUnstyled
                    >> Query.fromHtml
                )
                [ it "shows concourse logo" <|
                    Query.children []
                        >> Query.index 1
                        >> Query.has
                            [ style
                                [ ( "background-image"
                                  , "url(/public/images/concourse-logo-white.svg)"
                                  )
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
                    , it "has pointer cursor" <|
                        Query.has [ style [ ( "cursor", "pointer" ) ] ]
                    , it "has link to the relevant pipeline page" <|
                        Event.simulate Event.click
                            >> Event.expect
                                (TopBar.Msgs.GoToRoute <|
                                    Routes.Pipeline
                                        { id =
                                            { teamName = "team"
                                            , pipelineName = "pipeline"
                                            }
                                        , groups = []
                                        }
                                )
                    ]
                , it "has dark grey background" <|
                    Query.has [ style [ ( "background-color", backgroundGrey ) ] ]
                , it "lays out contents horizontally" <|
                    Query.has [ style [ ( "display", "flex" ) ] ]
                , it "maximizes spacing between the left and right navs" <|
                    Query.has [ style [ ( "justify-content", "space-between" ) ] ]
                , it "renders the login component last" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.has [ id "login-component" ]
                ]
            , context "when logged out"
                viewNormally
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
                (Tuple.first
                    >> TopBar.view (UserState.UserStateLoggedIn sampleUser) Model.None
                    >> toUnstyled
                    >> Query.fromHtml
                )
                [ it "renders the login component last" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.has [ id "login-component" ]
                , it "renders login component with a maximum width" <|
                    Query.find [ id "login-component" ]
                        >> Query.has [ style [ ( "max-width", "20%" ) ] ]
                , it "renders login container with relative position" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style [ ( "position", "relative" ) ] ]
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
                            [ style [ ( "border-left", "1px solid " ++ borderGrey ) ] ]
                , it "renders login container tall enough" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style [ ( "line-height", lineHeight ) ] ]
                , it "has the login username styles" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "user-id" ]
                        >> Expect.all
                            [ Query.has
                                [ style
                                    [ ( "padding", "0 30px" )
                                    , ( "cursor", "pointer" )
                                    , ( "display", "flex" )
                                    , ( "align-items", "center" )
                                    , ( "justify-content", "center" )
                                    , ( "flex-grow", "1" )
                                    ]
                                ]
                            , Query.children []
                                >> Query.index 0
                                >> Query.has
                                    [ style
                                        [ ( "overflow", "hidden" )
                                        , ( "text-overflow", "ellipsis" )
                                        ]
                                    ]
                            ]
                , it "shows the logged in username when the user is logged in" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "user-id" ]
                        >> Query.has [ text "test" ]
                , it "ToggleUserMenu message is received when login menu is clicked" <|
                    Query.find [ id "login-container" ]
                        >> Event.simulate Event.click
                        >> Event.expect TopBar.Msgs.ToggleUserMenu
                , it "does not render the logout button" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "user-id" ]
                        >> Query.hasNot [ id "logout-button" ]
                ]
            , it "clicking a pinned resource navigates to the pinned resource page" <|
                Tuple.mapSecond (always [])
                    >> TopBar.update
                        (TopBar.Msgs.GoToRoute
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
                    >> Expect.equal [ Effects.NavigateTo "/teams/t/pipelines/p/resources/r" ]
            , context "when pipeline is paused"
                (Tuple.first
                    >> TopBar.view (UserState.UserStateLoggedIn sampleUser)
                        (Model.HasPipeline
                            { pinnedResources = []
                            , pipeline = { teamName = "t", pipelineName = "p" }
                            , isPaused = True
                            }
                        )
                    >> toUnstyled
                    >> Query.fromHtml
                )
                [ it "has blue background" <|
                    Query.has [ style [ ( "background-color", pausedBlue ) ] ]
                , it "draws almost-white line to the left of login container" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style [ ( "border-left", "1px solid " ++ almostWhite ) ] ]
                ]
            ]
        , rspecStyleDescribe "rendering user menus on clicks"
            (TopBar.init
                { route =
                    Routes.Pipeline
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            }
                        , groups = []
                        }
                }
            )
            [ it "shows user menu when ToggleUserMenu msg is received" <|
                TopBar.update TopBar.Msgs.ToggleUserMenu
                    >> Tuple.first
                    >> TopBar.view (UserState.UserStateLoggedIn sampleUser) Model.None
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.has [ id "logout-button" ]
            , it "renders user menu content when ToggleUserMenu msg is received and logged in" <|
                TopBar.update TopBar.Msgs.ToggleUserMenu
                    >> Tuple.first
                    >> TopBar.view (UserState.UserStateLoggedIn sampleUser) Model.None
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
                TopBar.update TopBar.Msgs.ToggleUserMenu
                    >> Tuple.first
                    >> TopBar.view (UserState.UserStateLoggedIn sampleUser) Model.None
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "logout-button" ]
                    >> Event.simulate Event.click
                    >> Event.expect TopBar.Msgs.LogOut
            , it "shows 'login' when LoggedOut Msg is successful" <|
                TopBar.handleCallback (Callback.LoggedOut (Ok ()))
                    >> viewNormally
                    >> Query.find [ id "login-item" ]
                    >> Query.has [ text "login" ]
            ]
        , rspecStyleDescribe "login component when user is logged out"
            (TopBar.init
                { route =
                    Routes.Pipeline
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            }
                        , groups = []
                        }
                }
                |> viewNormally
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
            (TopBar.init
                { route =
                    Routes.Pipeline
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            }
                        , groups = []
                        }
                }
            )
            [ it "redirects to login page when you click login" <|
                Tuple.first
                    >> update TopBar.Msgs.LogIn
                    >> Tuple.second
                    >> Expect.equal [ Effects.RedirectToLogin ]
            ]
        , rspecStyleDescribe "rendering top bar on build page"
            (TopBar.init
                { route =
                    Routes.Build
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = "job"
                            , buildName = "1"
                            }
                        , highlight = Routes.HighlightNothing
                        }
                }
                |> viewNormally
            )
            [ it "should pad the breadcrumbs to max size so they can be left-aligned" <|
                Query.find
                    [ id "breadcrumbs" ]
                    >> Query.has [ style [ ( "flex-grow", "1" ) ] ]
            , it "pipeline breadcrumb should have a link to the pipeline page when viewing build details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (TopBar.Msgs.GoToRoute <|
                            Routes.Pipeline
                                { id =
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                                , groups = []
                                }
                        )
            , context "job breadcrumb"
                (Query.find [ id "breadcrumb-job" ])
                [ it "is laid out horizontally with appropriate spacing" <|
                    Query.has
                        [ style
                            [ ( "display", "inline-block" )
                            , ( "padding", "0 10px" )
                            ]
                        ]
                , it "has job icon rendered first" <|
                    Query.has jobBreadcrumbSelector
                , it "has build name after job icon" <|
                    Query.has [ text "job" ]
                , it "does not appear clickable" <|
                    Query.hasNot [ style [ ( "cursor", "pointer" ) ] ]
                ]
            ]
        , rspecStyleDescribe "rendering top bar on resource page"
            (TopBar.init
                { route =
                    Routes.Resource
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , resourceName = "resource"
                            }
                        , page = Nothing
                        }
                }
                |> viewNormally
            )
            [ it "should pad the breadcrumbs to max size so they can be left-aligned" <|
                Query.find
                    [ id "breadcrumbs" ]
                    >> Query.has [ style [ ( "flex-grow", "1" ) ] ]
            , it "pipeline breadcrumb should have a link to the pipeline page when viewing resource details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (TopBar.Msgs.GoToRoute <|
                            Routes.Pipeline
                                { id =
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                                , groups = []
                                }
                        )
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
            (TopBar.init
                { route =
                    Routes.Job
                        { id =
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = "job"
                            }
                        , page = Nothing
                        }
                }
                |> viewNormally
            )
            [ it "should pad the breadcrumbs to max size so they can be left-aligned" <|
                Query.find
                    [ id "breadcrumbs" ]
                    >> Query.has [ style [ ( "flex-grow", "1" ) ] ]
            , it "pipeline breadcrumb should have a link to the pipeline page when viewing job details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (TopBar.Msgs.GoToRoute <|
                            Routes.Pipeline
                                { id =
                                    { teamName = "team"
                                    , pipelineName = "pipeline"
                                    }
                                , groups = []
                                }
                        )
            , it "there is a / between pipeline and job in breadcrumb" <|
                Query.findAll [ tag "li" ]
                    >> Expect.all
                        [ Query.index 1 >> Query.has [ class "breadcrumb-separator" ]
                        , Query.index 0 >> Query.has [ id "breadcrumb-pipeline" ]
                        , Query.index 2 >> Query.has [ id "breadcrumb-job" ]
                        ]
            ]
        , rspecStyleDescribe "when checking search bar values"
            (Application.init
                { turbulenceImgSrc = ""
                , notFoundImgSrc = ""
                , csrfToken = ""
                , authToken = ""
                , pipelineRunningKeyframes = ""
                }
                { href = ""
                , host = ""
                , hostname = ""
                , protocol = ""
                , origin = ""
                , port_ = ""
                , pathname = "/"
                , search = "?search=test"
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
                |> Application.handleCallback
                    (Effects.SubPage 1)
                    (Callback.APIDataFetched
                        (Ok
                            ( 0
                            , { teams =
                                    [ Concourse.Team 1 "team1"
                                    , Concourse.Team 2 "team2"
                                    ]
                              , pipelines = [ onePipeline "team1" ]
                              , jobs = []
                              , resources = []
                              , user = Nothing
                              , version = ""
                              }
                            )
                        )
                    )
                |> Tuple.first
            )
            [ it "renders the search bar with the text in the search query" <|
                Application.view
                    >> Query.fromHtml
                    >> Query.find [ id TopBar.searchInputId ]
                    >> Query.has [ tag "input", attribute <| Attr.value "test" ]
            , it "sends a FilterMsg when the clear search button is clicked" <|
                Application.view
                    >> Query.fromHtml
                    >> Query.find [ id "search-container" ]
                    >> Query.find [ id "search-clear" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar <|
                                    TopBar.Msgs.FilterMsg ""
                        )
            , it "clear search button has full opacity when there is a query" <|
                Application.view
                    >> Query.fromHtml
                    >> Query.find [ id "search-clear" ]
                    >> Query.has [ style [ ( "opacity", "1" ) ] ]
            ]
        , rspecStyleDescribe "rendering search bar on dashboard page"
            (Application.init
                { turbulenceImgSrc = ""
                , notFoundImgSrc = ""
                , csrfToken = ""
                , authToken = ""
                , pipelineRunningKeyframes = ""
                }
                { href = ""
                , host = ""
                , hostname = ""
                , protocol = ""
                , origin = ""
                , port_ = ""
                , pathname = "/"
                , search = ""
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
                |> Application.handleCallback
                    (Effects.SubPage 1)
                    (Callback.APIDataFetched
                        (Ok
                            ( 0
                            , { teams =
                                    [ Concourse.Team 1 "team1"
                                    , Concourse.Team 2 "team2"
                                    ]
                              , pipelines = [ onePipeline "team1" ]
                              , jobs = []
                              , resources = []
                              , user = Nothing
                              , version = ""
                              }
                            )
                        )
                    )
                |> Tuple.first
            )
            [ context "when desktop sized"
                (Application.handleCallback
                    (Effects.SubPage 1)
                    (ScreenResized { width = 1500, height = 900 })
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                )
                [ it "renders search bar" <|
                    Query.has [ id TopBar.searchInputId ]
                , it "search bar is an input field" <|
                    Query.find [ id TopBar.searchInputId ]
                        >> Query.has [ tag "input" ]
                , it "renders search bar with transparent background to remove white of search bar" <|
                    Query.find [ id TopBar.searchInputId ]
                        >> Query.has [ style [ ( "background-color", "transparent" ) ] ]
                , it "search bar does not use browser's built-in autocomplete" <|
                    Query.find [ id TopBar.searchInputId ]
                        >> Query.has
                            [ attribute <| Attr.attribute "autocomplete" "off" ]
                , it "sets magnifying glass on search bar in correct position" <|
                    Query.find [ id TopBar.searchInputId ]
                        >> Query.has
                            [ style
                                [ ( "background-image"
                                  , "url('public/images/ic-search-white-24px.svg')"
                                  )
                                , ( "background-position", "12px 8px" )
                                , ( "background-repeat", "no-repeat" )
                                ]
                            ]
                , it "styles search border and input text colour" <|
                    Query.find [ id TopBar.searchInputId ]
                        >> Query.has
                            [ style
                                [ ( "border", searchBarBorder )
                                , ( "color", "#fff" )
                                , ( "font-size", "1.15em" )
                                , ( "font-family", "Inconsolata, monospace" )
                                ]
                            ]
                , it "renders search with appropriate size and padding" <|
                    Query.find [ id TopBar.searchInputId ]
                        >> Query.has
                            [ style
                                [ ( "height", searchBarHeight )
                                , ( "width", searchBarWidth )
                                , ( "padding", searchBarPadding )
                                ]
                            ]
                , it "does not have an outline when focused" <|
                    Query.find [ id TopBar.searchInputId ]
                        >> Query.has [ style [ ( "outline", "0" ) ] ]
                , it "has placeholder text" <|
                    Query.find [ id TopBar.searchInputId ]
                        >> Query.has [ tag "input", attribute <| Attr.placeholder "search" ]
                , it "has a search container" <|
                    Query.has [ id "search-container" ]
                , it "search container is positioned appropriately" <|
                    Query.find [ id "search-container" ]
                        >> Expect.all
                            [ Query.has
                                [ style
                                    [ ( "position", "relative" )
                                    , ( "display", "flex" )
                                    , ( "flex-direction", "column" )
                                    , ( "align-items", "stretch" )
                                    ]
                                ]
                            , Query.hasNot [ style [ ( "flex-grow", "1" ) ] ]
                            ]
                , it "search container is sized correctly" <|
                    Query.find [ id "search-container" ]
                        >> Expect.all
                            [ Query.has [ style [ ( "margin", "12px" ) ] ]
                            , Query.hasNot [ style [ ( "height", "56px" ) ] ]
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
                                [ ( "background-image"
                                  , "url('public/images/ic-close-white-24px.svg')"
                                  )
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
            , context "when mobile sized"
                (Application.handleCallback
                    (Effects.SubPage 1)
                    (ScreenResized { width = 400, height = 900 })
                    >> Tuple.first
                )
                [ it "should not have a search bar" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.hasNot
                            [ id TopBar.searchInputId ]
                , it "should have a magnifying glass icon" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "show-search-button" ]
                        >> Query.has
                            [ style
                                [ ( "background-image", "url('public/images/ic-search-white-24px.svg')" )
                                , ( "background-position", "12px 8px" )
                                , ( "background-repeat", "no-repeat" )
                                ]
                            ]
                , it "shows the login component" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.has [ id "login-component" ]
                , context "after clicking the search icon"
                    (Application.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar <|
                                    TopBar.Msgs.ShowSearchInput
                        )
                    )
                    [ it "tells the ui to focus on the search bar" <|
                        Tuple.second
                            >> Expect.equal
                                [ ( Effects.SubPage 1
                                  , ""
                                  , Effects.Focus TopBar.searchInputId
                                  )
                                ]
                    , context "the ui"
                        (Tuple.first
                            >> Application.view
                            >> Query.fromHtml
                        )
                        [ it "renders search bar" <|
                            Query.has [ id TopBar.searchInputId ]
                        , it "search bar is an input field" <|
                            Query.find [ id TopBar.searchInputId ]
                                >> Query.has [ tag "input" ]
                        , it "has placeholder text" <|
                            Query.find [ id TopBar.searchInputId ]
                                >> Query.has [ tag "input", attribute <| Attr.placeholder "search" ]
                        , it "has a search container" <|
                            Query.has [ id "search-container" ]
                        , it "positions the search container appropriately" <|
                            Query.find [ id "search-container" ]
                                >> Query.has
                                    [ style
                                        [ ( "position", "relative" )
                                        , ( "display", "flex" )
                                        , ( "flex-direction", "column" )
                                        , ( "align-items", "stretch" )
                                        , ( "flex-grow", "1" )
                                        ]
                                    ]
                        , it "search container is sized correctly" <|
                            Query.find [ id "search-container" ]
                                >> Expect.all
                                    [ Query.has [ style [ ( "margin", "12px" ) ] ]
                                    , Query.hasNot [ style [ ( "height", "56px" ) ] ]
                                    ]
                        , it "has a clear search button container" <|
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
                        , it "hides the login component" <|
                            Query.hasNot [ id "login-component" ]
                        ]
                    , context "after the focus returns"
                        (Tuple.first
                            >> Application.update
                                (Msgs.SubMsg 1 <|
                                    SubPage.Msgs.DashboardMsg <|
                                        Dashboard.Msgs.FromTopBar <|
                                            TopBar.Msgs.FocusMsg
                                )
                            >> Tuple.first
                        )
                        [ it "should display a dropdown of options" <|
                            Application.view
                                >> Query.fromHtml
                                >> Query.find [ id "search-dropdown" ]
                                >> Query.findAll [ tag "li" ]
                                >> Expect.all
                                    [ Query.count (Expect.equal 2)
                                    , Query.index 0 >> Query.has [ text "status:" ]
                                    , Query.index 1 >> Query.has [ text "team:" ]
                                    ]
                        , it "the search dropdown is positioned below the search bar" <|
                            Application.view
                                >> Query.fromHtml
                                >> Query.find [ id "search-dropdown" ]
                                >> Expect.all
                                    [ Query.has
                                        [ style
                                            [ ( "top", "100%" )
                                            , ( "margin", "0" )
                                            ]
                                        ]
                                    , Query.hasNot [ style [ ( "position", "absolute" ) ] ]
                                    ]
                        , it "the search dropdown is the same width as search bar" <|
                            Application.view
                                >> Query.fromHtml
                                >> Query.find [ id "search-dropdown" ]
                                >> Query.has [ style [ ( "width", "100%" ) ] ]
                        , context "after the search is blurred"
                            (Application.update
                                (Msgs.SubMsg 1 <|
                                    SubPage.Msgs.DashboardMsg <|
                                        Dashboard.Msgs.FromTopBar <|
                                            TopBar.Msgs.BlurMsg
                                )
                                >> Tuple.first
                                >> Application.view
                                >> Query.fromHtml
                            )
                            [ it "should not have a search bar" <|
                                Query.hasNot
                                    [ id TopBar.searchInputId ]
                            , it "should have a magnifying glass icon" <|
                                Query.find [ id "show-search-button" ]
                                    >> Query.has
                                        [ style
                                            [ ( "background-image", "url('public/images/ic-search-white-24px.svg')" )
                                            , ( "background-position", "12px 8px" )
                                            , ( "background-repeat", "no-repeat" )
                                            ]
                                        ]
                            , it "shows the login component" <|
                                Query.has [ id "login-component" ]
                            ]
                        , context "after the search is blurred with a search query"
                            (Application.update
                                (Msgs.SubMsg 1 <|
                                    SubPage.Msgs.DashboardMsg <|
                                        Dashboard.Msgs.FromTopBar <|
                                            TopBar.Msgs.FilterMsg "query"
                                )
                                >> Tuple.first
                                >> Application.update
                                    (Msgs.SubMsg 1 <|
                                        SubPage.Msgs.DashboardMsg <|
                                            Dashboard.Msgs.FromTopBar <|
                                                TopBar.Msgs.BlurMsg
                                    )
                                >> Tuple.first
                                >> Application.view
                                >> Query.fromHtml
                            )
                            [ it "should have a search bar" <|
                                Query.has [ id TopBar.searchInputId ]
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
            (Application.init
                { turbulenceImgSrc = ""
                , notFoundImgSrc = ""
                , csrfToken = ""
                , authToken = ""
                , pipelineRunningKeyframes = ""
                }
                { href = ""
                , host = ""
                , hostname = ""
                , protocol = ""
                , origin = ""
                , port_ = ""
                , pathname = "/"
                , search = ""
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
                |> Application.handleCallback
                    (Effects.SubPage 1)
                    (Callback.APIDataFetched
                        (Ok
                            ( 0
                            , { teams =
                                    [ Concourse.Team 1 "team1"
                                    , Concourse.Team 2 "team2"
                                    ]
                              , pipelines = [ onePipeline "team1" ]
                              , jobs = []
                              , resources = []
                              , user = Nothing
                              , version = ""
                              }
                            )
                        )
                    )
                |> Tuple.first
            )
            [ it
                ("shows the list of statuses when "
                    ++ "`status:` is typed in the search bar"
                )
              <|
                Application.update
                    (Msgs.SubMsg 1 <|
                        SubPage.Msgs.DashboardMsg <|
                            Dashboard.Msgs.FromTopBar <|
                                TopBar.Msgs.FocusMsg
                    )
                    >> Tuple.first
                    >> Application.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar <|
                                    TopBar.Msgs.FilterMsg "status:"
                        )
                    >> Tuple.first
                    >> Application.view
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
                Application.update
                    (Msgs.SubMsg 1 <|
                        SubPage.Msgs.DashboardMsg <|
                            Dashboard.Msgs.FromTopBar <|
                                TopBar.Msgs.FocusMsg
                    )
                    >> Tuple.first
                    >> Application.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar <|
                                    TopBar.Msgs.FilterMsg "status:"
                        )
                    >> Tuple.first
                    >> Application.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar <|
                                    TopBar.Msgs.FilterMsg "status: pending"
                        )
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                    >> Query.findAll [ id "search-dropdown" ]
                    >> Query.first
                    >> Query.children []
                    >> Query.count (Expect.equal 0)
            ]
        , rspecStyleDescribe "when search query is `status:`"
            (Application.init
                { turbulenceImgSrc = ""
                , notFoundImgSrc = ""
                , csrfToken = ""
                , authToken = ""
                , pipelineRunningKeyframes = ""
                }
                { href = ""
                , host = ""
                , hostname = ""
                , protocol = ""
                , origin = ""
                , port_ = ""
                , pathname = "/"
                , search = "?search=status:"
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
                |> Application.handleCallback
                    (Effects.SubPage 1)
                    (Callback.APIDataFetched
                        (Ok
                            ( 0
                            , { teams =
                                    [ Concourse.Team 1 "team1"
                                    , Concourse.Team 2 "team2"
                                    ]
                              , pipelines = [ onePipeline "team1" ]
                              , jobs = []
                              , resources = []
                              , user = Nothing
                              , version = ""
                              }
                            )
                        )
                    )
                |> Tuple.first
            )
            [ it "should display a dropdown of status options when the search bar is focused" <|
                Application.update
                    (Msgs.SubMsg 1 <|
                        SubPage.Msgs.DashboardMsg <|
                            Dashboard.Msgs.FromTopBar <|
                                TopBar.Msgs.FocusMsg
                    )
                    >> Tuple.first
                    >> Application.view
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
            (Application.init
                { turbulenceImgSrc = ""
                , notFoundImgSrc = ""
                , csrfToken = ""
                , authToken = ""
                , pipelineRunningKeyframes = ""
                }
                { href = ""
                , host = ""
                , hostname = ""
                , protocol = ""
                , origin = ""
                , port_ = ""
                , pathname = "/"
                , search = "?search=team:"
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
            )
            [ it "when there are teams the dropdown displays them" <|
                Application.handleCallback
                    (Effects.SubPage 1)
                    (Callback.APIDataFetched
                        (Ok
                            ( 0
                            , { teams = [ Concourse.Team 1 "team1", Concourse.Team 2 "team2" ]
                              , pipelines = [ onePipeline "team1" ]
                              , jobs = []
                              , resources = []
                              , user = Nothing
                              , version = ""
                              }
                            )
                        )
                    )
                    >> Tuple.first
                    >> Application.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar TopBar.Msgs.FocusMsg
                        )
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.children []
                    >> Expect.all
                        [ Query.count (Expect.equal 2)
                        , Query.first >> Query.has [ tag "li", text "team1" ]
                        , Query.index 1 >> Query.has [ tag "li", text "team2" ]
                        ]
            , it "when there are many teams, the dropdown only displays the first 10" <|
                Application.handleCallback
                    (Effects.SubPage 1)
                    (Callback.APIDataFetched
                        (Ok
                            ( 0
                            , { teams =
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
                              , pipelines = [ onePipeline "team1" ]
                              , jobs = []
                              , resources = []
                              , user = Nothing
                              , version = ""
                              }
                            )
                        )
                    )
                    >> Tuple.first
                    >> Application.update
                        (Msgs.SubMsg 1 <|
                            SubPage.Msgs.DashboardMsg <|
                                Dashboard.Msgs.FromTopBar TopBar.Msgs.FocusMsg
                        )
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.children []
                    >> Query.count (Expect.equal 10)
            ]
        , rspecStyleDescribe "dropdown stuff"
            (let
                ( model, effects ) =
                    TopBar.init
                        { route =
                            Routes.Dashboard (Routes.Normal Nothing)
                        }
             in
             ( { model
                | groups =
                    [ { pipelines =
                            [ { id = 0
                              , name = "pipeline"
                              , teamName = "team"
                              , public = True
                              , jobs = []
                              , resourceError = False
                              , status = PipelineStatus.PipelineStatusPaused
                              }
                            ]
                      , teamName = "team"
                      , tag = Nothing
                      }
                    ]
               }
             , effects
             )
            )
            [ context "before receiving FocusMsg"
                viewNormally
                [ it "has no dropdown" <|
                    Query.findAll [ id "search-dropdown" ]
                        >> Query.count (Expect.equal 0)
                , it "sends FocusMsg when focusing on search bar" <|
                    Query.find [ id TopBar.searchInputId ]
                        >> Event.simulate Event.focus
                        >> Event.expect TopBar.Msgs.FocusMsg
                ]
            , it "hitting '/' focuses search input" <|
                Tuple.first
                    >> flip (,) []
                    >> TopBar.handleDelivery (KeyDown 191)
                    >> Tuple.second
                    >> Expect.equal [ Effects.Focus TopBar.searchInputId ]
            , it "hitting shift + '/' (= '?') does not focus search input" <|
                Tuple.first
                    >> flip (,) []
                    >> TopBar.handleDelivery (KeyDown Keycodes.shift)
                    >> TopBar.handleDelivery (KeyDown 191)
                    >> Tuple.second
                    >> Expect.equal []
            , it "pressing + releasing shift, then '/', focuses search input" <|
                Tuple.first
                    >> flip (,) []
                    >> TopBar.handleDelivery (KeyDown Keycodes.shift)
                    >> TopBar.handleDelivery (KeyUp Keycodes.shift)
                    >> TopBar.handleDelivery (KeyDown 191)
                    >> Tuple.second
                    >> Expect.equal [ Effects.Focus TopBar.searchInputId ]
            , it "hitting other keys does not cause dropdown to expand" <|
                TopBar.handleDelivery (KeyDown <| Char.toCode 'A')
                    >> viewNormally
                    >> Query.findAll [ id "search-dropdown" ]
                    >> Query.count (Expect.equal 0)
            , context "after receiving FocusMsg"
                (TopBar.update TopBar.Msgs.FocusMsg)
                ([ testDropdown [] [ 0, 1 ] ]
                    ++ [ context "after down arrow keypress"
                            (TopBar.handleDelivery (KeyDown 40))
                            ([ testDropdown [ 0 ] [ 1 ] ]
                                ++ [ context "after second down arrow keypress"
                                        (TopBar.handleDelivery (KeyDown 40))
                                        ([ testDropdown [ 1 ] [ 0 ] ]
                                            ++ [ context "after loop around down arrow keypress"
                                                    (TopBar.handleDelivery (KeyDown 40))
                                                    [ testDropdown [ 0 ] [ 1 ] ]
                                               , context "after hitting enter"
                                                    (TopBar.handleDelivery (KeyDown 13)
                                                        >> viewNormally
                                                    )
                                                    [ it "updates the query" <|
                                                        Query.find [ id TopBar.searchInputId ]
                                                            >> Query.has [ attribute <| Attr.value "team: " ]
                                                    ]
                                               ]
                                        )
                                   , context "after hitting enter"
                                        (Tuple.mapSecond (always [])
                                            >> TopBar.handleDelivery
                                                (KeyDown 13)
                                        )
                                        [ it "updates the query" <|
                                            viewNormally
                                                >> Query.find
                                                    [ id TopBar.searchInputId ]
                                                >> Query.has
                                                    [ attribute <|
                                                        Attr.value "status: "
                                                    ]
                                        , it "updates the URL" <|
                                            Tuple.second
                                                >> Expect.equal
                                                    [ Effects.ModifyUrl
                                                        "?search=status%3A%20"
                                                    ]
                                        ]
                                   ]
                            )
                       , context "after up arrow keypress"
                            (TopBar.handleDelivery (KeyDown 38))
                            ([ testDropdown [ 1 ] [ 0 ] ]
                                ++ [ context "after second up arrow keypress"
                                        (TopBar.handleDelivery (KeyDown 38))
                                        ([ testDropdown [ 0 ] [ 1 ] ]
                                            ++ [ context "after loop around up arrow keypress"
                                                    (TopBar.handleDelivery (KeyDown 38))
                                                    [ testDropdown [ 1 ] [ 0 ] ]
                                               ]
                                        )
                                   ]
                            )
                       ]
                    ++ [ context "on ESC keypress"
                            (Tuple.mapSecond (always [])
                                >> TopBar.handleDelivery (KeyDown 27)
                            )
                            [ it "search input is blurred" <|
                                Tuple.second
                                    >> Expect.equal [ Effects.Blur TopBar.searchInputId ]
                            ]
                       ]
                )
            , context "after receiving FocusMsg and then BlurMsg"
                (TopBar.update TopBar.Msgs.FocusMsg
                    >> TopBar.update TopBar.Msgs.BlurMsg
                    >> viewNormally
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


sampleUser : Concourse.User
sampleUser =
    { id = "1", userName = "test", name = "Bob", email = "bob@bob.com", teams = Dict.empty }


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


onePipeline : String -> Concourse.Pipeline
onePipeline teamName =
    { id = 0
    , name = "pipeline"
    , paused = False
    , public = True
    , teamName = teamName
    , groups = []
    }


viewNormally : ( Model.Model {}, List Effects.Effect ) -> Query.Single TopBar.Msgs.Msg
viewNormally =
    Tuple.first >> TopBar.view UserStateLoggedOut Model.None >> toUnstyled >> Query.fromHtml


testDropdown : List Int -> List Int -> ( Model.Model {}, List Effects.Effect ) -> Test
testDropdown selecteds notSelecteds =
    context "ui"
        viewNormally
        [ it "has a dropdown when search bar is focused" <|
            Query.find [ id "search-container" ]
                >> Query.has [ id "search-dropdown" ]
        , it "should trigger a FilterMsg when typing in the search bar" <|
            Query.find [ id TopBar.searchInputId ]
                >> Event.simulate (Event.input "test")
                >> Event.expect (TopBar.Msgs.FilterMsg "test")
        , context "dropdown elements"
            (Query.findAll [ tag "li" ])
            [ it "have the same width and padding as search bar" <|
                eachHasStyle [ ( "padding", searchBarPadding ) ]
            , it "have the same height as the search bar" <|
                eachHasStyle [ ( "line-height", searchBarHeight ) ]
            , it "have no bullet points" <|
                eachHasStyle [ ( "list-style-type", "none" ) ]
            , it "have the same border style as the search bar" <|
                eachHasStyle [ ( "border", searchBarBorder ) ]
            , it "are vertically aligned flush to each other" <|
                eachHasStyle [ ( "margin-top", "-1px" ) ]
            , it "have slightly larger font" <|
                eachHasStyle [ ( "font-size", "1.15em" ) ]
            , it "have a pointer cursor" <|
                eachHasStyle [ ( "cursor", "pointer" ) ]
            ]
        , it "the search dropdown is positioned below the search bar" <|
            Query.find [ id "search-dropdown" ]
                >> Query.has
                    [ style
                        [ ( "position", "absolute" )
                        , ( "top", "100%" )
                        , ( "margin", "0" )
                        ]
                    ]
        , it "the search dropdown is the same width as search bar" <|
            Query.find [ id "search-dropdown" ]
                >> Query.has [ style [ ( "width", "100%" ) ] ]
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
                >> Event.expect (TopBar.Msgs.FilterMsg "team:")
        , it "when status is clicked, it should trigger a FilterMsg for status" <|
            Query.find [ id "search-dropdown" ]
                >> Query.find [ tag "li", containing [ text "status:" ] ]
                >> Event.simulate Event.mouseDown
                >> Event.expect (TopBar.Msgs.FilterMsg "status:")
        , it "sends BlurMsg when blurring the search bar" <|
            Query.find [ id TopBar.searchInputId ]
                >> Event.simulate Event.blur
                >> Event.expect TopBar.Msgs.BlurMsg
        , context "selected highlighting"
            (Query.findAll [ tag "li" ])
            (List.concat
                (List.map
                    (\idx ->
                        [ it ("has the first element highlighted " ++ toString idx) <|
                            Query.index idx
                                >> Query.has [ style [ ( "background-color", "#1e1d1d" ) ] ]
                        , it ("has white text " ++ toString idx) <|
                            Query.index idx
                                >> Query.has [ style [ ( "color", "#fff" ) ] ]
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
                        [ it ("has the other elements not highlighted " ++ toString idx) <|
                            Query.index idx
                                >> Query.has [ style [ ( "background-color", dropdownBackgroundGrey ) ] ]
                        , it ("have light grey text " ++ toString idx) <|
                            Query.index idx
                                >> Query.has [ style [ ( "color", "#9b9b9b" ) ] ]
                        ]
                    )
                    notSelecteds
                )
            )
        ]
