module JobTests exposing (..)

import ElmTest exposing (..)
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Pagination exposing (Direction(..))
import Job exposing (update, Action(..))
import Date
import Array
import Http
import Dict

all : Test
all =
  suite "Job"
    [ suite "update" <|
      let
        someJobInfo =
          { name = "some-job"
          , pipelineName = "some-pipeline"
          , teamName = "some-team"
          }
      in let
        someBuild =
          { id = 123
          , name = "45"
          , job = Just someJobInfo
          , status = Succeeded
          , duration =
            { startedAt = Just (Date.fromTime 0)
            , finishedAt = Just (Date.fromTime 0)
            }
          , reapTime = Just (Date.fromTime 0)
          }
      in let
        someJob =
          { name = "some-job"
          , pipelineName = "some-pipeline"
          , teamName = "some-team"
          , finishedBuild = Just someBuild
          , paused = False
          , disableManualTrigger = False
          }
        defaultModel =
          { jobInfo = someJobInfo
          , job = Nothing
          , pausedChanging = False
          , buildsWithResources = Nothing
          , now = 0
          , page = { direction = Concourse.Pagination.Since 0, limit = 100 }
          , pagination = { previousPage = Nothing, nextPage = Nothing }
          }
      in
        [ test "JobBuildsFetched" <|

          assertEqual
            { defaultModel
            | buildsWithResources = Just <| Array.fromList
                [ { buildWithResources = Nothing, nextBuild = someBuild } ]
            , page = { direction = Concourse.Pagination.Since 124, limit = 1 }
            } <|
            fst <|
              update
                (JobBuildsFetched <| Ok
                  { content = [ someBuild ]
                  , pagination = { previousPage = Nothing, nextPage = Nothing }
                  }
                )
                defaultModel
        , test "JobBuildsFetched error" <|
          assertEqual
            defaultModel <|
            fst <|
              update (JobBuildsFetched <| Err Http.NetworkError) defaultModel
      , test "JobFetched" <|
        assertEqual
          { defaultModel | job = (Just someJob)
          } <|
          fst <|
            update (JobFetched <| Ok someJob ) defaultModel
      , test "JobFetched error" <|
        assertEqual
          defaultModel <|
          fst <|
            update (JobFetched <| Err Http.NetworkError ) defaultModel
        , test "BuildResourcesFetched" <|
          let
            buildInput =
              { name = "some-input"
              , resource = "some-resource"
              , type' = "git"
              , version = Dict.fromList [ ("version", "v1") ]
              , metadata = [ { name = "some", value = "metadata" } ]
              , firstOccurrence = True
              }
            buildOutput =
              { resource = "some-resource"
              , version = Dict.fromList [ ("version", "v2") ]
              }
          in let
            buildResources = { inputs = [ buildInput ], outputs = [ buildOutput ] }
          in let
            fetchedBuildResources = { index = 1, result = Ok buildResources }
          in
          assertEqual
            { defaultModel | buildsWithResources = Just <| Array.fromList
              [ { buildWithResources = Nothing, nextBuild = someBuild }
              , { buildWithResources = Just {build = someBuild, resources = buildResources}, nextBuild = someBuild }
              ]
            } <|
            fst <|
              update (BuildResourcesFetched fetchedBuildResources)
                { defaultModel | buildsWithResources = Just <| Array.fromList
                  [ { buildWithResources = Nothing, nextBuild = someBuild }
                  , { buildWithResources = Nothing, nextBuild = someBuild }
                  ]
                }
        , test "BuildResourcesFetched error" <|
          assertEqual
            { defaultModel | buildsWithResources = Just <| Array.fromList
              [ { buildWithResources = Nothing, nextBuild = someBuild }
              , { buildWithResources = Nothing, nextBuild = someBuild }
              ]
            } <|
            fst <|
              update (BuildResourcesFetched { index = 1, result = Err Http.NetworkError })
                { defaultModel | buildsWithResources = Just <| Array.fromList
                  [ { buildWithResources = Nothing, nextBuild = someBuild }
                  , { buildWithResources = Nothing, nextBuild = someBuild }
                  ]
                }
        , test "TogglePaused" <|
          assertEqual
          { defaultModel | job = Just { someJob | paused = True }, pausedChanging = True } <|
          fst <|
            update TogglePaused { defaultModel | job = Just someJob }
        , test "PausedToggled" <|
          assertEqual
          { defaultModel | job = Just someJob, pausedChanging = False } <|
          fst <|
            update (PausedToggled <| Ok ()) { defaultModel | job = Just someJob }
        , test "PausedToggled error" <|
          assertEqual
          { defaultModel | job = Just someJob } <|
          fst <|
            update (PausedToggled <| Err Http.NetworkError) { defaultModel | job = Just someJob }
        , test "PausedToggled unauthorized" <|
          assertEqual
          { defaultModel | job = Just someJob } <|
          fst <|
            update (PausedToggled <| Err <| Http.BadResponse 401 "") { defaultModel | job = Just someJob }
        ]
    ]
