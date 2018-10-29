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
        [ describe "given resource is pinned statically"
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
                            |> hasTealOutline
                , test "pin button on pinned version has a teal outline" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> hasTealOutline
                , test "version header on pinned version has a teal outline" <|
                    \_ ->
                        init
                            |> givenResourcePinnedDynamically
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> findLast [ tag "div", containing [ text version ] ]
                            |> hasTealOutline
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
                            |> Query.has [ text "pinned in pipeline config" ]
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
                            |> Query.hasNot [ text "pinned in pipeline config" ]
                ]
            , describe "per-version pin icons"
                [ test "unpinned versions are lower opacity" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector otherVersion)
                            |> Query.has [ style [ ( "opacity", "0.5" ) ] ]
                , test "mousing over a per-version pin icon sends ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.find pinButtonSelector
                            |> Event.simulate Event.mouseOver
                            |> Event.expect Resource.ToggleVersionTooltip
                , test "shows tooltip on the per-version pin icon on ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.has [ text "enable via pipeline config" ]
                , test "mousing off a per-version pin icon sends ToggleVersionTooltip" <|
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
                , test "hides tooltip on the per-version pin icon on ToggleVersionTooltip" <|
                    \_ ->
                        init
                            |> givenResourcePinnedStatically
                            |> givenVersions
                            |> toggleVersionTooltip
                            |> toggleVersionTooltip
                            |> queryView
                            |> Query.find (versionSelector version)
                            |> Query.hasNot [ text "enable via pipeline config" ]
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
                        |> Query.hasNot [ text "pinned in pipeline config" ]
            , test "pin button on pinned version has a teal outline" <|
                \_ ->
                    init
                        |> givenResourcePinnedStatically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> Query.find pinButtonSelector
                        |> hasTealOutline
            , test "version header on pinned version has a teal outline" <|
                \_ ->
                    init
                        |> givenResourcePinnedDynamically
                        |> givenVersions
                        |> queryView
                        |> Query.find (versionSelector version)
                        |> findLast [ tag "div", containing [ text version ] ]
                        |> hasTealOutline
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
                        |> Resource.update Resource.ToggleVersionTooltip
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.hasNot [ text "enable via pipeline config" ]
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


givenResourcePinnedStatically : Resource.Model -> Resource.Model
givenResourcePinnedStatically =
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


givenResourcePinnedDynamically : Resource.Model -> Resource.Model
givenResourcePinnedDynamically =
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
    anyVersionSelector ++ [ containing [ text version ] ]


anyVersionSelector : List Selector
anyVersionSelector =
    [ tag "li" ]


pinButtonSelector : List Selector
pinButtonSelector =
    [ attribute (Attr.attribute "aria-label" "Pin Resource Version") ]


hasTealOutline : Query.Single msg -> Expectation
hasTealOutline =
    Query.has [ style [ ( "border", "1px solid " ++ tealHex ) ] ]


findLast : List Selector -> Query.Single msg -> Query.Single msg
findLast selectors =
    Query.findAll selectors >> Query.index -1
