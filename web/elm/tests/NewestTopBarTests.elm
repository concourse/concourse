module NewestTopBarTests exposing (all)

import Callback exposing (Callback(..))
import Dict
import Effects
import Expect exposing (..)
import Html.Attributes as Attr
import Html.Styled exposing (toUnstyled)
import NewestTopBar
import QueryString
import Routes
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector as Selector exposing (attribute, class, containing, id, style, tag, text)
import UserState exposing (UserState(..))


rspecStyleDescribe : String -> model -> List (model -> Test) -> Test
rspecStyleDescribe description beforeEach subTests =
    Test.describe description
        (subTests |> List.map (\f -> f beforeEach))


it : String -> (model -> Expectation) -> model -> Test
it desc expectationFunc model =
    Test.test desc <|
        \_ -> expectationFunc model


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
    "#504b4b"


all : Test
all =
    describe "NewestTopBar"
        [ rspecStyleDescribe "rendering top bar on pipeline page"
            (NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                |> Tuple.first
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "concourse logo is visible on top bar" <|
                Query.children []
                    >> Query.index 1
                    >> Query.has
                        [ style
                            [ ( "background-image", "url(/public/images/concourse_logo_white.svg)" )
                            , ( "background-position", "50% 50%" )
                            , ( "background-repeat", "no-repeat" )
                            , ( "background-size", "42px 42px" )
                            , ( "width", topBarHeight )
                            , ( "height", topBarHeight )
                            ]
                        ]
            , it "top bar renders pipeline breadcrumb selector" <|
                Query.has [ id "breadcrumb-pipeline" ]
            , it "top bar has pipeline breadcrumb with icon rendered first" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.first
                    >> Query.has pipelineBreadcrumbSelector
            , it "top bar has pipeline name after pipeline icon" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has
                        [ text "pipeline" ]
            , it "pipeline breadcrumb should have a link to the pipeline page" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.children []
                    >> Query.index 1
                    >> Query.has [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]
            , it "top bar is 56px tall with dark grey background" <|
                Query.has [ style [ ( "background-color", backgroundGrey ), ( "height", lineHeight ) ] ]
            , it "top bar lays out contents horizontally" <|
                Query.has [ style [ ( "display", "flex" ) ] ]
            , it "top bar centers contents vertically" <|
                Query.has [ style [ ( "align-items", "center" ) ] ]
            , it "top bar maximizes spacing between the left and right navs" <|
                Query.has [ style [ ( "justify-content", "space-between" ) ] ]
            , it "renders the login component as the last item in the top bar" <|
                Query.children []
                    >> Query.index -1
                    >> Query.has [ id "login-component" ]
            ]
        , rspecStyleDescribe "login component when user is logged in"
            (NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                |> Tuple.first
                |> logInUser
                |> NewestTopBar.view
                |> toUnstyled
                |> Query.fromHtml
            )
            [ it "renders the login component as the last item in the top bar" <|
                Query.children []
                    >> Query.index -1
                    >> Query.has [ id "login-component" ]
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
            , it "shows the logged in username when the user is logged in" <|
                Query.children []
                    >> Query.index -1
                    >> Query.find [ id "login-item" ]
                    >> Query.has [ text "test" ]
            , it "ToggleUserMenu message is received when login menu is clicked" <|
                Query.find [ id "login-container" ]
                    >> Event.simulate Event.click
                    >> Event.expect NewestTopBar.ToggleUserMenu
            , it "renders login menu with empty content" <|
                Query.children []
                    >> Query.index -1
                    >> Query.find [ id "login-item" ]
                    >> Expect.all [ Query.has [ id "login-menu" ], Query.find [ id "login-menu" ] >> Query.children [] >> Query.count (Expect.equal 0) ]
            ]
        , rspecStyleDescribe "rendering user menus on clicks"
            (NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
                |> Tuple.first
                |> logInUser
            )
            [ it "shows user menu when ToggleUserMenu msg is received" <|
                NewestTopBar.update NewestTopBar.ToggleUserMenu
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.has [ id "logout-button" ]
            , it "renders user menu content when ToggleUserMenu msg is received and logged in" <|
                NewestTopBar.update NewestTopBar.ToggleUserMenu
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
                NewestTopBar.update NewestTopBar.ToggleUserMenu
                    >> Tuple.first
                    >> NewestTopBar.view
                    >> toUnstyled
                    >> Query.fromHtml
                    >> Query.find [ id "logout-button" ]
                    >> Event.simulate Event.click
                    >> Event.expect NewestTopBar.LogOut
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
            (NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" }
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
            (NewestTopBar.init { logical = Routes.Pipeline "team" "pipeline", queries = QueryString.empty, page = Nothing, hash = "" })
            [ it "redirects to login page when you click login" <|
                Tuple.first
                    >> NewestTopBar.update NewestTopBar.LogIn
                    >> Tuple.second
                    >> Expect.equal [ Effects.RedirectToLogin ]
            ]
        , rspecStyleDescribe "rendering top bar on build page"
            (NewestTopBar.init { logical = Routes.Build "team" "pipeline" "job" "1", queries = QueryString.empty, page = Nothing, hash = "" }
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
            (NewestTopBar.init { logical = Routes.Resource "team" "pipeline" "resource", queries = QueryString.empty, page = Nothing, hash = "" }
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
            (NewestTopBar.init { logical = Routes.Job "team" "pipeline" "job", queries = QueryString.empty, page = Nothing, hash = "" }
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
        , rspecStyleDescribe "rendering search on dashboard page"
            (NewestTopBar.init { logical = Routes.Dashboard, queries = QueryString.empty, page = Nothing, hash = "" }
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
                            [ ( "background-image", "url('public/images/ic_search_white_24px.svg')" )
                            , ( "background-position", "12px 8px" )
                            , ( "background-repeat", "no-repeat" )
                            ]
                        ]
            , it "styles search border and input text colour" <|
                Query.find [ id "search-bar" ]
                    >> Query.has
                        [ style
                            [ ( "border", "1px solid " ++ searchBarBorder )
                            , ( "color", "#fff" )
                            , ( "font-size", "1.15em" )
                            , ( "font-family", "Inconsolata, monospace" )
                            ]
                        ]
            , it "renders search with appropriate size and padding" <|
                Query.find [ id "search-bar" ]
                    >> Query.has [ style [ ( "height", "30px" ), ( "width", "220px" ), ( "padding", "0 42px" ) ] ]
            ]
        ]


logoutUser : NewestTopBar.Model -> NewestTopBar.Model
logoutUser model =
    { model | userState = UserState.UserStateLoggedOut }


logInUser : NewestTopBar.Model -> NewestTopBar.Model
logInUser model =
    { model | userState = UserState.UserStateLoggedIn { id = "1", userName = "test", name = "Bob", email = "bob@bob.com", teams = Dict.empty } }


pipelineBreadcrumbSelector : List Selector.Selector
pipelineBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic_breadcrumb_pipeline.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


jobBreadcrumbSelector : List Selector.Selector
jobBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic_breadcrumb_job.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]


resourceBreadcrumbSelector : List Selector.Selector
resourceBreadcrumbSelector =
    [ style
        [ ( "background-image", "url(/public/images/ic_breadcrumb_resource.svg)" )
        , ( "background-repeat", "no-repeat" )
        ]
    ]
