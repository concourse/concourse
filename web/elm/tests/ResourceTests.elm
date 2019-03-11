module ResourceTests exposing (all)

import Application.Application as Application
import Application.Msgs as Msgs
import Callback exposing (Callback(..))
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
import Date
import Dict
import Effects
import Expect exposing (..)
import Html.Attributes as Attr
import Http
import Resource.Models as Models
import Resource.Msgs
import SubPage.Msgs
import Subscription exposing (Delivery(..), Interval(..))
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
import UserState exposing (UserState(..))


commentButtonBlue : String
commentButtonBlue =
    "#196AC8"


teamName : String
teamName =
    "some-team"


pipelineName : String
pipelineName =
    "some-pipeline"


resourceName : String
resourceName =
    "some-resource"


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
    "#5C3BD1"


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
                loggingOut : () -> ( Application.Model, List ( Effects.LayoutDispatch, String, Effects.Effect ) )
                loggingOut _ =
                    init
                        |> handleCallback
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
                        |> handleCallback
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
                        [ ( Effects.SubPage 1, csrfToken, Effects.NavigateTo "/" ) ]
            ]
        , test "subscribes to the five second interval" <|
            \_ ->
                init
                    |> Application.subscriptions
                    |> List.member (Subscription.OnClockTick FiveSeconds)
                    |> Expect.true "not subscribed to the five second interval?"
        , test "autorefreshes resource and versions every 5 seconds" <|
            \_ ->
                init
                    |> Application.update (Msgs.DeliveryReceived (ClockTicked FiveSeconds 0))
                    |> Tuple.second
                    |> Expect.equal
                        [ ( Effects.SubPage 1
                          , csrfToken
                          , Effects.FetchResource
                                { resourceName = resourceName
                                , pipelineName = pipelineName
                                , teamName = teamName
                                }
                          )
                        , ( Effects.SubPage 1
                          , csrfToken
                          , Effects.FetchVersionedResources
                                { resourceName = resourceName
                                , pipelineName = pipelineName
                                , teamName = teamName
                                }
                                Nothing
                          )
                        ]
        , test "autorefresh respects expanded state" <|
            \_ ->
                init
                    |> givenResourceIsNotPinned
                    |> givenVersionsWithoutPagination
                    |> update
                        (Resource.Msgs.ExpandVersionedResource versionID)
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
                        (Resource.Msgs.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> handleCallback
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
                        (Resource.Msgs.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> handleCallback
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
                                    [ style
                                        [ ( "background-image", checkIcon ) ]
                                    ]
                            , Query.find (versionSelector otherVersion)
                                >> Query.find checkboxSelector
                                >> Query.has
                                    [ style
                                        [ ( "background-image", checkIcon ) ]
                                    ]
                            ]
            , test "disabled versions do not have checkmarks" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> Query.hasNot
                            [ style
                                [ ( "background-image", checkIcon ) ]
                            ]
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
                            (resourceMsg <|
                                Resource.Msgs.ToggleVersion
                                    Models.Disable
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
                        |> handleCallback (Callback.VersionToggled Models.Disable versionID (Ok ()))
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
                        |> handleCallback (Callback.VersionToggled Models.Disable versionID badResponse)
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
                            (resourceMsg <|
                                Resource.Msgs.ToggleVersion
                                    Models.Enable
                                    disabledVersionID
                            )
            , test "receiving a (ToggleVersion Enable) msg causes the relevant checkbox to go into a transition state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> Application.update
                            (resourceMsg <|
                                Resource.Msgs.ToggleVersion
                                    Models.Enable
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
                            (resourceMsg <|
                                Resource.Msgs.ToggleVersion
                                    Models.Enable
                                    disabledVersionID
                            )
                        |> Tuple.first
                        |> handleCallback
                            (Callback.VersionToggled
                                Models.Enable
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
                            (resourceMsg <|
                                Resource.Msgs.ToggleVersion
                                    Models.Enable
                                    disabledVersionID
                            )
                        |> Tuple.first
                        |> handleCallback
                            (Callback.VersionToggled
                                Models.Enable
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
                                [ style [ ( "margin-right", "10px" ) ] ]
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
                                (resourceMsg Resource.Msgs.TogglePinBarTooltip)
                , test "TogglePinBarTooltip causes tooltip to appear" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.has pinBarTooltipSelector
                , test "pin bar tooltip has text 'pinned in pipeline config'" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has [ text "pinned in pipeline config" ]
                , test "pin bar tooltip is positioned above and near the left of the pin bar" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has
                                [ style
                                    [ ( "position", "absolute" )
                                    , ( "top", "-10px" )
                                    , ( "left", "30px" )
                                    ]
                                ]
                , test "pin bar tooltip is light grey" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has
                                [ style [ ( "background-color", tooltipGreyHex ) ] ]
                , test "pin bar tooltip has a bit of padding around text" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has
                                [ style [ ( "padding", "5px" ) ] ]
                , test "pin bar tooltip appears above other elements in the DOM" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.find pinBarTooltipSelector
                            |> Query.has
                                [ style [ ( "z-index", "2" ) ] ]
                , test "mousing out of pin bar sends TogglePinBarTooltip message" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.find [ id "pin-bar" ]
                            |> Event.simulate Event.mouseLeave
                            |> Event.expect
                                (resourceMsg Resource.Msgs.TogglePinBarTooltip)
                , test "when mousing off pin bar, tooltip disappears" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> togglePinBarTooltip
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
                            |> Query.has [ style [ ( "opacity", "0.5" ) ] ]
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
                                (resourceMsg Resource.Msgs.ToggleVersionTooltip)
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
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.has versionTooltipSelector
                , test "keeps tooltip on the pinned version's pin button on autorefresh" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> toggleVersionTooltip
                            |> givenVersionsWithoutPagination
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.has versionTooltipSelector
                , test "mousing off the pinned version's pin button sends ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> Event.simulate Event.mouseOut
                            |> Event.expect
                                (resourceMsg Resource.Msgs.ToggleVersionTooltip)
                , test "mousing off an unpinned version's pin button doesn't send any msg" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersionsWithoutPagination
                            |> toggleVersionTooltip
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
                            |> toggleVersionTooltip
                            |> toggleVersionTooltip
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
                                    >> Query.has [ style [ ( "background-color", "#1e1d1d" ) ] ]
                                )
                ]
            ]
        , describe "given resource is pinned dynamically"
            [ test "when mousing over pin bar, tooltip does not appear" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> update Resource.Msgs.TogglePinBarTooltip
                        |> Tuple.first
                        |> queryView
                        |> Query.hasNot pinBarTooltipSelector
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
                            (resourceMsg Resource.Msgs.UnpinVersion)
            , test "mousing over pin icon triggers PinIconHover msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Event.simulate Event.mouseEnter
                        |> Event.expect
                            (resourceMsg <| Resource.Msgs.PinIconHover True)
            , test "TogglePinIconHover msg causes pin icon to have dark background" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> update (Resource.Msgs.PinIconHover True)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has [ style [ ( "background-color", darkGreyHex ) ] ]
            , test "mousing off pin icon triggers PinIconHover msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> update (Resource.Msgs.PinIconHover True)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Event.simulate Event.mouseLeave
                        |> Event.expect
                            (resourceMsg <| Resource.Msgs.PinIconHover False)
            , test "second TogglePinIconHover msg causes pin icon to have transparent background color" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> update (Resource.Msgs.PinIconHover True)
                        |> Tuple.first
                        |> update (Resource.Msgs.PinIconHover False)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has [ style [ ( "background-color", "transparent" ) ] ]
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
                        |> Event.expect (resourceMsg Resource.Msgs.UnpinVersion)
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
                        |> handleCallback (Callback.VersionUnpinned (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "resource refreshes on successful VersionUnpinned msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> clickToUnpin
                        |> handleCallback (Callback.VersionUnpinned (Ok ()))
                        |> Tuple.second
                        |> Expect.equal
                            [ ( Effects.SubPage 1
                              , csrfToken
                              , Effects.FetchResource
                                    { resourceName = resourceName
                                    , pipelineName = pipelineName
                                    , teamName = teamName
                                    }
                              )
                            ]
            , test "pin bar shows unpinned state upon receiving failing (VersionUnpinned) msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> clickToUnpin
                        |> handleCallback (Callback.VersionUnpinned badResponse)
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
                        |> Query.has [ style [ ( "background-image", "url(/public/images/pin-ic-white.svg)" ) ] ]
            , test "does not show tooltip on the pin button on ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> toggleVersionTooltip
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
                        |> Query.has [ style [ ( "opacity", "0.5" ) ] ]
            , test "pin icon on pin bar is white" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has [ style [ ( "background-image", "url(/public/images/pin-ic-white.svg)" ) ] ]
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
                                >> Query.has [ style [ ( "background-color", "#1e1d1d" ) ] ]
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
                            [ style [ ( "padding-bottom", "300px" ) ] ]
            , describe "pin comment bar" <|
                let
                    commentBar : Application.Model -> Query.Single Msgs.Msg
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
                                [ style
                                    [ ( "background-color", almostBlack ) ]
                                ]
                , test "pin comment bar is fixed to viewport bottom" <|
                    \_ ->
                        init
                            |> givenResourcePinnedWithComment
                            |> commentBar
                            |> Query.has
                                [ style
                                    [ ( "position", "fixed" )
                                    , ( "bottom", "0" )
                                    ]
                                ]
                , test "pin comment bar is as wide as the viewport" <|
                    \_ ->
                        init
                            |> givenResourcePinnedWithComment
                            |> commentBar
                            |> Query.has [ style [ ( "width", "100%" ) ] ]
                , test "pin comment bar is 300px tall" <|
                    \_ ->
                        init
                            |> givenResourcePinnedWithComment
                            |> commentBar
                            |> Query.has [ style [ ( "height", "300px" ) ] ]
                , test "pin comment bar centers contents horizontally" <|
                    \_ ->
                        init
                            |> givenResourcePinnedWithComment
                            |> commentBar
                            |> Query.has
                                [ style
                                    [ ( "display", "flex" )
                                    , ( "justify-content", "center" )
                                    ]
                                ]
                , describe "contents" <|
                    let
                        contents : Application.Model -> Query.Single Msgs.Msg
                        contents =
                            commentBar >> Query.children [] >> Query.first
                    in
                    [ test "is 700px wide" <|
                        \_ ->
                            init
                                |> givenResourcePinnedWithComment
                                |> contents
                                |> Query.has [ style [ ( "width", "700px" ) ] ]
                    , test "has vertical padding" <|
                        \_ ->
                            init
                                |> givenResourcePinnedWithComment
                                |> contents
                                |> Query.has
                                    [ style [ ( "padding", "20px 0" ) ] ]
                    , test "lays out vertically and left-aligned" <|
                        \_ ->
                            init
                                |> givenResourcePinnedWithComment
                                |> contents
                                |> Query.has
                                    [ style
                                        [ ( "display", "flex" )
                                        , ( "flex-direction", "column" )
                                        ]
                                    ]
                    , describe "header" <|
                        let
                            header : Application.Model -> Query.Single Msgs.Msg
                            header =
                                contents >> Query.children [] >> Query.first
                        in
                        [ test "lays out horizontally" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> header
                                    |> Query.has
                                        [ style [ ( "display", "flex" ) ] ]
                        , test "aligns contents to top" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> header
                                    |> Query.has
                                        [ style
                                            [ ( "align-items"
                                              , "flex-start"
                                              )
                                            ]
                                        ]
                        , test "doesn't squish vertically" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> header
                                    |> Query.has
                                        [ style [ ( "flex-shrink", "0" ) ] ]
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
                                            [ style [ ( "display", "flex" ) ] ]
                            , test "centers contents vertically" <|
                                \_ ->
                                    init
                                        |> givenResourcePinnedWithComment
                                        |> iconContainer
                                        |> Query.has
                                            [ style
                                                [ ( "align-items"
                                                  , "center"
                                                  )
                                                ]
                                            ]
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
                                            [ style
                                                [ ( "background-image"
                                                  , "url(/public/images/"
                                                        ++ messageIcon
                                                        ++ ")"
                                                  )
                                                , ( "background-size", "contain" )
                                                , ( "width", "24px" )
                                                , ( "height", "24px" )
                                                , ( "margin-right", "10px" )
                                                ]
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
                                                ++ [ style
                                                        [ ( "margin-right"
                                                          , "10px"
                                                          )
                                                        ]
                                                   ]
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
                                        [ style
                                            [ ( "align-self"
                                              , "center"
                                              )
                                            ]
                                        ]
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
                                        [ style
                                            [ ( "margin", "10px 0" )
                                            , ( "flex-grow", "1" )
                                            ]
                                        ]
                        , test "pre has vertical scroll on overflow" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "pre" ]
                                    |> Query.has
                                        [ style [ ( "overflow-y", "auto" ) ] ]
                        , test "pre has padding" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "pre" ]
                                    |> Query.has
                                        [ style [ ( "padding", "10px" ) ] ]
                        , test "contains a spacer at the bottom" <|
                            \_ ->
                                init
                                    |> givenResourcePinnedWithComment
                                    |> contents
                                    |> Query.children []
                                    |> Query.index -1
                                    |> Query.has
                                        [ style [ ( "height", "24px" ) ] ]
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
                                        [ style
                                            [ ( "margin", "10px 0" )
                                            , ( "flex-grow", "1" )
                                            ]
                                        ]
                        , test "textarea has no resize handle" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> textarea
                                    |> Query.has
                                        [ style [ ( "resize", "none" ) ] ]
                        , test "textarea has padding" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> textarea
                                    |> Query.has
                                        [ style [ ( "padding", "10px" ) ] ]
                        , test "textarea matches app font" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> textarea
                                    |> Query.has
                                        [ style
                                            [ ( "font-size", "12px" )
                                            , ( "font-family", "Inconsolata, monospace" )
                                            , ( "font-weight", "700" )
                                            ]
                                        ]
                        , test "textarea has same color scheme as comment bar" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> textarea
                                    |> Query.has
                                        [ style
                                            [ ( "background-color", "transparent" )
                                            , ( "color", almostWhiteHex )
                                            , ( "outline", "none" )
                                            , ( "border", "1px solid " ++ lightGreyHex )
                                            ]
                                        ]
                        , describe "when editing the textarea" <|
                            let
                                givenUserEditedComment =
                                    update (Resource.Msgs.EditComment "foo")
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
                                            (resourceMsg <|
                                                Resource.Msgs.EditComment "foo"
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
                                            [ style
                                                [ ( "border"
                                                  , "1px solid " ++ commentButtonBlue
                                                  )
                                                ]
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
                                        [ style
                                            [ ( "border"
                                              , "1px solid "
                                                    ++ commentButtonBlue
                                              )
                                            ]
                                        ]
                                    }
                                , mouseEnterMsg =
                                    resourceMsg <|
                                        Resource.Msgs.Hover
                                            Models.SaveComment
                                , mouseLeaveMsg =
                                    resourceMsg <|
                                        Resource.Msgs.Hover
                                            Models.None
                                , updateFunc =
                                    \msg ->
                                        Application.update msg
                                            >> Tuple.first
                                , hoveredSelector =
                                    { description = "blue background"
                                    , selector =
                                        [ style
                                            [ ( "background-color"
                                              , commentButtonBlue
                                              )
                                            , ( "cursor", "pointer" )
                                            ]
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
                                            (resourceMsg
                                                Resource.Msgs.FocusTextArea
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
                                        |> givenControlKeyDown
                                        |> pressEnterKey
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ ( Effects.SubPage 1
                                              , csrfToken
                                              , Effects.SetPinComment
                                                    { teamName = teamName
                                                    , pipelineName = pipelineName
                                                    , resourceName = resourceName
                                                    }
                                                    "foo"
                                              )
                                            ]
                            , test "Left Command + Enter sends SaveComment msg" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> givenTextareaFocused
                                        |> givenLeftCommandKeyDown
                                        |> pressEnterKey
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ ( Effects.SubPage 1
                                              , csrfToken
                                              , Effects.SetPinComment
                                                    { teamName = teamName
                                                    , pipelineName = pipelineName
                                                    , resourceName = resourceName
                                                    }
                                                    "foo"
                                              )
                                            ]
                            , test "Right Command + Enter sends SaveComment msg" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> givenTextareaFocused
                                        |> givenRightCommandKeyDown
                                        |> pressEnterKey
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ ( Effects.SubPage 1
                                              , csrfToken
                                              , Effects.SetPinComment
                                                    { teamName = teamName
                                                    , pipelineName = pipelineName
                                                    , resourceName = resourceName
                                                    }
                                                    "foo"
                                              )
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
                                            (resourceMsg
                                                Resource.Msgs.BlurTextArea
                                            )
                            , test "Ctrl-Enter after blurring input does nothing" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> givenTextareaFocused
                                        |> givenTextareaBlurred
                                        |> givenControlKeyDown
                                        |> pressEnterKey
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
                                        |> givenControlKeyDown
                                        |> givenControlKeyUp
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
                                            (resourceMsg <|
                                                Resource.Msgs.SaveComment "foo"
                                            )
                            , test "SaveComment msg makes API call" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> givenUserEditedComment
                                        |> update
                                            (Resource.Msgs.SaveComment "foo")
                                        |> Tuple.second
                                        |> Expect.equal
                                            [ ( Effects.SubPage 1
                                              , csrfToken
                                              , Effects.SetPinComment
                                                    { teamName = teamName
                                                    , pipelineName = pipelineName
                                                    , resourceName = resourceName
                                                    }
                                                    "foo"
                                              )
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
                                                (Resource.Msgs.SaveComment "foo")
                                            |> Tuple.first

                                    viewButton : Application.Model -> Query.Single Msgs.Msg
                                    viewButton =
                                        commentBar
                                            >> Query.find [ tag "button" ]
                                in
                                [ test "shows spinner" <|
                                    \_ ->
                                        givenCommentSavingInProgress
                                            |> viewButton
                                            |> Query.has
                                                [ style
                                                    [ ( "animation"
                                                      , "container-rotate 1568ms linear infinite"
                                                      )
                                                    , ( "height", "12px" )
                                                    , ( "width", "12px" )
                                                    ]
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
                                                (Resource.Msgs.Hover
                                                    Models.SaveComment
                                                )
                                            |> Tuple.first
                                            |> viewButton
                                            |> Query.has
                                                [ style
                                                    [ ( "background-color"
                                                      , "transparent"
                                                      )
                                                    ]
                                                ]
                                ]
                            , describe "saving comment API callback"
                                [ test "on success, shows pristine state" <|
                                    \_ ->
                                        init
                                            |> givenUserIsAuthorized
                                            |> givenResourcePinnedWithComment
                                            |> givenUserEditedComment
                                            |> update
                                                (Resource.Msgs.SaveComment
                                                    "foo"
                                                )
                                            |> Tuple.first
                                            |> handleCallback
                                                (Callback.CommentSet
                                                    (Ok ())
                                                )
                                            |> Tuple.first
                                            |> commentBar
                                            |> Query.find [ tag "button" ]
                                            |> Query.has
                                                [ containing [ text "save" ]
                                                , style
                                                    [ ( "background-color"
                                                      , "transparent"
                                                      )
                                                    , ( "border"
                                                      , "1px solid "
                                                            ++ lightGreyHex
                                                      )
                                                    , ( "cursor", "default" )
                                                    ]
                                                ]
                                , test "on success, refetches data" <|
                                    \_ ->
                                        init
                                            |> givenUserIsAuthorized
                                            |> givenResourcePinnedWithComment
                                            |> givenUserEditedComment
                                            |> update
                                                (Resource.Msgs.SaveComment
                                                    "foo"
                                                )
                                            |> Tuple.first
                                            |> handleCallback
                                                (Callback.CommentSet (Ok ()))
                                            |> Tuple.second
                                            |> Expect.equal
                                                [ ( Effects.SubPage 1
                                                  , csrfToken
                                                  , Effects.FetchResource
                                                        { teamName = teamName
                                                        , pipelineName = pipelineName
                                                        , resourceName = resourceName
                                                        }
                                                  )
                                                ]
                                , test "on error, shows edited state" <|
                                    \_ ->
                                        init
                                            |> givenUserIsAuthorized
                                            |> givenResourcePinnedWithComment
                                            |> givenUserEditedComment
                                            |> update
                                                (Resource.Msgs.SaveComment
                                                    "foo"
                                                )
                                            |> Tuple.first
                                            |> handleCallback
                                                (Callback.CommentSet
                                                    badResponse
                                                )
                                            |> Tuple.first
                                            |> update
                                                (Resource.Msgs.Hover
                                                    Models.SaveComment
                                                )
                                            |> Tuple.first
                                            |> commentBar
                                            |> Query.find [ tag "button" ]
                                            |> Query.has
                                                [ style
                                                    [ ( "border"
                                                      , "1px solid "
                                                            ++ commentButtonBlue
                                                      )
                                                    , ( "cursor", "pointer" )
                                                    , ( "background-color"
                                                      , commentButtonBlue
                                                      )
                                                    ]
                                                ]
                                , test "on error, refetches data" <|
                                    \_ ->
                                        init
                                            |> givenUserIsAuthorized
                                            |> givenResourcePinnedWithComment
                                            |> givenUserEditedComment
                                            |> update
                                                (Resource.Msgs.SaveComment
                                                    "foo"
                                                )
                                            |> Tuple.first
                                            |> handleCallback
                                                (Callback.CommentSet
                                                    badResponse
                                                )
                                            |> Tuple.second
                                            |> Expect.equal
                                                [ ( Effects.SubPage 1
                                                  , csrfToken
                                                  , Effects.FetchResource
                                                        { teamName = teamName
                                                        , pipelineName = pipelineName
                                                        , resourceName = resourceName
                                                        }
                                                  )
                                                ]
                                ]
                            , test "edit without changing leaves button alone" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> update
                                            (Resource.Msgs.EditComment
                                                "some pin comment"
                                            )
                                        |> Tuple.first
                                        |> commentBar
                                        |> Query.find [ tag "button" ]
                                        |> Query.has
                                            [ style
                                                [ ( "border"
                                                  , "1px solid " ++ lightGreyHex
                                                  )
                                                ]
                                            ]
                            , test "when unchanged button doesn't hover" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedWithComment
                                        |> update
                                            (Resource.Msgs.EditComment
                                                "some pin comment"
                                            )
                                        |> Tuple.first
                                        |> update
                                            (Resource.Msgs.Hover
                                                Models.SaveComment
                                            )
                                        |> Tuple.first
                                        |> commentBar
                                        |> Query.find [ tag "button" ]
                                        |> Query.has
                                            [ style
                                                [ ( "background-color"
                                                  , "transparent"
                                                  )
                                                , ( "cursor", "default" )
                                                ]
                                            ]
                            , test "no comment and empty edit leaves button" <|
                                \_ ->
                                    init
                                        |> givenUserIsAuthorized
                                        |> givenResourcePinnedDynamically
                                        |> update
                                            (Resource.Msgs.EditComment "")
                                        |> Tuple.first
                                        |> commentBar
                                        |> Query.find [ tag "button" ]
                                        |> Query.has
                                            [ style
                                                [ ( "border"
                                                  , "1px solid " ++ lightGreyHex
                                                  )
                                                ]
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
                                        [ style
                                            [ ( "border", "1px solid " ++ lightGreyHex )
                                            , ( "background-color", "transparent" )
                                            , ( "color", almostWhiteHex )
                                            , ( "padding", "5px 10px" )
                                            , ( "outline", "none" )
                                            ]
                                        ]
                        , test "button matches app font" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "button" ]
                                    |> Query.has
                                        [ style
                                            [ ( "font-size", "12px" )
                                            , ( "font-family", "Inconsolata, monospace" )
                                            , ( "font-weight", "700" )
                                            ]
                                        ]
                        , test "button aligns to the right" <|
                            \_ ->
                                init
                                    |> givenUserIsAuthorized
                                    |> givenResourcePinnedWithComment
                                    |> commentBar
                                    |> Query.find [ tag "button" ]
                                    |> Query.has
                                        [ style [ ( "align-self", "flex-end" ) ] ]
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
                            [ style [ ( "padding-bottom", "300px" ) ] ]
            , test "then nothing has purple border" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.hasNot purpleOutlineSelector
            , describe "version headers" <|
                let
                    allVersions : () -> Query.Multiple Msgs.Msg
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
                                        [ style
                                            [ ( "border"
                                              , "1px solid " ++ almostBlack
                                              )
                                            , ( "background-color"
                                              , almostBlack
                                              )
                                            ]
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
                                    [ style
                                        [ ( "margin-right", "5px" )
                                        , ( "width", "25px" )
                                        , ( "height", "25px" )
                                        , ( "background-repeat", "no-repeat" )
                                        , ( "background-position", "50% 50%" )
                                        ]
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
                                    [ style
                                        [ ( "margin-right", "5px" )
                                        , ( "width", "25px" )
                                        , ( "height", "25px" )
                                        , ( "background-repeat", "no-repeat" )
                                        , ( "background-position", "50% 50%" )
                                        ]
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
                                    [ style [ ( "position", "relative" ) ] ]
                            )
                , test "version headers lay out horizontally, centering" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index 2
                                >> Query.has
                                    [ style
                                        [ ( "display", "flex" )
                                        , ( "align-items", "center" )
                                        ]
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
                                    [ style [ ( "flex-grow", "1" ) ] ]
                            )
                , test "version headers have pointer cursor" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index 2
                                >> Query.has
                                    [ style [ ( "cursor", "pointer" ) ] ]
                            )
                , test "version headers have contents offset from the left" <|
                    allVersions
                        >> Query.each
                            (Query.children []
                                >> Query.first
                                >> Query.children []
                                >> Query.index 2
                                >> Query.has
                                    [ style [ ( "padding-left", "10px" ) ] ]
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
                        |> toggleVersionTooltip
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
                                >> Query.has [ style [ ( "background-color", "#1e1d1d" ) ] ]
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
                            (resourceMsg <| Resource.Msgs.PinVersion versionID)
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
                        |> handleCallback (Callback.VersionPinned (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasPinnedState version
            , test "pin bar shows unpinned state upon receiving failing (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> handleCallback (Callback.VersionPinned badResponse)
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "pin button on 'v1' shows unpinned state upon receiving failing (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> handleCallback (Callback.VersionPinned badResponse)
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
                        |> Query.has [ style [ ( "flex-grow", "1" ) ] ]
            , test "pin bar margin causes outline to appear inset from the rest of the secondary top bar" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.has [ style [ ( "margin", "10px" ) ] ]
            , test "there is some space between the check age and the pin bar" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.has [ style [ ( "padding-left", "7px" ) ] ]
            , test "pin bar lays out contents horizontally, centering them vertically" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "align-items", "center" )
                                ]
                            ]
            , test "pin bar is positioned relatively, to facilitate a tooltip" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.has [ style [ ( "position", "relative" ) ] ]
            , test "pin icon is a 25px square icon" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has
                            [ style
                                [ ( "background-repeat", "no-repeat" )
                                , ( "background-position", "50% 50%" )
                                , ( "height", "25px" )
                                , ( "width", "25px" )
                                ]
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
                                    [ style
                                        [ ( "padding", "5px" )
                                        , ( "display", "flex" )
                                        , ( "align-items", "center" )
                                        , ( "border-left"
                                          , "1px solid " ++ middleGrey
                                          )
                                        ]
                                    , containing
                                        (iconSelector
                                            { image =
                                                "baseline-chevron-left-24px.svg"
                                            , size = "24px"
                                            }
                                            ++ [ style
                                                    [ ( "padding", "5px" )
                                                    , ( "opacity", "0.5" )
                                                    ]
                                               ]
                                        )
                                    ]
                            , Query.index 1
                                >> Query.has
                                    [ style
                                        [ ( "padding", "5px" )
                                        , ( "display", "flex" )
                                        , ( "align-items", "center" )
                                        , ( "border-left"
                                          , "1px solid " ++ middleGrey
                                          )
                                        ]
                                    , containing
                                        (iconSelector
                                            { image =
                                                "baseline-chevron-right-24px.svg"
                                            , size = "24px"
                                            }
                                            ++ [ style
                                                    [ ( "padding", "5px" )
                                                    , ( "opacity", "0.5" )
                                                    ]
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
                        [ style
                            [ ( "padding", "5px" )
                            , ( "display", "flex" )
                            , ( "align-items", "center" )
                            , ( "border-left"
                              , "1px solid " ++ middleGrey
                              )
                            ]
                        , containing
                            (iconSelector
                                { image =
                                    "baseline-chevron-left-24px.svg"
                                , size = "24px"
                                }
                                ++ [ style
                                        [ ( "padding", "5px" )
                                        , ( "opacity", "1" )
                                        ]
                                   , attribute <| Attr.href urlPath
                                   ]
                            )
                        ]
                    }
                , hoveredSelector =
                    { description =
                        "left chevron with light grey circular bg"
                    , selector =
                        [ style
                            [ ( "padding", "5px" )
                            , ( "display", "flex" )
                            , ( "align-items", "center" )
                            , ( "border-left"
                              , "1px solid " ++ middleGrey
                              )
                            ]
                        , containing
                            (iconSelector
                                { image =
                                    "baseline-chevron-left-24px.svg"
                                , size = "24px"
                                }
                                ++ [ style
                                        [ ( "padding", "5px" )
                                        , ( "opacity", "1" )
                                        , ( "border-radius", "50%" )
                                        , ( "background-color"
                                          , "#504b4b"
                                          )
                                        ]
                                   , attribute <| Attr.href urlPath
                                   ]
                            )
                        ]
                    }
                , mouseEnterMsg =
                    resourceMsg <|
                        Resource.Msgs.Hover Models.PreviousPage
                , mouseLeaveMsg =
                    resourceMsg <|
                        Resource.Msgs.Hover Models.None
                }
            ]
        , describe "check bar" <|
            let
                checkBar userState =
                    let
                        callback =
                            case userState of
                                UserStateLoggedIn user ->
                                    ( Effects.Layout, UserFetched (Ok user) )

                                UserStateLoggedOut ->
                                    ( Effects.SubPage 1, LoggedOut (Ok ()) )

                                UserStateUnknown ->
                                    ( Effects.Layout, EmptyCallback )
                    in
                    uncurry Application.handleCallback callback
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
                        |> Query.has [ style [ ( "display", "flex" ) ] ]
            , test "has two children: check button and status bar" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> checkBar UserStateLoggedOut
                        |> Query.children []
                        |> Query.count (Expect.equal 2)
            , describe "when unauthenticated"
                [ defineHoverBehaviour
                    { name = "check button"
                    , setup = init |> givenResourceIsNotPinned
                    , query = checkBar UserStateLoggedOut >> Query.children [] >> Query.first
                    , unhoveredSelector =
                        { description = "black button with grey refresh icon"
                        , selector =
                            [ style
                                [ ( "height", "28px" )
                                , ( "width", "28px" )
                                , ( "background-color", almostBlack )
                                , ( "margin-right", "5px" )
                                ]
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style
                                            [ ( "opacity", "0.5" )
                                            , ( "margin", "4px" )
                                            ]
                                       ]
                            ]
                        }
                    , mouseEnterMsg =
                        resourceMsg <|
                            Resource.Msgs.Hover Models.CheckButton
                    , mouseLeaveMsg =
                        resourceMsg <|
                            Resource.Msgs.Hover Models.None
                    , hoveredSelector =
                        { description = "black button with white refresh icon"
                        , selector =
                            [ style
                                [ ( "height", "28px" )
                                , ( "width", "28px" )
                                , ( "background-color", almostBlack )
                                , ( "margin-right", "5px" )
                                , ( "cursor", "pointer" )
                                ]
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style
                                            [ ( "opacity", "1" )
                                            , ( "margin", "4px" )
                                            , ( "background-size", "contain" )
                                            ]
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
                            |> Event.expect (resourceMsg (Resource.Msgs.CheckRequested False))
                , test "Check msg redirects to login" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> update (Resource.Msgs.CheckRequested False)
                            |> Tuple.second
                            |> Expect.equal
                                [ ( Effects.SubPage 1
                                  , csrfToken
                                  , Effects.RedirectToLogin
                                  )
                                ]
                , test "check bar text does not change" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> update (Resource.Msgs.CheckRequested False)
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
                            [ style
                                [ ( "height", "28px" )
                                , ( "width", "28px" )
                                , ( "background-color", almostBlack )
                                , ( "margin-right", "5px" )
                                ]
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style
                                            [ ( "opacity", "0.5" )
                                            , ( "margin", "4px" )
                                            ]
                                       ]
                            ]
                        }
                    , mouseEnterMsg =
                        resourceMsg <| Resource.Msgs.Hover Models.CheckButton
                    , mouseLeaveMsg =
                        resourceMsg <| Resource.Msgs.Hover Models.None
                    , hoveredSelector =
                        { description = "black button with white refresh icon"
                        , selector =
                            [ style
                                [ ( "height", "28px" )
                                , ( "width", "28px" )
                                , ( "background-color", almostBlack )
                                , ( "margin-right", "5px" )
                                , ( "cursor", "pointer" )
                                ]
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style
                                            [ ( "opacity", "1" )
                                            , ( "margin", "4px" )
                                            , ( "background-size", "contain" )
                                            ]
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
                            |> Event.expect (resourceMsg (Resource.Msgs.CheckRequested True))
                , test "Check msg has CheckResource side effect" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Resource.Msgs.CheckRequested True)
                            |> Tuple.second
                            |> Expect.equal
                                [ ( Effects.SubPage 1
                                  , csrfToken
                                  , Effects.DoCheck
                                        { resourceName = resourceName
                                        , pipelineName = pipelineName
                                        , teamName = teamName
                                        }
                                  )
                                ]
                , describe "while check in progress" <|
                    let
                        givenCheckInProgress : Application.Model -> Application.Model
                        givenCheckInProgress =
                            givenResourceIsNotPinned
                                >> givenUserIsAuthorized
                                >> update (Resource.Msgs.CheckRequested True)
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
                                    [ style [ ( "display", "flex" ) ]
                                    , containing
                                        [ style
                                            [ ( "animation"
                                              , "container-rotate 1568ms "
                                                    ++ "linear infinite"
                                              )
                                            , ( "height", "14px" )
                                            , ( "width", "14px" )
                                            , ( "margin", "7px" )
                                            ]
                                        ]
                                    ]
                    , defineHoverBehaviour
                        { name = "check button"
                        , setup = init |> givenCheckInProgress
                        , query = checkBar (UserStateLoggedIn sampleUser) >> Query.children [] >> Query.first
                        , unhoveredSelector =
                            { description = "black button with white refresh icon"
                            , selector =
                                [ style
                                    [ ( "height", "28px" )
                                    , ( "width", "28px" )
                                    , ( "background-color", almostBlack )
                                    , ( "margin-right", "5px" )
                                    , ( "cursor", "default" )
                                    ]
                                , containing <|
                                    iconSelector
                                        { size = "20px"
                                        , image = "baseline-refresh-24px.svg"
                                        }
                                        ++ [ style
                                                [ ( "opacity", "1" )
                                                , ( "margin", "4px" )
                                                ]
                                           ]
                                ]
                            }
                        , mouseEnterMsg =
                            resourceMsg <|
                                Resource.Msgs.Hover Models.CheckButton
                        , mouseLeaveMsg =
                            resourceMsg <| Resource.Msgs.Hover Models.None
                        , hoveredSelector =
                            { description = "black button with white refresh icon"
                            , selector =
                                [ style
                                    [ ( "height", "28px" )
                                    , ( "width", "28px" )
                                    , ( "background-color", almostBlack )
                                    , ( "margin-right", "5px" )
                                    , ( "cursor", "default" )
                                    ]
                                , containing <|
                                    iconSelector
                                        { size = "20px"
                                        , image = "baseline-refresh-24px.svg"
                                        }
                                        ++ [ style
                                                [ ( "opacity", "1" )
                                                , ( "margin", "4px" )
                                                ]
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
                            |> update (Resource.Msgs.CheckRequested True)
                            |> Tuple.first
                            |> handleCallback (Callback.Checked <| Ok ())
                            |> Tuple.first
                            |> checkBar (UserStateLoggedIn sampleUser)
                            |> Query.children []
                            |> Query.index -1
                            |> Query.has
                                (iconSelector
                                    { size = "28px"
                                    , image = "ic-success-check.svg"
                                    }
                                    ++ [ style
                                            [ ( "background-size"
                                              , "14px 14px"
                                              )
                                            ]
                                       ]
                                )
                , test "when check resolves successfully, resource and versions refresh" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Resource.Msgs.CheckRequested True)
                            |> Tuple.first
                            |> handleCallback (Callback.Checked <| Ok ())
                            |> Tuple.second
                            |> Expect.equal
                                [ ( Effects.SubPage 1
                                  , csrfToken
                                  , Effects.FetchResource
                                        { resourceName = resourceName
                                        , pipelineName = pipelineName
                                        , teamName = teamName
                                        }
                                  )
                                , ( Effects.SubPage 1
                                  , csrfToken
                                  , Effects.FetchVersionedResources
                                        { resourceName = resourceName
                                        , pipelineName = pipelineName
                                        , teamName = teamName
                                        }
                                        Nothing
                                  )
                                ]
                , test "when check resolves unsuccessfully, status is error" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Resource.Msgs.CheckRequested True)
                            |> Tuple.first
                            |> handleCallback
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
                                    ++ [ style
                                            [ ( "background-size"
                                              , "14px 14px"
                                              )
                                            ]
                                       ]
                                )
                , test "when check resolves unsuccessfully, resource refreshes" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Resource.Msgs.CheckRequested True)
                            |> Tuple.first
                            |> handleCallback
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
                                [ ( Effects.SubPage 1
                                  , csrfToken
                                  , Effects.FetchResource
                                        { resourceName = resourceName
                                        , pipelineName = pipelineName
                                        , teamName = teamName
                                        }
                                  )
                                ]
                , test "when check returns 401, redirects to login" <|
                    \_ ->
                        init
                            |> givenResourceIsNotPinned
                            |> givenUserIsAuthorized
                            |> update (Resource.Msgs.CheckRequested True)
                            |> Tuple.first
                            |> handleCallback
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
                            |> Expect.equal
                                [ ( Effects.SubPage 1
                                  , csrfToken
                                  , Effects.RedirectToLogin
                                  )
                                ]
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
                            [ style
                                [ ( "height", "28px" )
                                , ( "width", "28px" )
                                , ( "background-color", almostBlack )
                                , ( "margin-right", "5px" )
                                ]
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style
                                            [ ( "opacity", "0.5" )
                                            , ( "margin", "4px" )
                                            ]
                                       ]
                            ]
                        }
                    , mouseEnterMsg =
                        resourceMsg <| Resource.Msgs.Hover Models.CheckButton
                    , mouseLeaveMsg =
                        resourceMsg <| Resource.Msgs.Hover Models.None
                    , hoveredSelector =
                        { description = "black button with grey refresh icon"
                        , selector =
                            [ style
                                [ ( "height", "28px" )
                                , ( "width", "28px" )
                                , ( "background-color", almostBlack )
                                , ( "margin-right", "5px" )
                                ]
                            , containing <|
                                iconSelector
                                    { size = "20px"
                                    , image = "baseline-refresh-24px.svg"
                                    }
                                    ++ [ style
                                            [ ( "opacity", "0.5" )
                                            , ( "margin", "4px" )
                                            ]
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
                            |> handleCallback
                                (Callback.ResourceFetched <|
                                    Ok
                                        { teamName = teamName
                                        , pipelineName = pipelineName
                                        , name = resourceName
                                        , failingToCheck = False
                                        , checkError = ""
                                        , checkSetupError = ""
                                        , lastChecked = Just (Date.fromTime 0)
                                        , pinnedVersion = Nothing
                                        , pinnedInConfig = False
                                        , pinComment = Nothing
                                        }
                                )
                            |> Tuple.first
                            |> Application.update
                                (Msgs.DeliveryReceived <| ClockTicked OneSecond (2 * Time.second))
                            |> Tuple.first
                            |> queryView
                            |> Query.find [ id "last-checked" ]
                            |> Query.has [ text "2s ago" ]
                ]
            , test "unsuccessful check shows a warning icon on the right" <|
                \_ ->
                    init
                        |> handleCallback
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
                                ++ [ style
                                        [ ( "background-size", "14px 14px" ) ]
                                   , containing [ text "some error" ]
                                   ]
                            )
            ]
        ]


csrfToken : String
csrfToken =
    "csrf_token"


init : Application.Model
init =
    Application.init
        { turbulenceImgSrc = ""
        , notFoundImgSrc = ""
        , csrfToken = csrfToken
        , authToken = ""
        , pipelineRunningKeyframes = ""
        }
        { href = ""
        , host = ""
        , hostname = ""
        , protocol = ""
        , origin = ""
        , port_ = ""
        , pathname =
            "/teams/"
                ++ teamName
                ++ "/pipelines/"
                ++ pipelineName
                ++ "/resources/"
                ++ resourceName
        , search = ""
        , hash = ""
        , username = ""
        , password = ""
        }
        |> Tuple.first


update :
    Resource.Msgs.Msg
    -> Application.Model
    -> ( Application.Model, List ( Effects.LayoutDispatch, String, Effects.Effect ) )
update =
    resourceMsg >> Application.update


handleCallback :
    Callback.Callback
    -> Application.Model
    -> ( Application.Model, List ( Effects.LayoutDispatch, String, Effects.Effect ) )
handleCallback =
    Application.handleCallback (Effects.SubPage 1)


resourceMsg : Resource.Msgs.Msg -> Msgs.Msg
resourceMsg =
    SubPage.Msgs.ResourceMsg >> Msgs.SubMsg 1


givenUserIsAuthorized : Application.Model -> Application.Model
givenUserIsAuthorized =
    Application.handleCallback
        Effects.Layout
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
    handleCallback
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
                }
        )
        >> Tuple.first


givenResourcePinnedDynamically : Application.Model -> Application.Model
givenResourcePinnedDynamically =
    handleCallback
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
                }
        )
        >> Tuple.first


givenResourcePinnedWithComment : Application.Model -> Application.Model
givenResourcePinnedWithComment =
    handleCallback
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
                }
        )
        >> Tuple.first


givenResourceIsNotPinned : Application.Model -> Application.Model
givenResourceIsNotPinned =
    handleCallback
        (Callback.ResourceFetched <|
            Ok
                { teamName = teamName
                , pipelineName = pipelineName
                , name = resourceName
                , failingToCheck = False
                , checkError = ""
                , checkSetupError = ""
                , lastChecked = Nothing
                , pinnedVersion = Nothing
                , pinnedInConfig = False
                , pinComment = Nothing
                }
        )
        >> Tuple.first


queryView : Application.Model -> Query.Single Msgs.Msg
queryView =
    Application.view
        >> Query.fromHtml


togglePinBarTooltip : Application.Model -> Application.Model
togglePinBarTooltip =
    update Resource.Msgs.TogglePinBarTooltip
        >> Tuple.first


toggleVersionTooltip : Application.Model -> Application.Model
toggleVersionTooltip =
    update Resource.Msgs.ToggleVersionTooltip
        >> Tuple.first


clickToPin : Models.VersionId -> Application.Model -> Application.Model
clickToPin versionID =
    update (Resource.Msgs.PinVersion versionID)
        >> Tuple.first


clickToUnpin : Application.Model -> Application.Model
clickToUnpin =
    update Resource.Msgs.UnpinVersion
        >> Tuple.first


clickToDisable : Models.VersionId -> Application.Model -> Application.Model
clickToDisable versionID =
    update (Resource.Msgs.ToggleVersion Models.Disable versionID)
        >> Tuple.first


givenVersionsWithoutPagination : Application.Model -> Application.Model
givenVersionsWithoutPagination =
    handleCallback
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
    handleCallback
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
    update Resource.Msgs.FocusTextArea
        >> Tuple.first


givenTextareaBlurred : Application.Model -> Application.Model
givenTextareaBlurred =
    update Resource.Msgs.BlurTextArea
        >> Tuple.first


givenControlKeyDown : Application.Model -> Application.Model
givenControlKeyDown =
    Application.update (Msgs.DeliveryReceived <| KeyDown 17)
        >> Tuple.first


givenLeftCommandKeyDown : Application.Model -> Application.Model
givenLeftCommandKeyDown =
    Application.update (Msgs.DeliveryReceived <| KeyDown 91)
        >> Tuple.first


givenRightCommandKeyDown : Application.Model -> Application.Model
givenRightCommandKeyDown =
    Application.update (Msgs.DeliveryReceived <| KeyDown 93)
        >> Tuple.first


givenControlKeyUp : Application.Model -> Application.Model
givenControlKeyUp =
    Application.update (Msgs.DeliveryReceived <| KeyUp 17)
        >> Tuple.first


pressEnterKey :
    Application.Model
    -> ( Application.Model, List ( Effects.LayoutDispatch, String, Effects.Effect ) )
pressEnterKey =
    Application.update (Msgs.DeliveryReceived <| KeyDown 13)


versionSelector : String -> List Selector
versionSelector version =
    anyVersionSelector ++ [ containing [ text version ] ]


anyVersionSelector : List Selector
anyVersionSelector =
    [ tag "li" ]


pinButtonSelector : List Selector
pinButtonSelector =
    [ attribute (Attr.attribute "aria-label" "Pin Resource Version") ]


pointerCursor : List Selector
pointerCursor =
    [ style [ ( "cursor", "pointer" ) ] ]


defaultCursor : List Selector
defaultCursor =
    [ style [ ( "cursor", "default" ) ] ]


checkboxSelector : List Selector
checkboxSelector =
    [ attribute (Attr.attribute "aria-label" "Toggle Resource Version Enabled") ]


hasCheckbox : Query.Single msg -> Expectation
hasCheckbox =
    Query.findAll checkboxSelector
        >> Query.count (Expect.equal 1)


purpleOutlineSelector : List Selector
purpleOutlineSelector =
    [ style [ ( "border", "1px solid " ++ purpleHex ) ] ]


findLast : List Selector -> Query.Single msg -> Query.Single msg
findLast selectors =
    Query.findAll selectors >> Query.index -1


pinBarTooltipSelector : List Selector
pinBarTooltipSelector =
    [ id "pin-bar-tooltip" ]


versionTooltipSelector : List Selector
versionTooltipSelector =
    [ style
        [ ( "position", "absolute" )
        , ( "bottom", "25px" )
        , ( "background-color", tooltipGreyHex )
        , ( "z-index", "2" )
        , ( "padding", "5px" )
        , ( "width", "170px" )
        ]
    , containing [ text "enable via pipeline config" ]
    ]


pinButtonHasTransitionState : Query.Single msg -> Expectation
pinButtonHasTransitionState =
    Expect.all
        [ Query.has loadingSpinnerSelector
        , Query.hasNot [ style [ ( "background-image", "url(/public/images/pin-ic-white.svg)" ) ] ]
        ]


pinButtonHasUnpinnedState : Query.Single msg -> Expectation
pinButtonHasUnpinnedState =
    Expect.all
        [ Query.has [ style [ ( "background-image", "url(/public/images/pin-ic-white.svg)" ) ] ]
        , Query.hasNot purpleOutlineSelector
        ]


pinBarHasUnpinnedState : Query.Single msg -> Expectation
pinBarHasUnpinnedState =
    Query.find [ id "pin-bar" ]
        >> Expect.all
            [ Query.has [ style [ ( "border", "1px solid " ++ lightGreyHex ) ] ]
            , Query.findAll [ style [ ( "background-image", "url(/public/images/pin-ic-grey.svg)" ) ] ]
                >> Query.count (Expect.equal 1)
            , Query.hasNot [ tag "table" ]
            ]


pinBarHasPinnedState : String -> Query.Single msg -> Expectation
pinBarHasPinnedState version =
    Query.find [ id "pin-bar" ]
        >> Expect.all
            [ Query.has [ style [ ( "border", "1px solid " ++ purpleHex ) ] ]
            , Query.has [ text version ]
            , Query.findAll [ style [ ( "background-image", "url(/public/images/pin-ic-white.svg)" ) ] ]
                >> Query.count (Expect.equal 1)
            ]


loadingSpinnerSelector : List Selector
loadingSpinnerSelector =
    [ style
        [ ( "animation"
          , "container-rotate 1568ms linear infinite"
          )
        , ( "height", "12.5px" )
        , ( "width", "12.5px" )
        , ( "margin", "6.25px" )
        ]
    ]


checkboxHasTransitionState : Query.Single msg -> Expectation
checkboxHasTransitionState =
    Expect.all
        [ Query.has loadingSpinnerSelector
        , Query.hasNot
            [ style
                [ ( "background-image"
                  , "url(/public/images/checkmark-ic.svg)"
                  )
                ]
            ]
        ]


checkboxHasDisabledState : Query.Single msg -> Expectation
checkboxHasDisabledState =
    Expect.all
        [ Query.hasNot loadingSpinnerSelector
        , Query.hasNot
            [ style
                [ ( "background-image"
                  , "url(/public/images/checkmark-ic.svg)"
                  )
                ]
            ]
        ]


checkboxHasEnabledState : Query.Single msg -> Expectation
checkboxHasEnabledState =
    Expect.all
        [ Query.hasNot loadingSpinnerSelector
        , Query.has [ style [ ( "background-image", "url(/public/images/checkmark-ic.svg)" ) ] ]
        ]


versionHasDisabledState : Query.Single msg -> Expectation
versionHasDisabledState =
    Expect.all
        [ Query.has [ style [ ( "opacity", "0.5" ) ] ]
        , Query.find checkboxSelector
            >> checkboxHasDisabledState
        ]
