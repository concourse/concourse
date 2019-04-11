module ResourceTests exposing (all)

import Application.Application as Application
import Common exposing (queryView)
import Concourse
import Concourse.Pagination exposing (Direction(..))
import DashboardTests
    exposing
        ( almostBlack
        , darkGrey
        , defineHoverBehaviour
        , iconSelector
        , middleGrey
        )
import Dict
import Expect exposing (..)
import Html.Attributes as Attr
import Http
import Keyboard
import Message.Callback as Callback exposing (Callback(..))
import Message.Effects as Effects
import Message.Message
import Message.Subscription as Subscription exposing (Delivery(..), Interval(..))
import Message.TopLevelMessage as Msgs
import Resource.Models as Models
import Routes
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector
    exposing
        ( Selector
        , attribute
        , class
        , containing
        , id
        , style
        , tag
        , text
        )
import Time
import Url
import UserState exposing (UserState(..))


commentButtonBlue : String
commentButtonBlue =
    "#196ac8"


teamName : String
teamName =
    "some-team"


pipelineName : String
pipelineName =
    "some-pipeline"


resourceName : String
resourceName =
    "some-resource"


resourceIcon : String
resourceIcon =
    "some-icon"


versionID : Models.VersionId
versionID =
    { teamName = teamName
    , pipelineName = pipelineName
    , resourceName = resourceName
    , versionID = 1
    }


otherVersionID : Models.VersionId
otherVersionID =
    { teamName = teamName
    , pipelineName = pipelineName
    , resourceName = resourceName
    , versionID = 2
    }


disabledVersionID : Models.VersionId
disabledVersionID =
    { teamName = teamName
    , pipelineName = pipelineName
    , resourceName = resourceName
    , versionID = 3
    }


version : String
version =
    "v1"


otherVersion : String
otherVersion =
    "v2"


disabledVersion : String
disabledVersion =
    "v3"


purpleHex : String
purpleHex =
    "#5c3bd1"


fadedBlackHex : String
fadedBlackHex =
    "#1e1d1d80"


almostWhiteHex : String
almostWhiteHex =
    "#e6e7e8"


lightGreyHex : String
lightGreyHex =
    "#3d3c3c"


tooltipGreyHex : String
tooltipGreyHex =
    "#9b9b9b"


darkGreyHex : String
darkGreyHex =
    "#1e1d1d"


badResponse : Result Http.Error ()
badResponse =
    Err <|
        Http.BadStatus
            { url = ""
            , status =
                { code = 500
                , message = "server error"
                }
            , headers = Dict.empty
            , body = ""
            }


all : Test
all =
    describe "resource page"
        [ describe "when logging out" <|
            let
                loggingOut : () -> ( Application.Model, List Effects.Effect )
                loggingOut _ =
                    init
                        |> Application.handleCallback
                            (Callback.UserFetched <|
                                Ok
                                    { id = "test"
                                    , userName = "test"
                                    , name = "test"
                                    , email = "test"
                                    , teams =
                                        Dict.fromList
                                            [ ( teamName, [ "member" ] )
                                            ]
                                    }
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.LoggedOut (Ok ()))
            in
            [ test "updates top bar state" <|
                loggingOut
                    >> Tuple.first
                    >> queryView
                    >> Query.find [ id "top-bar-app" ]
                    >> Query.children []
                    >> Query.index -1
                    >> Query.has [ text "login" ]
            , test "redirects to dashboard" <|
                loggingOut
                    >> Tuple.second
                    >> Expect.equal
                        [ Effects.NavigateTo <|
                            Routes.toString <|
                                Routes.Dashboard <|
                                    Routes.Normal Nothing
                        ]
            ]
        , test "has title with resouce name" <|
            \_ ->
                init
                    |> Application.view
                    |> .title
                    |> Expect.equal "some-resource - Concourse"
        , test "fetches time zone on page load" <|
            \_ ->
                Application.init
                    { turbulenceImgSrc = ""
                    , notFoundImgSrc = "notfound.svg"
                    , csrfToken = "csrf_token"
                    , authToken = ""
                    , instanceName = ""
                    , pipelineRunningKeyframes = "pipeline-running"
                    }
                    { protocol = Url.Http
                    , host = ""
                    , port_ = Nothing
                    , path =
                        "/teams/"
                            ++ teamName
                            ++ "/pipelines/"
                            ++ pipelineName
                            ++ "/resources/"
                            ++ resourceName
                    , query = Nothing
                    , fragment = Nothing
                    }
                    |> Tuple.second
                    |> List.member Effects.GetCurrentTimeZone
                    |> Expect.true "should get timezone"
        , test "subscribes to the five second interval" <|
            \_ ->
                init
                    |> Application.subscriptions
                    |> List.member (Subscription.OnClockTick FiveSeconds)
                    |> Expect.true "not subscribed to the five second interval?"
        , test "autorefreshes resource and versions every 5 seconds" <|
            \_ ->
                init
                    |> Application.update
                        (Msgs.DeliveryReceived
                            (ClockTicked FiveSeconds <|
                                Time.millisToPosix 0
                            )
                        )
                    |> Tuple.second
                    |> Expect.equal
                        [ Effects.FetchResource
                            { resourceName = resourceName
                            , pipelineName = pipelineName
                            , teamName = teamName
                            }
                        , Effects.FetchVersionedResources
                            { resourceName = resourceName
                            , pipelineName = pipelineName
                            , teamName = teamName
                            }
                            Nothing
                        ]
        , test "autorefresh respects expanded state" <|
            \_ ->
                init
                    |> givenResourceIsNotPinned
                    |> givenVersionsWithoutPagination
                    |> update
                        (Message.Message.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> givenVersionsWithoutPagination
                    |> queryView
                    |> Query.find (versionSelector version)
                    |> Query.has [ text "metadata" ]
        , test "autorefresh respects 'Inputs To'" <|
            \_ ->
                init
                    |> givenResourceIsNotPinned
                    |> givenVersionsWithoutPagination
                    |> update
                        (Message.Message.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.InputToFetched
                            (Ok
                                ( versionID
                                , [ { id = 0
                                    , name = "some-build"
                                    , job =
                                        Just
                                            { teamName = teamName
                                            , pipelineName = pipelineName
                                            , jobName = "some-job"
                                            }
                                    , status = Concourse.BuildStatusSucceeded
                                    , duration =
                                        { startedAt = Nothing
                                        , finishedAt = Nothing
                                        }
                                    , reapTime = Nothing
                                    }
                                  ]
                                )
                            )
                        )
                    |> Tuple.first
                    |> givenVersionsWithoutPagination
                    |> queryView
                    |> Query.find (versionSelector version)
                    |> Query.has [ text "some-build" ]
        , test "autorefresh respects 'Outputs Of'" <|
            \_ ->
                init
                    |> givenResourceIsNotPinned
                    |> givenVersionsWithoutPagination
                    |> update
                        (Message.Message.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> Application.handleCallback
                        (Callback.OutputOfFetched
                            (Ok
                                ( versionID
                                , [ { id = 0
                                    , name = "some-build"
                                    , job =
                                        Just
                                            { teamName = teamName
                                            , pipelineName = pipelineName
                                            , jobName = "some-job"
                                            }
                                    , status = Concourse.BuildStatusSucceeded
                                    , duration =
                                        { startedAt = Nothing
                                        , finishedAt = Nothing
                                        }
                                    , reapTime = Nothing
                                    }
                                  ]
                                )
                            )
                        )
                    |> Tuple.first
                    |> givenVersionsWithoutPagination
                    |> queryView
                    |> Query.find (versionSelector version)
                    |> Query.has [ text "some-build" ]
        , describe "page header with icon" <|
            let
                pageHeader =
                    init
                        |> givenResourceHasIcon
                        |> queryView
                        |> Query.find [ id "page-header" ]
            in
            [ describe "resource name"
                [ test "on the left is the resource name" <|
                    \_ ->
                        pageHeader
                            |> Query.children []
                            |> Query.index 0
                            |> Query.has [ tag "svg", text resourceName, tag "h1" ]
                ]
            , describe "resource icon"
                [ test "on the left is the resource icon" <|
                    \_ ->
                        pageHeader
                            |> Query.children []
                            |> Query.index 0
                            |> Query.has [ tag "svg" ]
                ]
            ]
        , describe "page header" <|
            let
                pageHeader =
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "page-header" ]
            in
            [ test "sticks to the top of the viewport" <|
                \_ ->
                    pageHeader
                        |> Query.has
                            [ style "position" "fixed"
                            , style "top" "54px"
                            , style "z-index" "1"
                            ]
            , test "fills the top of the screen with dark grey background" <|
                \_ ->
                    pageHeader
                        |> Query.has
                            [ style "height" "60px"
                            , style "width" "100%"
                            , style "background-color" "#2a2929"
                            ]
            , test "lays out contents horizontally, stretching them vertically" <|
                \_ ->
                    pageHeader
                        |> Query.has
                            [ style "display" "flex"
                            , style "align-items" "stretch"
                            ]
            , describe "resource name"
                [ test "on the left is the resource name" <|
                    \_ ->
                        pageHeader
                            |> Query.children []
                            |> Query.index 0
                            |> Query.has [ text resourceName, tag "h1" ]
                , test "the text is large and vertically centred" <|
                    \_ ->
                        pageHeader
                            |> Query.children []
                            |> Query.index 0
                            |> Query.has
                                [ style "font-weight" "700"
                                , style "margin-left" "18px"
                                , style "display" "flex"
                                , style "align-items" "center"
                                , style "justify-content" "center"
                                ]
                ]
            , describe "last checked"
                [ test "last checked view is second from left" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> Application.update
                                (Msgs.DeliveryReceived <|
                                    Subscription.ClockTicked Subscription.OneSecond <|
                                        Time.millisToPosix 1000
                                )
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "page-header" ]
                            |> Query.children []
                            |> Query.index 1
                            |> Query.has [ text "1s ago" ]
                , test "last checked view displays its contents centred" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> Application.update
                                (Msgs.DeliveryReceived <|
                                    Subscription.ClockTicked Subscription.OneSecond <|
                                        Time.millisToPosix 1000
                                )
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "page-header" ]
                            |> Query.children []
                            |> Query.index 1
                            |> Query.has
                                [ style "display" "flex"
                                , style "align-items" "center"
                                , style "justify-content" "center"
                                , style "margin-left" "24px"
                                ]
                ]
            , describe "pagination"
                [ test "pagination is last on the right" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenVersionsWithPagination
                            |> queryView
                            |> Query.find [ id "page-header" ]
                            |> Query.children []
                            |> Query.index -1
                            |> Query.has [ id "pagination" ]
                , test "pagination displays the pages horizontally" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenVersionsWithPagination
                            |> queryView
                            |> Query.find [ id "pagination" ]
                            |> Query.has
                                [ style "display" "flex"
                                , style "align-items" "stretch"
                                ]
                , describe "pagination chevrons"
                    [ test "with no pages" <|
                        \_ ->
                            init
                                |> givenResourceIsNotPinned
                                |> givenVersionsWithoutPagination
                                |> queryView
                                |> Query.find [ id "pagination" ]
                                |> Query.children []
                                |> Expect.all
                                    [ Query.index 0
                                        >> Query.has
                                            [ style "padding" "5px"
                                            , style "display" "flex"
                                            , style "align-items" "center"
                                            , style "border-left" <|
                                                "1px solid "
                                                    ++ middleGrey
                                            , containing
                                                (iconSelector
                                                    { image =
                                                        "baseline-chevron-left-24px.svg"
                                                    , size = "24px"
                                                    }
                                                    ++ [ style "padding" "5px"
                                                       , style "opacity" "0.5"
                                                       ]
                                                )
                                            ]
                                    , Query.index 1
                                        >> Query.has
                                            [ style "padding" "5px"
                                            , style "display" "flex"
                                            , style "align-items" "center"
                                            , style "border-left" <|
                                                "1px solid "
                                                    ++ middleGrey
                                            , containing
                                                (iconSelector
                                                    { image =
                                                        "baseline-chevron-right-24px.svg"
                                                    , size = "24px"
                                                    }
                                                    ++ [ style "padding" "5px"
                                                       , style "opacity" "0.5"
                                                       ]
                                                )
                                            ]
                                    ]
                    , defineHoverBehaviour <|
                        let
                            urlPath =
                                "/teams/some-team/pipelines/some-pipeline/resources/some-resource?since=1&limit=1"
                        in
                        { name = "left pagination chevron with previous page"
                        , setup =
                            init
                                |> givenResourceIsNotPinned
                                |> givenVersionsWithPagination
                        , query =
                            queryView
                                >> Query.find [ id "pagination" ]
                                >> Query.children []
                                >> Query.index 0
                        , updateFunc = \msg -> Application.update msg >> Tuple.first
                        , unhoveredSelector =
                            { description = "white left chevron"
                            , selector =
                                [ style "padding" "5px"
                                , style "display" "flex"
                                , style "align-items" "center"
                                , style "border-left" <|
                                    "1px solid "
                                        ++ middleGrey
                                , containing
                                    (iconSelector
                                        { image =
                                            "baseline-chevron-left-24px.svg"
                                        , size = "24px"
                                        }
                                        ++ [ style "padding" "5px"
                                           , style "opacity" "1"
                                           , attribute <| Attr.href urlPath
                                           ]
                                    )
                                ]
                            }
                        , hoveredSelector =
                            { description =
                                "left chevron with light grey circular bg"
                            , selector =
                                [ style "padding" "5px"
                                , style "display" "flex"
                                , style "align-items" "center"
                                , style "border-left" <|
                                    "1px solid "
                                        ++ middleGrey
                                , containing
                                    (iconSelector
                                        { image =
                                            "baseline-chevron-left-24px.svg"
                                        , size = "24px"
                                        }
                                        ++ [ style "padding" "5px"
                                           , style "opacity" "1"
                                           , style "border-radius" "50%"
                                           , style "background-color" <|
                                                "#504b4b"
                                           , attribute <| Attr.href urlPath
                                           ]
                                    )
                                ]
                            }
                        , mouseEnterMsg =
                            Msgs.Update <|
                                Message.Message.Hover <|
                                    Just Message.Message.PreviousPageButton
                        , mouseLeaveMsg =
                            Msgs.Update <|
                                Message.Message.Hover Nothing
                        }
                    ]
                ]
            ]
        , describe "page body" <|
            [ test "has horizontal padding of 10px" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "body" ]
                        |> Query.has
                            [ style "padding-left" "10px"
                            , style "padding-right" "10px"
                            ]
            ]
        , describe "checkboxes" <|
            let
                checkIcon =
                    "url(/public/images/checkmark-ic.svg)"
            in
            [ test "there is a checkbox for every version" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find [ class "resource-versions" ]
                        |> Query.findAll anyVersionSelector
                        |> Query.each hasCheckbox
            , test "there is a pointer cursor for every checkbox" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find [ class "resource-versions" ]
                        |> Query.findAll anyVersionSelector
                        |> Query.each
                            (Query.find checkboxSelector
                                >> Query.has pointerCursor
                            )
            , test "enabled versions have checkmarks" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Expect.all
                            [ Query.find (versionSelector version)
                                >> Query.find checkboxSelector
                                >> Query.has
                                    [ style "background-image" checkIcon ]
                            , Query.find (versionSelector otherVersion)
                                >> Query.find checkboxSelector
                                >> Query.has
                                    [ style "background-image" checkIcon ]
                            ]
            , test "disabled versions do not have checkmarks" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> Query.hasNot [ style "background-image" checkIcon ]
            , test "clicking the checkbox on an enabled version triggers a ToggleVersion msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> Event.simulate Event.click
                        |> Event.expect
                            (Msgs.Update <|
                                Message.Message.ToggleVersion
                                    Message.Message.Disable
                                    versionID
                            )
            , test "receiving a (ToggleVersion Disable) msg causes the relevant checkbox to go into a transition state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> clickToDisable versionID
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> checkboxHasTransitionState
            , test "autorefreshing after receiving a ToggleVersion msg causes the relevant checkbox to stay in a transition state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> clickToDisable versionID
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> checkboxHasTransitionState
            , test "receiving a successful VersionToggled msg causes the relevant checkbox to appear unchecked" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> clickToDisable versionID
                        |> Application.handleCallback (Callback.VersionToggled Message.Message.Disable versionID (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> versionHasDisabledState
            , test "receiving an error on VersionToggled msg causes the checkbox to go back to its checked state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> clickToDisable versionID
                        |> Application.handleCallback (Callback.VersionToggled Message.Message.Disable versionID badResponse)
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> checkboxHasEnabledState
            , test "clicking the checkbox on a disabled version triggers a ToggleVersion msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> Event.simulate Event.click
                        |> Event.expect
                            (Msgs.Update <|
                                Message.Message.ToggleVersion
                                    Message.Message.Enable
                                    disabledVersionID
                            )
            , test "receiving a (ToggleVersion Enable) msg causes the relevant checkbox to go into a transition state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> Application.update
                            (Msgs.Update <|
                                Message.Message.ToggleVersion
                                    Message.Message.Enable
                                    disabledVersionID
                            )
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> checkboxHasTransitionState
            , test "receiving a successful VersionToggled msg causes the relevant checkbox to appear checked" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> Application.update
                            (Msgs.Update <|
                                Message.Message.ToggleVersion
                                    Message.Message.Enable
                                    disabledVersionID
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.VersionToggled
                                Message.Message.Enable
                                disabledVersionID
                                (Ok ())
                            )
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> checkboxHasEnabledState
            , test "receiving a failing VersionToggled msg causes the relevant checkbox to return to its unchecked state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> Application.update
                            (Msgs.Update <|
                                Message.Message.ToggleVersion
                                    Message.Message.Enable
                                    disabledVersionID
                            )
                        |> Tuple.first
                        |> Application.handleCallback
                            (Callback.VersionToggled
                                Message.Message.Enable
                                disabledVersionID
                                badResponse
                            )
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> checkboxHasDisabledState
            ]
        , describe "given resource is pinned statically"
            [ describe "pin bar"
                [ test "then pinned version is visible in pin bar" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> queryView
                            |> Query.find [ id "pin-bar" ]
                            |> Query.has [ text version ]
                , test "then pin bar has purple border" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> queryView
                            |> Query.find [ id "pin-bar" ]
                            |> Query.has purpleOutlineSelector
                , test "pin icon on pin bar has default cursor" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> queryView
                            |> Query.find [ id "pin-icon" ]
                            |> Query.has defaultCursor
                , test "clicking pin icon on pin bar does nothing" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> queryView
                            |> Query.find [ id "pin-icon" ]
                            |> Event.simulate Event.click
                            |> Event.toResult
                            |> Expect.err
                , test "there is a bit of space betwen the pin icon and the version in the pin bar" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> queryView
                            |> Query.find [ id "pin-icon" ]
                            |> Query.has
                                [ style "margin-right" "10px" ]
                , test "mousing over pin icon does nothing" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> queryView
                            |> Query.find [ id "pin-icon" ]
                            |> Event.simulate Event.mouseEnter
                            |> Event.toResult
                            |> Expect.err
                , test "pin button on pinned version has a purple outline" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> Query.has purpleOutlineSelector
                , test "checkbox on pinned version has a purple outline" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find checkboxSelector
                            |> Query.has purpleOutlineSelector
                , test "all pin buttons have default cursor" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find [ class "resource-versions" ]
                            |> Query.findAll anyVersionSelector
                            |> Query.each
                                (Query.find pinButtonSelector
                                    >> Query.has defaultCursor
                                )
                , test "version header on pinned version has a purple outline" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> findLast [ tag "div", containing [ text version ] ]
                            |> Query.has purpleOutlineSelector
                , test "mousing over pin bar sends TogglePinBarTooltip message" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> queryView
                            |> Query.find [ id "pin-bar" ]
                            |> Event.simulate Event.mouseEnter
                            |> Event.expect
                                (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinBar)
                , test "TogglePinBarTooltip causes tooltip to appear" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> hoverOverPinBar
                            |> queryView
                            |> Query.has pinBarTooltipSelector
                , test "pin bar tooltip has text 'pinned in pipeline config'" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> hoverOverPinBar
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has [ text "pinned in pipeline config" ]
                , test "pin bar tooltip is positioned above and near the left of the pin bar" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> hoverOverPinBar
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has
                                [ style "position" "absolute"
                                , style "top" "-10px"
                                , style "left" "30px"
                                ]
                , test "pin bar tooltip is light grey" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> hoverOverPinBar
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has
                                [ style "background-color" tooltipGreyHex ]
                , test "pin bar tooltip has a bit of padding around text" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> hoverOverPinBar
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has
                                [ style "padding" "5px" ]
                , test "pin bar tooltip appears above other elements in the DOM" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> hoverOverPinBar
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has
                                [ style "z-index" "2" ]
                , test "mousing out of pin bar sends Hover Nothing message" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> hoverOverPinBar
                            |> queryView
                            |> Query.find [ id "pin-bar" ]
                            |> Event.simulate Event.mouseLeave
                            |> Event.expect
                                (Msgs.Update <| Message.Message.Hover Nothing)
                , test "when mousing off pin bar, tooltip disappears" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> hoverOverPinBar
                            |> update (Message.Message.Hover Nothing)
                            |> Tuple.first
                            |> queryView
                            |> Query.hasNot pinBarTooltipSelector
                ]
            , describe "per-version pin buttons"
                [ test "unpinned versions are lower opacity" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find (versionSelector otherVersion)
                            |> Query.has [ style "opacity" "0.5" ]
                , test "mousing over the pinned version's pin button sends ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> Event.simulate Event.mouseOver
                            |> Event.expect
                                (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinButton)
                , test "mousing over an unpinned version's pin button doesn't send any msg" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find (versionSelector otherVersion)
                            |> Query.find pinButtonSelector
                            |> Event.simulate Event.mouseOver
                            |> Event.toResult
                            |> Expect.err
                , test "shows tooltip on the pinned version's pin button on ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> hoverOverPinButton
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.has versionTooltipSelector
                , test "keeps tooltip on the pinned version's pin button on autorefresh" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> hoverOverPinButton
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.has versionTooltipSelector
                , test "mousing off the pinned version's pin button sends Hover Nothing" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> hoverOverPinButton
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> Event.simulate Event.mouseOut
                            |> Event.expect
                                (Msgs.Update <| Message.Message.Hover Nothing)
                , test "mousing off an unpinned version's pin button doesn't send any msg" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> hoverOverPinButton
                            |> queryView
                            |> Query.find (versionSelector otherVersion)
                            |> Query.find pinButtonSelector
                            |> Event.simulate Event.mouseOut
                            |> Event.toResult
                            |> Expect.err
                , test "hides tooltip on the pinned version's pin button on ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> hoverOverPinButton
                            |> update (Message.Message.Hover Nothing)
                            |> Tuple.first
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.hasNot versionTooltipSelector
                , test "clicking on pin button on pinned version doesn't send any msg" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> clickToUnpin
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> Event.simulate Event.click
                            |> Event.toResult
                            |> Expect.err
                , test "all pin buttons have dark background" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find [ class "resource-versions" ]
                            |> Query.findAll anyVersionSelector
                            |> Query.each
                                (Query.find pinButtonSelector
                                    >> Query.has [ style "background-color" "#1e1d1d" ]
                                )
                ]
            ]
        , describe "given resource is pinned dynamically"
            [ test "when mousing over pin bar, tooltip does not appear" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Event.simulate Event.mouseEnter
                        |> Event.toResult
                        |> Expect.err
            , test "pin icon on pin bar has pointer cursor" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has pointerCursor
            , test "clicking pin icon on bar triggers UnpinVersion msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Event.simulate Event.click
                        |> Event.expect
                            (Msgs.Update Message.Message.UnpinVersion)
            , test "mousing over pin icon triggers PinIconHover msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Event.simulate Event.mouseEnter
                        |> Event.expect
                            (Msgs.Update <| Message.Message.Hover <| Just Message.Message.PinIcon)
            , test "TogglePinIconHover msg causes pin icon to have dark background" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> update (Message.Message.Hover <| Just Message.Message.PinIcon)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has [ style "background-color" darkGreyHex ]
            , test "mousing off pin icon triggers Hover Nothing msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> update (Message.Message.Hover <| Just Message.Message.PinIcon)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Event.simulate Event.mouseLeave
                        |> Event.expect
                            (Msgs.Update <| Message.Message.Hover <| Nothing)
            , test "second TogglePinIconHover msg causes pin icon to have transparent background color" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> update (Message.Message.Hover <| Just Message.Message.PinIcon)
                        |> Tuple.first
                        |> update (Message.Message.Hover Nothing)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has [ style "background-color" "transparent" ]
            , test "pin button on pinned version has a purple outline" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Query.has purpleOutlineSelector
            , test "checkbox on pinned version has a purple outline" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> Query.has purpleOutlineSelector
            , test "pin button on pinned version has a pointer cursor" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Query.has pointerCursor
            , test "pin button on an unpinned version has a default cursor" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector otherVersion)
                        |> Query.find pinButtonSelector
                        |> Query.has defaultCursor
            , test "clicking on pin button on pinned version will trigger UnpinVersion msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Event.simulate Event.click
                        |> Event.expect (Msgs.Update Message.Message.UnpinVersion)
            , test "pin button on pinned version shows transition state when (UnpinVersion) is received" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> clickToUnpin
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> pinButtonHasTransitionState
            , test "pin button on 'v1' still shows transition state on autorefresh before VersionUnpinned is recieved" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> clickToUnpin
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> pinButtonHasTransitionState
            , test "pin bar shows unpinned state when upon successful VersionUnpinned msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> clickToUnpin
                        |> Application.handleCallback (Callback.VersionUnpinned (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "resource refreshes on successful VersionUnpinned msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> clickToUnpin
                        |> Application.handleCallback (Callback.VersionUnpinned (Ok ()))
                        |> Tuple.second
                        |> Expect.equal
                            [ Effects.FetchResource
                                { resourceName = resourceName
                                , pipelineName = pipelineName
                                , teamName = teamName
                                }
                            ]
            , test "pin bar shows unpinned state upon receiving failing (VersionUnpinned) msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> clickToUnpin
                        |> Application.handleCallback (Callback.VersionUnpinned badResponse)
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasPinnedState version
            , test "version header on pinned version has a purple outline" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> findLast [ tag "div", containing [ text version ] ]
                        |> Query.has purpleOutlineSelector
            , test "pin button on pinned version has a white icon" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Query.has [ style "background-image" "url(/public/images/pin-ic-white.svg)" ]
            , test "does not show tooltip on the pin button on ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> hoverOverPinButton
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.hasNot versionTooltipSelector
            , test "unpinned versions are lower opacity" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector otherVersion)
                        |> Query.has [ style "opacity" "0.5" ]
            , test "pin icon on pin bar is white" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has [ style "background-image" "url(/public/images/pin-ic-white.svg)" ]
            , test "all pin buttons have dark background" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find [ class "resource-versions" ]
                        |> Query.findAll anyVersionSelector
                        |> Query.each
                            (Query.find pinButtonSelector
                                >> Query.has [ style "background-color" "#1e1d1d" ]
                            )
            , test "pin comment bar is visible" <|
                \_ ->
                    init
                        |> givenResourcePinnedWithComment
                        |> queryView
                        |> Query.has [ id "comment-bar" ]
            , test "body has padding to accomodate pin comment bar" <|
                \_ ->
                    init
                        |> givenResourcePinnedWithComment
                        |> queryView
                        |> Query.find [ id "body" ]
                        |> Query.has
                            [ style "padding-bottom" "300px" ]
            , describe "pin comment bar" <|
                let
                    commentBar : Application.Model -> Query.Single Msgs.TopLevelMessage
                    commentBar =
                        queryView
                            >> Query.find [ id "comment-bar" ]
                in
                [ test "pin comment bar has dark background" <|
                    \_ ->
                        init
                            |> givenResourcePinnedWithComment
                            |> commentBar
                            |> Query.has
                                [ style "background-color" almostBlack ]
                , test "pin comment bar is fixed to viewport bottom" <|
                    \_ ->
                        init
                            |> givenResourcePinnedWithComment
                            |> commentBar
                            |> Query.has
                                [ style "position" "fixed"
                                , style "bottom" "0"
                                ]
                , test "pin comment bar is as wide as the viewport" <|
                    \_ ->
                        init
                            |> givenResourcePinnedWithComment
                            |> commentBar
                            |> Query.has [ style "width" "100%" ]
                , test "pin comment bar is 300px tall" <|
                    \_ ->
                        init
                            |> givenResourcePinnedWithComment
                            |> commentBar
                            |> Query.has [ style "height" "300px" ]
                , test "pin comment bar centers contents horizontally" <|
                    \_ ->
                        init
                            |> givenResourcePinnedWithComment
                            |> commentBar
                            |> Query.has
                                [ style "display" "flex"
                                , style "justify-content" "center"
                                ]
                , describe "contents" <|
                    let
                        contents : Application.Model -> Query.Single Msgs.TopLevelMessage
                        contents =
                            commentBar >> Query.children [] >> Query.first
                    in
                    [ test "is 700px wide" <|
                        \_ ->
                            init
                                |> givenResourcePinnedWithComment
                                |> contents
                                |> Query.has [ style "width" "700px" ]
                    , test "has vertical padding" <|
                        \_ ->
                            init
                                |> givenResourcePinnedWithComment
                                |> contents
                                |> Query.has
                                    [ style "padding" "20px 0" ]
                    , test "lays out vertically and left-aligned" <|
                        \_ ->
                            init
                                |> givenResourcePinnedWithComment
                                |> contents
                                |> Query.has
                                    [ style "display" "flex"
                                    , style "flex-direction" "column"
                                    ]
                    , describe "header" <|
                        let
                            header : Application.Model -> Query.Single Msgs.TopLevelMessage
                            header =
                                contents >> Query.children [] >> Query.first
                        in
                        [ test "lays out horizontally" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> header
                                    |> Query.has
                                        [ style "display" "flex" ]
                        , test "aligns contents to top" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> header
                                    |> Query.has
                                        [ style "align-items" "flex-start" ]
                        , test "doesn't squish vertically" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> header
                                    |> Query.has
                                        [ style "flex-shrink" "0" ]
                        , test "has two children" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> header
                                    |> Query.children []
                                    |> Query.count (Expect.equal 2)
                        , describe "icon container" <|
                            let
                                iconContainer =
                                    header
                                        >> Query.children []
                                        >> Query.first
                            in
                            [ test "lays out horizontally" <|
                                \_ ->
                                    init
                                        |> givenResourcePinnedWithComment
                                        |> iconContainer
                                        |> Query.has
                                            [ style "display" "flex" ]
                            , test "centers contents vertically" <|
                                \_ ->
                                    init
                                        |> givenResourcePinnedWithComment
                                        |> iconContainer
                                        |> Query.has
                                            [ style "align-items" "center" ]
                            , test "has message icon at the left" <|
                                let
                                    messageIcon =
                                        "baseline-message.svg"
                                in
                                \_ ->
                                    init
                                        |> givenResourcePinnedWithComment
                                        |> iconContainer
                                        |> Query.children []
                                        |> Query.first
                                        |> Query.has
                                            [ style "background-image" <|
                                                "url(/public/images/"
                                                    ++ messageIcon
                                                    ++ ")"
                                            , style "background-size" "contain"
                                            , style "width" "24px"
                                            , style "height" "24px"
                                            , style "margin-right" "10px"
                                            ]
                            , test "has pin icon on the right" <|
                                let
                                    pinIcon =
                                        "pin-ic-white.svg"
                                in
                                \_ ->
                                    init
                                        |> givenResourcePinnedWithComment
                                        |> iconContainer
                                        |> Query.children []
                                        |> Query.index 1
                                        |> Query.has
                                            (iconSelector
                                                { image = pinIcon
                                                , size = "20px"
                                                }
                                                ++ [ style "margin-right" "10px" ]
                                            )
                            ]
                        , test "second item is the pinned version" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> header
                                    |> Query.children []
                                    |> Query.index 1
                                    |> Query.has [ text version ]
                        , test "pinned version is vertically centered" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> header
                                    |> Query.children []
                                    |> Query.index 1
                                    |> Query.has
                                        [ style "align-self" "center" ]
                        ]
                    , describe "when unauthenticated"
                        [ test "contains a pre" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.has [ tag "pre" ]
                        , test "pre contains the comment" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "pre" ]
                                    |> Query.has [ text "some pin comment" ]
                        , test "pre fills vertical space and has margin" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "pre" ]
                                    |> Query.has
                                        [ style "margin" "10px 0"
                                        , style "flex-grow" "1"
                                        ]
                        , test "pre has vertical scroll on overflow" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "pre" ]
                                    |> Query.has
                                        [ style "overflow-y" "auto" ]
                        , test "pre has padding" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "pre" ]
                                    |> Query.has
                                        [ style "padding" "10px" ]
                        , test "contains a spacer at the bottom" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> contents
                                    |> Query.children []
                                    |> Query.index -1
                                    |> Query.has
                                        [ style "height" "24px" ]
                        ]
                    , describe "when authorized" <|
                        let
                            textarea =
                                Query.find [ tag "textarea" ]
                        in
                        [ test "contains a textarea" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.has [ tag "textarea" ]
                        , test "textarea has comment as value" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "textarea" ]
                                    |> Query.has
                                        [ attribute <|
                                            Attr.value "some pin comment"
                                        ]
                        , test "textarea has placeholder" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "textarea" ]
                                    |> Query.has
                                        [ attribute <|
                                            Attr.placeholder
                                                "enter a comment"
                                        ]
                        , test "textarea has 10px vertical margin, stretches vertically" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> textarea
                                    |> Query.has
                                        [ style "margin" "10px 0"
                                        , style "flex-grow" "1"
                                        ]
                        , test "textarea has no resize handle" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> textarea
                                    |> Query.has
                                        [ style "resize" "none" ]
                        , test "textarea has padding" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> textarea
                                    |> Query.has
                                        [ style "padding" "10px" ]
                        , test "textarea matches app font" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> textarea
                                    |> Query.has
                                        [ style "font-size" "12px"
                                        , style "font-family" "Inconsolata, monospace"
                                        , style "font-weight" "700"
                                        ]
                        , test "textarea has same color scheme as comment bar" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> textarea
                                    |> Query.has
                                        [ style "background-color" "transparent"
                                        , style "color" almostWhiteHex
                                        , style "outline" "none"
                                        , style "border" <| "1px solid " ++ lightGreyHex
                                        ]
                        , describe "when editing the textarea" <|
                            let
                                givenUserEditedComment =
                                    update (Message.Message.EditComment "foo")
                                        >> Tuple.first
                            in
                            [ test "input in textarea produces EditComment msg" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> commentBar
                                        |> textarea
                                        |> Event.simulate (Event.input "foo")
                                        |> Event.expect
                                            (Msgs.Update <|
                                                Message.Message.EditComment "foo"
                                            )
                            , test "EditComment updates textarea value" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> commentBar
                                        |> textarea
                                        |> Query.has
                                            [ attribute <|
                                                Attr.value "foo"
                                            ]
                            , test "autorefresh doesn't change textarea" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> givenResourcePinnedWithComment
                                        |> commentBar
                                        |> textarea
                                        |> Query.has
                                            [ attribute <|
                                                Attr.value "foo"
                                            ]
                            , test "button outline turns blue" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> commentBar
                                        |> Query.find [ tag "button" ]
                                        |> Query.has
                                            [ style "border" <|
                                                "1px solid "
                                                    ++ commentButtonBlue
                                            ]
                            , defineHoverBehaviour
                                { name = "save comment button"
                                , setup =
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                , query =
                                    commentBar
                                        >> Query.find [ tag "button" ]
                                , unhoveredSelector =
                                    { description = "blue border"
                                    , selector =
                                        [ style "border" <|
                                            "1px solid "
                                                ++ commentButtonBlue
                                        ]
                                    }
                                , mouseEnterMsg =
                                    Msgs.Update <|
                                        Message.Message.Hover <|
                                            Just Message.Message.SaveCommentButton
                                , mouseLeaveMsg =
                                    Msgs.Update <|
                                        Message.Message.Hover Nothing
                                , updateFunc =
                                    \msg ->
                                        Application.update msg
                                            >> Tuple.first
                                , hoveredSelector =
                                    { description = "blue background"
                                    , selector =
                                        [ style "background-color" commentButtonBlue
                                        , style "cursor" "pointer"
                                        ]
                                    }
                                }
                            , test "focusing textarea triggers FocusTextArea msg" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> commentBar
                                        |> Query.find [ tag "textarea" ]
                                        |> Event.simulate Event.focus
                                        |> Event.expect
                                            (Msgs.Update
                                                Message.Message.FocusTextArea
                                            )
                            , test
                                ("keydown subscription active when "
                                    ++ "textarea is focused"
                                )
                              <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenTextareaFocused
                                        |> Application.subscriptions
                                        |> List.member Subscription.OnKeyDown
                                        |> Expect.true "why are we not subscribed to keydowns!?"
                            , test
                                ("keyup subscription active when "
                                    ++ "textarea is focused"
                                )
                              <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenTextareaFocused
                                        |> Application.subscriptions
                                        |> List.member Subscription.OnKeyUp
                                        |> Expect.true "why are we not subscribed to keyups!?"
                            , test "Ctrl-Enter sends SaveComment msg" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> givenTextareaFocused
                                        |> pressControlEnter
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ Effects.SetPinComment
                                                { teamName = teamName
                                                , pipelineName = pipelineName
                                                , resourceName = resourceName
                                                }
                                                "foo"
                                            ]
                            , test "Command + Enter sends SaveComment msg" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> givenTextareaFocused
                                        |> pressMetaEnter
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ Effects.SetPinComment
                                                { teamName = teamName
                                                , pipelineName = pipelineName
                                                , resourceName = resourceName
                                                }
                                                "foo"
                                            ]
                            , test "blurring input triggers BlurTextArea msg" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> givenTextareaFocused
                                        |> commentBar
                                        |> Query.find [ tag "textarea" ]
                                        |> Event.simulate Event.blur
                                        |> Event.expect
                                            (Msgs.Update
                                                Message.Message.BlurTextArea
                                            )
                            , test "Ctrl-Enter after blurring input does nothing" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> givenTextareaFocused
                                        |> givenTextareaBlurred
                                        |> pressControlEnter
                                        |> Tuple.second
                                        |> Expect.equal []
                            , test
                                ("releasing Ctrl key and pressing enter "
                                    ++ "does nothing"
                                )
                              <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> givenTextareaFocused
                                        |> pressEnterKey
                                        |> Tuple.second
                                        |> Expect.equal []
                            , test "button click sends SaveComment msg" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> commentBar
                                        |> Query.find [ tag "button" ]
                                        |> Event.simulate Event.click
                                        |> Event.expect
                                            (Msgs.Update <|
                                                Message.Message.SaveComment "foo"
                                            )
                            , test "SaveComment msg makes API call" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> update
                                            (Message.Message.SaveComment "foo")
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ Effects.SetPinComment
                                                { teamName = teamName
                                                , pipelineName = pipelineName
                                                , resourceName = resourceName
                                                }
                                                "foo"
                                            ]
                            , describe "button loading state" <|
                                let
                                    givenCommentSavingInProgress : Application.Model
                                    givenCommentSavingInProgress =
                                        init
                                            |> givenUserIsAuthorized
                                            |> givenResourcePinnedWithComment
                                            |> givenUserEditedComment
                                            |> update
                                                (Message.Message.SaveComment "foo")
                                            |> Tuple.first

                                    viewButton : Application.Model -> Query.Single Msgs.TopLevelMessage
                                    viewButton =
                                        commentBar
                                            >> Query.find [ tag "button" ]
                                in
                                [ test "shows spinner" <|
                                    \_ ->
                                        givenCommentSavingInProgress
                                            |> viewButton
                                            |> Query.has
                                                [ style "animation"
                                                    "container-rotate 1568ms linear infinite"
                                                , style "height" "12px"
                                                , style "width" "12px"
                                                ]
                                , test "clears button text" <|
                                    \_ ->
                                        givenCommentSavingInProgress
                                            |> viewButton
                                            |> Query.hasNot [ text "save" ]
                                , test "has transparent background on hover" <|
                                    \_ ->
                                        givenCommentSavingInProgress
                                            |> update
                                                (Message.Message.Hover <|
                                                    Just Message.Message.SaveCommentButton
                                                )
                                            |> Tuple.first
                                            |> viewButton
                                            |> Query.has
                                                [ style "background-color" "transparent" ]
                                ]
                            , describe "saving comment API callback"
                                [ test "on success, shows pristine state" <|
                                    \_ ->
                                        init
                                            |> givenUserIsAuthorized
                                            |> givenResourcePinnedWithComment
                                            |> givenUserEditedComment
                                            |> update
                                                (Message.Message.SaveComment
                                                    "foo"
                                                )
                                            |> Tuple.first
                                            |> Application.handleCallback
                                                (Callback.CommentSet
                                                    (Ok ())
                                                )
                                            |> Tuple.first
                                            |> commentBar
                                            |> Query.find [ tag "button" ]
                                            |> Query.has
                                                [ containing [ text "save" ]
                                                , style "background-color"
                                                    "transparent"
                                                , style "border" <|
                                                    "1px solid "
                                                        ++ lightGreyHex
                                                , style "cursor" "default"
                                                ]
                                , test "on success, refetches data" <|
                                    \_ ->
                                        init
                                            |> givenUserIsAuthorized
                                            |> givenResourcePinnedWithComment
                                            |> givenUserEditedComment
                                            |> update
                                                (Message.Message.SaveComment
                                                    "foo"
                                                )
                                            |> Tuple.first
                                            |> Application.handleCallback
                                                (Callback.CommentSet (Ok ()))
                                            |> Tuple.second
                                            |> Expect.equal
                                                [ Effects.FetchResource
                                                    { teamName = teamName
                                                    , pipelineName = pipelineName
                                                    , resourceName = resourceName
                                                    }
                                                ]
                                , test "on error, shows edited state" <|
                                    \_ ->
                                        init
                                            |> givenUserIsAuthorized
                                            |> givenResourcePinnedWithComment
                                            |> givenUserEditedComment
                                            |> update
                                                (Message.Message.SaveComment
                                                    "foo"
                                                )
                                            |> Tuple.first
                                            |> Application.handleCallback
                                                (Callback.CommentSet
                                                    badResponse
                                                )
                                            |> Tuple.first
                                            |> update
                                                (Message.Message.Hover <|
                                                    Just Message.Message.SaveCommentButton
                                                )
                                            |> Tuple.first
                                            |> commentBar
                                            |> Query.find [ tag "button" ]
                                            |> Query.has
                                                [ style "border" <|
                                                    "1px solid "
                                                        ++ commentButtonBlue
                                                , style "cursor" "pointer"
                                                , style "background-color"
                                                    commentButtonBlue
                                                ]
                                , test "on error, refetches data" <|
                                    \_ ->
                                        init
                                            |> givenUserIsAuthorized
                                            |> givenResourcePinnedWithComment
                                            |> givenUserEditedComment
                                            |> update
                                                (Message.Message.SaveComment
                                                    "foo"
                                                )
                                            |> Tuple.first
                                            |> Application.handleCallback
                                                (Callback.CommentSet
                                                    badResponse
                                                )
                                            |> Tuple.second
                                            |> Expect.equal
                                                [ Effects.FetchResource
                                                    { teamName = teamName
                                                    , pipelineName = pipelineName
                                                    , resourceName = resourceName
                                                    }
                                                ]
                                ]
                            , test "edit without changing leaves button alone" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> update
                                            (Message.Message.EditComment
                                                "some pin comment"
                                            )
                                        |> Tuple.first
                                        |> commentBar
                                        |> Query.find [ tag "button" ]
                                        |> Query.has
                                            [ style "border" <|
                                                "1px solid "
                                                    ++ lightGreyHex
                                            ]
                            , test "when unchanged button doesn't hover" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> update
                                            (Message.Message.EditComment
                                                "some pin comment"
                                            )
                                        |> Tuple.first
                                        |> update
                                            (Message.Message.Hover <|
                                                Just Message.Message.SaveCommentButton
                                            )
                                        |> Tuple.first
                                        |> commentBar
                                        |> Query.find [ tag "button" ]
                                        |> Query.has
                                            [ style "background-color" "transparent"
                                            , style "cursor" "default"
                                            ]
                            , test "no comment and empty edit leaves button" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedDynamically
                                        |> update
                                            (Message.Message.EditComment "")
                                        |> Tuple.first
                                        |> commentBar
                                        |> Query.find [ tag "button" ]
                                        |> Query.has
                                            [ style "border" <|
                                                "1px solid "
                                                    ++ lightGreyHex
                                            ]
                            ]
                        , test "contains a button" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.has [ tag "button" ]
                        , test "button has text 'save'" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "button" ]
                                    |> Query.has [ text "save" ]
                        , test "button is flat and black" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "button" ]
                                    |> Query.has
                                        [ style "border" <| "1px solid " ++ lightGreyHex
                                        , style "background-color" "transparent"
                                        , style "color" almostWhiteHex
                                        , style "padding" "5px 10px"
                                        , style "outline" "none"
                                        ]
                        , test "button matches app font" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "button" ]
                                    |> Query.has
                                        [ style "font-size" "12px"
                                        , style "font-family" "Inconsolata, monospace"
                                        , style "font-weight" "700"
                                        ]
                        , test "button aligns to the right" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "button" ]
                                    |> Query.has
                                        [ style "align-self" "flex-end" ]
                        ]
                    ]
                ]
            ]
        , describe "given resource is not pinned"
            [ test "pin comment bar is not visible" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.hasNot [ id "comment-bar" ]
            , test "body does not have padding to accomodate comment bar" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "body" ]
                        |> Query.hasNot
                            [ style "padding-bottom" "300px" ]
            , test "then nothing has purple border" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.hasNot purpleOutlineSelector
            , describe "version headers" <|
                let
                    allVersions : () -> Query.Multiple Msgs.TopLevelMessage
                    allVersions _ =
                        init
                            |> givenResourceIsNotPinned
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find [ class "resource-versions" ]
                            |> Query.findAll anyVersionSelector
                in
                [ test "contain elements that are black with a black border" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.each
                                    (Query.has
                                        [ style "border" <|
                                            "1px solid "
                                                ++ almostBlack
                                        , style "background-color"
                                            almostBlack
                                        ]
                                    )
                            )
                , test "checkboxes are 25px x 25px with icon-type backgrounds" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.first
                                >> Query.has
                                    [ style "margin-right" "5px"
                                    , style "width" "25px"
                                    , style "height" "25px"
                                    , style "background-repeat" "no-repeat"
                                    , style "background-position" "50% 50%"
                                    ]
                            )
                , test "pin buttons are 25px x 25px with icon-type backgrounds" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index 1
                                >> Query.has
                                    [ style "margin-right" "5px"
                                    , style "width" "25px"
                                    , style "height" "25px"
                                    , style "background-repeat" "no-repeat"
                                    , style "background-position" "50% 50%"
                                    ]
                            )
                , test "pin buttons are positioned to anchor their tooltips" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index 1
                                >> Query.has
                                    [ style "position" "relative" ]
                            )
                , test "version headers lay out horizontally, centering" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index 2
                                >> Query.has
                                    [ style "display" "flex"
                                    , style "align-items" "center"
                                    ]
                            )
                , test "version headers fill horizontal space" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index 2
                                >> Query.has
                                    [ style "flex-grow" "1" ]
                            )
                , test "version headers have pointer cursor" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index 2
                                >> Query.has
                                    [ style "cursor" "pointer" ]
                            )
                , test "version headers have contents offset from the left" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index 2
                                >> Query.has
                                    [ style "padding-left" "10px" ]
                            )
                ]
            , test "pin icon on pin bar has default cursor" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has defaultCursor
            , test "clicking pin icon on pin bar does nothing" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Event.simulate Event.click
                        |> Event.toResult
                        |> Expect.err
            , test "mousing over pin icon does nothing" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Event.simulate Event.mouseEnter
                        |> Event.toResult
                        |> Expect.err
            , test "does not show tooltip on the pin icon on ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> hoverOverPinButton
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.hasNot versionTooltipSelector
            , test "all pin buttons have pointer cursor" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find [ class "resource-versions" ]
                        |> Query.findAll anyVersionSelector
                        |> Query.each
                            (Query.find pinButtonSelector
                                >> Query.has pointerCursor
                            )
            , test "all pin buttons have dark background" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find [ class "resource-versions" ]
                        |> Query.findAll anyVersionSelector
                        |> Query.each
                            (Query.find pinButtonSelector
                                >> Query.has [ style "background-color" "#1e1d1d" ]
                            )
            , test "sends PinVersion msg when pin button clicked" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Event.simulate Event.click
                        |> Event.expect
                            (Msgs.Update <| Message.Message.PinVersion versionID)
            , test "pin button on 'v1' shows transition state when (PinVersion v1) is received" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> pinButtonHasTransitionState
            , test "other pin buttons disabled when (PinVersion v1) is received" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> queryView
                        |> Query.find (versionSelector otherVersion)
                        |> Query.find pinButtonSelector
                        |> Event.simulate Event.click
                        |> Event.toResult
                        |> Expect.err
            , test "pin bar shows unpinned state when (PinVersion v1) is received" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "pin button on 'v1' still shows transition state on autorefresh before VersionPinned returns" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> pinButtonHasTransitionState
            , test "pin bar reflects 'v2' when upon successful (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> Application.handleCallback (Callback.VersionPinned (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasPinnedState version
            , test "pin bar shows unpinned state upon receiving failing (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> Application.handleCallback (Callback.VersionPinned badResponse)
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "pin button on 'v1' shows unpinned state upon receiving failing (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> Application.handleCallback (Callback.VersionPinned badResponse)
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> pinButtonHasUnpinnedState
            , test "pin bar expands horizontally to fill available space" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.has [ style "flex-grow" "1" ]
            , test "pin bar margin causes outline to appear inset from the rest of the secondary top bar" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.has [ style "margin" "10px" ]
            , test "there is some space between the check age and the pin bar" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.has [ style "padding-left" "7px" ]
            , test "pin bar lays out contents horizontally, centering them vertically" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.has
                            [ style "display" "flex"
                            , style "align-items" "center"
                            ]
            , test "pin bar is positioned relatively, to facilitate a tooltip" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.has [ style "position" "relative" ]
            , test "pin icon is a 25px square icon" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has
                            [ style "background-repeat" "no-repeat"
                            , style "background-position" "50% 50%"
                            , style "height" "25px"
                            , style "width" "25px"
                            ]
            ]
        , describe "given versioned resource fetched"
            [ test "there is a pin button for each version" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.findAll pinButtonSelector
                        |> Query.count (Expect.equal 1)
            ]
        , describe "check bar" <|
            let
                checkBar userState =
                    let
                        callback =
                            case userState of
                                UserStateLoggedIn user ->
                                    UserFetched (Ok user)

                                UserStateLoggedOut ->
                                    LoggedOut (Ok ())

                                UserStateUnknown ->
                                    EmptyCallback
                    in
                    Application.handleCallback callback
                        >> Tuple.first
                        >> queryView
                        >> Query.find [ class "resource-check-status" ]
                        >> Query.children []
                        >> Query.first
            in
            [ test "lays out horizontally" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> checkBar UserStateLoggedOut
                        |> Query.has [ style "display" "flex" ]
            , test "has two children: check button and status bar" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> checkBar UserStateLoggedOut
                        |> Query.children []
                        |> Query.count (Expect.equal 2)
            , describe "status bar"
                [ test "lays out horizontally and spreads its children" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> checkBar UserStateLoggedOut
                            |> Query.children []
                            |> Query.index 1
                            |> Query.has
                                [ style "display" "flex"
                                , style "justify-content" "space-between"
                                ]
                , test "fills out the check bar and centers children" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> checkBar UserStateLoggedOut
                            |> Query.children []
                            |> Query.index 1
                            |> Query.has
                                [ style "align-items" "center"
                                , style "height" "28px"
                                , style "flex-grow" "1"
                                , style "padding-left" "5px"
                                ]
                , test "has a dark grey background" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> checkBar UserStateLoggedOut
                            |> Query.children []
                            |> Query.index 1
                            |> Query.has
                                [ style "background" "#1e1d1d" ]
                ]
            , describe "when unauthenticated"
                [ defineHoverBehaviour
                    { name = "check button"
                    , setup = init |> givenResourceIsNotPinned
                    , query = checkBar UserStateLoggedOut >> Query.children [] >> Query.first
                    , unhoveredSelector =
                        { description = "black button with grey refresh icon"
                        , selector =
                            [ style "height" "28px"
                            , style "width" "28px"
                            , style "background-color" almostBlack
                            , style "margin-right" "5px"
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style "opacity" "0.5"
                                       , style "margin" "4px"
                                       ]
                            ]
                        }
                    , mouseEnterMsg =
                        Msgs.Update <|
                            Message.Message.Hover <|
                                Just Message.Message.CheckButton
                    , mouseLeaveMsg =
                        Msgs.Update <|
                            Message.Message.Hover Nothing
                    , hoveredSelector =
                        { description = "black button with white refresh icon"
                        , selector =
                            [ style "height" "28px"
                            , style "width" "28px"
                            , style "background-color" almostBlack
                            , style "margin-right" "5px"
                            , style "cursor" "pointer"
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style "opacity" "1"
                                       , style "margin" "4px"
                                       , style "background-size" "contain"
                                       ]
                            ]
                        }
                    , updateFunc = \msg -> Application.update msg >> Tuple.first
                    }
                , test "clicking check button sends Check msg" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> checkBar UserStateLoggedOut
                            |> Query.children []
                            |> Query.first
                            |> Event.simulate Event.click
                            |> Event.expect (Msgs.Update (Message.Message.CheckRequested False))
                , test "Check msg redirects to login" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> update (Message.Message.CheckRequested False)
                            |> Tuple.second
                            |> Expect.equal [ Effects.RedirectToLogin ]
                , test "check bar text does not change" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> update (Message.Message.CheckRequested False)
                            |> Tuple.first
                            |> checkBar UserStateLoggedOut
                            |> Query.find [ tag "h3" ]
                            |> Query.has [ text "checking successfully" ]
                ]
            , describe "when authorized" <|
                let
                    sampleUser : Concourse.User
                    sampleUser =
                        { id = "test"
                        , userName = "test"
                        , name = "test"
                        , email = "test"
                        , teams = Dict.fromList [ ( teamName, [ "member" ] ) ]
                        }
                in
                [ defineHoverBehaviour
                    { name = "check button when authorized"
                    , setup =
                        init
                            |> givenResourceIsNotPinned
                    , query = checkBar (UserStateLoggedIn sampleUser) >> Query.children [] >> Query.first
                    , unhoveredSelector =
                        { description = "black button with grey refresh icon"
                        , selector =
                            [ style "height" "28px"
                            , style "width" "28px"
                            , style "background-color" almostBlack
                            , style "margin-right" "5px"
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style "opacity" "0.5"
                                       , style "margin" "4px"
                                       ]
                            ]
                        }
                    , mouseEnterMsg =
                        Msgs.Update <| Message.Message.Hover <| Just Message.Message.CheckButton
                    , mouseLeaveMsg =
                        Msgs.Update <| Message.Message.Hover Nothing
                    , hoveredSelector =
                        { description = "black button with white refresh icon"
                        , selector =
                            [ style "height" "28px"
                            , style "width" "28px"
                            , style "background-color" almostBlack
                            , style "margin-right" "5px"
                            , style "cursor" "pointer"
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style "opacity" "1"
                                       , style "margin" "4px"
                                       , style "background-size" "contain"
                                       ]
                            ]
                        }
                    , updateFunc = \msg -> Application.update msg >> Tuple.first
                    }
                , test "clicking check button sends Check msg" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> checkBar (UserStateLoggedIn sampleUser)
                            |> Query.children []
                            |> Query.first
                            |> Event.simulate Event.click
                            |> Event.expect (Msgs.Update (Message.Message.CheckRequested True))
                , test "Check msg has CheckResource side effect" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Message.Message.CheckRequested True)
                            |> Tuple.second
                            |> Expect.equal
                                [ Effects.DoCheck
                                    { resourceName = resourceName
                                    , pipelineName = pipelineName
                                    , teamName = teamName
                                    }
                                ]
                , describe "while check in progress" <|
                    let
                        givenCheckInProgress : Application.Model -> Application.Model
                        givenCheckInProgress =
                            givenResourceIsNotPinned
                                >> givenUserIsAuthorized
                                >> update (Message.Message.CheckRequested True)
                                >> Tuple.first
                    in
                    [ test "check bar text says 'currently checking'" <|
                        \_ ->
                            init
                                |> givenCheckInProgress
                                |> checkBar (UserStateLoggedIn sampleUser)
                                |> Query.find [ tag "h3" ]
                                |> Query.has [ text "currently checking" ]
                    , test "clicking check button does nothing" <|
                        \_ ->
                            init
                                |> givenCheckInProgress
                                |> checkBar (UserStateLoggedIn sampleUser)
                                |> Query.children []
                                |> Query.first
                                |> Event.simulate Event.click
                                |> Event.toResult
                                |> Expect.err
                    , test "status icon is spinner" <|
                        \_ ->
                            init
                                |> givenCheckInProgress
                                |> checkBar (UserStateLoggedIn sampleUser)
                                |> Query.children []
                                |> Query.index -1
                                |> Query.has
                                    [ style "display" "flex"
                                    , containing
                                        [ style "animation" <|
                                            "container-rotate 1568ms "
                                                ++ "linear infinite"
                                        , style "height" "14px"
                                        , style "width" "14px"
                                        , style "margin" "7px"
                                        ]
                                    ]
                    , defineHoverBehaviour
                        { name = "check button"
                        , setup = init |> givenCheckInProgress
                        , query = checkBar (UserStateLoggedIn sampleUser) >> Query.children [] >> Query.first
                        , unhoveredSelector =
                            { description = "black button with white refresh icon"
                            , selector =
                                [ style "height" "28px"
                                , style "width" "28px"
                                , style "background-color" almostBlack
                                , style "margin-right" "5px"
                                , style "cursor" "default"
                                , containing <|
                                    iconSelector
                                        { size = "20px"
                                        , image = "baseline-refresh-24px.svg"
                                        }
                                        ++ [ style "opacity" "1"
                                           , style "margin" "4px"
                                           ]
                                ]
                            }
                        , mouseEnterMsg =
                            Msgs.Update <|
                                Message.Message.Hover <|
                                    Just Message.Message.CheckButton
                        , mouseLeaveMsg =
                            Msgs.Update <| Message.Message.Hover Nothing
                        , hoveredSelector =
                            { description = "black button with white refresh icon"
                            , selector =
                                [ style "height" "28px"
                                , style "width" "28px"
                                , style "background-color" almostBlack
                                , style "margin-right" "5px"
                                , style "cursor" "default"
                                , containing <|
                                    iconSelector
                                        { size = "20px"
                                        , image = "baseline-refresh-24px.svg"
                                        }
                                        ++ [ style "opacity" "1"
                                           , style "margin" "4px"
                                           ]
                                ]
                            }
                        , updateFunc = \msg -> Application.update msg >> Tuple.first
                        }
                    ]
                , test "when check resolves successfully, status is check" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Message.Message.CheckRequested True)
                            |> Tuple.first
                            |> Application.handleCallback (Callback.Checked <| Ok ())
                            |> Tuple.first
                            |> checkBar (UserStateLoggedIn sampleUser)
                            |> Query.children []
                            |> Query.index -1
                            |> Query.has
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-success-check.svg"
                                    }
                                    ++ [ style "background-size" "14px 14px" ]
                                )
                , test "when check resolves successfully, resource and versions refresh" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Message.Message.CheckRequested True)
                            |> Tuple.first
                            |> Application.handleCallback (Callback.Checked <| Ok ())
                            |> Tuple.second
                            |> Expect.equal
                                [ Effects.FetchResource
                                    { resourceName = resourceName
                                    , pipelineName = pipelineName
                                    , teamName = teamName
                                    }
                                , Effects.FetchVersionedResources
                                    { resourceName = resourceName
                                    , pipelineName = pipelineName
                                    , teamName = teamName
                                    }
                                    Nothing
                                ]
                , test "when check resolves unsuccessfully, status is error" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Message.Message.CheckRequested True)
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.Checked <|
                                    Err <|
                                        Http.BadStatus
                                            { url = ""
                                            , status =
                                                { code = 400
                                                , message = "bad request"
                                                }
                                            , headers = Dict.empty
                                            , body = ""
                                            }
                                )
                            |> Tuple.first
                            |> checkBar (UserStateLoggedIn sampleUser)
                            |> Query.children []
                            |> Query.index -1
                            |> Query.has
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-exclamation-triangle.svg"
                                    }
                                    ++ [ style "background-size" "14px 14px" ]
                                )
                , test "when check resolves unsuccessfully, resource refreshes" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Message.Message.CheckRequested True)
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.Checked <|
                                    Err <|
                                        Http.BadStatus
                                            { url = ""
                                            , status =
                                                { code = 400
                                                , message = "bad request"
                                                }
                                            , headers = Dict.empty
                                            , body = ""
                                            }
                                )
                            |> Tuple.second
                            |> Expect.equal
                                [ Effects.FetchResource
                                    { resourceName = resourceName
                                    , pipelineName = pipelineName
                                    , teamName = teamName
                                    }
                                ]
                , test "when check returns 401, redirects to login" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Message.Message.CheckRequested True)
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.Checked <|
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
                            |> Tuple.second
                            |> Expect.equal [ Effects.RedirectToLogin ]
                ]
            , describe "when unauthorized" <|
                let
                    sampleUser : Concourse.User
                    sampleUser =
                        { id = "test"
                        , userName = "test"
                        , name = "test"
                        , email = "test"
                        , teams = Dict.fromList [ ( teamName, [ "viewer" ] ) ]
                        }
                in
                [ defineHoverBehaviour
                    { name = "check button"
                    , setup =
                        init
                            |> givenResourceIsNotPinned
                    , query = checkBar (UserStateLoggedIn sampleUser) >> Query.children [] >> Query.first
                    , unhoveredSelector =
                        { description = "black button with grey refresh icon"
                        , selector =
                            [ style "height" "28px"
                            , style "width" "28px"
                            , style "background-color" almostBlack
                            , style "margin-right" "5px"
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style "opacity" "0.5"
                                       , style "margin" "4px"
                                       ]
                            ]
                        }
                    , mouseEnterMsg =
                        Msgs.Update <| Message.Message.Hover <| Just Message.Message.CheckButton
                    , mouseLeaveMsg =
                        Msgs.Update <| Message.Message.Hover Nothing
                    , hoveredSelector =
                        { description = "black button with grey refresh icon"
                        , selector =
                            [ style "height" "28px"
                            , style "width" "28px"
                            , style "background-color" almostBlack
                            , style "margin-right" "5px"
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style "opacity" "0.5"
                                       , style "margin" "4px"
                                       ]
                            ]
                        }
                    , updateFunc = \msg -> Application.update msg >> Tuple.first
                    }
                , test "clicking check button does nothing" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> checkBar (UserStateLoggedIn sampleUser)
                            |> Query.children []
                            |> Query.first
                            |> Event.simulate Event.click
                            |> Event.toResult
                            |> Expect.err
                , test "'last checked' time updates with clock ticks" <|
                    \_ ->
                        init
                            |> Application.handleCallback
                                (Callback.ResourceFetched <|
                                    Ok
                                        { teamName = teamName
                                        , pipelineName = pipelineName
                                        , name = resourceName
                                        , failingToCheck = False
                                        , checkError = ""
                                        , checkSetupError = ""
                                        , lastChecked = Just (Time.millisToPosix 0)
                                        , pinnedVersion = Nothing
                                        , pinnedInConfig = False
                                        , pinComment = Nothing
                                        , icon = Nothing
                                        }
                                )
                            |> Tuple.first
                            |> Application.update
                                (Msgs.DeliveryReceived <|
                                    ClockTicked OneSecond <|
                                        Time.millisToPosix (2 * 1000)
                                )
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "last-checked" ]
                            |> Query.has [ text "2s ago" ]
                , test "'last checked' tooltip respects timezone" <|
                    \_ ->
                        init
                            |> Application.handleCallback
                                (Callback.ResourceFetched <|
                                    Ok
                                        { teamName = teamName
                                        , pipelineName = pipelineName
                                        , name = resourceName
                                        , failingToCheck = False
                                        , checkError = ""
                                        , checkSetupError = ""
                                        , lastChecked =
                                            Just
                                                (Time.millisToPosix 0)
                                        , pinnedVersion = Nothing
                                        , pinnedInConfig = False
                                        , pinComment = Nothing
                                        , icon = Nothing
                                        }
                                )
                            |> Tuple.first
                            |> Application.handleCallback
                                (Callback.GotCurrentTimeZone <|
                                    Time.customZone (5 * 60) []
                                )
                            |> Tuple.first
                            |> Application.update
                                (Msgs.DeliveryReceived <|
                                    ClockTicked OneSecond <|
                                        Time.millisToPosix 1000
                                )
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "last-checked" ]
                            |> Query.has
                                [ attribute <|
                                    Attr.title "Jan 1 1970 05:00:00 AM"
                                ]
                ]
            , test "unsuccessful check shows a warning icon on the right" <|
                \_ ->
                    init
                        |> Application.handleCallback
                            (Callback.ResourceFetched <|
                                Ok
                                    { teamName = teamName
                                    , pipelineName = pipelineName
                                    , name = resourceName
                                    , failingToCheck = True
                                    , checkError = "some error"
                                    , checkSetupError = ""
                                    , lastChecked = Nothing
                                    , pinnedVersion = Nothing
                                    , pinnedInConfig = False
                                    , pinComment = Nothing
                                    , icon = Nothing
                                    }
                            )
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ class "resource-check-status" ]
                        |> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-exclamation-triangle.svg"
                                }
                                ++ [ style "background-size" "14px 14px"
                                   , containing [ text "some error" ]
                                   ]
                            )
            ]
        ]


csrfToken : String
csrfToken =
    "csrf_token"


flags : Application.Flags
flags =
    { turbulenceImgSrc = ""
    , notFoundImgSrc = ""
    , csrfToken = csrfToken
    , authToken = ""
    , instanceName = ""
    , pipelineRunningKeyframes = ""
    }


init : Application.Model
init =
    Common.init
        ("/teams/"
            ++ teamName
            ++ "/pipelines/"
            ++ pipelineName
            ++ "/resources/"
            ++ resourceName
        )


update :
    Message.Message.Message
    -> Application.Model
    -> ( Application.Model, List Effects.Effect )
update =
    Msgs.Update >> Application.update


givenUserIsAuthorized : Application.Model -> Application.Model
givenUserIsAuthorized =
    Application.handleCallback
        (Callback.UserFetched <|
            Ok
                { id = "test"
                , userName = "test"
                , name = "test"
                , email = "test"
                , teams =
                    Dict.fromList
                        [ ( teamName, [ "member" ] )
                        ]
                }
        )
        >> Tuple.first


givenResourcePinnedStatically : Application.Model -> Application.Model
givenResourcePinnedStatically =
    Application.handleCallback
        (Callback.ResourceFetched <|
            Ok
                { teamName = teamName
                , pipelineName = pipelineName
                , name = resourceName
                , failingToCheck = False
                , checkError = ""
                , checkSetupError = ""
                , lastChecked = Nothing
                , pinnedVersion = Just (Dict.fromList [ ( "version", version ) ])
                , pinnedInConfig = True
                , pinComment = Nothing
                , icon = Nothing
                }
        )
        >> Tuple.first


givenResourcePinnedDynamically : Application.Model -> Application.Model
givenResourcePinnedDynamically =
    Application.handleCallback
        (Callback.ResourceFetched <|
            Ok
                { teamName = teamName
                , pipelineName = pipelineName
                , name = resourceName
                , failingToCheck = False
                , checkError = ""
                , checkSetupError = ""
                , lastChecked = Nothing
                , pinnedVersion = Just (Dict.fromList [ ( "version", version ) ])
                , pinnedInConfig = False
                , pinComment = Nothing
                , icon = Nothing
                }
        )
        >> Tuple.first


givenResourcePinnedWithComment : Application.Model -> Application.Model
givenResourcePinnedWithComment =
    Application.handleCallback
        (Callback.ResourceFetched <|
            Ok
                { teamName = teamName
                , pipelineName = pipelineName
                , name = resourceName
                , failingToCheck = False
                , checkError = ""
                , checkSetupError = ""
                , lastChecked = Nothing
                , pinnedVersion =
                    Just (Dict.fromList [ ( "version", version ) ])
                , pinnedInConfig = False
                , pinComment = Just "some pin comment"
                , icon = Nothing
                }
        )
        >> Tuple.first


givenResourceIsNotPinned : Application.Model -> Application.Model
givenResourceIsNotPinned =
    Application.handleCallback
        (Callback.ResourceFetched <|
            Ok
                { teamName = teamName
                , pipelineName = pipelineName
                , name = resourceName
                , failingToCheck = False
                , checkError = ""
                , checkSetupError = ""
                , lastChecked = Just (Time.millisToPosix 0)
                , pinnedVersion = Nothing
                , pinnedInConfig = False
                , pinComment = Nothing
                , icon = Nothing
                }
        )
        >> Tuple.first


givenResourceHasIcon : Application.Model -> Application.Model
givenResourceHasIcon =
    Application.handleCallback
        (Callback.ResourceFetched <|
            Ok
                { teamName = teamName
                , pipelineName = pipelineName
                , name = resourceName
                , failingToCheck = False
                , checkError = ""
                , checkSetupError = ""
                , lastChecked = Just (Time.millisToPosix 0)
                , pinnedVersion = Nothing
                , pinnedInConfig = False
                , pinComment = Nothing
                , icon = Just resourceIcon
                }
        )
        >> Tuple.first


hoverOverPinBar : Application.Model -> Application.Model
hoverOverPinBar =
    update (Message.Message.Hover <| Just Message.Message.PinBar)
        >> Tuple.first


hoverOverPinButton : Application.Model -> Application.Model
hoverOverPinButton =
    update (Message.Message.Hover <| Just Message.Message.PinButton)
        >> Tuple.first


clickToPin : Models.VersionId -> Application.Model -> Application.Model
clickToPin vid =
    update (Message.Message.PinVersion vid)
        >> Tuple.first


clickToUnpin : Application.Model -> Application.Model
clickToUnpin =
    update Message.Message.UnpinVersion
        >> Tuple.first


clickToDisable : Models.VersionId -> Application.Model -> Application.Model
clickToDisable vid =
    update (Message.Message.ToggleVersion Message.Message.Disable vid)
        >> Tuple.first


givenVersionsWithoutPagination : Application.Model -> Application.Model
givenVersionsWithoutPagination =
    Application.handleCallback
        (Callback.VersionedResourcesFetched <|
            Ok
                ( Nothing
                , { content =
                        [ { id = versionID.versionID
                          , version = Dict.fromList [ ( "version", version ) ]
                          , metadata = []
                          , enabled = True
                          }
                        , { id = otherVersionID.versionID
                          , version = Dict.fromList [ ( "version", otherVersion ) ]
                          , metadata = []
                          , enabled = True
                          }
                        , { id = disabledVersionID.versionID
                          , version = Dict.fromList [ ( "version", disabledVersion ) ]
                          , metadata = []
                          , enabled = False
                          }
                        ]
                  , pagination =
                        { previousPage = Nothing
                        , nextPage = Nothing
                        }
                  }
                )
        )
        >> Tuple.first


givenVersionsWithPagination : Application.Model -> Application.Model
givenVersionsWithPagination =
    Application.handleCallback
        (Callback.VersionedResourcesFetched <|
            Ok
                ( Nothing
                , { content =
                        [ { id = versionID.versionID
                          , version = Dict.fromList [ ( "version", version ) ]
                          , metadata = []
                          , enabled = True
                          }
                        , { id = otherVersionID.versionID
                          , version = Dict.fromList [ ( "version", otherVersion ) ]
                          , metadata = []
                          , enabled = True
                          }
                        , { id = disabledVersionID.versionID
                          , version = Dict.fromList [ ( "version", disabledVersion ) ]
                          , metadata = []
                          , enabled = False
                          }
                        ]
                  , pagination =
                        { previousPage =
                            Just
                                { direction = Since 1
                                , limit = 1
                                }
                        , nextPage =
                            Just
                                { direction = Since 100
                                , limit = 1
                                }
                        }
                  }
                )
        )
        >> Tuple.first


givenTextareaFocused : Application.Model -> Application.Model
givenTextareaFocused =
    update Message.Message.FocusTextArea
        >> Tuple.first


givenTextareaBlurred : Application.Model -> Application.Model
givenTextareaBlurred =
    update Message.Message.BlurTextArea
        >> Tuple.first


pressEnterKey :
    Application.Model
    -> ( Application.Model, List Effects.Effect )
pressEnterKey =
    Application.update
        (Msgs.DeliveryReceived <|
            KeyDown
                { ctrlKey = False
                , shiftKey = False
                , metaKey = False
                , code = Keyboard.Enter
                }
        )


pressControlEnter :
    Application.Model
    -> ( Application.Model, List Effects.Effect )
pressControlEnter =
    Application.update
        (Msgs.DeliveryReceived <|
            KeyDown
                { ctrlKey = True
                , shiftKey = False
                , metaKey = False
                , code = Keyboard.Enter
                }
        )


pressMetaEnter :
    Application.Model
    -> ( Application.Model, List Effects.Effect )
pressMetaEnter =
    Application.update
        (Msgs.DeliveryReceived <|
            KeyDown
                { ctrlKey = False
                , shiftKey = False
                , metaKey = True
                , code = Keyboard.Enter
                }
        )


versionSelector : String -> List Selector
versionSelector v =
    anyVersionSelector ++ [ containing [ text v ] ]


anyVersionSelector : List Selector
anyVersionSelector =
    [ tag "li" ]


pinButtonSelector : List Selector
pinButtonSelector =
    [ attribute (Attr.attribute "aria-label" "Pin Resource Version") ]


pointerCursor : List Selector
pointerCursor =
    [ style "cursor" "pointer" ]


defaultCursor : List Selector
defaultCursor =
    [ style "cursor" "default" ]


checkboxSelector : List Selector
checkboxSelector =
    [ attribute (Attr.attribute "aria-label" "Toggle Resource Version Enabled") ]


hasCheckbox : Query.Single msg -> Expectation
hasCheckbox =
    Query.findAll checkboxSelector
        >> Query.count (Expect.equal 1)


purpleOutlineSelector : List Selector
purpleOutlineSelector =
    [ style "border" <| "1px solid " ++ purpleHex ]


findLast : List Selector -> Query.Single msg -> Query.Single msg
findLast selectors =
    Query.findAll selectors >> Query.index -1


pinBarTooltipSelector : List Selector
pinBarTooltipSelector =
    [ id "pin-bar-tooltip" ]


versionTooltipSelector : List Selector
versionTooltipSelector =
    [ style "position" "absolute"
    , style "bottom" "25px"
    , style "background-color" tooltipGreyHex
    , style "z-index" "2"
    , style "padding" "5px"
    , style "width" "170px"
    , containing [ text "enable via pipeline config" ]
    ]


pinButtonHasTransitionState : Query.Single msg -> Expectation
pinButtonHasTransitionState =
    Expect.all
        [ Query.has loadingSpinnerSelector
        , Query.hasNot [ style "background-image" "url(/public/images/pin-ic-white.svg)" ]
        ]


pinButtonHasUnpinnedState : Query.Single msg -> Expectation
pinButtonHasUnpinnedState =
    Expect.all
        [ Query.has [ style "background-image" "url(/public/images/pin-ic-white.svg)" ]
        , Query.hasNot purpleOutlineSelector
        ]


pinBarHasUnpinnedState : Query.Single msg -> Expectation
pinBarHasUnpinnedState =
    Query.find [ id "pin-bar" ]
        >> Expect.all
            [ Query.has [ style "border" <| "1px solid " ++ lightGreyHex ]
            , Query.findAll [ style "background-image" "url(/public/images/pin-ic-grey.svg)" ]
                >> Query.count (Expect.equal 1)
            , Query.hasNot [ tag "table" ]
            ]


pinBarHasPinnedState : String -> Query.Single msg -> Expectation
pinBarHasPinnedState v =
    Query.find [ id "pin-bar" ]
        >> Expect.all
            [ Query.has [ style "border" <| "1px solid " ++ purpleHex ]
            , Query.has [ text v ]
            , Query.findAll [ style "background-image" "url(/public/images/pin-ic-white.svg)" ]
                >> Query.count (Expect.equal 1)
            ]


loadingSpinnerSelector : List Selector
loadingSpinnerSelector =
    [ style "animation" "container-rotate 1568ms linear infinite"
    , style "height" "12.5px"
    , style "width" "12.5px"
    , style "margin" "6.25px"
    ]


checkboxHasTransitionState : Query.Single msg -> Expectation
checkboxHasTransitionState =
    Expect.all
        [ Query.has loadingSpinnerSelector
        , Query.hasNot
            [ style "background-image" "url(/public/images/checkmark-ic.svg)" ]
        ]


checkboxHasDisabledState : Query.Single msg -> Expectation
checkboxHasDisabledState =
    Expect.all
        [ Query.hasNot loadingSpinnerSelector
        , Query.hasNot
            [ style "background-image" "url(/public/images/checkmark-ic.svg)" ]
        ]


checkboxHasEnabledState : Query.Single msg -> Expectation
checkboxHasEnabledState =
    Expect.all
        [ Query.hasNot loadingSpinnerSelector
        , Query.has
            [ style "background-image" "url(/public/images/checkmark-ic.svg)" ]
        ]


versionHasDisabledState : Query.Single msg -> Expectation
versionHasDisabledState =
    Expect.all
        [ Query.has [ style "opacity" "0.5" ]
        , Query.find checkboxSelector
            >> checkboxHasDisabledState
        ]
