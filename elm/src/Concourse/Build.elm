module Concourse.Build exposing (..)

import Date exposing (Date)
import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

import Concourse.Pagination exposing (Paginated, Page)
import Concourse.BuildStatus exposing (BuildStatus)

type alias Build =
  { id : BuildId
  , url : String
  , name : String
  , job : Maybe BuildJob
  , status : BuildStatus
  , duration : BuildDuration
  , reapTime : Maybe Date
  }

type alias BuildId =
  Int

type alias BuildJob =
  { name : String
  , teamName : String
  , pipelineName : String
  }

type alias BuildDuration =
  { startedAt : Maybe Date
  , finishedAt : Maybe Date
  }

fetch : BuildId -> Task Http.Error Build
fetch buildId =
  Http.get decode ("/api/v1/builds/" ++ toString buildId)

abort : BuildId -> Task Http.Error ()
abort buildId =
  let
    post =
      Http.send Http.defaultSettings
        { verb = "POST"
        , headers = []
        , url = "/api/v1/builds/" ++ toString buildId ++ "/abort"
        , body = Http.empty
        }
  in
    Task.mapError promoteHttpError post `Task.andThen` handleResponse

fetchJobBuilds : BuildJob -> Maybe Page -> Task Http.Error (Paginated Build)
fetchJobBuilds job page =
  let
    url = "/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.name ++ "/builds"
  in
    Concourse.Pagination.fetch decode url page

url : Build -> String
url build =
  case build.job of
    Nothing ->
      "/builds/" ++ toString build.id

    Just {name, teamName, pipelineName} ->
      "/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/jobs/" ++ name ++ "/builds/" ++ build.name

decode : Json.Decode.Decoder Build
decode =
  Json.Decode.object7 Build
    ("id" := Json.Decode.int)
    ("url" := Json.Decode.string)
    ("name" := Json.Decode.string)
    (Json.Decode.maybe (Json.Decode.object3 BuildJob
      ("job_name" := Json.Decode.string)
      ("team_name" := Json.Decode.string)
      ("pipeline_name" := Json.Decode.string)))
    ("status" := Concourse.BuildStatus.decode)
    (Json.Decode.object2 BuildDuration
      (Json.Decode.maybe ("start_time" := (Json.Decode.map dateFromSeconds Json.Decode.float)))
      (Json.Decode.maybe ("end_time" := (Json.Decode.map dateFromSeconds Json.Decode.float))))
    (Json.Decode.maybe ("reap_time" := (Json.Decode.map dateFromSeconds Json.Decode.float)))

handleResponse : Http.Response -> Task Http.Error ()
handleResponse response =
  if 200 <= response.status && response.status < 300 then
    Task.succeed ()
  else
    Task.fail (Http.BadResponse response.status response.statusText)

promoteHttpError : Http.RawError -> Http.Error
promoteHttpError rawError =
  case rawError of
    Http.RawTimeout -> Http.Timeout
    Http.RawNetworkError -> Http.NetworkError

dateFromSeconds : Float -> Date
dateFromSeconds = Date.fromTime << ((*) 1000)
