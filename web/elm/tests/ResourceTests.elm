module ResourceTests exposing (..)

import Dict
import Html.Styled as HS
import Resource
import Test exposing (..)
import Test.Html.Event as Event
import Test.Html.Query as Query
import Test.Html.Selector exposing (id, text)


teamName : String
teamName =
    "some-team"


pipelineName : String
pipelineName =
    "some-pipeline"


resourceName : String
resourceName =
    "some-resource"


version : String
version =
    "v1"


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


all : Test
all =
    describe "resource page"
        [ describe "given resource is pinned via pipeline config"
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
                        |> Query.has [ text "border:1px solid #03dac4" ]
            , test "mousing over pin bar sends TogglePinBarTooltip message" <|
                \_ ->
                    init
                        |> givenResourcePinnedViaConfig
                        |> queryView
                        |> Query.find [ id "pin-bar" ]
                        |> Event.simulate Event.mouseOver
                        |> Event.expect Resource.TogglePinBarTooltip
            , test "TogglePinBarTooltip cases tooltip to appear" <|
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
                        |> Event.simulate Event.mouseOut
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
            ]
        , describe "given resource is not pinned"
            [ test "then nothing has teal border" <|
                \_ ->
                    init
                        |> givenResourceUnpinned
                        |> queryView
                        |> Query.hasNot [ text "border:1px solid #03dac4" ]
            ]
        ]
