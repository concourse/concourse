module ResourceTests exposing (all)

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
import Html.Styled as HS
import Http
import Resource
import Resource.Models as Models
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
import TopBar


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


purpleHex : String
purpleHex =
    "#5C3BD1"


fadedBlackHex : String
fadedBlackHex =
    "#1e1d1d80"


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
        [ test "autorefresh respects expanded state" <|
            \_ ->
                init
                    |> givenResourceIsNotPinned
                    |> givenVersionsWithoutPagination
                    |> Resource.update
                        (Resource.ExpandVersionedResource versionID)
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
                    |> Resource.update
                        (Resource.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> Resource.update
                        (Resource.InputToFetched versionID
                            (Ok
                                [ { id = 0
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
                    |> Resource.update
                        (Resource.ExpandVersionedResource versionID)
                    |> Tuple.first
                    |> Resource.update
                        (Resource.OutputOfFetched versionID
                            (Ok
                                [ { id = 0
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
            , test
                ("clicking the checkbox on an enabled version triggers"
                    ++ " a ToggleVersion msg"
                )
              <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find checkboxSelector
                        |> Event.simulate Event.click
                        |> Event.expect
                            (Resource.ToggleVersion Models.Disable versionID)
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
                        |> Resource.update (Resource.VersionToggled Models.Disable versionID (Ok ()))
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
                        |> Resource.update (Resource.VersionToggled Models.Disable versionID badResponse)
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
                        |> Event.expect (Resource.ToggleVersion Models.Enable disabledVersionID)
            , test "receiving a (ToggleVersion Enable) msg causes the relevant checkbox to go into a transition state" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersionsWithoutPagination
                        |> Resource.update
                            (Resource.ToggleVersion Models.Enable disabledVersionID)
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
                        |> Resource.update
                            (Resource.ToggleVersion Models.Enable disabledVersionID)
                        |> Tuple.first
                        |> Resource.update (Resource.VersionToggled Models.Enable disabledVersionID (Ok ()))
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
                        |> Resource.update
                            (Resource.ToggleVersion Models.Enable disabledVersionID)
                        |> Tuple.first
                        |> Resource.update (Resource.VersionToggled Models.Enable disabledVersionID badResponse)
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
                            |> Event.expect Resource.TogglePinBarTooltip
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
                            |> Event.expect Resource.ToggleVersionTooltip
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
                            |> Event.expect Resource.ToggleVersionTooltip
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
                        |> Resource.update Resource.TogglePinBarTooltip
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
                        |> Event.expect Resource.UnpinVersion
            , test "mousing over pin icon triggers PinIconHover msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Event.simulate Event.mouseEnter
                        |> Event.expect (Resource.PinIconHover True)
            , test "TogglePinIconHover msg causes pin icon to have dark background" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> Resource.update (Resource.PinIconHover True)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Query.has [ style [ ( "background-color", darkGreyHex ) ] ]
            , test "mousing off pin icon triggers PinIconHover msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> Resource.update (Resource.PinIconHover True)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ id "pin-icon" ]
                        |> Event.simulate Event.mouseLeave
                        |> Event.expect (Resource.PinIconHover False)
            , test "second TogglePinIconHover msg causes pin icon to have transparent background color" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> Resource.update (Resource.PinIconHover True)
                        |> Tuple.first
                        |> Resource.update (Resource.PinIconHover False)
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
                        |> Event.expect Resource.UnpinVersion
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
                        |> Resource.update (Resource.VersionUnpinned (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "pin bar shows unpinned state upon receiving failing (VersionUnpinned) msg" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersionsWithoutPagination
                        |> clickToUnpin
                        |> Resource.update (Resource.VersionUnpinned badResponse)
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
            ]
        , describe "given resource is not pinned"
            [ test "then nothing has purple border" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> queryView
                        |> Query.hasNot purpleOutlineSelector
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
                        |> Event.expect (Resource.PinVersion versionID)
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
                        |> Resource.update (Resource.VersionPinned (Ok ()))
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasPinnedState version
            , test "pin bar shows unpinned state upon receiving failing (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> Resource.update (Resource.VersionPinned badResponse)
                        |> Tuple.first
                        |> queryView
                        |> pinBarHasUnpinnedState
            , test "pin button on 'v1' shows unpinned state upon receiving failing (VersionPinned v1) msg" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> givenVersionsWithoutPagination
                        |> clickToPin versionID
                        |> Resource.update (Resource.VersionPinned badResponse)
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
                        "/teams/some-team/pipelines/some-pipeline/resources/some-resource?limit=1&since=1"
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
                , updateFunc = \msg -> Resource.update msg >> Tuple.first
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
                    Resource.Hover Models.PreviousPage
                , mouseLeaveMsg =
                    Resource.Hover Models.None
                }
            ]
        , describe "check bar" <|
            let
                checkBar =
                    queryView
                        >> Query.find [ class "resource-check-status" ]
                        >> Query.children []
                        >> Query.first
            in
            [ test "lays out horizontally" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> checkBar
                        |> Query.has [ style [ ( "display", "flex" ) ] ]
            , test "has two children: check button and status bar" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> checkBar
                        |> Query.children []
                        |> Query.count (Expect.equal 2)
            , defineHoverBehaviour
                { name = "check button when unauthenticated"
                , setup = init |> givenResourceIsNotPinned
                , query = checkBar >> Query.children [] >> Query.first
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
                , mouseEnterMsg = Resource.Hover Models.CheckButton
                , mouseLeaveMsg = Resource.Hover Models.None
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
                , updateFunc = \msg -> Resource.update msg >> Tuple.first
                }
            , defineHoverBehaviour
                { name = "check button when authorized"
                , setup =
                    init
                        |> givenResourceIsNotPinned
                        |> Resource.update
                            (Resource.TopBarMsg <|
                                TopBar.UserFetched <|
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
                , query = checkBar >> Query.children [] >> Query.first
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
                , mouseEnterMsg = Resource.Hover Models.CheckButton
                , mouseLeaveMsg = Resource.Hover Models.None
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
                , updateFunc = \msg -> Resource.update msg >> Tuple.first
                }
            , defineHoverBehaviour
                { name = "check button when unauthorized"
                , setup =
                    init
                        |> givenResourceIsNotPinned
                        |> Resource.update
                            (Resource.TopBarMsg <|
                                TopBar.UserFetched <|
                                    Ok
                                        { id = "test"
                                        , userName = "test"
                                        , name = "test"
                                        , email = "test"
                                        , teams =
                                            Dict.fromList
                                                [ ( teamName, [ "viewer" ] )
                                                ]
                                        }
                            )
                        |> Tuple.first
                , query = checkBar >> Query.children [] >> Query.first
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
                , mouseEnterMsg = Resource.Hover Models.CheckButton
                , mouseLeaveMsg = Resource.Hover Models.None
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
                , updateFunc = \msg -> Resource.update msg >> Tuple.first
                }
            , test "status bar lays out horizontally maximizing space" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> checkBar
                        |> Query.children []
                        |> Query.index -1
                        |> Query.has
                            [ style
                                [ ( "display", "flex" )
                                , ( "justify-content", "space-between" )
                                , ( "align-items", "center" )
                                , ( "flex-grow", "1" )
                                , ( "height", "28px" )
                                , ( "background", almostBlack )
                                , ( "padding-left", "5px" )
                                ]
                            ]
            , test "successful check shows a checkmark on the right" <|
                \_ ->
                    init
                        |> givenResourceIsNotPinned
                        |> checkBar
                        |> Query.children []
                        |> Query.index -1
                        |> Query.has
                            (iconSelector
                                { size = "28px"
                                , image = "ic-success-check.svg"
                                }
                                ++ [ style
                                        [ ( "background-size", "14px 14px" ) ]
                                   ]
                            )
            , test "unsuccessful check shows a warning icon on the right" <|
                \_ ->
                    init
                        |> Resource.update
                            (Resource.ResourceFetched <|
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


givenResourceIsNotPinned : Resource.Model -> Resource.Model
givenResourceIsNotPinned =
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
    Resource.update (Resource.ToggleVersion Models.Disable versionID)
        >> Tuple.first


givenVersionsWithoutPagination : Resource.Model -> Resource.Model
givenVersionsWithoutPagination =
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


givenVersionsWithPagination : Resource.Model -> Resource.Model
givenVersionsWithPagination =
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
    [ text "enable via pipeline config" ]


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
