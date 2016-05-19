module Concourse.Job exposing (..)

import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

import Concourse.Build exposing (Build, BuildJob)

type alias Job =
  { name: String
  , pipelineName: String
  , teamName: String
  , finishedBuild: Maybe Build
  , paused: Bool
  , disableManualTrigger: Bool
  }

fetchJob : BuildJob -> Task Http.Error Job
fetchJob job =
  Http.get (decode job.teamName job.pipelineName) ("/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.name)

decode : String -> String -> Json.Decode.Decoder Job
decode teamName pipelineName =
  Json.Decode.object4 (init teamName pipelineName)
    ("name" := Json.Decode.string )
    (Json.Decode.maybe ("finished_build" := Concourse.Build.decode))
    (Json.Decode.maybe ("paused" := Json.Decode.bool))
    (Json.Decode.maybe ("disable_manual_trigger" := Json.Decode.bool))

init : String -> String -> String -> Maybe Build -> Maybe Bool -> Maybe Bool -> Job
init teamName pipelineName name finishedBuild maybePaused maybeDisableManualTrigger =
  { name = name
  , teamName = teamName
  , pipelineName = pipelineName
  , finishedBuild = finishedBuild
  , paused =
    case maybePaused of
      Nothing -> False
      Just paused -> paused
  , disableManualTrigger =
    case maybeDisableManualTrigger of
      Nothing -> False
      Just disableManualTrigger -> disableManualTrigger
  }

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
