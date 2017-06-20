module JobTests exposing (..)

import Test exposing (..)
import Expect exposing (..)
import Concourse exposing (BuildStatus(..), BuildId, Build, Job)
import Concourse.Pagination exposing (Direction(..))
import Job exposing (update, Msg(..))
import Date
import Array
import Http
import Dict


all : Test
all =
    describe "Job"
        [ describe "update" <|
            let
                someJobInfo =
                    { jobName = "some-job"
                    , pipelineName = "some-pipeline"
                    , teamName = "some-team"
                    }
            in
                let
                    someBuild : Build
                    someBuild =
                        { id = 123
                        , url = ""
                        , name = "45"
                        , job = Just someJobInfo
                        , status = BuildStatusSucceeded
                        , duration =
                            { startedAt = Just (Date.fromTime 0)
                            , finishedAt = Just (Date.fromTime 0)
                            }
                        , reapTime = Just (Date.fromTime 0)
                        }
                in
                    let
                        someJob : Concourse.Job
                        someJob =
                            { name = "some-job"
                            , pipeline = { pipelineName = "some-pipeline", teamName = "some-team" }
                            , url = ""
                            , nextBuild = Nothing
                            , finishedBuild = Just someBuild
                            , paused = False
                            , disableManualTrigger = False
                            , inputs = []
                            , outputs = []
                            , groups = []
                            }

                        defaultModel : Job.Model
                        defaultModel =
                            { ports = { title = (\_ -> Cmd.none) }
                            , jobIdentifier = someJobInfo
                            , job = Nothing
                            , pausedChanging = False
                            , buildsWithResources = { content = [], pagination = { previousPage = Nothing, nextPage = Nothing } }
                            , currentPage = Nothing -- { direction = Concourse.Pagination.Since 0, limit = 100 }
                            , now = 0
                            , csrfToken = ""
                            }
                    in
                        [ test "JobBuildsFetched" <|
                            \_ ->
                                let
                                    bwr =
                                        defaultModel.buildsWithResources
                                in
                                    Expect.equal
                                        { defaultModel
                                            | currentPage = Just { direction = Concourse.Pagination.Since 124, limit = 1 }
                                            , buildsWithResources = { bwr | content = [ { build = someBuild, resources = Nothing } ] }
                                        }
                                    <|
                                        Tuple.first <|
                                            update
                                                (JobBuildsFetched <|
                                                    Ok
                                                        { content = [ someBuild ]
                                                        , pagination = { previousPage = Nothing, nextPage = Nothing }
                                                        }
                                                )
                                                defaultModel
                        , test "JobBuildsFetched error" <|
                            \_ ->
                                Expect.equal
                                    defaultModel
                                <|
                                    Tuple.first <|
                                        update (JobBuildsFetched <| Err Http.NetworkError) defaultModel
                        , test "JobFetched" <|
                            \_ ->
                                Expect.equal
                                    { defaultModel
                                        | job = (Just someJob)
                                    }
                                <|
                                    Tuple.first <|
                                        update (JobFetched <| Ok someJob) defaultModel
                        , test "JobFetched error" <|
                            \_ ->
                                Expect.equal
                                    defaultModel
                                <|
                                    Tuple.first <|
                                        update (JobFetched <| Err Http.NetworkError) defaultModel
                        , test "BuildResourcesFetched" <|
                            \_ ->
                                let
                                    buildInput =
                                        { name = "some-input"
                                        , resource = "some-resource"
                                        , type_ = "git"
                                        , version = Dict.fromList [ ( "version", "v1" ) ]
                                        , metadata = [ { name = "some", value = "metadata" } ]
                                        , firstOccurrence = True
                                        }

                                    buildOutput =
                                        { resource = "some-resource"
                                        , version = Dict.fromList [ ( "version", "v2" ) ]
                                        }
                                in
                                    let
                                        buildResources =
                                            { inputs = [ buildInput ], outputs = [ buildOutput ] }
                                    in
                                        Expect.equal
                                            defaultModel
                                        <|
                                            Tuple.first <|
                                                update (BuildResourcesFetched 1 (Ok buildResources))
                                                    defaultModel
                        , test "BuildResourcesFetched error" <|
                            \_ ->
                                Expect.equal
                                    defaultModel
                                <|
                                    Tuple.first <|
                                        update (BuildResourcesFetched 1 (Err Http.NetworkError))
                                            defaultModel
                        , test "TogglePaused" <|
                            \_ ->
                                Expect.equal
                                    { defaultModel | job = Just { someJob | paused = True }, pausedChanging = True }
                                <|
                                    Tuple.first <|
                                        update TogglePaused { defaultModel | job = Just someJob }
                        , test "PausedToggled" <|
                            \_ ->
                                Expect.equal
                                    { defaultModel | job = Just someJob, pausedChanging = False }
                                <|
                                    Tuple.first <|
                                        update (PausedToggled <| Ok ()) { defaultModel | job = Just someJob }
                        , test "PausedToggled error" <|
                            \_ ->
                                Expect.equal
                                    { defaultModel | job = Just someJob }
                                <|
                                    Tuple.first <|
                                        update (PausedToggled <| Err Http.NetworkError) { defaultModel | job = Just someJob }
                        , test "PausedToggled unauthorized" <|
                            \_ ->
                                Expect.equal
                                    { defaultModel | job = Just someJob }
                                <|
                                    Tuple.first <|
                                        update (PausedToggled <| Err <| Http.BadStatus { url = "http://example.com", status = { code = 401, message = "" }, headers = Dict.empty, body = "" }) { defaultModel | job = Just someJob }
                        ]
        ]
