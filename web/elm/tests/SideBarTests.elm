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
import Url


all : Test
all =
    describe "dashboard sidebar"
        [ test "left hand section lays out horizontally" <|
            given iAmViewingTheDashboardOnANonPhoneScreen
                >> when iAmLookingAtTheLeftHandSectionOfTheTopBar
                >> then_ iSeeItLaysOutHorizontally
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


iAmLookingAtTheLeftHandSectionOfTheTopBar =
    Common.queryView
        >> Query.find [ id "top-bar-app" ]
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
