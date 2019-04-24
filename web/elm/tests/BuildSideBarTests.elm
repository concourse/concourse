module BuildSideBarTests exposing (all)

import Application.Application as Application
import Common
import Concourse
import DashboardTests
import Expect
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Message
import Message.TopLevelMessage as TopLevelMessage
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (id, style)
import Url


all : Test
all =
    describe "build page sidebar"
        [ test "fetches pipelines on page load" <|
            when iOpenTheBuildPage
                >> then_ myBrowserFetchesPipelines
        , describe "one-off build page"
            [ test "build page on non-phone screen has hamburger menu" <|
                given iAmLookingAtAOneOffBuildPageOnANonPhoneScreen
                    >> when iAmLookingAtTheLeftSideOfTheTopBar
                    >> then_ iSeeAHamburgerMenu
            , test "page below top bar lays out horizontally" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingBelowTheTopBar
                    >> then_ iSeeItLaysOutHorizontally
            , test "side bar renders on the left" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheLeftSideOfThePage
                    >> then_ iSeeASideBar
            ]
        ]


iOpenTheBuildPage _ =
    Application.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = ""
        , csrfToken = ""
        , authToken = ""
        , pipelineRunningKeyframes = ""
        }
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = "/builds/1"
        , query = Nothing
        , fragment = Nothing
        }


iAmLookingAtAOneOffBuildPageOnANonPhoneScreen =
    iOpenTheBuildPage
        >> Tuple.first
        >> Application.handleCallback
            (Callback.ScreenResized
                { scene =
                    { width = 0
                    , height = 0
                    }
                , viewport =
                    { x = 0
                    , y = 0
                    , width = 1200
                    , height = 900
                    }
                }
            )
        >> Tuple.first
        >> Application.handleCallback
            (Callback.BuildFetched
                (Ok
                    ( 1
                    , { id = 1
                      , name = "1"
                      , job = Nothing
                      , status = Concourse.BuildStatusStarted
                      , duration = { startedAt = Nothing, finishedAt = Nothing }
                      , reapTime = Nothing
                      }
                    )
                )
            )
        >> Tuple.first
        >> Application.handleCallback
            (Callback.PipelinesFetched
                (Ok
                    [ { id = 0
                      , name = "pipeline"
                      , paused = False
                      , public = True
                      , teamName = "team"
                      , groups = []
                      }
                    ]
                )
            )
        >> Tuple.first


iHaveAnOpenSideBar =
    iAmLookingAtAOneOffBuildPageOnANonPhoneScreen
        >> Application.update
            (TopLevelMessage.Update <| Message.Click Message.HamburgerMenu)
        >> Tuple.first


iAmLookingAtTheLeftSideOfTheTopBar =
    Common.queryView
        >> Query.find [ id "top-bar-app" ]
        >> Query.children []
        >> Query.first


iSeeAHamburgerMenu =
    Query.has
        (DashboardTests.iconSelector
            { size = "54px"
            , image = "baseline-menu-24px.svg"
            }
        )


given =
    identity


when =
    identity


then_ =
    identity


myBrowserFetchesPipelines =
    Tuple.second
        >> List.member Effects.FetchPipelines
        >> Expect.true "should fetch pipelines"


iAmLookingAtTheLeftSideOfThePage =
    iAmLookingBelowTheTopBar
        >> Query.children []
        >> Query.first


iSeeASideBar =
    Query.has [ id "side-bar" ]


iAmLookingBelowTheTopBar =
    Common.queryView
        >> Query.find [ id "page-below-top-bar" ]


iSeeItLaysOutHorizontally =
    Query.has [ style "display" "flex" ]
