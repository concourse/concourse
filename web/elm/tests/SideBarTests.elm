module SideBarTests exposing (all)

import Application.Application as Application
import Colors
import Common
import DashboardTests
import Expect
import Html.Attributes as Attr
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Message
import Message.Subscription as Subscription
import Message.TopLevelMessage as TopLevelMessage
import Routes
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, containing, id, style, tag, text)
import Time
import Url


all : Test
all =
    describe "dashboard sidebar"
        [ test "fetches pipelines on page load" <|
            when iVisitTheDashboard
                >> then_ myBrowserFetchesPipelines
        , test "left hand section of top bar lays out horizontally" <|
            given iAmViewingTheDashboardOnANonPhoneScreen
                >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                >> then_ iSeeItLaysOutHorizontally
        , test "top bar is exactly 54px tall" <|
            given iAmViewingTheDashboardOnANonPhoneScreen
                >> when iAmLookingAtTheTopBar
                >> then_ iSeeItIs54PxTall
        , describe "hamburger icon"
            [ test "appears in the top bar on non-phone screens" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> when iAmLookingAtTheFirstChild
                    >> then_ iSeeAHamburgerIcon
            , test "does not appear on phone screens" <|
                given iAmViewingTheDashboardOnAPhoneScreen
                    >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> then_ iSeeNoHamburgerIcon
            , test "is clickable" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> when iAmLookingAtTheHamburgerMenu
                    >> then_ (itIsClickable Message.HamburgerMenu)
            , DashboardTests.defineHoverBehaviour
                { name = "hamburger icon"
                , setup = iAmViewingTheDashboardOnANonPhoneScreen ()
                , query = iAmLookingAtTheHamburgerMenu
                , unhoveredSelector =
                    { description = "grey"
                    , selector = [ containing [ style "opacity" "0.5" ] ]
                    }
                , hoverable = Message.HamburgerMenu
                , hoveredSelector =
                    { description = "white"
                    , selector = [ containing [ style "opacity" "1" ] ]
                    }
                }
            , test "background becomes lighter on click" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheHamburgerMenu
                    >> then_ iSeeALighterBackground
            , test "background toggles back to dark" <|
                given iHaveAnOpenSideBar
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheHamburgerMenu
                    >> then_ iSeeADarkerBackground
            ]
        , describe "sidebar layout"
            [ test "page below top bar consists of side bar and page content" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeTwoChildren
            , test "sidebar and page contents are side by side" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeItLaysOutHorizontally
            , test "page contents are on the right" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtThePageContents
                    >> then_ iSeeTheUsualDashboardContentsScrollingIndependently
            , test "sidebar is separated from top bar by a thin line" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeADividingLineAbove
            , test "sidebar has frame background" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeADarkerBackground
            , test "sidebar fills height" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItFillsHeight
            , test "sidebar does not shrink" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItDoesNotShrink
            , test "sidebar scrolls independently" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItScrollsIndependently
            , test "sidebar is 275px wide" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItIs275PxWide
            , test "sidebar has right padding" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItHasRightPadding
            ]
        , describe "teams list"
            [ test "team header lays out horizontally" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamHeader
                    >> then_ iSeeItLaysOutHorizontally
            , test "team lays out vertically" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeam
                    >> then_ iSeeItLaysOutVertically
            , test "team has narrower lines" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeam
                    >> then_ iSeeItHasNarrowerLines
            , test "team has top padding" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeam
                    >> then_ iSeeItHasTopPadding
            , test "team header contains an icon group and team name" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamHeader
                    >> then_ iSeeTwoChildren
            , test "icon group is the same width as the hamburger icon" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeItIsAsWideAsTheHamburgerIcon
            , test "icon group lays out horizontally" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeItLaysOutHorizontally
            , test "icon group spreads and centers contents" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeItSpreadsAndCentersContents
            , test "icon group has 5px padding" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeItHas5PxPadding
            , test "icon group contains a team icon and an arrow" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeTwoChildren
            , test "team icon is a picture of two people" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamIcon
                    >> then_ iSeeAPictureOfTwoPeople
            , test "arrow is pointing right" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheArrow
                    >> then_ iSeeARightPointingArrow
            , test "team name has text content of team's name" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeTheTeamName
            , test "team name has large font" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeLargeFont
            , test "team name has padding and margin" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeItHasPaddingAndMargin
            , test "team name has invisble border" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeItHasInvisibleBorder
            , test "team name will ellipsize if it is too long" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeItEllipsizesLongText
            , test "team header is clickable" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamHeader
                    >> then_ (itIsClickable <| Message.SideBarTeam "one-team")
            , DashboardTests.defineHoverBehaviour
                { name = "team header"
                , setup =
                    iAmViewingTheDashboardOnANonPhoneScreen ()
                        |> iClickedTheHamburgerIcon
                , query = iAmLookingAtTheTeamHeader
                , unhoveredSelector =
                    { description = "grey"
                    , selector = [ style "opacity" "0.5" ]
                    }
                , hoverable = Message.SideBarTeam "one-team"
                , hoveredSelector =
                    { description = "white"
                    , selector = [ style "opacity" "1" ]
                    }
                }
            , test "chevron points down when group is clicked" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheArrow
                    >> then_ iSeeABrightDownPointingArrow
            , test "chevron still points down after data refreshes" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> given dataRefreshes
                    >> when iAmLookingAtTheArrow
                    >> then_ iSeeABrightDownPointingArrow
            , test "team name is bright when group is clicked" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeItIsBright
            , test "pipeline list expands when header is clicked" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheTeam
                    >> then_ iSeeItLaysOutVertically
            , test "pipeline list has two children" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtThePipelineList
                    >> then_ iSeeTwoChildren
            , test "pipeline list lays out vertically" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtThePipelineList
                    >> then_ iSeeItLaysOutVertically
            , test "pipeline has two children" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipeline
                    >> then_ iSeeTwoChildren
            , test "pipeline lays out horizontally" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipeline
                    >> then_ iSeeItLaysOutHorizontally
            , test "pipeline centers contents" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipeline
                    >> then_ iSeeItCentersContents
            , test "pipeline has 2.5px padding" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipeline
                    >> then_ iSeeItHas2Point5PxPadding
            , test "pipeline has icon on the left" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineIcon
                    >> then_ iSeeAPipelineIcon
            , test "pipeline icon has left margin" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineIcon
                    >> then_ iSeeItHasLeftMargin
            , test "pipeline icon does not shrink when pipeline name is long" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineIcon
                    >> then_ iSeeItDoesNotShrink
            , test "pipeline icon is dim" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineIcon
                    >> then_ iSeeItIsDim
            , test "pipeline link has 2.5px padding" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineLink
                    >> then_ iSeeItHas2Point5PxPadding
            , test "first pipeline link contains text of pipeline name" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineLink
                    >> then_ iSeeItContainsThePipelineName
            , test "pipeline link is a link to the pipeline" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineLink
                    >> then_ iSeeItIsALinkToTheFirstPipeline
            , test "pipeline link has large font" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineLink
                    >> then_ iSeeLargeFont
            , test "pipeline link will ellipsize if it is too long" <|
                given iHaveAnOpenSideBar
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineLink
                    >> then_ iSeeItEllipsizesLongText
            , DashboardTests.defineHoverBehaviour
                { name = "pipeline link"
                , setup =
                    iAmViewingTheDashboardOnANonPhoneScreen ()
                        |> iClickedTheHamburgerIcon
                        |> iClickedThePipelineGroup
                , query = iAmLookingAtTheFirstPipelineLink
                , unhoveredSelector =
                    { description = "grey"
                    , selector =
                        [ style "opacity" "0.5"
                        , style "border" <| "1px solid " ++ Colors.frame
                        ]
                    }
                , hoverable =
                    Message.SideBarPipeline
                        { pipelineName = "pipeline"
                        , teamName = "one-team"
                        }
                , hoveredSelector =
                    { description = "white with grey square highlight"
                    , selector =
                        [ style "opacity" "1"
                        , style "border" "1px solid #525151"
                        , style "background-color" "#2f2e2e"
                        ]
                    }
                }
            ]
        , test "sidebar remains expanded when toggling high-density view" <|
            given iHaveAnOpenSideBar
                >> given iToggledToHighDensity
                >> when iAmLookingAtThePageBelowTheTopBar
                >> then_ iSeeTwoChildren
        , test "fetches pipelines every 5 seconds" <|
            given iAmViewingTheDashboardOnANonPhoneScreen
                >> when fiveSecondsPass
                >> then_ myBrowserFetchesPipelines
        ]


given =
    identity


when =
    identity


then_ =
    identity


iAmViewingTheDashboardOnANonPhoneScreen =
    iAmViewingTheDashboard
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


iAmLookingAtTheTopBar =
    Common.queryView >> Query.find [ id "top-bar-app" ]


iSeeItIs54PxTall =
    Query.has [ style "height" "54px" ]


iAmLookingAtTheLeftHandSectionOfTheTopBar =
    iAmLookingAtTheTopBar
        >> Query.children []
        >> Query.first


iAmLookingAtTheFirstChild =
    Query.children [] >> Query.first


iSeeAHamburgerIcon =
    Query.has
        (DashboardTests.iconSelector
            { size = hamburgerIconWidth
            , image = "baseline-menu-24px.svg"
            }
        )


hamburgerIconWidth =
    "54px"


iSeeItLaysOutHorizontally =
    Query.has [ style "display" "flex" ]


iSeeItLaysOutVertically =
    Query.has [ style "display" "flex", style "flex-direction" "column" ]


iAmViewingTheDashboardOnAPhoneScreen =
    iAmViewingTheDashboard
        >> Application.handleCallback
            (Callback.ScreenResized
                { scene =
                    { width = 0
                    , height = 0
                    }
                , viewport =
                    { x = 0
                    , y = 0
                    , width = 360
                    , height = 640
                    }
                }
            )
        >> Tuple.first


iAmViewingTheDashboard =
    iVisitTheDashboard
        >> Tuple.first
        >> dataRefreshes


iVisitTheDashboard _ =
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
        , path = "/"
        , query = Nothing
        , fragment = Nothing
        }


dataRefreshes =
    Application.handleCallback
        (Callback.APIDataFetched
            (Ok
                ( Time.millisToPosix 0
                , { teams =
                        [ { name = "one-team", id = 0 }
                        , { name = "other-team", id = 1 }
                        ]
                  , pipelines =
                        [ { id = 0
                          , name = "pipeline"
                          , paused = False
                          , public = True
                          , teamName = "one-team"
                          , groups = []
                          }
                        , { id = 1
                          , name = "other-pipeline"
                          , paused = False
                          , public = True
                          , teamName = "one-team"
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
        )
        >> Tuple.first
        >> Application.handleCallback
            (Callback.PipelinesFetched <|
                Ok
                    [ { id = 0
                      , name = "pipeline"
                      , paused = False
                      , public = True
                      , teamName = "one-team"
                      , groups = []
                      }
                    , { id = 1
                      , name = "other-pipeline"
                      , paused = False
                      , public = True
                      , teamName = "one-team"
                      , groups = []
                      }
                    ]
            )
        >> Tuple.first


thereAreNoPipelines =
    Application.handleCallback
        (Callback.APIDataFetched
            (Ok
                ( Time.millisToPosix 0
                , { teams = []
                  , pipelines = []
                  , jobs = []
                  , resources = []
                  , user = Nothing
                  , version = "0.0.0-dev"
                  }
                )
            )
        )
        >> Tuple.first
        >> Application.handleCallback (Callback.PipelinesFetched <| Ok [])
        >> Tuple.first


iSeeNoHamburgerIcon =
    Query.hasNot
        (DashboardTests.iconSelector
            { size = hamburgerIconWidth
            , image = "baseline-menu-24px.svg"
            }
        )


iAmLookingAtTheHamburgerMenu =
    iAmLookingAtTheLeftHandSectionOfTheTopBar >> iAmLookingAtTheFirstChild


itIsClickable domID =
    Expect.all
        [ Query.has [ style "cursor" "pointer" ]
        , Event.simulate Event.click
            >> Event.expect
                (TopLevelMessage.Update <|
                    Message.Click domID
                )
        ]


iClickedTheHamburgerIcon =
    Application.update
        (TopLevelMessage.Update <| Message.Click Message.HamburgerMenu)
        >> Tuple.first


iSeeALighterBackground =
    Query.has [ style "background-color" "#333333" ]


iSeeADarkerBackground =
    Query.has [ style "background-color" Colors.frame ]


iSeeTwoChildren =
    Query.children [] >> Query.count (Expect.equal 2)


iAmLookingAtThePageBelowTheTopBar =
    Common.queryView
        >> Query.find [ id "page-below-top-bar" ]


iAmLookingAtThePageContents =
    iAmLookingAtThePageBelowTheTopBar
        >> Query.children []
        >> Query.index 1


iSeeTheUsualDashboardContentsScrollingIndependently =
    Expect.all
        [ Query.has
            [ style "box-sizing" "border-box"
            , style "display" "flex"
            , style "height" "100%"
            , style "width" "100%"
            , style "overflow-y" "auto"
            ]
        , Query.has [ text "pipeline" ]
        ]


iAmLookingAtTheSideBar =
    iAmLookingAtThePageBelowTheTopBar >> Query.children [] >> Query.first


iSeeADividingLineAbove =
    Query.has [ style "border-top" <| "1px solid " ++ Colors.background ]


iSeeItIs275PxWide =
    Query.has [ style "width" "275px", style "box-sizing" "border-box" ]


iAmLookingAtTheTeam =
    iAmLookingAtTheSideBar
        >> Query.children [ containing [ text "one-team" ] ]
        >> Query.first


iAmLookingAtTheIconGroup =
    iAmLookingAtTheTeamHeader >> Query.children [] >> Query.first


iSeeItIsAsWideAsTheHamburgerIcon =
    Query.has
        [ style "width" hamburgerIconWidth
        , style "box-sizing" "border-box"
        ]


iAmLookingAtTheTeamIcon =
    iAmLookingAtTheIconGroup >> Query.children [] >> Query.first


iSeeAPictureOfTwoPeople =
    Query.has
        (DashboardTests.iconSelector
            { size = "20px"
            , image = "baseline-people-24px.svg"
            }
        )


iAmLookingAtTheArrow =
    iAmLookingAtTheIconGroup >> Query.children [] >> Query.index 1


iSeeARightPointingArrow =
    Query.has
        (DashboardTests.iconSelector
            { size = "20px"
            , image = "baseline-keyboard-arrow-right-24px.svg"
            }
        )


iSeeTheTeamName =
    Query.has [ text "one-team" ]


iSeeItSpreadsAndCentersContents =
    Query.has
        [ style "align-items" "center"
        , style "justify-content" "space-between"
        ]


iSeeItHas5PxPadding =
    Query.has [ style "padding" "5px" ]


iSeeItHasPaddingAndMargin =
    Query.has [ style "padding" "2.5px", style "margin" "2.5px" ]


iSeeLargeFont =
    Query.has [ style "font-size" "18px" ]


iSeeItEllipsizesLongText =
    Query.has
        [ style "white-space" "nowrap"
        , style "overflow" "hidden"
        , style "text-overflow" "ellipsis"
        ]


iSeeItScrollsIndependently =
    Query.has [ style "overflow-y" "auto" ]


iSeeItFillsHeight =
    Query.has [ style "height" "100%", style "box-sizing" "border-box" ]


iSeeItDoesNotShrink =
    Query.has [ style "flex-shrink" "0" ]


iSeeItHasRightPadding =
    Query.has [ style "padding-right" "10px" ]


iClickedThePipelineGroup =
    Application.update
        (TopLevelMessage.Update <| Message.Click <| Message.SideBarTeam "one-team")
        >> Tuple.first


iSeeABrightDownPointingArrow =
    Query.has
        (style "opacity" "1"
            :: DashboardTests.iconSelector
                { size = "20px"
                , image = "baseline-keyboard-arrow-down-24px.svg"
                }
        )


iSeeItIsBright =
    Query.has [ style "opacity" "1" ]


iAmLookingAtThePipelineList =
    iAmLookingAtTheTeam >> Query.children [] >> Query.index 1


iAmLookingAtTheFirstPipeline =
    iAmLookingAtThePipelineList >> Query.children [] >> Query.first


iAmLookingAtTheFirstPipelineLink =
    iAmLookingAtTheFirstPipeline >> Query.children [] >> Query.index 1


iSeeItContainsThePipelineName =
    Query.has [ text "pipeline" ]


iAmLookingAtTheTeamHeader =
    iAmLookingAtTheTeam >> Query.children [] >> Query.first


iAmLookingAtTheTeamName =
    iAmLookingAtTheTeamHeader >> Query.children [] >> Query.index 1


iSeeItIsALinkToTheFirstPipeline =
    Query.has [ tag "a", attribute <| Attr.href "/teams/one-team/pipelines/pipeline" ]


iToggledToHighDensity =
    Application.update
        (TopLevelMessage.DeliveryReceived <|
            Subscription.RouteChanged <|
                Routes.Dashboard Routes.HighDensity
        )
        >> Tuple.first


fiveSecondsPass =
    Application.handleDelivery
        (Subscription.ClockTicked
            Subscription.FiveSeconds
            (Time.millisToPosix 0)
        )


myBrowserFetchesPipelines =
    Tuple.second
        >> List.member Effects.FetchPipelines
        >> Expect.true "should fetch pipelines"


iHaveAnOpenSideBar =
    iAmViewingTheDashboardOnANonPhoneScreen
        >> iClickedTheHamburgerIcon


iSeeItHasTopPadding =
    Query.has [ style "padding-top" "5px" ]


iSeeItHasInvisibleBorder =
    Query.has [ style "border" <| "1px solid " ++ Colors.frame ]


iSeeItHasNarrowerLines =
    Query.has [ style "line-height" "1.2" ]


iAmLookingAtTheFirstPipelineIcon =
    iAmLookingAtTheFirstPipeline >> Query.children [] >> Query.first


iSeeAPipelineIcon =
    Query.has
        [ style "background-image"
            "url(/public/images/ic-breadcrumb-pipeline.svg)"
        , style "background-repeat" "no-repeat"
        , style "height" "16px"
        , style "width" "32px"
        , style "background-size" "contain"
        ]


iSeeItCentersContents =
    Query.has [ style "align-items" "center" ]


iSeeItHasLeftMargin =
    Query.has [ style "margin-left" "22px" ]


iSeeItIsDim =
    Query.has [ style "opacity" "0.4" ]


iSeeItHas2Point5PxPadding =
    Query.has [ style "padding" "2.5px" ]
