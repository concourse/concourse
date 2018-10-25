module ResourceTests exposing (..)

import Dict
import Expect exposing (..)
import Html.Styled as HS
import Html.Attributes as Attr
import Resource
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, containing, id, tag, text)


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


version : String
version =
    "v1"


tealHex : String
tealHex =
    "#03dac4"


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
                    |> Query.has [ text "background-color:#1e1d1d" ]
        , test "viewPin has faded background when disabled" <|
            \_ ->
                Resource.viewPin
                    { id = versionID
                    , showTooltip = False
                    , pinState = Resource.Disabled
                    }
                    |> HS.toUnstyled
                    |> Query.fromHtml
                    |> Query.has [ text "background-color:#1e1d1d80" ]
        , test "viewCheckbox has dark background when disabled" <|
            \_ ->
                Resource.viewCheckbox
                    { id = versionID
                    , enabled = False
                    }
                    |> HS.toUnstyled
                    |> Query.fromHtml
                    |> Query.has
                        [ text "background-color:#1e1d1d"
                        , text "background-image:url(/public/images/x_ic.svg)"
                        ]
        , test "viewVersionHeader has dark text when disabled" <|
            \_ ->
                Resource.viewVersionHeader
                    { id = versionID
                    , enabled = False
                    , version = Dict.fromList [ ( "version", version ) ]
                    }
                    |> HS.toUnstyled
                    |> Query.fromHtml
                    |> Query.has
                        [ text "color:#e6e7e880" ]
        , test "viewVersionHeader has light text when enabled" <|
            \_ ->
                Resource.viewVersionHeader
                    { id = versionID
                    , enabled = True
                    , version = Dict.fromList [ ( "version", version ) ]
                    }
                    |> HS.toUnstyled
                    |> Query.fromHtml
                    |> Query.has
                        [ text "color:#e6e7e8" ]
        , test "version body is transparent when disabled" <|
            \_ ->
                Resource.viewVersionBody
                    { inputTo = []
                    , outputOf = []
                    , metadata = []
                    , enabled = False
                    }
                    |> HS.toUnstyled
                    |> Query.fromHtml
                    |> Query.has [ text "opacity:0.5" ]
        , test "version body is opaque when enabled" <|
            \_ ->
                Resource.viewVersionBody
                    { inputTo = []
                    , outputOf = []
                    , metadata = []
                    , enabled = True
                    }
                    |> HS.toUnstyled
                    |> Query.fromHtml
                    |> Query.hasNot [ text "opacity:0.5" ]
        , describe "given resource is pinned via pipeline config"
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
        , describe "given resource is pinned not via pipeline config"
            [ test "when mousing over pin bar, tooltip does not appear" <|
                \_ ->
                    init
                        |> givenResourcePinnedNotViaConfig
                        |> Resource.update Resource.TogglePinBarTooltip
                        |> Tuple.first
                        |> queryView
                        |> Query.hasNot [ text "pinned in pipeline config" ]
            , test "something has a teal background" <|
                \_ ->
                    init
                        |> givenResourcePinnedNotViaConfig
                        |> givenVersions
                        |> queryView
                        |> Query.has [ text <| "background-color:" ++ tealHex ]
            ]
        , describe "given resource is not pinned"
            [ test "then nothing has teal border" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> queryView
                        |> Query.hasNot [ text <| "border:1px solid" ++ tealHex ]
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
            , test "mousing over a pin icon sends ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.find [ attribute (Attr.attribute "aria-label" "Pin Resource Version") ]
                        |> Event.simulate Event.mouseOver
                        |> Event.expect (Resource.ToggleVersionTooltip versionID)
            , test "shows tooltip on the pin icon on ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> Resource.update (Resource.ToggleVersionTooltip versionID)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.has [ text "enable via pipeline config" ]
            , test "mousing off a pin icon sends ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> Resource.update (Resource.ToggleVersionTooltip versionID)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.find [ attribute (Attr.attribute "aria-label" "Pin Resource Version") ]
                        |> Event.simulate Event.mouseOut
                        |> Event.expect (Resource.ToggleVersionTooltip versionID)
            , test "hides tooltip on the pin icon on ToggleVersionTooltip" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> givenVersions
                        |> Resource.update (Resource.ToggleVersionTooltip versionID)
                        |> Tuple.first
                        |> Resource.update (Resource.ToggleVersionTooltip versionID)
                        |> Tuple.first
                        |> queryView
                        |> Query.find [ tag "li", containing [ text version ] ]
                        |> Query.hasNot [ text "enable via pipeline config" ]
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


givenVersions : Resource.Model -> Resource.Model
givenVersions =
    Resource.update
        (Resource.VersionedResourcesFetched Nothing <|
            Ok
                { content =
                    [ { id = versionID
                      , version = Dict.fromList [ ( "version", version ) ]
                      , enabled = False
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
