module Concourse.Job exposing (..)

import Http
import Json.Decode exposing ((:=))
import Json.Decode.Extra exposing ((|:))
import Task exposing (Task)

import Concourse.Build exposing (Build, BuildJob)

type alias Job =
  { teamName : String
  , pipelineName : String
  , name : String
  , url : String
  , nextBuild : Maybe Build
  , finishedBuild : Maybe Build
  , paused : Bool
  , disableManualTrigger : Bool
  , inputs : List Input
  , outputs : List Output
  , groups : List String
  }

type alias Input =
  { name : String
  , resource : String
  , passed : List String
  , trigger : Bool
  }

type alias Output =
  { name : String
  , resource : String
  }

type alias PipelineLocator =
  { teamName : String
  , pipelineName : String
  }

fetchJob : BuildJob -> Task Http.Error Job
fetchJob job =
  Http.get (decode job.teamName job.pipelineName) ("/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.name)

fetchJobs : PipelineLocator -> Task Http.Error (List Job)
fetchJobs {teamName, pipelineName} =
  Http.get (Json.Decode.list (decode teamName pipelineName))
    ("/api/v1/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/jobs")

optional : a -> Json.Decode.Decoder (Maybe a) -> Json.Decode.Decoder a
optional default =
  Json.Decode.map (Maybe.withDefault default)

decode : String -> String -> Json.Decode.Decoder Job
decode teamName pipelineName =
  Json.Decode.succeed (Job teamName pipelineName)
    |: ("name" := Json.Decode.string)
    |: ("url" := Json.Decode.string)
    |: (Json.Decode.maybe ("next_build" := Concourse.Build.decode))
    |: (Json.Decode.maybe ("finished_build" := Concourse.Build.decode))
    |: (optional False <| Json.Decode.maybe ("paused" := Json.Decode.bool))
    |: (optional False <| Json.Decode.maybe ("disable_manual_trigger" := Json.Decode.bool))
    |: (optional [] <| Json.Decode.maybe ("inputs" := Json.Decode.list decodeInput))
    |: (optional [] <| Json.Decode.maybe ("outputs" := Json.Decode.list decodeOutput))
    |: (optional [] <| Json.Decode.maybe ("groups" := Json.Decode.list Json.Decode.string))

decodeInput : Json.Decode.Decoder Input
decodeInput =
  Json.Decode.object4 Input
    ("name" := Json.Decode.string)
    ("resource" := Json.Decode.string)
    (optional [] <| Json.Decode.maybe ("passed" := Json.Decode.list Json.Decode.string))
    (optional False <| Json.Decode.maybe ("trigger" := Json.Decode.bool))

decodeOutput : Json.Decode.Decoder Output
decodeOutput =
  Json.Decode.object2 Output
    ("name" := Json.Decode.string)
    ("resource" := Json.Decode.string)

pause : BuildJob -> Task Http.Error ()
pause jobInfo = pauseUnpause True jobInfo

unpause : BuildJob -> Task Http.Error ()
unpause jobInfo = pauseUnpause False jobInfo

pauseUnpause : Bool -> BuildJob -> Task Http.Error ()
pauseUnpause pause jobInfo =
  let
    action =
      if pause
        then  "pause"
        else  "unpause"
  in let
    put =
      Http.send Http.defaultSettings
        { verb = "PUT"
        , headers = []
        , url = "/api/v1/teams/" ++ jobInfo.teamName ++ "/pipelines/" ++ jobInfo.pipelineName ++ "/jobs/" ++ jobInfo.name ++ "/" ++ action
        , body = Http.empty
        }
  in
    Task.mapError promoteHttpError put `Task.andThen` handleResponse

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
