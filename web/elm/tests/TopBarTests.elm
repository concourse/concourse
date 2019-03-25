module TopBarTests exposing (all)

import Application.Application as Application
import Char
import Concourse
import Dashboard.SearchBar as SearchBar
import DashboardTests exposing (defineHoverBehaviour, iconSelector)
import Dict
import Expect exposing (..)
import Html.Attributes as Attr
import Http
import Keycodes
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
    flip (,) [] >> Login.update msg


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
        [ rspecStyleDescribe "when on pipeline page"
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
                , pathname = "/teams/team/pipelines/pipeline"
                , search = ""
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
            )
            [ context "when login state unknown"
                (Application.view >> Query.fromHtml)
                [ it "shows concourse logo" <|
                    Query.children []
                        >> Query.index 0
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
                                (ApplicationMsgs.Update <|
                                    Msgs.GoToRoute <|
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
                (Application.handleCallback
                    (Callback.UserFetched <|
                        Err <|
                            Http.BadStatus
                                { url = ""
                                , status =
                                    { code = 401
                                    , message = "unauthorized"
                                    }
                                , headers = Dict.empty
                                , body = ""
                                }
                    )
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
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
                    >> Application.view
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
                        >> Event.expect
                            (ApplicationMsgs.Update Msgs.ToggleUserMenu)
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
                            [ style [ ( "background-image", "url(/public/images/ic-pause-white.svg)" ) ] ]
                , it "draws lighter grey line to the left of pause pipeline button" <|
                    Query.find [ id "top-bar-pause-toggle" ]
                        >> Query.has
                            [ style [ ( "border-left", "1px solid " ++ borderGrey ) ] ]
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
                        [ Effects.NavigateTo "/teams/t/pipelines/p/resources/r" ]
            , context "when pipeline is paused"
                (Application.handleCallback
                    (Callback.PipelineFetched <|
                        Ok
                            { id = 0
                            , name = "p"
                            , paused = True
                            , public = True
                            , teamName = "t"
                            , groups = []
                            }
                    )
                    >> Tuple.first
                    >> Application.handleCallback
                        (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                )
                [ it "has blue background" <|
                    Query.has [ style [ ( "background-color", pausedBlue ) ] ]
                , it "draws almost-white line to the left of login container" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "login-container" ]
                        >> Query.has
                            [ style
                                [ ( "border-left"
                                  , "1px solid " ++ almostWhite
                                  )
                                ]
                            ]
                ]
            ]
        , rspecStyleDescribe "rendering user menus on clicks"
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
                , pathname = "/teams/team/pipelines/pipeline"
                , search = ""
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
            )
            [ it "shows user menu when ToggleUserMenu msg is received" <|
                Application.handleCallback
                    (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update Msgs.ToggleUserMenu)
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                    >> Query.has [ id "logout-button" ]
            , it "renders user menu content when ToggleUserMenu msg is received and logged in" <|
                Application.handleCallback
                    (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update Msgs.ToggleUserMenu)
                    >> Tuple.first
                    >> Application.view
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
            , it "when logout is clicked, a LogOut TopLevelMessage is sent" <|
                Application.handleCallback
                    (Callback.UserFetched <| Ok sampleUser)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update Msgs.ToggleUserMenu)
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                    >> Query.find [ id "logout-button" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (ApplicationMsgs.Update Msgs.LogOut)
            , it "shows 'login' when LoggedOut TopLevelMessage is successful" <|
                Application.handleCallback
                    (Callback.LoggedOut <| Ok ())
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                    >> Query.find [ id "login-item" ]
                    >> Query.has [ text "login" ]
            ]
        , rspecStyleDescribe "login component when user is logged out"
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
                , pathname = "/teams/team/pipelines/pipeline"
                , search = ""
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
                |> Application.handleCallback
                    (Callback.LoggedOut (Ok ()))
                |> Tuple.first
                |> Application.view
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
                    (Callback.LoggedOut (Ok ()))
            )
            [ it "redirects to login page when you click login" <|
                Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update Msgs.LogIn)
                    >> Tuple.second
                    >> Expect.equal [ Effects.RedirectToLogin ]
            ]
        , rspecStyleDescribe "rendering top bar on build page"
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
                , pathname = "/teams/team/pipelines/pipeline/jobs/job/builds/1"
                , search = ""
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
                |> Application.view
                |> Query.fromHtml
            )
            [ it "should pad the breadcrumbs to max size so they can be left-aligned" <|
                Query.find
                    [ id "breadcrumbs" ]
                    >> Query.has [ style [ ( "flex-grow", "1" ) ] ]
            , it "pipeline breadcrumb should have a link to the pipeline page when viewing build details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (ApplicationMsgs.Update <|
                            Msgs.GoToRoute <|
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
                , pathname = "/teams/team/pipelines/pipeline/resources/resource"
                , search = ""
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
                |> Application.view
                |> Query.fromHtml
            )
            [ it "should pad the breadcrumbs to max size so they can be left-aligned" <|
                Query.find
                    [ id "breadcrumbs" ]
                    >> Query.has [ style [ ( "flex-grow", "1" ) ] ]
            , it "pipeline breadcrumb should have a link to the pipeline page when viewing resource details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (ApplicationMsgs.Update <|
                            Msgs.GoToRoute <|
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
                , pathname = "/teams/team/pipelines/pipeline/jobs/job"
                , search = ""
                , hash = ""
                , username = ""
                , password = ""
                }
                |> Tuple.first
                |> Application.view
                |> Query.fromHtml
            )
            [ it "should pad the breadcrumbs to max size so they can be left-aligned" <|
                Query.find
                    [ id "breadcrumbs" ]
                    >> Query.has [ style [ ( "flex-grow", "1" ) ] ]
            , it "pipeline breadcrumb should have a link to the pipeline page when viewing job details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (ApplicationMsgs.Update <|
                            Msgs.GoToRoute <|
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
                    >> Query.find [ id SearchBar.searchInputId ]
                    >> Query.has [ tag "input", attribute <| Attr.value "test" ]
            , it "sends a FilterMsg when the clear search button is clicked" <|
                Application.view
                    >> Query.fromHtml
                    >> Query.find [ id "search-container" ]
                    >> Query.find [ id "search-clear" ]
                    >> Event.simulate Event.click
                    >> Event.expect
                        (ApplicationMsgs.Update <| Msgs.FilterMsg "")
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
                    (ScreenResized { width = 1500, height = 900 })
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                )
                [ it "renders search bar" <|
                    Query.has [ id SearchBar.searchInputId ]
                , it "search bar is an input field" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has [ tag "input" ]
                , it "renders search bar with transparent background to remove white of search bar" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has [ style [ ( "background-color", "transparent" ) ] ]
                , it "search bar does not use browser's built-in autocomplete" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has
                            [ attribute <| Attr.attribute "autocomplete" "off" ]
                , it "sets magnifying glass on search bar in correct position" <|
                    Query.find [ id SearchBar.searchInputId ]
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
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has
                            [ style
                                [ ( "border", searchBarBorder )
                                , ( "color", "#ffffff" )
                                , ( "font-size", "1.15em" )
                                , ( "font-family", "Inconsolata, monospace" )
                                ]
                            ]
                , it "renders search with appropriate size and padding" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has
                            [ style
                                [ ( "height", searchBarHeight )
                                , ( "width", searchBarWidth )
                                , ( "padding", searchBarPadding )
                                ]
                            ]
                , it "does not have an outline when focused" <|
                    Query.find [ id SearchBar.searchInputId ]
                        >> Query.has [ style [ ( "outline", "0" ) ] ]
                , it "has placeholder text" <|
                    Query.find [ id SearchBar.searchInputId ]
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
                    (ScreenResized { width = 400, height = 900 })
                    >> Tuple.first
                )
                [ it "should not have a search bar" <|
                    Application.view
                        >> Query.fromHtml
                        >> Query.hasNot
                            [ id SearchBar.searchInputId ]
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
                        (ApplicationMsgs.Update Msgs.ShowSearchInput)
                    )
                    [ it "tells the ui to focus on the search bar" <|
                        Tuple.second
                            >> Expect.equal
                                [ Effects.Focus SearchBar.searchInputId ]
                    , context "the ui"
                        (Tuple.first
                            >> Application.view
                            >> Query.fromHtml
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
                                (ApplicationMsgs.Update Msgs.FocusMsg)
                            >> Tuple.first
                        )
                        [ it "should display a dropdown of options" <|
                            Application.view
                                >> Query.fromHtml
                                >> Query.find [ id "search-dropdown" ]
                                >> Query.findAll [ tag "li" ]
                                >> Expect.all
                                    [ Query.count (Expect.equal 2)
                                    , Query.index 0 >> Query.has [ text "status: " ]
                                    , Query.index 1 >> Query.has [ text "team: " ]
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
                                (ApplicationMsgs.Update Msgs.BlurMsg)
                                >> Tuple.first
                                >> Application.view
                                >> Query.fromHtml
                            )
                            [ it "should not have a search bar" <|
                                Query.hasNot
                                    [ id SearchBar.searchInputId ]
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
                                (ApplicationMsgs.Update <|
                                    Msgs.FilterMsg "query"
                                )
                                >> Tuple.first
                                >> Application.update
                                    (ApplicationMsgs.Update <| Msgs.BlurMsg)
                                >> Tuple.first
                                >> Application.view
                                >> Query.fromHtml
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
                    (ApplicationMsgs.Update Msgs.FocusMsg)
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.Update <|
                            Msgs.FilterMsg "status:"
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
                    (ApplicationMsgs.Update Msgs.FocusMsg)
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
                        (ApplicationMsgs.Update Msgs.FocusMsg)
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
                        (ApplicationMsgs.Update Msgs.FocusMsg)
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                    >> Query.find [ id "search-dropdown" ]
                    >> Query.children []
                    >> Query.count (Expect.equal 10)
            ]
        , rspecStyleDescribe "dropdown stuff"
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
                    (Callback.APIDataFetched <|
                        Ok
                            ( 0
                            , { teams = [ { id = 0, name = "team" } ]
                              , pipelines =
                                    [ { id = 0
                                      , name = "pipeline"
                                      , paused = False
                                      , public = True
                                      , teamName = "team"
                                      , groups = []
                                      }
                                    ]
                              , jobs = []
                              , resources = []
                              , user = Nothing
                              , version = "0.0.0-dev"
                              }
                            )
                    )
                |> Tuple.first
            )
            [ context "before receiving FocusMsg"
                (Application.view >> Query.fromHtml)
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
                    (ApplicationMsgs.DeliveryReceived <| KeyDown 191)
                    >> Tuple.second
                    >> Expect.equal [ Effects.Focus SearchBar.searchInputId ]
            , it "hitting shift + '/' (= '?') does not focus search input" <|
                Application.update
                    (ApplicationMsgs.DeliveryReceived <|
                        KeyDown Keycodes.shift
                    )
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.DeliveryReceived <|
                            KeyDown 191
                        )
                    >> Tuple.second
                    >> Expect.equal []
            , it "pressing + releasing shift, then '/', focuses search input" <|
                Application.update
                    (ApplicationMsgs.DeliveryReceived <|
                        KeyDown Keycodes.shift
                    )
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.DeliveryReceived <|
                            KeyUp Keycodes.shift
                        )
                    >> Tuple.first
                    >> Application.update
                        (ApplicationMsgs.DeliveryReceived <|
                            KeyDown 191
                        )
                    >> Tuple.second
                    >> Expect.equal [ Effects.Focus SearchBar.searchInputId ]
            , it "hitting other keys does not cause dropdown to expand" <|
                Application.update
                    (ApplicationMsgs.DeliveryReceived <|
                        KeyDown <|
                            Char.toCode 'A'
                    )
                    >> Tuple.first
                    >> Application.view
                    >> Query.fromHtml
                    >> Query.findAll [ id "search-dropdown" ]
                    >> Query.count (Expect.equal 0)
            , context "after receiving FocusMsg"
                (Application.update (ApplicationMsgs.Update Msgs.FocusMsg))
                ([ testDropdown [] [ 0, 1 ] ]
                    ++ [ context "after down arrow keypress"
                            (Tuple.first
                                >> Application.update
                                    (ApplicationMsgs.DeliveryReceived <|
                                        KeyDown 40
                                    )
                            )
                            ([ testDropdown [ 0 ] [ 1 ] ]
                                ++ [ context "after second down arrow keypress"
                                        (Tuple.first
                                            >> Application.update
                                                (ApplicationMsgs.DeliveryReceived <|
                                                    KeyDown 40
                                                )
                                        )
                                        ([ testDropdown [ 1 ] [ 0 ] ]
                                            ++ [ context "after loop around down arrow keypress"
                                                    (Tuple.first
                                                        >> Application.update
                                                            (ApplicationMsgs.DeliveryReceived <|
                                                                KeyDown 40
                                                            )
                                                    )
                                                    [ testDropdown [ 0 ] [ 1 ] ]
                                               , context "after hitting enter"
                                                    (Tuple.first
                                                        >> Application.update
                                                            (ApplicationMsgs.DeliveryReceived <|
                                                                KeyDown 13
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
                                                    KeyDown 13
                                                )
                                        )
                                        [ it "updates the query" <|
                                            Tuple.first
                                                >> Application.view
                                                >> Query.fromHtml
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
                                                        "/?search=status: "
                                                    ]
                                        ]
                                   ]
                            )
                       , context "after up arrow keypress"
                            (Tuple.first
                                >> Application.update
                                    (ApplicationMsgs.DeliveryReceived <|
                                        KeyDown 38
                                    )
                            )
                            ([ testDropdown [ 1 ] [ 0 ] ]
                                ++ [ context "after second up arrow keypress"
                                        (Tuple.first
                                            >> Application.update
                                                (ApplicationMsgs.DeliveryReceived <|
                                                    KeyDown 38
                                                )
                                        )
                                        ([ testDropdown [ 0 ] [ 1 ] ]
                                            ++ [ context "after loop around up arrow keypress"
                                                    (Tuple.first
                                                        >> Application.update
                                                            (ApplicationMsgs.DeliveryReceived <|
                                                                KeyDown 38
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
                                        KeyDown 27
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
        , describe "pause toggle" <|
            let
                givenPipelinePaused =
                    Application.init
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
                        , pathname = "/teams/t/pipelines/p"
                        , search = ""
                        , hash = ""
                        , username = ""
                        , password = ""
                        }
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.PipelineFetched <|
                                Ok
                                    { id = 0
                                    , name = "p"
                                    , paused = True
                                    , public = True
                                    , teamName = "t"
                                    , groups = []
                                    }
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
                        Msgs.TogglePipelinePaused
                            pipelineIdentifier
                            True
            in
            [ defineHoverBehaviour
                { name = "play pipeline icon when authorized"
                , setup = givenPipelinePaused |> givenUserAuthorized
                , query =
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-pause-toggle" ]
                        >> Query.children []
                        >> Query.first
                , updateFunc =
                    \msg ->
                        Application.update msg
                            >> Tuple.first
                , unhoveredSelector =
                    { description = "faded play button with light border"
                    , selector =
                        [ style
                            [ ( "opacity", "0.5" )
                            , ( "margin", "17px" )
                            , ( "cursor", "pointer" )
                            ]
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = "ic-play-white.svg"
                                }
                    }
                , hoveredSelector =
                    { description = "white play button with light border"
                    , selector =
                        [ style
                            [ ( "opacity", "1" )
                            , ( "margin", "17px" )
                            , ( "cursor", "pointer" )
                            ]
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = "ic-play-white.svg"
                                }
                    }
                , mouseEnterMsg =
                    ApplicationMsgs.Update <|
                        Msgs.Hover <|
                            Just <|
                                Msgs.PipelineButton
                                    { pipelineName = "p"
                                    , teamName = "t"
                                    }
                , mouseLeaveMsg =
                    ApplicationMsgs.Update <|
                        Msgs.Hover Nothing
                }
            , defineHoverBehaviour
                { name = "play pipeline icon when unauthenticated"
                , setup = givenPipelinePaused
                , query =
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-pause-toggle" ]
                        >> Query.children []
                        >> Query.first
                , updateFunc =
                    \msg ->
                        Application.update msg
                            >> Tuple.first
                , unhoveredSelector =
                    { description = "faded play button with light border"
                    , selector =
                        [ style
                            [ ( "opacity", "0.5" )
                            , ( "margin", "17px" )
                            , ( "cursor", "pointer" )
                            ]
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = "ic-play-white.svg"
                                }
                    }
                , hoveredSelector =
                    { description = "white play button with light border"
                    , selector =
                        [ style
                            [ ( "opacity", "1" )
                            , ( "margin", "17px" )
                            , ( "cursor", "pointer" )
                            ]
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = "ic-play-white.svg"
                                }
                    }
                , mouseEnterMsg =
                    ApplicationMsgs.Update <|
                        Msgs.Hover <|
                            Just <|
                                Msgs.PipelineButton
                                    { pipelineName = "p"
                                    , teamName = "t"
                                    }
                , mouseLeaveMsg =
                    ApplicationMsgs.Update <|
                        Msgs.Hover Nothing
                }
            , defineHoverBehaviour
                { name = "play pipeline icon when unauthorized"
                , setup = givenPipelinePaused |> givenUserUnauthorized
                , query =
                    Application.view
                        >> Query.fromHtml
                        >> Query.find [ id "top-bar-pause-toggle" ]
                        >> Query.children []
                        >> Query.first
                , updateFunc =
                    \msg ->
                        Application.update msg
                            >> Tuple.first
                , unhoveredSelector =
                    { description = "faded play button with light border"
                    , selector =
                        [ style
                            [ ( "opacity", "0.5" )
                            , ( "margin", "17px" )
                            , ( "cursor", "default" )
                            ]
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = "ic-play-white.svg"
                                }
                    }
                , hoveredSelector =
                    { description = "faded play button with light border"
                    , selector =
                        [ style
                            [ ( "margin", "17px" )
                            , ( "cursor", "default" )
                            , ( "opacity", "0.5" )
                            ]
                        ]
                            ++ iconSelector
                                { size = "20px"
                                , image = "ic-play-white.svg"
                                }
                    }
                , mouseEnterMsg =
                    ApplicationMsgs.Update <|
                        Msgs.Hover <|
                            Just <|
                                Msgs.PipelineButton
                                    { pipelineName = "p"
                                    , teamName = "t"
                                    }
                , mouseLeaveMsg =
                    ApplicationMsgs.Update <|
                        Msgs.Hover Nothing
                }
            , test "clicking play button sends TogglePipelinePaused msg" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-pause-toggle" ]
                        |> Query.children []
                        |> Query.first
                        |> Event.simulate Event.click
                        |> Event.expect toggleMsg
            , test "play button unclickable for non-members" <|
                \_ ->
                    givenPipelinePaused
                        |> givenUserUnauthorized
                        |> Application.view
                        |> Query.fromHtml
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
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-pause-toggle" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has
                            [ style
                                [ ( "animation"
                                  , "container-rotate 1568ms linear infinite"
                                  )
                                , ( "height", "20px" )
                                , ( "width", "20px" )
                                ]
                            ]
            , test "successful PipelineToggled callback turns topbar dark" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.update toggleMsg
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.PipelineToggled pipelineIdentifier <| Ok ())
                        |> Tuple.first
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style [ ( "background-color", backgroundGrey ) ] ]
            , test "successful callback turns spinner into pause button" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.update toggleMsg
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.PipelineToggled pipelineIdentifier <| Ok ())
                        |> Tuple.first
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-pause-toggle" ]
                        |> Query.children []
                        |> Query.first
                        |> Query.has
                            (iconSelector
                                { size = "20px"
                                , image = "ic-pause-white.svg"
                                }
                            )
            , test "Unauthorized PipelineToggled callback redirects to login" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.handleCallback
                            (Callback.PipelineToggled pipelineIdentifier <|
                                Err <|
                                    Http.BadStatus
                                        { url = "http://example.com"
                                        , status =
                                            { code = 401
                                            , message = "unauthorized"
                                            }
                                        , headers = Dict.empty
                                        , body = ""
                                        }
                            )
                        |> Tuple.second
                        |> Expect.equal
                            [ Effects.RedirectToLogin ]
            , test "erroring PipelineToggled callback leaves topbar blue" <|
                \_ ->
                    givenPipelinePaused
                        |> Application.handleCallback
                            (Callback.PipelineToggled pipelineIdentifier <|
                                Err <|
                                    Http.BadStatus
                                        { url = "http://example.com"
                                        , status =
                                            { code = 500
                                            , message = ""
                                            }
                                        , headers = Dict.empty
                                        , body = ""
                                        }
                            )
                        |> Tuple.first
                        |> Application.view
                        |> Query.fromHtml
                        |> Query.find [ id "top-bar-app" ]
                        |> Query.has
                            [ style [ ( "background-color", pausedBlue ) ] ]
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


viewNormally :
    ( Application.Model, List Effects.Effect )
    -> Query.Single ApplicationMsgs.TopLevelMessage
viewNormally =
    Tuple.first >> Application.view >> Query.fromHtml


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
                        [ it ("has the first element highlighted " ++ toString idx) <|
                            Query.index idx
                                >> Query.has [ style [ ( "background-color", "#1e1d1d" ) ] ]
                        , it ("has white text " ++ toString idx) <|
                            Query.index idx
                                >> Query.has [ style [ ( "color", "#ffffff" ) ] ]
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
