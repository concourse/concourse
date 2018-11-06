module ResourceTests exposing (..)

import Concourse
import Dict
import Expect exposing (..)
import Html.Styled as HS
import Html.Attributes as Attr
import Http
import Resource
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (Selector, attribute, containing, class, id, tag, style, text)


teamName : String
teamName =
    "some-team"


pipelineName : String
pipelineName =
    "some-pipeline"


resourceName : String
resourceName =
    "some-resource"


versionID : Int
versionID =
    1


otherVersionID : Int
otherVersionID =
    2


disabledVersionID : Int
disabledVersionID =
    3


version : String
version =
    "v1"


otherVersion : String
otherVersion =
    "v2"


disabledVersion : String
disabledVersion =
    "v3"


tealHex : String
tealHex =
    "#03dac4"


fadedBlackHex : String
fadedBlackHex =
    "#1e1d1d80"


lightGreyHex : String
lightGreyHex =
    "#3d3c3c"


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
        [ test "autorefresh respects expanded state" <|
            \_ ->
                init
                    |> givenResourceUnpinned
                    |> givenVersions
                    |> Resource.update
                        (Resource.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> givenVersions
                    |> queryView
                    |> Query.find (versionSelector version)
                    |> Query.has [ text "metadata" ]
        , test "autorefresh respects 'Inputs To'" <|
            \_ ->
                init
                    |> givenResourceUnpinned
                    |> givenVersions
                    |> Resource.update
                        (Resource.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> Resource.update
                        (Resource.InputToFetched versionID
                            (Ok
                                [ { id = 0
                                  , name = "some-build"
                                  , job = Just { teamName = teamName, pipelineName = pipelineName, jobName = "some-job" }
                                  , status = Concourse.BuildStatusSucceeded
                                  , duration = { startedAt = Nothing, finishedAt = Nothing }
                                  , reapTime = Nothing
                                  }
                                ]
                            )
                        )
                    |> Tuple.first
                    |> givenVersions
                    |> queryView
                    |> Query.find (versionSelector version)
                    |> Query.has [ text "some-build" ]
        , test "autorefresh respects 'Outputs Of'" <|
            \_ ->
                init
                    |> givenResourceUnpinned
                    |> givenVersions
                    |> Resource.update
                        (Resource.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> Resource.update
                        (Resource.OutputOfFetched versionID
                            (Ok
                                [ { id = 0
                                  , name = "some-build"
                                  , job = Just { teamName = teamName, pipelineName = pipelineName, jobName = "some-job" }
                                  , status = Concourse.BuildStatusSucceeded
                                  , duration = { startedAt = Nothing, finishedAt = Nothing }
                                  , reapTime = Nothing
                                  }
                                ]
                            )
                        )
                    |> Tuple.first
                    |> givenVersions
                    |> queryView
                    |> Query.find (versionSelector version)
                    |> Query.has [ text "some-build" ]
        , describe "checkboxes"
            [ test "there is a checkbox for every version" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> queryView
                        |> Query.findAll (anyVersionSelector)
                        |> Query.each hasCheckbox
            , test "there is a pointer cursor for every checkbox" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> queryView
                        |> Query.findAll (anyVersionSelector)
                        |> Query.each (Query.find checkboxSelector >> Query.has pointerCursor)
            , test "enabled versions have checkmarks" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> queryView
                        |> Expect.all
                            [ Query.find (versionSelector version)
                                >> Query.find checkboxSelector
                                >> Query.has [ style [ ( "background-image", "url(/public/images/checkmark_ic.svg)" ) ] ]
                            , Query.find (versionSelector otherVersion)
                                >> Query.find checkboxSelector
                                >> Query.has [ style [ ( "background-image", "url(/public/images/checkmark_ic.svg)" ) ] ]
                            ]
            , test "disabled versions do not have checkmarks" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> Query.hasNot [ style [ ( "background-image", "url(/public/images/checkmark_ic.svg)" ) ] ]
            , test "clicking the checkbox on an enabled version triggers a ToggleVersion msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> Event.simulate Event.click
                        |> Event.expect (Resource.ToggleVersion Resource.Disable versionID)
            , test "receiving a (ToggleVersion Disable) msg causes the relevant checkbox to go into a transition state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> clickToDisable versionID
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> checkboxHasTransitionState
            , test "autorefreshing after receiving a ToggleVersion msg causes the relevant checkbox to stay in a transition state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> clickToDisable versionID
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> checkboxHasTransitionState
            , test "receiving a successful VersionToggled msg causes the relevant checkbox to appear unchecked" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> clickToDisable versionID
                        |> Resource.update (Resource.VersionToggled Resource.Disable versionID (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> versionHasDisabledState
            , test "receiving an error on VersionToggled msg causes the checkbox to go back to its checked state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> clickToDisable versionID
                        |> Resource.update (Resource.VersionToggled Resource.Disable versionID (badResponse))
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> checkboxHasEnabledState
            , test "clicking the checkbox on a disabled version triggers a ToggleVersion msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> Event.simulate Event.click
                        |> Event.expect (Resource.ToggleVersion Resource.Enable disabledVersionID)
            , test "receiving a (ToggleVersion Enable) msg causes the relevant checkbox to go into a transition state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> Resource.update
                            (Resource.ToggleVersion Resource.Enable disabledVersionID)
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> checkboxHasTransitionState
            , test "receiving a successful VersionToggled msg causes the relevant checkbox to appear checked" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> Resource.update
                            (Resource.ToggleVersion Resource.Enable disabledVersionID)
                        |> Tuple.first
                        |> Resource.update (Resource.VersionToggled Resource.Enable disabledVersionID (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector disabledVersion)
                        |> Query.find checkboxSelector
                        |> checkboxHasEnabledState
            , test "receiving a failing VersionToggled msg causes the relevant checkbox to return to its unchecked state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> Resource.update
                            (Resource.ToggleVersion Resource.Enable disabledVersionID)
                        |> Tuple.first
                        |> Resource.update (Resource.VersionToggled Resource.Enable disabledVersionID badResponse)
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
                            |> Query.has [ text version ]
                , test "then pin bar has teal border" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> queryView
                            |> Query.has tealOutlineSelector
                , test "pin button on pinned version has a teal outline" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> Query.has tealOutlineSelector
                , test "checkbox on pinned version has a teal outline" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find checkboxSelector
                            |> Query.has tealOutlineSelector
                , test "all pin buttons have default cursor" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> queryView
                            |> Query.findAll anyVersionSelector
                            |> Query.each
                                (Query.find pinButtonSelector
                                    >> Query.has defaultCursor
                                )
                , test "version header on pinned version has a teal outline" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> findLast [ tag "div", containing [ text version ] ]
                            |> Query.has tealOutlineSelector
                , test "mousing over pin bar sends TogglePinBarTooltip message" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> queryView
                            |> Query.find [ id "pin-bar" ]
                            |> Event.simulate Event.mouseEnter
                            |> Event.expect Resource.TogglePinBarTooltip
                , test "TogglePinBarTooltip causes tooltip to appear" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.has pinBarTooltipSelector
                , test "mousing out of pin bar sends TogglePinBarTooltip message" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.find [ id "pin-bar" ]
                            |> Event.simulate Event.mouseLeave
                            |> Event.expect Resource.TogglePinBarTooltip
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
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector otherVersion)
                            |> Query.has [ style [ ( "opacity", "0.5" ) ] ]
                , test "mousing over the pinned version's pin button sends ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> Event.simulate Event.mouseOver
                            |> Event.expect Resource.ToggleVersionTooltip
                , test "mousing over an unpinned version's pin button doesn't send any msg" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
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
                            |> givenVersions
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.has versionTooltipSelector
                , test "keeps tooltip on the pinned version's pin button on autorefresh" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> toggleVersionTooltip
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.has versionTooltipSelector
                , test "mousing off the pinned version's pin button sends ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> Event.simulate Event.mouseOut
                            |> Event.expect Resource.ToggleVersionTooltip
                , test "mousing off an unpinned version's pin button doesn't send any msg" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
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
                            |> givenVersions
                            |> toggleVersionTooltip
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.hasNot versionTooltipSelector
                , test "clicking on pin button on pinned version doesn't send any msg" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
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
                            |> givenResourceUnpinned
                            |> givenVersions
                            |> queryView
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
                        |> Resource.update Resource.TogglePinBarTooltip
                        |> Tuple.first
                        |> queryView
                        |> Query.hasNot pinBarTooltipSelector
            , test "pin button on pinned version has a teal outline" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Query.has tealOutlineSelector
            , test "checkbox on pinned version has a teal outline" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> Query.has tealOutlineSelector
            , test "pin button on pinned version has a pointer cursor" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Query.has pointerCursor
            , test "pin button on an unpinned version has a default cursor" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector otherVersion)
                        |> Query.find pinButtonSelector
                        |> Query.has defaultCursor
            , test "clicking on pin button on pinned version will trigger UnpinVersion msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Event.simulate Event.click
                        |> Event.expect Resource.UnpinVersion
            , test "pin button on pinned version shows transition state when (UnpinVersion) is received" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> clickToUnpin
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> pinButtonHasTransitionState
            , test "pin button on 'v1' still shows transition state on autorefresh before VersionUnpinned is recieved" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
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
                        |> givenVersions
                        |> clickToUnpin
                        |> Resource.update (Resource.VersionUnpinned (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "pin bar shows unpinned state upon receiving failing (VersionUnpinned) msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> clickToUnpin
                        |> Resource.update (Resource.VersionUnpinned badResponse)
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasPinnedState version
            , test "version header on pinned version has a teal outline" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> findLast [ tag "div", containing [ text version ] ]
                        |> Query.has tealOutlineSelector
            , test "pin button on pinned version has a white icon" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Query.has [ style [ ( "background-image", "url(/public/images/pin_ic_white.svg)" ) ] ]
            , test "does not show tooltip on the pin button on ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> toggleVersionTooltip
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.hasNot versionTooltipSelector
            , test "unpinned versions are lower opacity" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector otherVersion)
                        |> Query.has [ style [ ( "opacity", "0.5" ) ] ]
            , test "pin icon on pinbar is white" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Query.find [ tag "div" ]
                        |> Query.has [ style [ ( "background-image", "url(/public/images/pin_ic_white.svg)" ) ] ]
            , test "all pin buttons have dark background" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> queryView
                        |> Query.findAll anyVersionSelector
                        |> Query.each
                            (Query.find pinButtonSelector
                                >> Query.has [ style [ ( "background-color", "#1e1d1d" ) ] ]
                            )
            ]
        , describe "given resource is not pinned"
            [ test "then nothing has teal border" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> queryView
                        |> Query.hasNot tealOutlineSelector
            , test "does not show tooltip on the pin icon on ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> toggleVersionTooltip
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.hasNot versionTooltipSelector
            , test "all pin buttons have pointer cursor" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> queryView
                        |> Query.findAll anyVersionSelector
                        |> Query.each
                            (Query.find pinButtonSelector
                                >> Query.has pointerCursor
                            )
            , test "all pin buttons have dark background" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> queryView
                        |> Query.findAll anyVersionSelector
                        |> Query.each
                            (Query.find pinButtonSelector
                                >> Query.has [ style [ ( "background-color", "#1e1d1d" ) ] ]
                            )
            , test "sends PinVersion msg when pin button clicked" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> Event.simulate Event.click
                        |> Event.expect (Resource.PinVersion versionID)
            , test "pin button on 'v1' shows transition state when (PinVersion v1) is received" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> clickToPin versionID
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> pinButtonHasTransitionState
            , test "other pin buttons disabled when (PinVersion v1) is received" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
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
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> clickToPin versionID
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "pin button on 'v1' still shows transition state on autorefresh before VersionPinned returns" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> clickToPin versionID
                        |> givenResourceUnpinned
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> pinButtonHasTransitionState
            , test "pin bar reflects 'v2' when upon successful (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> clickToPin versionID
                        |> Resource.update (Resource.VersionPinned (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasPinnedState version
            , test "pin bar shows unpinned state upon receiving failing (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> clickToPin versionID
                        |> Resource.update (Resource.VersionPinned badResponse)
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "pin button on 'v1' shows unpinned state upon receiving failing (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> clickToPin versionID
                        |> Resource.update (Resource.VersionPinned badResponse)
                        |> Tuple.first
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> pinButtonHasUnpinnedState
            ]
        , describe "given versioned resource fetched"
            [ test "there is a pin icon for each version" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.findAll pinButtonSelector
                        |> Query.count (Expect.equal 1)
            ]
        ]


init : Resource.Model
init =
    Resource.init
        { title = always Cmd.none }
        { teamName = teamName
        , pipelineName = pipelineName
        , resourceName = resourceName
        , paging = Nothing
        , csrfToken = ""
        }
        |> Tuple.first


givenResourcePinnedStatically : Resource.Model -> Resource.Model
givenResourcePinnedStatically =
    Resource.update
        (Resource.ResourceFetched <|
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
                }
        )
        >> Tuple.first


givenResourcePinnedDynamically : Resource.Model -> Resource.Model
givenResourcePinnedDynamically =
    Resource.update
        (Resource.ResourceFetched <|
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
                }
        )
        >> Tuple.first


givenResourceUnpinned : Resource.Model -> Resource.Model
givenResourceUnpinned =
    Resource.update
        (Resource.ResourceFetched <|
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
                }
        )
        >> Tuple.first


queryView : Resource.Model -> Query.Single Resource.Msg
queryView =
    Resource.view
        >> HS.toUnstyled
        >> Query.fromHtml


togglePinBarTooltip : Resource.Model -> Resource.Model
togglePinBarTooltip =
    Resource.update Resource.TogglePinBarTooltip
        >> Tuple.first


toggleVersionTooltip : Resource.Model -> Resource.Model
toggleVersionTooltip =
    Resource.update Resource.ToggleVersionTooltip
        >> Tuple.first


clickToPin : Int -> Resource.Model -> Resource.Model
clickToPin versionID =
    Resource.update (Resource.PinVersion versionID)
        >> Tuple.first


clickToUnpin : Resource.Model -> Resource.Model
clickToUnpin =
    Resource.update Resource.UnpinVersion
        >> Tuple.first


clickToDisable : Int -> Resource.Model -> Resource.Model
clickToDisable versionID =
    Resource.update (Resource.ToggleVersion Resource.Disable versionID)
        >> Tuple.first


givenVersions : Resource.Model -> Resource.Model
givenVersions =
    Resource.update
        (Resource.VersionedResourcesFetched Nothing <|
            Ok
                { content =
                    [ { id = versionID
                      , version = Dict.fromList [ ( "version", version ) ]
                      , metadata = []
                      , enabled = True
                      }
                    , { id = otherVersionID
                      , version = Dict.fromList [ ( "version", otherVersion ) ]
                      , metadata = []
                      , enabled = True
                      }
                    , { id = disabledVersionID
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
        >> Tuple.first


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


tealOutlineSelector : List Selector
tealOutlineSelector =
    [ style [ ( "border", "1px solid " ++ tealHex ) ] ]


findLast : List Selector -> Query.Single msg -> Query.Single msg
findLast selectors =
    Query.findAll selectors >> Query.index -1


pinBarTooltipSelector : List Selector
pinBarTooltipSelector =
    [ text "pinned in pipeline config" ]


versionTooltipSelector : List Selector
versionTooltipSelector =
    [ text "enable via pipeline config" ]


pinButtonHasTransitionState : Query.Single msg -> Expectation
pinButtonHasTransitionState =
    Expect.all
        [ Query.has loadingSpinnerSelector
        , Query.hasNot [ style [ ( "background-image", "url(/public/images/pin_ic_white.svg)" ) ] ]
        ]


pinButtonHasUnpinnedState : Query.Single msg -> Expectation
pinButtonHasUnpinnedState =
    Expect.all
        [ Query.has [ style [ ( "background-image", "url(/public/images/pin_ic_white.svg)" ) ] ]
        , Query.hasNot tealOutlineSelector
        ]


pinBarHasUnpinnedState : Query.Single msg -> Expectation
pinBarHasUnpinnedState =
    Query.find [ id "pin-bar" ]
        >> Expect.all
            [ Query.has [ style [ ( "border", "1px solid " ++ lightGreyHex ) ] ]
            , Query.findAll [ style [ ( "background-image", "url(/public/images/pin_ic_grey.svg)" ) ] ]
                >> Query.count (Expect.equal 1)
            , Query.hasNot [ tag "table" ]
            ]


pinBarHasPinnedState : String -> Query.Single msg -> Expectation
pinBarHasPinnedState version =
    Query.find [ id "pin-bar" ]
        >> Expect.all
            [ Query.has [ style [ ( "border", "1px solid " ++ tealHex ) ] ]
            , Query.has [ text version ]
            , Query.findAll [ style [ ( "background-image", "url(/public/images/pin_ic_white.svg)" ) ] ]
                >> Query.count (Expect.equal 1)
            ]


loadingSpinnerSelector : List Selector
loadingSpinnerSelector =
    [ class "fa-circle-o-notch" ]


checkboxHasTransitionState : Query.Single msg -> Expectation
checkboxHasTransitionState =
    Expect.all
        [ Query.has loadingSpinnerSelector
        , Query.hasNot [ style [ ( "background-image", "url(/public/images/checkmark_ic.svg)" ) ] ]
        ]


checkboxHasDisabledState : Query.Single msg -> Expectation
checkboxHasDisabledState =
    Expect.all
        [ Query.hasNot loadingSpinnerSelector
        , Query.hasNot [ style [ ( "background-image", "url(/public/images/checkmark_ic.svg)" ) ] ]
        ]


checkboxHasEnabledState : Query.Single msg -> Expectation
checkboxHasEnabledState =
    Expect.all
        [ Query.hasNot loadingSpinnerSelector
        , Query.has [ style [ ( "background-image", "url(/public/images/checkmark_ic.svg)" ) ] ]
        ]


versionHasDisabledState : Query.Single msg -> Expectation
versionHasDisabledState =
    Expect.all
        [ Query.has [ style [ ( "opacity", "0.5" ) ] ]
        , Query.find checkboxSelector
            >> checkboxHasDisabledState
        ]
