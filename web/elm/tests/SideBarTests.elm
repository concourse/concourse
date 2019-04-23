module SideBarTests exposing (all)

import Application.Application as Application
import Colors
import Common
import DashboardTests
import Expect
import Html.Attributes as Attr
import Message.Callback as Callback
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
                    >> then_ (itIsClickable Message.HamburgerMenu)
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
            , test "sidebar fills height" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItFillsHeight
            , test "sidebar does not shrink" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItDoesNotShrink
            , test "sidebar has right padding" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeItHasRightPadding
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
            [ test "sidebar contains two pipeline groups" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtTheSideBar
                    >> then_ iSeeTwoChildren
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
                    >> when iAmLookingAtTheArrow
                    >> then_ iSeeARightPointingArrow
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
            , test "pipeline group is clickable" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> when iAmLookingAtThePipelineGroup
                    >> then_ (itIsClickable <| Message.SideBarTeam "one-team")
            , DashboardTests.defineHoverBehaviour
                { name = "pipeline group"
                , setup =
                    iAmViewingTheDashboardOnANonPhoneScreen ()
                        |> iClickedTheHamburgerIcon
                , query = iAmLookingAtThePipelineGroup
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
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheExpandedArrow
                    >> then_ iSeeABrightDownPointingArrow
            , test "chevron still points down after data refreshes" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> given dataRefreshes
                    >> when iAmLookingAtTheExpandedArrow
                    >> then_ iSeeABrightDownPointingArrow
            , test "team name is bright when group is clicked" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheExpandedTeamName
                    >> then_ iSeeItIsBright
            , test "pipeline list expands when group is clicked" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtThePipelineGroup
                    >> then_ iSeeItLaysOutVertically
            , test "pipeline list has two children" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtThePipelineList
                    >> then_ iSeeTwoChildren
            , test "pipeline list lays out vertically" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtThePipelineList
                    >> then_ iSeeItLaysOutVertically
            , test "first pipeline link contains text of pipeline name" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineLink
                    >> then_ iSeeItContainsThePipelineName
            , test "pipeline link aligns with team name" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineLink
                    >> then_ iSeeItAlignsWithTheTeamName
            , test "pipeline link is a link to the pipeline" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineLink
                    >> then_ iSeeItIsALinkToTheFirstPipeline
            , test "pipeline link has large font" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
                    >> given iClickedThePipelineGroup
                    >> when iAmLookingAtTheFirstPipelineLink
                    >> then_ iSeeLargeFont
            , test "pipeline link will ellipsize if it is too long" <|
                given iAmViewingTheDashboardOnANonPhoneScreen
                    >> given iClickedTheHamburgerIcon
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
            given iAmViewingTheDashboardOnANonPhoneScreen
                >> given iClickedTheHamburgerIcon
                >> given iToggledToHighDensity
                >> when iAmLookingAtThePageBelowTheTopBar
                >> then_ iSeeTwoChildren
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
        |> dataRefreshes


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
            { size = hamburgerIconWidth
            , image = "baseline-menu-24px.svg"
            }
        )


iAmLookingAtTheHamburgerIcon =
    iAmLookingAtTheLeftHandSectionOfTheTopBar >> iAmLookingAtTheFirstChild


iSeeADividingLineToTheLeft =
    Query.has [ style "border-left" <| "1px solid " ++ Colors.background ]


itIsClickable domID =
    Expect.all
        [ Query.has [ style "cursor" "pointer" ]
        , Event.simulate Event.click
            >> Event.expect
                (TopLevelMessage.Update <|
                    Message.Click domID
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


iSeeItIsNeverMoreThan38PercentOfScreenWidth =
    Query.has [ style "max-width" "38%" ]


iAmLookingAtThePipelineGroup =
    iAmLookingAtTheSideBar
        >> Query.children [ containing [ text "one-team" ] ]
        >> Query.first


iAmLookingAtTheIconGroup =
    iAmLookingAtThePipelineGroup >> Query.children [] >> Query.first


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


iAmLookingAtTheTeamName =
    iAmLookingAtThePipelineGroup >> Query.children [] >> Query.index 1


iSeeTheTeamName =
    Query.has [ text "one-team" ]


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
    iAmLookingAtThePipelineGroup >> Query.children [] >> Query.index 1


iAmLookingAtTheFirstPipelineLink =
    iAmLookingAtThePipelineList >> Query.children [] >> Query.first


iSeeItContainsThePipelineName =
    Query.has [ text "pipeline" ]


iAmLookingAtTheExpandedArrow =
    iAmLookingAtTheExpandedIconGroup >> Query.children [] >> Query.index 1


iAmLookingAtTheExpandedIconGroup =
    iAmLookingAtThePipelineGroupHeader >> Query.children [] >> Query.first


iAmLookingAtThePipelineGroupHeader =
    iAmLookingAtThePipelineGroup >> Query.children [] >> Query.first


iAmLookingAtTheExpandedTeamName =
    iAmLookingAtThePipelineGroupHeader >> Query.children [] >> Query.index 1


iSeeItAlignsWithTheTeamName =
    Query.has [ style "margin-left" hamburgerIconWidth, style "padding" "5px" ]


iSeeItIsALinkToTheFirstPipeline =
    Query.has [ tag "a", attribute <| Attr.href "/teams/one-team/pipelines/pipeline" ]


iToggledToHighDensity =
    Application.update
        (TopLevelMessage.DeliveryReceived <|
            Subscription.RouteChanged <|
                Routes.Dashboard Routes.HighDensity
        )
        >> Tuple.first
