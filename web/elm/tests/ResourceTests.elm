module ResourceTests exposing (..)

import Dict
import Html.Styled as HS
import Resource
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (text)


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


givenPinnedResource : Resource.Model -> Resource.Model
givenPinnedResource =
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
                }
        )
        >> Tuple.first


givenUnpinnedResource : Resource.Model -> Resource.Model
givenUnpinnedResource =
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
                }
        )
        >> Tuple.first


queryView : Resource.Model -> Query.Single Resource.Msg
queryView =
    Resource.view
        >> HS.toUnstyled
        >> Query.fromHtml


all : Test
all =
    describe "resource page"
        [ describe "given resource is pinned"
            [ test "then pinned version is visible in pin bar" <|
                \_ ->
                    init
                        |> givenPinnedResource
                        |> queryView
                        |> Query.has [ text version ]
            , test "then pin bar has teal border" <|
                \_ ->
                    init
                        |> givenPinnedResource
                        |> queryView
                        |> Query.has [ text "border:1px solid #03dac4" ]
            ]
        , describe "given resource is not pinned"
            [ test "then nothing has teal border" <|
                \_ ->
                    init
                        |> givenUnpinnedResource
                        |> queryView
                        |> Query.hasNot [ text "border:1px solid #03dac4" ]
            ]
        ]
