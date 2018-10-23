module ResourceTests exposing (..)

import Dict
import Expect exposing (..)
import Html.Styled as HS
import Resource
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (text)


all : Test
all =
    describe "resource page"
        [ describe "when resource is pinned"
            [ test "shows pinned version in pin bar" <|
                let
                    teamName =
                        "some-team"

                    pipelineName =
                        "some-pipeline"

                    resourceName =
                        "some-resource"

                    version =
                        "v1"
                in
                    \_ ->
                        Resource.init
                            { title = always Cmd.none }
                            { teamName = teamName
                            , pipelineName = pipelineName
                            , resourceName = resourceName
                            , paging = Nothing
                            , csrfToken = ""
                            }
                            |> Tuple.first
                            |> Resource.update
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
                            |> Tuple.first
                            |> Resource.view
                            |> HS.toUnstyled
                            |> Query.fromHtml
                            |> Query.has [ text version ]
            ]
        ]
