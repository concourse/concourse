module SerializationTests exposing (all)

import Concourse exposing (JsonValue(..))
import Concourse.BuildStatus as BuildStatus
import Data
import Dict
import Expect
import Json.Decode
import Test exposing (Test, describe, test)
import Time


all : Test
all =
    describe "type serialization/deserialization" <|
        let
            instanceVars =
                Dict.fromList [ ( "k", JsonString "v" ) ]
        in
        [ test "job encoding/decoding are inverses" <|
            \_ ->
                let
                    job =
                        Data.job 1
                            |> Data.withPipelineInstanceVars instanceVars
                in
                job
                    |> Concourse.encodeJob
                    |> Json.Decode.decodeValue Concourse.decodeJob
                    |> Expect.equal (Ok job)
        , test "resource encoding/decoding are inverses" <|
            \_ ->
                let
                    resource =
                        Data.resource (Just "version")
                            |> Data.withPipelineInstanceVars instanceVars
                in
                resource
                    |> Concourse.encodeResource
                    |> Json.Decode.decodeValue Concourse.decodeResource
                    |> Expect.equal (Ok resource)
        , test "build encoding/decoding are inverses" <|
            \_ ->
                let
                    build =
                        Data.jobBuild BuildStatus.BuildStatusPending
                            |> Data.withTeamName "t"
                            |> Data.withDuration
                                { startedAt =
                                    Just <| Time.millisToPosix 1000
                                , finishedAt =
                                    Just <| Time.millisToPosix 2000
                                }
                            |> Data.withJob
                                (Just
                                    (Data.jobId
                                        |> Data.withTeamName "t"
                                        |> Data.withPipelineInstanceVars instanceVars
                                    )
                                )
                in
                build
                    |> Concourse.encodeBuild
                    |> Json.Decode.decodeValue Concourse.decodeBuild
                    |> Expect.equal (Ok build)
        , test "pipeline encoding/decoding are inverses" <|
            \_ ->
                let
                    pipeline =
                        Data.pipeline "team" 1
                            |> Data.withInstanceVars instanceVars
                in
                pipeline
                    |> Concourse.encodePipeline
                    |> Json.Decode.decodeValue Concourse.decodePipeline
                    |> Expect.equal (Ok pipeline)
        , test "team encoding/decoding are inverses" <|
            \_ ->
                let
                    team =
                        { id = 1
                        , name = "team"
                        }
                in
                team
                    |> Concourse.encodeTeam
                    |> Json.Decode.decodeValue Concourse.decodeTeam
                    |> Expect.equal (Ok team)
        ]
