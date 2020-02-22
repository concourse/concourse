module SerializationTests exposing (all)

import Concourse
import Concourse.BuildStatus as BuildStatus
import Data
import Expect
import Json.Decode
import Test exposing (Test, describe, test)


all : Test
all =
    describe "type serialization/deserialization"
        [ test "job encoding/decoding are inverses" <|
            \_ ->
                let
                    job =
                        Data.job 1
                in
                job
                    |> Concourse.encodeJob
                    |> Json.Decode.decodeValue Concourse.decodeJob
                    |> Expect.equal (Ok job)
        , test "build encoding/decoding are inverses" <|
            \_ ->
                let
                    build =
                        Data.jobBuild BuildStatus.BuildStatusPending
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
