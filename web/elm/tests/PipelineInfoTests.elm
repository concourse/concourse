module PipelineInfoTests exposing (all)

import Application.Application as Application
import Common exposing (queryView)
import Concourse
import Data
import Expect
import Json.Decode
import Json.Encode
import Message.Callback as Callback
import Routes
import Test exposing (Test, describe, test)
import Test.Html.Query as Query
import Test.Html.Selector exposing (id, style, tag, text)
import Url


applyUserData : Maybe Json.Decode.Value -> Concourse.Pipeline -> Concourse.Pipeline
applyUserData maybeUserData pipeline =
    case maybeUserData of
        Just value ->
            Data.withUserData value pipeline

        Nothing ->
            pipeline


userData : String -> Json.Decode.Value
userData json =
    Json.Decode.decodeString Json.Decode.value json
        |> Result.withDefault Json.Encode.null


openInfoPage : Maybe Json.Decode.Value -> Application.Model
openInfoPage maybeUserData =
    let
        pipeline =
            Data.pipeline "team" 1
                |> Data.withName "pipeline"
                |> applyUserData maybeUserData
    in
    Application.init Data.flags
        { protocol = Url.Http
        , host = ""
        , port_ = Nothing
        , path = "/teams/team/pipelines/pipeline/info"
        , query = Nothing
        , fragment = Nothing
        }
        |> Tuple.first
        |> Application.handleCallback (Callback.PipelineFetched (Ok pipeline))
        |> Tuple.first
        |> Application.handleCallback (Callback.AllPipelinesFetched (Ok [ pipeline ]))
        |> Tuple.first


all : Test
all =
    describe "PipelineInfo"
        [ describe "route"
            [ test "parses the info path" <|
                \_ ->
                    Routes.parsePath
                        { protocol = Url.Http
                        , host = ""
                        , port_ = Nothing
                        , path = "/teams/team/pipelines/pipeline/info"
                        , query = Nothing
                        , fragment = Nothing
                        }
                        |> Expect.equal
                            (Just (Routes.PipelineInfo Data.pipelineId))
            , test "builds the info path" <|
                \_ ->
                    Routes.PipelineInfo Data.pipelineId
                        |> Routes.toString
                        |> Expect.equal "/teams/team/pipelines/pipeline/info"
            , test "the plain pipeline path still wins over the info path" <|
                \_ ->
                    Routes.parsePath
                        { protocol = Url.Http
                        , host = ""
                        , port_ = Nothing
                        , path = "/teams/team/pipelines/pipeline"
                        , query = Nothing
                        , fragment = Nothing
                        }
                        |> Expect.equal
                            (Just
                                (Routes.Pipeline
                                    { id = Data.pipelineId, groups = [] }
                                )
                            )
            ]
        , describe "rendering user_data"
            [ test "a bare string renders as markdown" <|
                \_ ->
                    openInfoPage (Just (userData "\"# Heading\\n\\nbody text\""))
                        |> queryView
                        |> Query.find [ id "pipeline-info-markdown" ]
                        |> Query.has [ tag "h1", text "Heading" ]
            , test "a bare string renders no YAML block" <|
                \_ ->
                    openInfoPage (Just (userData "\"just notes\""))
                        |> queryView
                        |> Query.hasNot [ id "pipeline-info-yaml" ]
            , test "an object with a description renders it as markdown" <|
                \_ ->
                    openInfoPage
                        (Just (userData "{\"description\": \"# Title\", \"owner\": \"team-a\"}"))
                        |> queryView
                        |> Query.find [ id "pipeline-info-markdown" ]
                        |> Query.has [ tag "h1", text "Title" ]
            , test "an object with a description renders the rest as YAML" <|
                \_ ->
                    openInfoPage
                        (Just (userData "{\"description\": \"# Title\", \"owner\": \"team-a\"}"))
                        |> queryView
                        |> Query.find [ id "pipeline-info-yaml" ]
                        |> Query.has [ text "owner: team-a" ]
            , test "the description key is not repeated in the YAML block" <|
                \_ ->
                    openInfoPage
                        (Just (userData "{\"description\": \"# Title\", \"owner\": \"team-a\"}"))
                        |> queryView
                        |> Query.find [ id "pipeline-info-yaml" ]
                        |> Query.hasNot [ text "description:" ]
            , test "an object whose only key is description renders no YAML block" <|
                \_ ->
                    openInfoPage (Just (userData "{\"description\": \"# Title\"}"))
                        |> queryView
                        |> Query.hasNot [ id "pipeline-info-yaml" ]
            , test "an object with a non-string description renders wholly as YAML" <|
                \_ ->
                    openInfoPage (Just (userData "{\"description\": 42}"))
                        |> queryView
                        |> Query.find [ id "pipeline-info-yaml" ]
                        |> Query.has [ text "description: 42" ]
            , test "an object without a description renders as YAML" <|
                \_ ->
                    openInfoPage (Just (userData "{\"owner\": \"team-a\", \"tier\": 1}"))
                        |> queryView
                        |> Query.find [ id "pipeline-info-yaml" ]
                        |> Query.has [ text "owner: team-a" ]
            , test "an object without a description renders no markdown" <|
                \_ ->
                    openInfoPage (Just (userData "{\"owner\": \"team-a\"}"))
                        |> queryView
                        |> Query.hasNot [ id "pipeline-info-markdown" ]
            , test "an array renders as YAML" <|
                \_ ->
                    openInfoPage (Just (userData "[\"a\", \"b\"]"))
                        |> queryView
                        |> Query.find [ id "pipeline-info-yaml" ]
                        |> Query.has [ text "- a" ]
            , test "raw HTML in markdown is not rendered" <|
                \_ ->
                    openInfoPage
                        (Just (userData "\"<script>alert(1)</script>\""))
                        |> queryView
                        |> Query.find [ id "pipeline-info-markdown" ]
                        |> Query.hasNot [ tag "script" ]
            , test "the markdown and YAML blocks are separated by a gap" <|
                \_ ->
                    openInfoPage
                        (Just (userData "{\"description\": \"# Title\", \"owner\": \"team-a\"}"))
                        |> queryView
                        |> Query.find [ id "pipeline-info-body" ]
                        |> Query.has
                            [ style "display" "flex"
                            , style "flex-direction" "column"
                            , style "gap" "20px"
                            , style "line-height" "1.5"
                            ]
            , test "no user_data shows the empty state" <|
                \_ ->
                    openInfoPage Nothing
                        |> queryView
                        |> Query.has [ id "pipeline-info-empty" ]
            ]
        ]
