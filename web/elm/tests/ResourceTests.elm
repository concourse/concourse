module ResourceTests exposing (..)

import Dict
import Expect exposing (..)
import Html.Styled as HS
import Html.Attributes as Attr
import Resource
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (Selector, attribute, containing, id, tag, style, text)


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


version : String
version =
    "v1"


otherVersion : String
otherVersion =
    "v2"


tealHex : String
tealHex =
    "#03dac4"


fadedBlackHex : String
fadedBlackHex =
    "#1e1d1d80"


all : Test
all =
    describe "resource page"
        [ test "viewPin has dark background when enabled" <|
            \_ ->
                Resource.viewPin
                    { id = versionID
                    , showTooltip = False
                    , pinState = Resource.Unpinned
                    }
                    |> HS.toUnstyled
                    |> Query.fromHtml
                    |> Query.has [ style [ ( "background-color", "#1e1d1d" ) ] ]
        , test "viewPin has faded background when disabled" <|
            \_ ->
                Resource.viewPin
                    { id = versionID
                    , showTooltip = False
                    , pinState = Resource.Disabled
                    }
                    |> HS.toUnstyled
                    |> Query.fromHtml
                    |> Query.has [ style [ ( "background-color", fadedBlackHex ) ] ]
        , describe "given resource is pinned via pipeline config"
            [ describe "pin bar"
                [ test "then pinned version is visible in pin bar" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> queryView
                            |> Query.has [ text version ]
                , test "then pin bar has teal border" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> queryView
                            |> Query.has [ text <| "border:1px solid " ++ tealHex ]
                , test "mousing over pin bar sends TogglePinBarTooltip message" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> queryView
                            |> Query.find [ id "pin-bar" ]
                            |> Event.simulate Event.mouseEnter
                            |> Event.expect Resource.TogglePinBarTooltip
                , test "TogglePinBarTooltip causes tooltip to appear" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.has [ text "pinned in pipeline config" ]
                , test "mousing out of pin bar sends TogglePinBarTooltip message" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.find [ id "pin-bar" ]
                            |> Event.simulate Event.mouseLeave
                            |> Event.expect Resource.TogglePinBarTooltip
                , test "when mousing off pin bar, tooltip disappears" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> togglePinBarTooltip
                            |> togglePinBarTooltip
                            |> queryView
                            |> Query.hasNot [ text "pinned in pipeline config" ]
                ]
            , describe "per-version pin icons"
                [ test "unpinned version has faded background" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector otherVersion)
                            |> Query.find pinIconSelector
                            |> Query.has [ style [ ( "background-color", fadedBlackHex ) ] ]

                -- , test "pinned version has teal background" <|
                --     \_ ->
                --         init
                --             |> givenResourcePinnedViaConfig
                --             |> givenVersions
                --             |> queryView
                --             |> versionSelector version
                --             |> pinIconSelector
                --             |> Query.has [ style [ ( "background-color", tealHex ) ] ]
                , test "mousing over a per-version pin icon sends ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinIconSelector
                            |> Event.simulate Event.mouseOver
                            |> Event.expect Resource.ToggleVersionTooltip
                , test "shows tooltip on the per-version pin icon on ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> givenVersions
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.has [ text "enable via pipeline config" ]
                , test "mousing off a per-version pin icon sends ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> givenVersions
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinIconSelector
                            |> Event.simulate Event.mouseOut
                            |> Event.expect Resource.ToggleVersionTooltip
                , test "hides tooltip on the per-version pin icon on ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedViaConfig
                            |> givenVersions
                            |> toggleVersionTooltip
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.hasNot [ text "enable via pipeline config" ]
                ]
            ]
        , describe "given resource is pinned not via pipeline config"
            [ test "when mousing over pin bar, tooltip does not appear" <|
                \_ ->
                    init
                        |> givenResourcePinnedNotViaConfig
                        |> Resource.update Resource.TogglePinBarTooltip
                        |> Tuple.first
                        |> queryView
                        |> Query.hasNot [ text "pinned in pipeline config" ]
            , test "pin icon on pinned version has a teal background" <|
                \_ ->
                    init
                        |> givenResourcePinnedNotViaConfig
                        |> givenVersions
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.find [ attribute (Attr.attribute "aria-label" "Pin Resource Version") ]
                        |> Query.has [ style [ ( "background-color", tealHex ) ] ]
            , test "does not show tooltip on the pin icon on ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourcePinnedNotViaConfig
                        |> givenVersions
                        |> Resource.update Resource.ToggleVersionTooltip
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.hasNot [ text "enable via pipeline config" ]
            ]
        , describe "given resource is not pinned"
            [ test "then nothing has teal border" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> queryView
                        |> Query.hasNot [ text <| "border:1px solid" ++ tealHex ]
            , test "does not show tooltip on the pin icon on ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> Resource.update Resource.ToggleVersionTooltip
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.hasNot [ text "enable via pipeline config" ]
            , test "all pin icons have dark background" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> Resource.update Resource.ToggleVersionTooltip
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.hasNot [ text "enable via pipeline config" ]
            ]
        , describe "given versioned resource fetched"
            [ test "there is a pin icon for each version" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.findAll [ attribute (Attr.attribute "aria-label" "Pin Resource Version") ]
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


givenResourcePinnedViaConfig : Resource.Model -> Resource.Model
givenResourcePinnedViaConfig =
    Resource.update
        (Resource.ResourceFetched <|
            Ok
                { teamName = teamName
                , pipelineName = pipelineName
                , name = resourceName
                , paused = False
                , failingToCheck = False
                , checkError = ""
                , checkSetupError = ""
                , lastChecked = Nothing
                , pinnedVersion = Just (Dict.fromList [ ( "version", version ) ])
                , pinnedInConfig = True
                }
        )
        >> Tuple.first


givenResourcePinnedNotViaConfig : Resource.Model -> Resource.Model
givenResourcePinnedNotViaConfig =
    Resource.update
        (Resource.ResourceFetched <|
            Ok
                { teamName = teamName
                , pipelineName = pipelineName
                , name = resourceName
                , paused = False
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
                , paused = False
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


givenVersions : Resource.Model -> Resource.Model
givenVersions =
    Resource.update
        (Resource.VersionedResourcesFetched Nothing <|
            Ok
                { content =
                    [ { id = versionID
                      , version = Dict.fromList [ ( "version", version ) ]
                      , metadata = []
                      }
                    , { id = otherVersionID
                      , version = Dict.fromList [ ( "version", otherVersion ) ]
                      , metadata = []
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
    [ tag "li", containing [ text version ] ]


pinIconSelector : List Selector
pinIconSelector =
    [ attribute (Attr.attribute "aria-label" "Pin Resource Version") ]
