module TopBarTests exposing (all)

import Application.Application as Application
import Assets
import Char
import ColorValues
import Common exposing (defineHoverBehaviour, hoverOver, initCustomOpts, queryView)
import Concourse exposing (JsonValue(..))
import Dashboard.SearchBar as SearchBar
import DashboardTests exposing (iconSelector)
import Data exposing (featureFlags)
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
    "#4BAFF2"


topBarHeight : String
topBarHeight =
    "54px"


searchBarBorder : String -> String
searchBarBorder color =
    "1px solid " ++ color


searchBarHeight : String
searchBarHeight =
    "30px"


searchBarWidth : String
searchBarWidth =
    "251px"


searchBarPadding : String
searchBarPadding =
    "0 42px"


instanceVars : Concourse.InstanceVars
instanceVars =
    Dict.fromList [ ( "var1", JsonString "v1" ), ( "var2", JsonString "v2" ) ]


pipelineInstance : Concourse.Pipeline
pipelineInstance =
    Data.pipeline "team" 1
        |> Data.withName "pipeline"
        |> Data.withInstanceVars instanceVars


archivedPipelineInstance : Concourse.Pipeline
archivedPipelineInstance =
    Data.pipeline "team" 1
        |> Data.withName "pipeline"
        |> Data.withInstanceVars (Dict.fromList [ ( "foo", JsonString "bar" ) ])
        |> Data.withArchived True


all : Test
all =
    describe "TopBar"
        [ rspecStyleDescribe "when on pipeline page"
            (Common.init "/teams/team/pipelines/pipeline"
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok [ Data.pipeline "team" 1 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
            )
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
                , it "does not shows instance group breadcrumb" <|
                    Query.hasNot [ id "breadcrumb-instance-group" ]
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
                    Query.has [ style "background-color" ColorValues.grey100 ]
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
                , it "has the login username styles" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "user-id" ]
                        >> Expect.all
                            [ Query.has
                                [ style "margin" "19px 30px"
                                , style "cursor" "pointer"
                                , style "align-items" "center"
                                , style "justify-content" "center"
                                , style "flex-grow" "1"
                                , style "overflow" "hidden"
                                , style "text-overflow" "ellipsis"
                                ]
                            ]
                , it "shows the logged in displayUserId when the user is logged in" <|
                    Query.children []
                        >> Query.index -1
                        >> Query.find [ id "user-id" ]
                        >> Query.has [ text "displayUserIdTest" ]
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
                        >> Query.has
                            [ style "background-image" <|
                                Assets.backgroundImage <|
                                    Just Assets.PauseIcon
                            ]
                ]
            , it "clicking a pinned resource navigates to the pinned resource page" <|
                Application.update
                    (ApplicationMsgs.Update <|
                        Msgs.GoToRoute
                            (Routes.Resource
                                { id = Data.shortResourceId
                                , page = Nothing
                                , version = Nothing
                                , groups = []
                                }
                            )
                    )
                    >> Tuple.second
                    >> Expect.equal
                        [ Effects.NavigateTo <|
                            Routes.toString <|
                                Routes.Resource
                                    { id = Data.shortResourceId
                                    , page = Nothing
                                    , version = Nothing
                                    , groups = []
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
            , context "when hovering over the pinned icon"
                (hoverOver Msgs.TopBarPinIcon >> Tuple.first)
              <|
                let
                    testWithPinnedVersion version tooltipText =
                        Application.handleCallback
                            (Callback.ResourcesFetched <|
                                Ok
                                    [ Data.resource version ]
                            )
                            >> Tuple.first
                            >> queryView
                            >> Query.find [ id "tooltips" ]
                            >> Query.has [ text tooltipText ]
                in
                [ it "shows correct text when there's resources pinned" <|
                    testWithPinnedVersion (Just "v1")
                        "view pinned resources"
                , it "shows correct text when there's no resources pinned" <|
                    testWithPinnedVersion Nothing
                        "no pinned resources"
                ]
            , context "when hovering over the favorited icon"
                (hoverOver (Msgs.TopBarFavoritedIcon 0) >> Tuple.first)
              <|
                let
                    testWithFavoritedPipelines ids tooltipText =
                        Application.handleDelivery
                            (FavoritedPipelinesReceived <|
                                Ok ids
                            )
                            >> Tuple.first
                            >> Application.handleCallback
                                (Callback.PipelineFetched <|
                                    Ok <|
                                        (Data.pipeline "t" 0 |> Data.withName "p")
                                )
                            >> Tuple.first
                            >> queryView
                            >> Query.find [ id "tooltips" ]
                            >> Query.has [ text tooltipText ]
                in
                [ it "shows correct text when the pipeline is not favorited" <|
                    testWithFavoritedPipelines Set.empty
                        "favorite pipeline"
                , it "shows correct text when the pipeline is favorited" <|
                    testWithFavoritedPipelines (Set.singleton 0)
                        "unfavorite pipeline"
                ]
            , context "when hovering over the pause toggle icon"
                (hoverOver (Msgs.TopBarPauseToggle Data.shortPipelineId) >> Tuple.first)
              <|
                let
                    testWithPaused paused tooltipText =
                        Application.handleCallback
                            (PipelineFetched <|
                                Ok
                                    (Data.pipeline "t" 0
                                        |> Data.withName "p"
                                        |> Data.withPaused paused
                                    )
                            )
                            >> Tuple.first
                            >> queryView
                            >> Query.find [ id "tooltips" ]
                            >> Query.has [ text tooltipText ]
                in
                [ it "shows correct text when the pipeline is not paused" <|
                    testWithPaused False
                        "pause pipeline"
                , it "shows correct text when the pipeline is paused" <|
                    testWithPaused True
                        "unpause pipeline"
                ]
            ]
        , rspecStyleDescribe
            "when on pipeline page for an instanced pipeline"
            (Common.initRoute (Routes.Pipeline { id = Concourse.toPipelineId pipelineInstance, groups = [] })
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok
                            [ pipelineInstance
                            , archivedPipelineInstance
                            , Data.pipeline "team" 2 |> Data.withName "pipeline"
                            ]
                    )
                |> Tuple.first
                |> queryView
            )
            [ it "shows instance group breadcrumb" <|
                Query.has [ id "breadcrumb-instance-group" ]
            , context "instance group breadcrumb"
                (Query.find [ id "breadcrumb-instance-group" ])
                [ it "displays badge containing number of pipelines in group" <|
                    Query.children []
                        >> Query.first
                        >> Query.has [ text "2" ]
                , it "contains the name of the instance group" <|
                    Query.has [ text "pipeline" ]
                , it "is a link to the instance group view" <|
                    Query.has
                        [ tag "a"
                        , Common.routeHref <|
                            Routes.Dashboard
                                { searchType = Routes.Normal "team:\"team\" group:\"pipeline\""
                                , dashboardView = Routes.ViewNonArchivedPipelines
                                }
                        ]
                ]
            , it "has a pipeline breadcrumb" <|
                Query.has [ id "breadcrumb-pipeline" ]
            , context "pipeline breadcrumb"
                (Query.find [ id "breadcrumb-pipeline" ])
                [ it "renders icon first" <|
                    Query.children []
                        >> Query.first
                        >> Query.has pipelineBreadcrumbSelector
                , it "renders instance vars in hyphen notation second" <|
                    Query.children []
                        >> Query.index 1
                        >> Query.has [ text "v1-v2" ]
                , it "has pointer cursor" <|
                    Query.has [ style "cursor" "pointer" ]
                , it "is a link to the relevant pipeline page" <|
                    Query.has
                        [ tag "a"
                        , Common.routeHref (Routes.Pipeline { id = Concourse.toPipelineId pipelineInstance, groups = [] })
                        ]
                ]
            ]
        , rspecStyleDescribe
            "when on pipeline page for an instanced pipeline with no instance vars"
            (Common.init "/teams/team/pipelines/pipeline"
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok
                            [ pipelineInstance
                            , Data.pipeline "team" 2 |> Data.withName "pipeline"
                            ]
                    )
                |> Tuple.first
                |> queryView
            )
            [ it "shows instance group breadcrumb" <|
                Query.has [ id "breadcrumb-instance-group" ]
            , context "pipeline breadcrumb"
                (Query.find [ id "breadcrumb-pipeline" ])
                [ it "renders the empty set" <|
                    Query.children []
                        >> Query.index 1
                        >> Query.has [ text "{}" ]
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
                                , style "background-color" ColorValues.grey100
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
                        ]
            , it "has the login username styles" <|
                Query.children []
                    >> Query.index -1
                    >> Query.find [ id "login-item" ]
                    >> Query.has
                        [ style "margin" "19px 30px"
                        , style "cursor" "pointer"
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
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok [ Data.pipeline "team" 1 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
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
            , it "last breadcrumb has text overflow and ellipsis" <|
                Query.find [ id "breadcrumbs" ]
                    >> Query.children []
                    >> Query.index -1
                    >> Query.has
                        [ style "overflow" "hidden"
                        , style "flex" "1"
                        ]
            ]
        , rspecStyleDescribe
            "when on build page for an instanced pipeline"
            (Common.initRoute
                (Routes.Build
                    { id =
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , pipelineInstanceVars = instanceVars
                        , jobName = "job"
                        , buildName = "1"
                        }
                    , highlight = Routes.HighlightNothing
                    , groups = []
                    }
                )
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok [ pipelineInstance ]
                    )
                |> Tuple.first
                |> queryView
            )
            [ it "shows instance group breadcrumb" <|
                Query.has [ id "breadcrumb-instance-group" ]
            , it "has a pipeline breadcrumb" <|
                Query.has [ id "breadcrumb-pipeline" ]
            , context "pipeline breadcrumb"
                (Query.find [ id "breadcrumb-pipeline" ])
                [ it "renders instance vars in hyphen notation second" <|
                    Query.children []
                        >> Query.index 1
                        >> Query.has [ text "v1-v2" ]
                ]
            ]
        , rspecStyleDescribe "rendering top bar on resource page"
            (Common.init "/teams/team/pipelines/pipeline/resources/resource"
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok [ Data.pipeline "team" 1 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
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
            , it "resource breadcrumb should have a link to itself" <|
                Query.find [ id "breadcrumb-resource" ]
                    >> Query.has
                        [ tag "a"
                        , attribute <|
                            Attr.href
                                "/teams/team/pipelines/pipeline/resources/resource"
                        ]
            , it "last breadcrumb has text overflow and ellipsis" <|
                Query.find [ id "breadcrumbs" ]
                    >> Query.children []
                    >> Query.index -1
                    >> Query.has
                        [ style "overflow" "hidden"
                        , style "flex" "1"
                        ]
            ]
        , rspecStyleDescribe
            "when on resource page for an instanced pipeline"
            (Common.initRoute
                (Routes.Resource
                    { id =
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , pipelineInstanceVars = instanceVars
                        , resourceName = "resource"
                        }
                    , page = Nothing
                    , version = Nothing
                    , groups = []
                    }
                )
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok [ pipelineInstance ]
                    )
                |> Tuple.first
                |> queryView
            )
            [ it "shows instance group breadcrumb" <|
                Query.has [ id "breadcrumb-instance-group" ]
            , it "has a pipeline breadcrumb" <|
                Query.has [ id "breadcrumb-pipeline" ]
            , context "pipeline breadcrumb"
                (Query.find [ id "breadcrumb-pipeline" ])
                [ it "renders instance vars in hyphen notation second" <|
                    Query.children []
                        >> Query.index 1
                        >> Query.has [ text "v1-v2" ]
                ]
            ]
        , rspecStyleDescribe "rendering top bar on causality page"
            (Common.initCustom { initCustomOpts | featureFlags = { featureFlags | resource_causality = True } }
                "/teams/team/pipelines/pipeline/resources/resource/causality/1/downstream"
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok [ Data.pipeline "team" 1 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
                |> Application.handleCallback
                    (Callback.VersionedResourceFetched <|
                        Ok
                            { id = 1
                            , version = Dict.fromList [ ( "ver", "1" ) ]
                            , metadata = []
                            , enabled = True
                            }
                    )
                |> Tuple.first
                |> queryView
            )
            [ it "pipeline breadcrumb should have a link to the pipeline page when viewing resource details" <|
                Query.find [ id "breadcrumb-pipeline" ]
                    >> Query.has
                        [ tag "a"
                        , attribute <|
                            Attr.href
                                "/teams/team/pipelines/pipeline"
                        ]
            , it "resource breadcrumb should have a link to the resource page when viewing resource details" <|
                Query.find [ id "breadcrumb-resource" ]
                    >> Query.has
                        [ tag "a"
                        , attribute <|
                            Attr.href
                                "/teams/team/pipelines/pipeline/resources/resource"
                        ]
            , it "causality breadcrumb should have a link to the resource page with the version filter applied" <|
                Query.find [ id "breadcrumb-causality" ]
                    >> Query.has
                        [ tag "a"
                        , attribute <|
                            Attr.href
                                "/teams/team/pipelines/pipeline/resources/resource?filter=ver%3A1"
                        ]
            , it "last breadcrumb has text overflow and ellipsis" <|
                Query.find [ id "breadcrumbs" ]
                    >> Query.children []
                    >> Query.index -1
                    >> Query.has
                        [ style "overflow" "hidden"
                        , style "flex" "1"
                        ]
            ]
        , rspecStyleDescribe "rendering top bar on job page"
            (Common.init "/teams/team/pipelines/pipeline/jobs/job"
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok [ Data.pipeline "team" 1 |> Data.withName "pipeline" ]
                    )
                |> Tuple.first
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
            , it "last breadcrumb has text overflow and ellipsis" <|
                Query.find [ id "breadcrumbs" ]
                    >> Query.children []
                    >> Query.index -1
                    >> Query.has
                        [ style "overflow" "hidden"
                        , style "flex" "1"
                        ]
            ]
        , rspecStyleDescribe
            "when on job page for an instanced pipeline"
            (Common.initRoute
                (Routes.Job
                    { id =
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , pipelineInstanceVars = instanceVars
                        , jobName = "job"
                        }
                    , page = Nothing
                    , groups = []
                    }
                )
                |> Application.handleCallback
                    (Callback.AllPipelinesFetched <|
                        Ok [ pipelineInstance ]
                    )
                |> Tuple.first
                |> queryView
            )
            [ it "shows instance group breadcrumb" <|
                Query.has [ id "breadcrumb-instance-group" ]
            , it "has a pipeline breadcrumb" <|
                Query.has [ id "breadcrumb-pipeline" ]
            , context "pipeline breadcrumb"
                (Query.find [ id "breadcrumb-pipeline" ])
                [ it "renders instance vars in hyphen notation second" <|
                    Query.children []
                        >> Query.index 1
                        >> Query.has [ text "v1-v2" ]
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
                                , displayUserId = "displayUserIdTest"
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
                                , displayUserId = "displayUserIdTest"
                                }
                        )
                        >> Tuple.first

                pipelineIdentifier =
                    Data.shortPipelineId

                toggleMsg =
                    ApplicationMsgs.Update <|
                        Msgs.Click <|
                            Msgs.TopBarPauseToggle
                                pipelineIdentifier
            in
            [ defineHoverBehaviour
                { name = "play pipeline icon when authorized"
                , setup = givenPipelinePaused |> givenUserAuthorized
                , query = queryView >> Query.find [ id "top-bar-pause-toggle" ]
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
                    Msgs.TopBarPauseToggle pipelineIdentifier
                }
            , defineHoverBehaviour
                { name = "play pipeline icon when unauthenticated"
                , setup = givenPipelinePaused
                , query = queryView >> Query.find [ id "top-bar-pause-toggle" ]
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
                    Msgs.TopBarPauseToggle pipelineIdentifier
                }
            , defineHoverBehaviour
                { name = "play pipeline icon when unauthorized"
                , setup = givenPipelinePaused |> givenUserUnauthorized
                , query = queryView >> Query.find [ id "top-bar-pause-toggle" ]
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
                    Msgs.TopBarPauseToggle pipelineIdentifier
                }
            , test "clicking play button sends TogglePipelinePaused msg" <|
                \_ ->
                    givenPipelinePaused
                        |> queryView
                        |> Query.find [ id "top-bar-pause-toggle" ]
                        |> Event.simulate Event.click
                        |> Event.expect toggleMsg
            , test "play button unclickable for non-members" <|
                \_ ->
                    givenPipelinePaused
                        |> givenUserUnauthorized
                        |> queryView
                        |> Query.find [ id "top-bar-pause-toggle" ]
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
                            [ style "background-color" ColorValues.grey100 ]
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
    { id = "1", userName = "test", name = "Bob", isAdmin = False, email = "bob@bob.com", teams = Dict.empty, displayUserId = "displayUserIdTest" }


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
