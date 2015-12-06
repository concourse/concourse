module Concourse.Build where

import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

import Concourse.Pagination exposing (Paginated, Page)
import Concourse.BuildStatus exposing (BuildStatus)

type alias Build =
  { id : BuildId
  , name : String
  , job : Maybe Job
  , status : BuildStatus
  }

type alias BuildId =
  Int

type alias Job =
  { name : String
  , pipelineName : String
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

fetchJobBuilds : Job -> Maybe Page -> Task Http.Error (Paginated Build)
fetchJobBuilds job page =
  let
    url = "/api/v1/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.name ++ "/builds"
  in
    Concourse.Pagination.fetch decode url page

url : Build -> String
url build =
  case build.job of
    Nothing ->
      "/builds/" ++ toString build.id

    Just {name, pipelineName} ->
      "/pipelines/" ++ pipelineName ++ "/jobs/" ++ name ++ "/builds/" ++ build.name

decode : Json.Decode.Decoder Build
decode =
  Json.Decode.object4 Build
    ("id" := Json.Decode.int)
    ("name" := Json.Decode.string)
    (Json.Decode.maybe (Json.Decode.object2 Job
      ("job_name" := Json.Decode.string)
      ("pipeline_name" := Json.Decode.string)))
    ("status" := Concourse.BuildStatus.decode)

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

