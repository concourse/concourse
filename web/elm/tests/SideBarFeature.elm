module SideBarFeature exposing (all)

import Application.Application as Application
import Base64
import Colors
import Common
    exposing
        ( defineHoverBehaviour
        , given
        , iOpenTheBuildPage
        , myBrowserFetchedTheBuild
        , then_
        , when
        )
import Concourse
import DashboardTests
import Dict
import Expect
import Html.Attributes as Attr
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


pageLoadIsSideBarCompatible : (() -> ( Application.Model, List Effects.Effect )) -> List Test
pageLoadIsSideBarCompatible iAmLookingAtThePage =
    [ test "fetches pipelines on page load" <|
        when iAmLookingAtThePage
            >> then_ myBrowserFetchesPipelines
    , test "fetches screen size on page load" <|
        when iAmLookingAtThePage
            >> then_ myBrowserFetchesScreenSize
    , test "listens for sidebar state on page load" <|
        when iAmLookingAtThePage
            >> then_ myBrowserListensForSideBarStates
    , test "fetches sidebar state on page load" <|
        when iAmLookingAtThePage
            >> then_ myBrowserFetchesSideBarState
    ]


hasSideBar : (() -> ( Application.Model, List Effects.Effect )) -> List Test
hasSideBar iAmLookingAtThePage =
    let
        iHaveAnOpenSideBar_ =
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> given myBrowserFetchedPipelines
                >> given iClickedTheHamburgerIcon
    in
    [ test "top bar is exactly 54px tall" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> when iAmLookingAtTheTopBar
            >> then_ iSeeItIs54PxTall
    , describe "hamburger icon"
        [ test "appears in the top bar on non-phone screens" <|
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> given iAmLookingAtTheLeftHandSectionOfTheTopBar
                >> when iAmLookingAtTheFirstChild
                >> then_ iSeeAHamburgerIcon
        , test "does not appear on phone screens" <|
            given iAmLookingAtThePage
                >> given iAmOnAPhoneScreen
                >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                >> then_ iSeeNoHamburgerIcon
        , test "is clickable when there are pipelines" <|
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> given myBrowserFetchedPipelines
                >> when iAmLookingAtTheHamburgerMenu
                >> then_ (itIsClickable Message.HamburgerMenu)
        , describe "before pipelines are fetched"
            [ defineHoverBehaviour
                { name = "hamburger icon"
                , setup =
                    iAmLookingAtThePage ()
                        |> given iAmOnANonPhoneScreen
                        |> Tuple.first
                , query = (\a -> ( a, [] )) >> iAmLookingAtTheHamburgerMenu
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
            , test "is not clickable" <|
                given iAmLookingAtThePage
                    >> given iAmOnANonPhoneScreen
                    >> when iAmLookingAtTheHamburgerMenu
                    >> then_ itIsNotClickable
            ]
        , test "is not clickable when there are no pipelines" <|
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> given myBrowserFetchedNoPipelines
                >> when iAmLookingAtTheHamburgerMenu
                >> then_ itIsNotClickable
        , test """has a dark dividing line separating it from the concourse
                  logo""" <|
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> when iAmLookingAtTheHamburgerMenu
                >> then_ iSeeADarkDividingLineToTheRight
        , defineHoverBehaviour
            { name = "hamburger icon"
            , setup =
                iAmLookingAtThePage ()
                    |> iAmOnANonPhoneScreen
                    |> myBrowserFetchedPipelines
                    |> Tuple.first
            , query = (\a -> ( a, [] )) >> iAmLookingAtTheHamburgerMenu
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
        , test "browser saves sidebar state on click" <|
            when iHaveAnOpenSideBar_
                >> then_ myBrowserSavesSideBarState True
        , test "background becomes lighter on click" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheHamburgerMenu
                >> then_ iSeeALighterBackground
        , test "icon becomes bright on click" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheHamburgerIcon
                >> then_ iSeeItIsBright
        , test "browser toggles sidebar state on click" <|
            when iHaveAnOpenSideBar_
                >> given iClickedTheHamburgerIcon
                >> then_ myBrowserSavesSideBarState False
        , test "background toggles back to dark" <|
            given iHaveAnOpenSideBar_
                >> given iClickedTheHamburgerIcon
                >> when iAmLookingAtTheHamburgerMenu
                >> then_ iSeeADarkerBackground
        , test "when shrinking viewport hamburger icon disappears" <|
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> given iShrankTheViewport
                >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                >> then_ iDoNotSeeAHamburgerIcon
        , test "side bar does not expand before teams and pipelines are fetched" <|
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> given iClickedTheHamburgerIcon
                >> when iAmLookingAtThePageBelowTheTopBar
                >> then_ iSeeNoSideBar
        ]
    , describe "sidebar layout"
        [ test "sidebar state is read from sessionstorage" <|
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> given myBrowserFetchedPipelines
                >> given myBrowserReadSideBarState
                >> when iAmLookingAtThePageBelowTheTopBar
                >> then_ iSeeASideBar
        , test "page below top bar contains a side bar" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtThePageBelowTheTopBar
                >> then_ iSeeASideBar
        , test "when shrinking viewport sidebar disappears" <|
            given iHaveAnOpenSideBar_
                >> given iShrankTheViewport
                >> when iAmLookingAtThePageBelowTheTopBar
                >> then_ iSeeNoSideBar
        , test "page below top bar has exactly two children" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtThePageBelowTheTopBar
                >> then_ iSeeTwoChildren
        , test "sidebar and page contents are side by side" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtThePageBelowTheTopBar
                >> then_ iSeeItLaysOutHorizontally
        , test "sidebar is separated from top bar by a thin line" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTopBar
                >> then_ iSeeADividingLineBelow
        , test "sidebar is separated from page contents by a thin line" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeADividingLineToTheRight
        , test "sidebar has hamburger menu background" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeALighterBackground
        , test "sidebar fills height" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeItFillsHeight
        , test "sidebar does not shrink" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeItDoesNotShrink
        , test "sidebar scrolls independently" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeItScrollsIndependently
        , test "sidebar is 275px wide" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeItIs275PxWide
        , test "sidebar has right padding" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeItHasRightPadding
        , test "sidebar has bottom padding" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeItHasBottomPadding
        , test "toggles away" <|
            given iHaveAnOpenSideBar_
                >> given iClickedTheHamburgerIcon
                >> when iAmLookingAtThePageBelowTheTopBar
                >> then_ iSeeNoSideBar
        ]
    , describe "teams list" <|
        let
            iHaveAnExpandedTeam =
                iHaveAnOpenSideBar_ >> iClickedThePipelineGroup
        in
        [ test "sidebar contains pipeline groups" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeSomeChildren
        , test "team header lays out horizontally" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamHeader
                >> then_ iSeeItLaysOutHorizontally
        , test "team header centers contents" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamHeader
                >> then_ iSeeItCentersContents
        , test "team lays out vertically" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeam
                >> then_ iSeeItLaysOutVertically
        , test "team has narrower lines" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeam
                >> then_ iSeeItHasNarrowerLines
        , test "team has top padding" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeam
                >> then_ iSeeItHasTopPadding
        , test "team header contains team icon, arrow, and team name" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamHeader
                >> then_ iSeeThreeChildrenDivs
        , test "team icon is a picture of two people" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamIcon
                >> then_ iSeeAPictureOfTwoPeople
        , test "team icon does not shrink" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamIcon
                >> then_ iSeeItDoesNotShrink
        , test "arrow is pointing right" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheArrow
                >> then_ iSeeARightPointingArrow
        , test "arrow does not shrink" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheArrow
                >> then_ iSeeItDoesNotShrink
        , test "team name has text content of team's name" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamName
                >> then_ iSeeTheTeamName
        , test "team name has large font" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamName
                >> then_ iSeeMediumFont
        , test "team name has padding and margin" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamName
                >> then_ iSeeItHas2Point5PxPadding
        , test "team name stretches" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamName
                >> then_ iSeeItStretches
        , test "team name will ellipsize if it is too long" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamName
                >> then_ iSeeItEllipsizesLongText
        , test "team name will have an id" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamName
                >> then_ iSeeItHasAValidTeamId
        , test "team header is clickable" <|
            given iHaveAnOpenSideBar_
                >> when iAmLookingAtTheTeamHeader
                >> then_ (itIsClickable <| Message.SideBarTeam "team")
        , test "arrow points down when group is clicked" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheArrow
                >> then_ iSeeADownPointingArrow
        , test "arrow still points down after data refreshes" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> given dataRefreshes
                >> when iAmLookingAtTheArrow
                >> then_ iSeeADownPointingArrow
        , test "pipeline list expands when header is clicked" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheTeam
                >> then_ iSeeItLaysOutVertically
        , test "pipeline list has two children" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtThePipelineList
                >> then_ iSeeTwoChildren
        , test "pipeline list lays out vertically" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtThePipelineList
                >> then_ iSeeItLaysOutVertically
        , test "pipeline has two children" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipeline
                >> then_ iSeeTwoChildren
        , test "pipeline lays out horizontally" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipeline
                >> then_ iSeeItLaysOutHorizontally
        , test "pipeline centers contents" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipeline
                >> then_ iSeeItCentersContents
        , test "pipeline has 2.5px padding" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipeline
                >> then_ iSeeItHas2Point5PxPadding
        , test "pipeline has icon on the left" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineIcon
                >> then_ iSeeAPipelineIcon
        , test "pipeline icon has left margin" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineIcon
                >> then_ iSeeItHasLeftMargin
        , test "pipeline icon does not shrink when pipeline name is long" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineIcon
                >> then_ iSeeItDoesNotShrink
        , test "pipeline icon is dim" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineIcon
                >> then_ iSeeItIsDim
        , test "pipeline link has 2.5px padding" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineLink
                >> then_ iSeeItHas2Point5PxPadding
        , test "first pipeline link contains text of pipeline name" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineLink
                >> then_ iSeeItContainsThePipelineName
        , test "pipeline link is a link to the pipeline" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineLink
                >> then_ iSeeItIsALinkToTheFirstPipeline
        , test "pipeline link has large font" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineLink
                >> then_ iSeeMediumFont
        , test "pipeline link stretches" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineLink
                >> then_ iSeeItStretches
        , test "pipeline link will ellipsize if it is too long" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineLink
                >> then_ iSeeItEllipsizesLongText
        , test "pipeline link will have a valid id" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iAmLookingAtTheFirstPipelineLink
                >> then_ iSeeItHasAValidPipelineId
        , test "pipeline icon is bright when pipeline link is hovered" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> given iHoveredThePipelineLink
                >> when iAmLookingAtTheFirstPipelineIcon
                >> then_ iSeeItIsBright
        , test "hovering the pipelink link checks its viewport" <|
            given iHaveAnOpenSideBar_
                >> given iClickedThePipelineGroup
                >> when iHoveredTheFirstPipelineLink
                >> then_ myBrowserChecksAViewport
        , defineHoverBehaviour
            { name = "pipeline link"
            , setup =
                iAmViewingTheDashboardOnANonPhoneScreen ()
                    |> iClickedTheHamburgerIcon
                    |> iClickedThePipelineGroup
                    |> Tuple.first
            , query = (\a -> ( a, [] )) >> iAmLookingAtTheFirstPipelineLink
            , unhoveredSelector =
                { description = "grey"
                , selector =
                    [ style "opacity" "0.3"
                    , style "border" <| "1px solid " ++ Colors.sideBar
                    ]
                }
            , hoverable =
                Message.SideBarPipeline
                    { pipelineName = "pipeline"
                    , teamName = "team"
                    }
            , hoveredSelector =
                { description = "white with grey square highlight"
                , selector =
                    [ style "opacity" "1"
                    , style "border" "1px solid #525151"
                    , style "background-color" "#3A3A3A"
                    ]
                }
            }
        , test "subscribes to 5-second tick" <|
            given iAmLookingAtThePage
                >> then_ myBrowserNotifiesEveryFiveSeconds
        , test "fetches pipelines every 5 seconds" <|
            given iAmLookingAtThePage
                >> when fiveSecondsPass
                >> then_ myBrowserFetchesPipelines
        , test "sidebar has two pipeline groups" <|
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> given myBrowserFetchedPipelinesFromMultipleTeams
                >> given iClickedTheHamburgerIcon
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeTwoChildren
        , test "sidebar has text content of second team's name" <|
            given iAmLookingAtThePage
                >> given iAmOnANonPhoneScreen
                >> given myBrowserFetchedPipelinesFromMultipleTeams
                >> given iClickedTheHamburgerIcon
                >> when iAmLookingAtTheSideBar
                >> then_ iSeeTheSecondTeamName
        , test "pipeline names align with the teamName" <|
            given iHaveAnExpandedTeam
                >> when iAmLookingAtTheFirstPipelineIcon
                >> then_ iSeeItAlignsWithTheTeamName
        ]
    ]


hasCurrentPipelineInSideBar :
    (() -> ( Application.Model, List Effects.Effect ))
    -> List Test
hasCurrentPipelineInSideBar iAmLookingAtThePage =
    [ test "team containing current pipeline expands when opening sidebar" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> given iClickedTheHamburgerIcon
            >> when iAmLookingAtTheOtherPipelineList
            >> then_ iSeeOneChild
    , test "current team only automatically expands on page load" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> given iClickedTheHamburgerIcon
            >> given iClickedTheOtherPipelineGroup
            >> given iNavigateToTheDashboard
            >> given iNavigateBackToThePipelinePage
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> when iAmLookingAtTheOtherPipelineList
            >> then_ iSeeNoPipelineNames
    , test "current team has bright team icon" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> given iClickedTheHamburgerIcon
            >> when iAmLookingAtTheOtherTeamIcon
            >> then_ iSeeItIsBright
    , test "current team name is bright" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> given iClickedTheHamburgerIcon
            >> given iClickedTheOtherPipelineGroup
            >> when iAmLookingAtTheOtherTeamName
            >> then_ iSeeItIsBright
    , test "current pipeline name has a grey border" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> given iClickedTheHamburgerIcon
            >> when iAmLookingAtTheOtherPipelineName
            >> then_ iSeeAGreyBorder
    , test "current pipeline name has grey background" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> given iClickedTheHamburgerIcon
            >> when iAmLookingAtTheOtherPipelineName
            >> then_ iSeeADarkGreyBackground
    , test "current pipeline has bright pipeline icon" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> given iClickedTheHamburgerIcon
            >> when iAmLookingAtTheOtherPipelineIcon
            >> then_ iSeeItIsBright
    , test "current pipeline name is bright" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> given iClickedTheHamburgerIcon
            >> when iAmLookingAtTheOtherPipelineName
            >> then_ iSeeItIsBright
    , test "pipeline with same name on other team has invisible border" <|
        given iAmLookingAtThePage
            >> given iAmOnANonPhoneScreen
            >> given myBrowserFetchedPipelinesFromMultipleTeams
            >> given iClickedTheHamburgerIcon
            >> given iClickedThePipelineGroup
            >> when iAmLookingAtThePipelineWithTheSameName
            >> then_ iSeeAnInvisibleBorder
    ]


all : Test
all =
    describe "sidebar"
        [ describe "on dashboard page" <| hasSideBar (when iVisitTheDashboard)
        , describe "loading dashboard page" <| pageLoadIsSideBarCompatible iVisitTheDashboard
        , describe "dashboard page exceptions"
            [ test "page contents are to the right of the sidebar" <|
                given iHaveAnOpenSideBar
                    >> when iAmLookingAtThePageContents
                    >> then_ iSeeTheUsualDashboardContentsScrollingIndependently
            , test "sidebar remains expanded when toggling high-density view" <|
                given iHaveAnOpenSideBar
                    >> given iToggledToHighDensity
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeTwoChildren
            , test "left hand section of top bar lays out horizontally" <|
                given iVisitTheDashboard
                    >> given iAmOnANonPhoneScreen
                    >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> then_ iSeeItLaysOutHorizontally
            ]
        , describe "loading pipeline page" <| pageLoadIsSideBarCompatible iOpenedThePipelinePage
        , describe "on pipeline page" <| hasSideBar (when iOpenedThePipelinePage)
        , describe "pipeline page current pipeline" <|
            hasCurrentPipelineInSideBar (when iOpenedThePipelinePage)
        , describe "pipeline page exceptions"
            [ describe "hamburger icon"
                [ test "shows turbulence when pipelines fail to fetch" <|
                    given iAmViewingThePipelinePageOnANonPhoneScreen
                        >> when myBrowserFailsToFetchPipelines
                        >> then_ iSeeTheTurbulenceMessage

                -- TODO find a more general description
                ]
            , describe "sidebar"
                [ test "clicking a pipeline link respects sidebar state" <|
                    given iHaveAnExpandedPipelineGroup
                        >> when iClickAPipelineLink
                        >> then_ iSeeThePipelineGroupIsStillExpanded
                , test "navigating to the dashboard respects sidebar state" <|
                    given iHaveAnExpandedPipelineGroup
                        >> when iNavigateToTheDashboard
                        >> then_ iSeeThePipelineGroupIsStillExpanded
                ]
            ]
        , describe "loading build page" <| pageLoadIsSideBarCompatible iOpenTheBuildPage
        , describe "on build page" <| hasSideBar (when iOpenTheBuildPage)
        , describe "build page current pipeline" <|
            hasCurrentPipelineInSideBar (when iOpenTheJobBuildPage)
        , describe "build page exceptions"
            [ test "current team is expanded when pipelines are fetched before build" <|
                given iOpenTheBuildPage
                    >> given iAmOnANonPhoneScreen
                    >> given myBrowserFetchedPipelinesFromMultipleTeams
                    >> given myBrowserFetchedTheBuild
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheOtherPipelineList
                    >> then_ iSeeOneChild
            ]
        , describe "loading job page" <| pageLoadIsSideBarCompatible iOpenTheJobPage
        , describe "on job page" <| hasSideBar (when iOpenTheJobPage)
        , describe "job page current pipeline" <|
            hasCurrentPipelineInSideBar (when iOpenTheJobPage)
        , describe "loading resource page" <| pageLoadIsSideBarCompatible iOpenTheResourcePage
        , describe "on resource page" <| hasSideBar (when iOpenTheResourcePage)
        , describe "resource page current pipeline" <|
            hasCurrentPipelineInSideBar (when iOpenTheResourcePage)
        , describe "on notfound page" <| hasSideBar (when iOpenTheNotFoundPage)
        ]


iAmViewingTheDashboardOnANonPhoneScreen =
    iAmViewingTheDashboard
        >> iAmOnANonPhoneScreen


iAmOnANonPhoneScreen =
    Tuple.first
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


iAmLookingAtTheTopBar =
    Tuple.first >> Common.queryView >> Query.find [ id "top-bar-app" ]


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
        >> iAmOnAPhoneScreen


iAmOnAPhoneScreen =
    Tuple.first
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


iAmViewingTheDashboard =
    iVisitTheDashboard
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


apiDataLoads =
    Tuple.first
        >> Application.handleCallback
            (Callback.APIDataFetched
                (Ok
                    ( Time.millisToPosix 0
                    , { teams =
                            [ { name = "team", id = 0 }
                            , { name = "other-team", id = 1 }
                            ]
                      , pipelines =
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
                      , jobs = []
                      , resources = []
                      , user = Nothing
                      , version = "0.0.0-dev"
                      }
                    )
                )
            )


dataRefreshes =
    apiDataLoads
        >> Tuple.first
        >> Application.handleCallback
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
    iAmLookingAtTheTopBar
        >> Query.find [ id "hamburger-menu" ]


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
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.Click Message.HamburgerMenu)


iSeeALighterBackground =
    Query.has [ style "background-color" "#333333" ]


iSeeADarkerBackground =
    Query.has [ style "background-color" Colors.frame ]


iSeeTwoChildren =
    Query.children [] >> Query.count (Expect.equal 2)


iAmLookingAtThePageBelowTheTopBar =
    Tuple.first
        >> Common.queryView
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


iSeeADividingLineBelow =
    Query.has [ style "border-bottom" <| "1px solid " ++ Colors.frame ]


iSeeADividingLineToTheRight =
    Query.has [ style "border-right" <| "1px solid " ++ Colors.frame ]


iSeeItIs275PxWide =
    Query.has [ style "width" "275px", style "box-sizing" "border-box" ]


iAmLookingAtTheTeam =
    iAmLookingAtTheSideBar
        >> Query.children [ containing [ text "team" ] ]
        >> Query.first


iSeeItIsAsWideAsTheHamburgerIcon =
    Query.has
        [ style "width" hamburgerIconWidth
        , style "box-sizing" "border-box"
        ]


iAmLookingAtTheTeamIcon =
    iAmLookingAtTheTeamHeader >> Query.children [] >> Query.first


iSeeAPictureOfTwoPeople =
    Query.has
        (DashboardTests.iconSelector
            { size = "20px"
            , image = "baseline-people-24px.svg"
            }
        )


iAmLookingAtTheArrow =
    iAmLookingAtTheTeamHeader >> Query.children [] >> Query.index 1


iSeeARightPointingArrow =
    Query.has
        (DashboardTests.iconSelector
            { size = "12px"
            , image = "baseline-keyboard-arrow-right-24px.svg"
            }
        )


iSeeTheTeamName =
    Query.has [ text "team" ]


iSeeItSpreadsAndCentersContents =
    Query.has
        [ style "align-items" "center"
        , style "justify-content" "space-between"
        ]


iSeeItHas5PxPadding =
    Query.has [ style "padding" "5px" ]


iSeeItHasPaddingAndMargin =
    Query.has [ style "padding" "2.5px", style "margin" "2.5px" ]


iSeeMediumFont =
    Query.has [ style "font-size" "14px" ]


iSeeItEllipsizesLongText =
    Query.has
        [ style "white-space" "nowrap"
        , style "overflow" "hidden"
        , style "text-overflow" "ellipsis"
        ]


iSeeItHasAValidTeamId =
    Query.has [ id <| Base64.encode "team" ]


iSeeItHasAValidPipelineId =
    Query.has
        [ id <|
            (Base64.encode "team"
                ++ "_"
                ++ Base64.encode "pipeline"
            )
        ]


iSeeItScrollsIndependently =
    Query.has [ style "overflow-y" "auto" ]


iSeeItFillsHeight =
    Query.has [ style "height" "100%", style "box-sizing" "border-box" ]


iSeeItDoesNotShrink =
    Query.has [ style "flex-shrink" "0" ]


iSeeItHasRightPadding =
    Query.has [ style "padding-right" "10px" ]


iSeeItHasBottomPadding =
    Query.has [ style "padding-bottom" "10px" ]


iClickedThePipelineGroup =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <| Message.Click <| Message.SideBarTeam "team")


iSeeADownPointingArrow =
    Query.has
        (DashboardTests.iconSelector
            { size = "12px"
            , image = "baseline-keyboard-arrow-down-24px.svg"
            }
        )


iSeeItIsBright =
    Query.has [ style "opacity" "1" ]


iSeeItIsGreyedOut =
    Query.has [ style "opacity" "0.7" ]


iSeeItIsDim =
    Query.has [ style "opacity" "0.3" ]


iAmLookingAtThePipelineList =
    iAmLookingAtTheTeam >> Query.children [] >> Query.index 1


iAmLookingAtTheFirstPipeline =
    iAmLookingAtThePipelineList >> Query.children [] >> Query.first


iAmLookingAtTheFirstPipelineLink =
    iAmLookingAtTheFirstPipeline >> Query.children [] >> Query.index 1


iHoveredTheFirstPipelineLink =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <|
                Message.Hover <|
                    Just <|
                        Message.SideBarPipeline
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            }
            )


theFirstPipelineLinkWasOverflowing =
    Tuple.first
        >> Application.handleCallback
            (Callback.GotViewport <|
                Ok
                    { scene = { width = 1, height = 0 }
                    , viewport = { width = 0, height = 0, x = 0, y = 0 }

                    -- , element = { width = 0, height = 0, x = 0, y = 0 }
                    }
            )


iSeeItContainsThePipelineName =
    Query.has [ text "pipeline" ]


iAmLookingAtTheTeamHeader =
    iAmLookingAtTheTeam >> Query.children [] >> Query.first


iAmLookingAtTheTeamName =
    iAmLookingAtTheTeamHeader >> Query.children [] >> Query.index 2


iSeeItIsALinkToTheFirstPipeline =
    Query.has
        [ tag "a", attribute <| Attr.href "/teams/team/pipelines/pipeline" ]


iToggledToHighDensity =
    Tuple.first
        >> Application.update
            (TopLevelMessage.DeliveryReceived <|
                Subscription.RouteChanged <|
                    Routes.Dashboard Routes.HighDensity
            )


fiveSecondsPass =
    Tuple.first
        >> Application.handleDelivery
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
    Query.has [ style "margin-left" "28px" ]


iSeeItHas2Point5PxPadding =
    Query.has [ style "padding" "2.5px" ]


iSeeASideBar =
    Query.has [ id "side-bar" ]


iAmLookingAtTheLeftSideOfThePage =
    iAmLookingBelowTheTopBar
        >> Query.children []
        >> Query.first


iAmLookingBelowTheTopBar =
    Tuple.first
        >> Common.queryView
        >> Query.find [ id "page-below-top-bar" ]


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
    Tuple.first >> Application.handleDelivery (Subscription.WindowResized 300 300)


thePipelineIsPaused =
    Tuple.first
        >> Application.handleCallback
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


iAmLookingAtTheHamburgerIcon =
    iAmLookingAtTheHamburgerMenu
        >> Query.children []
        >> Query.first


iSeeADarkDividingLineToTheRight =
    Query.has
        [ style "border-right" <| "1px solid " ++ Colors.frame
        , style "opacity" "1"
        ]


iSeeAWhiteDividingLineToTheRight =
    Query.has
        [ style "border-right" <| "1px solid " ++ Colors.pausedTopbarSeparator ]


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


iDoNotSeeAHamburgerIcon =
    Query.hasNot
        (DashboardTests.iconSelector
            { size = hamburgerIconWidth
            , image = "baseline-menu-24px.svg"
            }
        )


iSeeNoSideBar =
    Query.hasNot [ id "side-bar" ]


myBrowserFetchedPipelinesFromMultipleTeams =
    Tuple.first
        >> Application.handleCallback
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


myBrowserFetchedPipelines =
    Tuple.first
        >> Application.handleCallback
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


itIsNotClickable =
    Expect.all
        [ Query.has [ style "cursor" "default" ]
        , Event.simulate Event.click >> Event.toResult >> Expect.err
        ]


iSeeTheTurbulenceMessage =
    Tuple.first
        >> Common.queryView
        >> Query.find [ class "error-message" ]
        >> Query.hasNot [ class "hidden" ]


myBrowserFailsToFetchPipelines =
    Tuple.first
        >> Application.handleCallback
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


iSeeSomeChildren =
    Query.children [] >> Query.count (Expect.greaterThan 0)


iAmLookingAtThePipelineGroup =
    iAmLookingAtTheSideBar >> Query.children [] >> Query.first


iAmLookingAtTheGroupHeader =
    iAmLookingAtThePipelineGroup >> Query.children [] >> Query.first


iAmLookingAtTheSecondPipelineGroup =
    iAmLookingAtTheSideBar >> Query.children [] >> Query.index 1


iSeeTheSecondTeamName =
    Query.has [ text "other-team" ]


iSeeABlueBackground =
    Query.has [ style "background-color" Colors.paused ]


myBrowserFetchedNoPipelines =
    Tuple.first >> Application.handleCallback (Callback.PipelinesFetched <| Ok [])


iHaveAnExpandedPipelineGroup =
    iHaveAnOpenSideBar >> iClickedThePipelineGroup


iAmLookingAtTheExpandedArrow =
    iAmLookingAtTheArrow


iHoveredThePipelineGroup =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <|
                Message.Hover <|
                    Just <|
                        Message.SideBarTeam "team"
            )


iHoveredThePipelineLink =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <|
                Message.Hover <|
                    Just <|
                        Message.SideBarPipeline
                            { pipelineName = "pipeline"
                            , teamName = "team"
                            }
            )


iHoveredNothing =
    Tuple.first
        >> Application.update (TopLevelMessage.Update <| Message.Hover Nothing)


iSeeTheTeamNameAbove =
    Query.children [] >> Query.first >> Query.has [ text "team" ]


iSeeThePipelineNameBelow =
    Query.children [] >> Query.index 1 >> Query.has [ text "pipeline" ]


iSeeNoPipelineNames =
    Query.hasNot [ text "pipeline" ]


iSeeAllPipelineNames =
    Query.children []
        >> Expect.all
            [ Query.index 0 >> Query.has [ text "pipeline" ]
            , Query.index 1 >> Query.has [ text "other-pipeline" ]
            ]


iClickedTheOtherPipelineGroup =
    Tuple.first
        >> Application.update
            (TopLevelMessage.Update <|
                Message.Click <|
                    Message.SideBarTeam "other-team"
            )


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
        >> Query.index 2


iAmLookingAtTheOtherTeamIcon =
    iAmLookingAtTheOtherPipelineGroup
        >> Query.children []
        >> Query.first
        >> Query.children []
        >> Query.first


iAmLookingAtTheOtherPipelineName =
    iAmLookingAtTheOtherPipelineList
        >> Query.children []
        >> Query.first
        >> Query.children []
        >> Query.index 1


iAmLookingAtTheOtherPipelineIcon =
    iAmLookingAtTheOtherPipelineList
        >> Query.children []
        >> Query.first
        >> Query.children []
        >> Query.first


iSeeItAlignsWithTheTeamName =
    Query.has [ style "margin-left" "28px" ]


iSeeItIsALinkToThePipeline =
    Query.has
        [ tag "a"
        , attribute <| Attr.href "/teams/team/pipelines/pipeline"
        ]


iClickAPipelineLink =
    Tuple.first
        >> Application.update
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


iSeeThePipelineGroupIsStillExpanded =
    iAmLookingAtThePipelineList >> iSeeAllPipelineNames


iNavigateToTheDashboard =
    Tuple.first
        >> Application.update
            (TopLevelMessage.DeliveryReceived <|
                Subscription.RouteChanged <|
                    Routes.Dashboard (Routes.Normal Nothing)
            )


iSeeOneChild =
    Query.children [] >> Query.count (Expect.equal 1)


iNavigateBackToThePipelinePage =
    Tuple.first
        >> Application.update
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


iSeeAGreyBorder =
    Query.has [ style "border" <| "1px solid " ++ Colors.groupBorderSelected ]


iSeeAnInvisibleBorder =
    Query.has [ style "border" <| "1px solid " ++ Colors.sideBar ]


iAmLookingAtThePipelineWithTheSameName =
    iAmLookingAtThePipelineList
        >> Query.children [ containing [ text "yet-another-pipeline" ] ]
        >> Query.first


myBrowserNotifiesEveryFiveSeconds =
    Tuple.first
        >> Application.subscriptions
        >> List.member (Subscription.OnClockTick Subscription.FiveSeconds)
        >> Expect.true "should tick every five seconds"


iOpenTheJobBuildPage =
    iOpenTheBuildPage
        >> myBrowserFetchedTheBuild


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


myBrowserFetchesScreenSize =
    Tuple.second
        >> List.member Effects.GetScreenSize
        >> Expect.true "should fetch screen size"


iOpenTheJobPage _ =
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
        , path = "/teams/other-team/pipelines/yet-another-pipeline/jobs/job"
        , query = Nothing
        , fragment = Nothing
        }


iOpenTheResourcePage _ =
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
        , path = "/teams/other-team/pipelines/yet-another-pipeline/resources/r"
        , query = Nothing
        , fragment = Nothing
        }


iOpenTheNotFoundPage =
    iOpenTheJobPage
        >> Tuple.first
        >> Application.handleCallback
            (Callback.JobFetched <|
                Err <|
                    Http.BadStatus
                        { url = "http://example.com"
                        , status =
                            { code = 404
                            , message = "not found"
                            }
                        , headers = Dict.empty
                        , body = ""
                        }
            )


iSeeAGreyBackground =
    Query.has [ style "background-color" "#353434" ]


iSeeADarkGreyBackground =
    Query.has [ style "background-color" "#272727" ]


iSeeItStretches =
    Query.has [ style "flex-grow" "1" ]


iSeeThreeChildrenDivs =
    Query.children [ tag "div" ] >> Query.count (Expect.equal 3)


myBrowserListensForSideBarStates =
    Tuple.first
        >> Application.subscriptions
        >> List.member Subscription.OnSideBarStateReceived
        >> Expect.true "should listen for sidebar states"


myBrowserReadSideBarState =
    Tuple.first
        >> Application.handleDelivery
            (Subscription.SideBarStateReceived (Just "true"))


myBrowserFetchesSideBarState =
    Tuple.second
        >> List.member Effects.LoadSideBarState
        >> Expect.true "should get sidebar state"


myBrowserSavesSideBarState isOpen =
    Tuple.second
        >> List.member (Effects.SaveSideBarState isOpen)
        >> Expect.true "should save sidebar state"


myBrowserChecksAViewport =
    Tuple.second
        >> List.any
            (\e ->
                case e of
                    Effects.GetViewportOf _ ->
                        True

                    _ ->
                        False
            )
        >> Expect.true "should get viewport"
