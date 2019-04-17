module SideBarTests exposing (all)

import Application.Application as Application
import Colors
import Common
import DashboardTests
import Expect
import Message.Callback as Callback
import Message.Message as Message
import Message.TopLevelMessage as TopLevelMessage
import Test exposing (Test, describe, test)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (id, style, text)
import Time
import Url


all : Test
all =
    describe "dashboard sidebar"
        [ test "left hand section of top bar lays out horizontally" <|
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
            , test "does not appear if there are no pipelines" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given thereAreNoPipelines
                    >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                    >> then_ iSeeNoHamburgerIcon
            , test "is separated from the concourse logo by a thin line" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> when iAmLookingAtTheConcourseLogo
                    >> then_ iSeeADividingLineToTheLeft
            , test "is clickable" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> when iAmLookingAtTheHamburgerIcon
                    >> then_ itIsClickable
            , DashboardTests.defineHoverBehaviour
                { name = "hamburger icon"
                , setup = iAmViewingTheDashboardOnANonPhoneScreen ()
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
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheHamburgerIcon
                    >> then_ iSeeALighterBackground
            , test "background toggles back to dark" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheHamburgerIcon
                    >> then_ iSeeADarkerBackground
            ]
        , describe "sidebar layout"
            [ test "page below top bar consists of side bar and page content" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeTwoChildren
            , test "page below top bar has no bottom padding" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeNoBottomPadding
            , test "sidebar and page contents are side by side" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePageBelowTheTopBar
                    >> then_ iSeeItLaysOutHorizontally
            , test "page contents are on the right" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePageContents
                    >> then_ iSeeTheUsualDashboardContentsScrollingIndependently
            , test "sidebar is separated from top bar by a thin line" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeADividingLineAbove
            , test "sidebar has frame background" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeADarkerBackground
            , test "sidebar scrolls independently" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItScrollsIndependently
            , test "sidebar is never more than 38% of screen width (golden ratio)" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItIsNeverMoreThan38PercentOfScreenWidth
            ]
        , describe "teams list"
            [ test "sidebar contains a pipeline group" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeOneChild
            , test "pipeline group lays out horizontally" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePipelineGroup
                    >> then_ iSeeItLaysOutHorizontally
            , test "pipeline group contains an icon group and team name" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePipelineGroup
                    >> then_ iSeeTwoChildren
            , test "icon group is the same width as the hamburger icon" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeItIsAsWideAsTheHamburgerIcon
            , test "icon group lays out horizontally" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeItLaysOutHorizontally
            , test "icon group spreads and centers contents" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeItSpreadsAndCentersContents
            , test "icon group has 5px padding" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeItHas5PxPadding
            , test "icon group contains a team icon and a chevron" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheIconGroup
                    >> then_ iSeeTwoChildren
            , test "team icon is a picture of two people" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheTeamIcon
                    >> then_ iSeeAPictureOfTwoPeople
            , test "chevron is pointing right" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheChevron
                    >> then_ iSeeARightPointingChevronArrow
            , test "team name has text content of team's name" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeTheTeamName
            , test "team name has large font" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeLargeFont
            , test "team name has 5px padding" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeItHas5PxPadding
            , test "team name will ellipsize if it is too long" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheTeamName
                    >> then_ iSeeItEllipsizesLongText
            ]
        ]


given : a -> a
given =
    identity


when : a -> a
when =
    identity


then_ : a -> a
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
            { size = "54px"
            , image = "baseline-menu-24px.svg"
            }
        )


iSeeItLaysOutHorizontally =
    Query.has [ style "display" "flex" ]


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


iAmViewingTheDashboard _ =
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
        |> Tuple.first
        |> Application.handleCallback
            (Callback.APIDataFetched
                (Ok
                    ( Time.millisToPosix 0
                    , { teams = [ { name = "team", id = 0 } ]
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
            )
        |> Tuple.first


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


iSeeNoHamburgerIcon =
    Query.hasNot
        (DashboardTests.iconSelector
            { size = "54px"
            , image = "baseline-menu-24px.svg"
            }
        )


iAmLookingAtTheHamburgerIcon =
    iAmLookingAtTheLeftHandSectionOfTheTopBar >> iAmLookingAtTheFirstChild


iSeeADividingLineToTheLeft =
    Query.has [ style "border-left" <| "1px solid " ++ Colors.background ]


itIsClickable =
    Expect.all
        [ Query.has [ style "cursor" "pointer" ]
        , Event.simulate Event.click
            >> Event.expect
                (TopLevelMessage.Update <|
                    Message.Click Message.HamburgerMenu
                )
        ]


iAmLookingAtTheConcourseLogo =
    iAmLookingAtTheLeftHandSectionOfTheTopBar
        >> Query.children []
        >> Query.index 1


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
            , style "padding-bottom" "50px"
            , style "height" "100%"
            , style "width" "100%"
            , style "overflow-y" "auto"
            ]
        , Query.has [ text "pipeline" ]
        ]


iAmLookingAtTheSideBar =
    iAmLookingAtThePageBelowTheTopBar
        >> Query.children []
        >> Query.first


iSeeADividingLineAbove =
    Query.has [ style "border-top" <| "1px solid " ++ Colors.background ]


iSeeItIsNeverMoreThan38PercentOfScreenWidth =
    Query.has [ style "max-width" "38%" ]


iSeeOneChild =
    Query.children [] >> Query.count (Expect.equal 1)


iAmLookingAtThePipelineGroup =
    iAmLookingAtTheSideBar >> Query.children [] >> Query.first


iAmLookingAtTheIconGroup =
    iAmLookingAtThePipelineGroup >> Query.children [] >> Query.first


iSeeItIsAsWideAsTheHamburgerIcon =
    Query.has
        [ style "width" "54px"
        , style "box-sizing" "border-box"
        , style "flex-shrink" "0"
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


iAmLookingAtTheChevron =
    iAmLookingAtTheIconGroup >> Query.children [] >> Query.index 1


iSeeARightPointingChevronArrow =
    Query.has
        (DashboardTests.iconSelector
            { size = "20px"
            , image = "baseline-chevron-right-24px.svg"
            }
        )


iSeeNoBottomPadding =
    Query.has [ style "padding-bottom" "0" ]


iAmLookingAtTheTeamName =
    iAmLookingAtThePipelineGroup >> Query.children [] >> Query.index 1


iSeeTheTeamName =
    Query.has [ text "team" ]


iSeeItSpreadsAndCentersContents =
    Query.has
        [ style "align-items" "center"
        , style "justify-content" "space-between"
        ]


iSeeItHas5PxPadding =
    Query.has [ style "padding" "5px" ]


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
