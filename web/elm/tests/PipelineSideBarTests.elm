module PipelineSideBarTests exposing
    ( all
    , iAmViewingThePipelinePage
    , iAmViewingThePipelinePageOnANonPhoneScreen
    )

import Application.Application as Application
import Colors
import Common
import DashboardTests
import Dict
import Expect
import Html.Attributes
import Http
import Message.Callback as Callback
import Message.Effects as Effects
import Message.Message as Message
import Message.Subscription as Subscription
import Message.TopLevelMessage as TopLevelMessage
import Routes
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector
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


all : Test
all =
    describe "pipeline sidebar"
        [ describe "hamburger icon"
            [ test "appears in the top bar on non-phone screens" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> then_ iSeeAHamburgerIcon
            , test """has a grey dividing line separating it from the concourse
                      logo""" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> then_ iSeeAGreyDividingLineToTheRight
            , test """has a white dividing line separating it from the concourse
                      logo when the pipeline is paused""" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given thePipelineIsPaused
                    >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> then_ iSeeAWhiteDividingLineToTheRight
            , test "has blue background when the pipeline is paused" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given thePipelineIsPaused
                    >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> then_ iSeeABlueBackground
            , test "does not appear in the top bar on phone screens" <|
                given iAmViewingThePipelinePageOnAPhoneScreen
                    >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> then_ iDoNotSeeAHamburgerIcon
            , test "when shrinking viewport hamburger icon disappears" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given iShrankTheViewport
                    >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> then_ iDoNotSeeAHamburgerIcon
            , describe "before pipelines are fetched"
                [ test "is not clickable" <|
                    given iAmViewingThePipelinePageOnANonPhoneScreen
                        >> when iAmLookingAtTheHamburgerIcon
                        >> then_ itIsNotClickable
                , DashboardTests.defineHoverBehaviour
                    { name = "hamburger icon"
                    , setup = iAmViewingThePipelinePageOnANonPhoneScreen ()
                    , query = iAmLookingAtTheHamburgerIcon
                    , unhoveredSelector =
                        { description = "grey"
                        , selector = [ containing [ style "opacity" "0.5" ] ]
                        }
                    , hoverable = Message.HamburgerMenu
                    , hoveredSelector =
                        { description = "still grey"
                        , selector = [ containing [ style "opacity" "0.5" ] ]
                        }
                    }
                ]
            , test "is not clickable when there are no pipelines" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedNoPipelines
                    >> when iAmLookingAtTheHamburgerIcon
                    >> then_ itIsNotClickable
            , test "shows turbulence when pipelines fail to fetch" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> when myBrowserFailsToFetchPipelines
                    >> then_ iSeeTheTurbulenceMessage
            , describe "after teams and pipelines are fetched"
                [ test "is clickable" <|
                    given iAmViewingThePipelinePageOnANonPhoneScreen
                        >> given myBrowserFetchedPipelines
                        >> when iAmLookingAtTheHamburgerIcon
                        >> then_ (itIsClickable Message.HamburgerMenu)
                , DashboardTests.defineHoverBehaviour
                    { name = "hamburger icon"
                    , setup =
                        iAmViewingThePipelinePageOnANonPhoneScreen ()
                            |> myBrowserFetchedPipelines
                    , query = iAmLookingAtTheHamburgerIcon
                    , unhoveredSelector =
                        { description = "grey"
                        , selector = [ style "opacity" "0.5" ]
                        }
                    , hoverable = Message.HamburgerMenu
                    , hoveredSelector =
                        { description = "white"
                        , selector = [ style "opacity" "1" ]
                        }
                    }
                , test "background becomes lighter on click" <|
                    given iAmViewingThePipelinePageOnANonPhoneScreen
                        >> given myBrowserFetchedPipelines
                        >> given iClickedTheHamburgerIcon
                        >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                        >> then_ iSeeALighterBackground
                , test "background toggles back to dark" <|
                    given iAmViewingThePipelinePageOnANonPhoneScreen
                        >> given myBrowserFetchedPipelines
                        >> given iClickedTheHamburgerIcon
                        >> given iClickedTheHamburgerIcon
                        >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                        >> then_ iSeeADarkerBackground
                ]
            ]
        , describe "sidebar"
            [ test "side bar does not expand before teams and pipelines are fetched" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeNoSideBar
            , test "side bar does not expand when there are no pipelines" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedNoPipelines
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeNoSideBar
            , test "page below top bar consists of side bar and page content" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeASideBar
            , test "side bar and page content appear side by side" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeItLaysOutHorizontally
            , test "toggles away" <|
                given iHaveAnOpenSideBar
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeNoSideBar
            , test "sidebar has frame background" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeADarkerBackground
            , test "sidebar contains pipeline groups" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeSomeChildren
            , test "group header lays out horizontally" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheGroupHeader
                    >> then_ iSeeItLaysOutHorizontally
            , test "group header contains an icon group and team name" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheGroupHeader
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
            , test "sidebar is separated from top bar by a thin line" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeADividingLineAbove
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
            , test "sidebar has two pipeline groups" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeTwoChildren
            , test "sidebar has text content of second team's name" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeTheSecondTeamName
            , test "sidebar shows team name for exposed pipeline" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelines
                    >> given iClickedTheHamburgerIcon
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
            , test "sidebar has right padding" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItHasRightPadding
            , test "sidebar scrolls independently" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItScrollsIndependently
            , test "sidebar is 275px wide" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItIs275PxWide
            , test "team name will ellipsize if it is too long" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeItEllipsizesLongText
            , test "group header is clickable" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtTheGroupHeader
                    >> then_ (itIsClickable <| Message.SideBarTeam "team")
            , describe "hovering group header"
                [ test "group header is hoverable" <|
                    given iHaveAnOpenSideBar
                        >> when iAmLookingAtTheGroupHeader
                        >> then_ (itIsHoverable <| Message.SideBarTeam "team")
                , describe "team icon hover states"
                    [ describe "when pipeline group is collapsed"
                        [ test "is greyed out when unhovered" <|
                            given iHaveAnOpenSideBar
                                >> given iHoveredNothing
                                >> when iAmLookingAtTheTeamIcon
                                >> then_ iSeeItIsGreyedOut
                        , test "is bright when hovered" <|
                            given iHaveAnOpenSideBar
                                >> given iHoveredThePipelineGroup
                                >> when iAmLookingAtTheTeamIcon
                                >> then_ iSeeItIsBright
                        ]
                    , describe "when pipeline group is expanded"
                        [ test "is greyed out when unhovered" <|
                            given iHaveAnExpandedPipelineGroup
                                >> given iHoveredNothing
                                >> when iAmLookingAtTheTeamIcon
                                >> then_ iSeeItIsGreyedOut
                        , test "is bright when hovered" <|
                            given iHaveAnExpandedPipelineGroup
                                >> given iHoveredThePipelineGroup
                                >> when iAmLookingAtTheTeamIcon
                                >> then_ iSeeItIsBright
                        ]
                    ]
                , describe "arrow hover states"
                    [ describe "when pipeline group is collapsed"
                        [ test "is greyed out when unhovered" <|
                            given iHaveAnOpenSideBar
                                >> given iHoveredNothing
                                >> when iAmLookingAtTheArrow
                                >> then_ iSeeItIsGreyedOut
                        , test "is bright when hovered" <|
                            given iHaveAnOpenSideBar
                                >> given iHoveredThePipelineGroup
                                >> when iAmLookingAtTheArrow
                                >> then_ iSeeItIsBright
                        ]
                    , describe "when pipeline group is expanded"
                        [ test "is bright when unhovered" <|
                            given iHaveAnExpandedPipelineGroup
                                >> given iHoveredNothing
                                >> when iAmLookingAtTheArrow
                                >> then_ iSeeItIsBright
                        , test "is bright when hovered" <|
                            given iHaveAnExpandedPipelineGroup
                                >> given iHoveredThePipelineGroup
                                >> when iAmLookingAtTheArrow
                                >> then_ iSeeItIsBright
                        ]
                    ]
                , describe "team name hover states"
                    [ describe "when pipeline group is collapsed"
                        [ test "is greyed out when unhovered" <|
                            given iHaveAnOpenSideBar
                                >> given iHoveredNothing
                                >> when iAmLookingAtTheTeamName
                                >> then_ iSeeItIsGreyedOut
                        , test "is bright when hovered" <|
                            given iHaveAnOpenSideBar
                                >> given iHoveredThePipelineGroup
                                >> when iAmLookingAtTheTeamName
                                >> then_ iSeeItIsBright
                        ]
                    , describe "when pipeline group is expanded"
                        [ test "is bright when unhovered" <|
                            given iHaveAnExpandedPipelineGroup
                                >> given iHoveredNothing
                                >> when iAmLookingAtTheTeamName
                                >> then_ iSeeItIsBright
                        , test "is bright when hovered" <|
                            given iHaveAnExpandedPipelineGroup
                                >> given iHoveredThePipelineGroup
                                >> when iAmLookingAtTheTeamName
                                >> then_ iSeeItIsBright
                        ]
                    ]
                ]
            , test "arrow points down when group is clicked" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtTheExpandedArrow
                    >> then_ iSeeABrightDownPointingArrow
            , test "expanded pipeline group lays out vertically" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtThePipelineGroup
                    >> then_ iSeeItLaysOutVertically
            , test "team name is at the top of expanded pipeline group" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtThePipelineGroup
                    >> then_ iSeeTheTeamNameAbove
            , test "pipeline name is at the bottom of expanded pipeline group" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtThePipelineGroup
                    >> then_ iSeeThePipelineNameBelow
            , test "arrow points right when an expanded group is clicked" <|
                given iHaveAnExpandedPipelineGroup
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheArrow
                    >> then_ iSeeARightPointingArrow
            , test "collapsed pipeline group has no pipeline list" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtThePipelineGroup
                    >> then_ iSeeNoPipelineNames
            , test "pipeline list includes all pipeline names" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtThePipelineList
                    >> then_ iSeeAllPipelineNames
            , test "pipeline names have large font" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtTheFirstPipeline
                    >> then_ iSeeLargeFont
            , test "pipeline names ellipsize if they are too long" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtTheFirstPipeline
                    >> then_ iSeeItEllipsizesLongText
            , test "pipeline names align with the teamName" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtTheFirstPipelineIcon
                    >> then_ iSeeItAlignsWithTheTeamName
            , test "pipeline names have 2.5px padding" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtTheFirstPipeline
                    >> then_ iSeeItHas2Point5PxPadding
            , DashboardTests.defineHoverBehaviour
                { name = "pipeline name"
                , setup =
                    iHaveAnExpandedPipelineGroup ()
                , query = iAmLookingAtTheFirstPipeline
                , unhoveredSelector =
                    { description = "grey text"
                    , selector =
                        [ style "opacity" "0.5"
                        , style "border" <| "1px solid " ++ Colors.frame
                        ]
                    }
                , hoverable =
                    Message.SideBarPipeline
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        }
                , hoveredSelector =
                    { description = "white text with grey background + border"
                    , selector =
                        [ style "opacity" "1"
                        , style "background-color" "#2f2e2e"
                        , style "border" "1px solid #525151"
                        ]
                    }
                }
            , test "pipeline names are links to their respective pages" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iAmLookingAtTheFirstPipeline
                    >> then_ iSeeItIsALinkToThePipeline
            , test "clicking a pipeline link respects sidebar state" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iClickAPipelineLink
                    >> then_ iSeeThePipelineGroupIsStillExpanded
            , test "navigating to the dashboard respects sidebar state" <|
                given iHaveAnExpandedPipelineGroup
                    >> when iNavigateToTheDashboard
                    >> then_ iSeeThePipelineGroupIsStillExpanded
            , test "team containing current pipeline expands when opening sidebar" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheOtherPipelineList
                    >> then_ iSeeOneChild
            , test "current team only automatically expands on page load" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedTheOtherPipelineGroup
                    >> given iNavigateToTheDashboard
                    >> given iNavigateBackToThePipelinePage
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> when iAmLookingAtTheOtherPipelineList
                    >> then_ iSeeNoPipelineNames
            , test "current team name has a grey border" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheOtherTeamName
                    >> then_ iSeeAGreyBorder
            , test "current team name is bright" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedTheOtherPipelineGroup
                    >> when iAmLookingAtTheOtherTeamName
                    >> then_ iSeeItIsBright
            , test "current pipeline name has a grey border" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheOtherPipeline
                    >> then_ iSeeAGreyBorder
            , test "current pipeline name is bright" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheOtherPipeline
                    >> then_ iSeeItIsBright
            , test "pipeline with same name on other team has dark border" <|
                given iAmViewingThePipelinePageOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtThePipelineWithTheSameName
                    >> then_ iSeeADarkBorder
            ]
        , test "fetches pipelines on page load" <|
            when iOpenedThePipelinePage
                >> then_ myBrowserFetchesPipelines
        , test "subscribes to 5-second tick" <|
            when iOpenedThePipelinePage
                >> then_ myBrowserNotifiesEveryFiveSeconds
        , test "refreshes pipeline list every five seconds" <|
            given iOpenedThePipelinePage
                >> when fiveSecondsPass
                >> then_ myBrowserFetchesPipelines
        ]


given =
    identity


when =
    identity


then_ =
    identity


iAmLookingAtTheTopBar =
    Common.queryView >> Query.find [ id "top-bar-app" ]


iAmLookingAtTheLeftHandSectionOfTheTopBar =
    iAmLookingAtTheTopBar
        >> Query.children []
        >> Query.first


iAmViewingThePipelinePageOnANonPhoneScreen =
    iAmViewingThePipelinePage
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


iAmViewingThePipelinePageOnAPhoneScreen =
    iAmViewingThePipelinePage
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


iOpenedThePipelinePage _ =
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
        , path = "/teams/other-team/pipelines/yet-another-pipeline"
        , query = Nothing
        , fragment = Nothing
        }


iAmViewingThePipelinePage =
    iOpenedThePipelinePage >> Tuple.first


iShrankTheViewport =
    Application.handleDelivery (Subscription.WindowResized 300 300) >> Tuple.first


thePipelineIsPaused =
    Application.handleCallback
        (Callback.PipelineFetched
            (Ok
                { id = 1
                , name = "pipeline"
                , paused = True
                , public = True
                , teamName = "team"
                , groups = []
                }
            )
        )
        >> Tuple.first


iAmLookingAtTheHamburgerIcon =
    iAmLookingAtTheTopBar
        >> Query.children []
        >> Query.first


iSeeAGreyDividingLineToTheRight =
    Query.has
        [ style "border-right" <| "1px solid " ++ Colors.background
        , style "opacity" "1"
        ]


iSeeAWhiteDividingLineToTheRight =
    Query.has [ style "border-right" <| "1px solid " ++ Colors.pausedTopbarSeparator ]


itIsClickable domID =
    Expect.all
        [ Query.has [ style "cursor" "pointer" ]
        , Event.simulate Event.click
            >> Event.expect
                (TopLevelMessage.Update <|
                    Message.Click domID
                )
        ]


itIsHoverable domID =
    Expect.all
        [ Event.simulate Event.mouseEnter
            >> Event.expect
                (TopLevelMessage.Update <|
                    Message.Hover <|
                        Just domID
                )
        , Event.simulate Event.mouseLeave
            >> Event.expect
                (TopLevelMessage.Update <|
                    Message.Hover Nothing
                )
        ]


hamburgerIconWidth =
    "54px"


iSeeAHamburgerIcon =
    Query.has
        (DashboardTests.iconSelector
            { size = hamburgerIconWidth
            , image = "baseline-menu-24px.svg"
            }
        )


iDoNotSeeAHamburgerIcon =
    Query.hasNot
        (DashboardTests.iconSelector
            { size = hamburgerIconWidth
            , image = "baseline-menu-24px.svg"
            }
        )


iClickedTheHamburgerIcon =
    Application.update
        (TopLevelMessage.Update <| Message.Click Message.HamburgerMenu)
        >> Tuple.first


iSeeALighterBackground =
    Query.has [ style "background-color" "#333333", style "opacity" "0.5" ]


iSeeADarkerBackground =
    Query.has [ style "background-color" Colors.frame ]


iAmLookingAtThePageBelowTheTopBar =
    Common.queryView >> Query.find [ id "page-below-top-bar" ]


iSeeASideBar =
    Query.has [ id "side-bar" ]


iSeeNoSideBar =
    Query.hasNot [ id "side-bar" ]


iSeeItLaysOutHorizontally =
    Query.has [ style "display" "flex" ]


iAmLookingAtTheSideBar =
    iAmLookingAtThePageBelowTheTopBar >> Query.find [ id "side-bar" ]


myBrowserFetchesPipelines =
    Tuple.second
        >> List.member Effects.FetchPipelines
        >> Expect.true "should fetch pipelines"


myBrowserFetchedPipelinesFromMultipleTeams =
    Application.handleCallback
        (Callback.PipelinesFetched <|
            Ok
                [ { id = 0
                  , name = "pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                , { id = 1
                  , name = "other-pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                , { id = 2
                  , name = "yet-another-pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                , { id = 3
                  , name = "yet-another-pipeline"
                  , paused = False
                  , public = True
                  , teamName = "other-team"
                  , groups = []
                  }
                ]
        )
        >> Tuple.first


myBrowserFetchedPipelines =
    Application.handleCallback
        (Callback.PipelinesFetched <|
            Ok
                [ { id = 0
                  , name = "pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                , { id = 1
                  , name = "other-pipeline"
                  , paused = False
                  , public = True
                  , teamName = "team"
                  , groups = []
                  }
                ]
        )
        >> Tuple.first


itIsNotClickable =
    Expect.all
        [ Query.has [ style "cursor" "default" ]
        , Event.simulate Event.click >> Event.toResult >> Expect.err
        ]


iSeeTheTurbulenceMessage =
    Common.queryView
        >> Query.find [ class "error-message" ]
        >> Query.hasNot [ class "hidden" ]


myBrowserFailsToFetchPipelines =
    Application.handleCallback
        (Callback.PipelinesFetched <|
            Err <|
                Http.BadStatus
                    { url = "http://example.com"
                    , status =
                        { code = 500
                        , message = "internal server error"
                        }
                    , headers = Dict.empty
                    , body = ""
                    }
        )
        >> Tuple.first


iSeeSomeChildren =
    Query.children [] >> Query.count (Expect.greaterThan 0)


iAmLookingAtThePipelineGroup =
    iAmLookingAtTheSideBar >> Query.children [] >> Query.first


iSeeTwoChildren =
    Query.children [] >> Query.count (Expect.equal 2)


iSeeItIsAsWideAsTheHamburgerIcon =
    Query.has
        [ style "width" hamburgerIconWidth
        , style "box-sizing" "border-box"
        , style "flex-shrink" "0"
        ]


iAmLookingAtTheGroupHeader =
    iAmLookingAtThePipelineGroup >> Query.children [] >> Query.first


iAmLookingAtTheIconGroup =
    iAmLookingAtTheGroupHeader >> Query.children [] >> Query.first


iSeeItSpreadsAndCentersContents =
    Query.has
        [ style "align-items" "center"
        , style "justify-content" "space-between"
        ]


iSeeItHas5PxPadding =
    Query.has [ style "padding" "5px" ]


iSeeItHas2Point5PxPadding =
    Query.has [ style "padding" "2.5px" ]


iSeeADividingLineAbove =
    Query.has [ style "border-top" <| "1px solid " ++ Colors.background ]


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


iAmLookingAtTheTeamName =
    iAmLookingAtTheGroupHeader >> Query.children [] >> Query.index 1


iSeeTheTeamName =
    Query.has [ text "team" ]


iHaveAnOpenSideBar =
    iAmViewingThePipelinePageOnANonPhoneScreen
        >> myBrowserFetchedPipelines
        >> iClickedTheHamburgerIcon


iSeeLargeFont =
    Query.has [ style "font-size" "18px" ]


iSeeItHasRightPadding =
    Query.has [ style "padding-right" "10px" ]


iAmLookingAtTheSecondPipelineGroup =
    iAmLookingAtTheSideBar >> Query.children [] >> Query.index 1


iSeeTheSecondTeamName =
    Query.has [ text "other-team" ]


iSeeItScrollsIndependently =
    Query.has [ style "overflow-y" "auto" ]


iSeeItIs275PxWide =
    Query.has [ style "width" "275px", style "box-sizing" "border-box" ]


iSeeItEllipsizesLongText =
    Query.has
        [ style "white-space" "nowrap"
        , style "overflow" "hidden"
        , style "text-overflow" "ellipsis"
        ]


iSeeABlueBackground =
    Query.has [ style "background-color" Colors.paused ]


myBrowserFetchedNoPipelines =
    Application.handleCallback (Callback.PipelinesFetched <| Ok [])
        >> Tuple.first


iClickedThePipelineGroup =
    Application.update
        (TopLevelMessage.Update <|
            Message.Click <|
                Message.SideBarTeam "team"
        )
        >> Tuple.first


iHaveAnExpandedPipelineGroup =
    iHaveAnOpenSideBar >> iClickedThePipelineGroup


iAmLookingAtTheExpandedArrow =
    iAmLookingAtTheArrow


iSeeABrightDownPointingArrow =
    Query.has
        (style "opacity" "1"
            :: DashboardTests.iconSelector
                { size = "20px"
                , image = "baseline-keyboard-arrow-down-24px.svg"
                }
        )


iSeeItIsGreyedOut =
    Query.has [ style "opacity" "0.5" ]


iHoveredThePipelineGroup =
    Application.update
        (TopLevelMessage.Update <|
            Message.Hover <|
                Just <|
                    Message.SideBarTeam "team"
        )
        >> Tuple.first


iSeeItIsBright =
    Query.has [ style "opacity" "1" ]


iHoveredNothing =
    Application.update (TopLevelMessage.Update <| Message.Hover Nothing)
        >> Tuple.first


iSeeItLaysOutVertically =
    Query.has [ style "display" "flex", style "flex-direction" "column" ]


iSeeTheTeamNameAbove =
    Query.children [] >> Query.first >> Query.has [ text "team" ]


iSeeThePipelineNameBelow =
    Query.children [] >> Query.index 1 >> Query.has [ text "pipeline" ]


iSeeNoPipelineNames =
    Query.hasNot [ text "pipeline" ]


iAmLookingAtThePipelineList =
    iAmLookingAtThePipelineGroup >> Query.children [] >> Query.index 1


iSeeAllPipelineNames =
    Query.children []
        >> Expect.all
            [ Query.index 0 >> Query.has [ text "pipeline" ]
            , Query.index 1 >> Query.has [ text "other-pipeline" ]
            ]


iClickedTheOtherPipelineGroup =
    Application.update
        (TopLevelMessage.Update <|
            Message.Click <|
                Message.SideBarTeam "other-team"
        )
        >> Tuple.first


iSeeTheSecondTeamsPipeline =
    Query.has [ text "yet-another-pipeline" ]


iAmLookingAtTheOtherPipelineGroup =
    iAmLookingAtTheSideBar
        >> Query.children [ containing [ text "other-team" ] ]
        >> Query.first


iAmLookingAtTheOtherPipelineList =
    iAmLookingAtTheOtherPipelineGroup
        >> Query.children []
        >> Query.index 1


iAmLookingAtTheOtherTeamName =
    iAmLookingAtTheOtherPipelineGroup
        >> Query.children []
        >> Query.first
        >> Query.children []
        >> Query.index 1


iAmLookingAtTheOtherPipeline =
    iAmLookingAtTheOtherPipelineList >> Query.children [] >> Query.first


iAmLookingAtTheFirstPipeline =
    iAmLookingAtThePipelineList
        >> Query.children []
        >> Query.first
        >> Query.find [ tag "a" ]


iAmLookingAtTheFirstPipelineIcon =
    iAmLookingAtThePipelineList
        >> Query.children []
        >> Query.first
        >> Query.children []
        >> Query.first


iSeeItAlignsWithTheTeamName =
    Query.has [ style "margin-left" "22px" ]


iSeeItIsALinkToThePipeline =
    Query.has
        [ tag "a"
        , attribute <| Html.Attributes.href "/teams/team/pipelines/pipeline"
        ]


iClickAPipelineLink =
    Application.update
        (TopLevelMessage.DeliveryReceived <|
            Subscription.RouteChanged <|
                Routes.Pipeline
                    { groups = []
                    , id =
                        { pipelineName = "other-pipeline"
                        , teamName = "team"
                        }
                    }
        )
        >> Tuple.first


iSeeThePipelineGroupIsStillExpanded =
    iAmLookingAtThePipelineList >> iSeeAllPipelineNames


iNavigateToTheDashboard =
    Application.update
        (TopLevelMessage.DeliveryReceived <|
            Subscription.RouteChanged <|
                Routes.Dashboard (Routes.Normal Nothing)
        )
        >> Tuple.first


iSeeOneChild =
    Query.children [] >> Query.count (Expect.equal 1)


iNavigateBackToThePipelinePage =
    Application.update
        (TopLevelMessage.DeliveryReceived <|
            Subscription.RouteChanged <|
                Routes.Pipeline
                    { groups = []
                    , id =
                        { pipelineName = "yet-another-pipeline"
                        , teamName = "other-team"
                        }
                    }
        )
        >> Tuple.first


iSeeAGreyBorder =
    Query.has [ style "border" <| "1px solid " ++ Colors.groupBorderSelected ]


iSeeADarkBorder =
    Query.has [ style "border" <| "1px solid " ++ Colors.frame ]


iAmLookingAtThePipelineWithTheSameName =
    iAmLookingAtThePipelineList
        >> Query.children [ containing [ text "yet-another-pipeline" ] ]
        >> Query.first


iSeeItHasPaddingAndMargin =
    Query.has [ style "padding" "2.5px", style "margin" "2.5px" ]


myBrowserNotifiesEveryFiveSeconds =
    Tuple.first
        >> Application.subscriptions
        >> List.member (Subscription.OnClockTick Subscription.FiveSeconds)
        >> Expect.true "should tick every five seconds"


fiveSecondsPass =
    Tuple.first
        >> Application.handleDelivery
            (Subscription.ClockTicked
                Subscription.FiveSeconds
                (Time.millisToPosix 0)
            )
