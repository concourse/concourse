module Concourse.Pipeline exposing (Pipeline, fetchPipelines, pause, unpause, order)

import Http
import Json.Decode exposing ((:=))
import Json.Encode
import Task exposing (Task)

type alias Pipeline =
  { name : String
  , url : String
  , paused : Bool
  , public : Bool
  , teamName : String
  }

order : String -> List String -> Task Http.Error ()
order teamName pipelineNames =
  let jsonifiedPipelineNames =
    List.map Json.Encode.string pipelineNames
  in let
    body = Json.Encode.encode 0 <| Json.Encode.list jsonifiedPipelineNames
  in let
    post =
      Http.send Http.defaultSettings
        { verb = "PUT"
        , headers = []
        , url = "/api/v1/teams/" ++ teamName ++ "/pipelines/ordering"
        , body = Http.string body
        }
  in
    Task.mapError promoteHttpError post `Task.andThen` handleResponse

fetchPipelines : Task Http.Error (List Pipeline)
fetchPipelines = Http.get (Json.Decode.list decode) "/api/v1/pipelines"

pause : String -> String -> Task Http.Error ()
pause = putAction "pause"

unpause : String -> String -> Task Http.Error ()
unpause = putAction "unpause"

putAction : String -> String -> String -> Task Http.Error ()
putAction action teamName pipelineName =
  let
    post =
      Http.send Http.defaultSettings
        { verb = "PUT"
        , headers = []
        , url = "/api/v1/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/" ++ action
        , body = Http.empty
        }
  in
    Task.mapError promoteHttpError post `Task.andThen` handleResponse

promoteHttpError : Http.RawError -> Http.Error
promoteHttpError rawError =
  case rawError of
    Http.RawTimeout -> Http.Timeout
    Http.RawNetworkError -> Http.NetworkError

handleResponse : Http.Response -> Task Http.Error ()
handleResponse response =
  if 200 <= response.status && response.status < 300 then
    Task.succeed ()
  else
    Task.fail (Http.BadResponse response.status response.statusText)

decode : Json.Decode.Decoder Pipeline
decode =
  Json.Decode.object5 Pipeline
    ("name" := Json.Decode.string)
    ("url" := Json.Decode.string)
    ("paused" := Json.Decode.bool)
    ("public" := Json.Decode.bool)
    ("team_name" := Json.Decode.string)
