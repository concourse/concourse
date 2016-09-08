module Concourse.Job exposing (..) -- TODO

import Http
import Task exposing (Task)
import Json.Decode

import Concourse

fetchJob : Concourse.JobIdentifier -> Task Http.Error Concourse.Job
fetchJob job =
  Http.get (Concourse.decodeJob { teamName = job.teamName, pipelineName = job.pipelineName })
    ("/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName)

fetchJobs : Concourse.PipelineIdentifier -> Task Http.Error (List Concourse.Job)
fetchJobs pi =
  Http.get (Json.Decode.list (Concourse.decodeJob pi))
    ("/api/v1/teams/" ++ pi.teamName ++ "/pipelines/" ++ pi.pipelineName ++ "/jobs")

triggerBuild : Concourse.JobIdentifier -> Task Http.Error Concourse.Build
triggerBuild job =
  let
    url = "/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName ++ "/builds"
  in
    Http.post Concourse.decodeBuild url Http.empty

pause : Concourse.JobIdentifier -> Task Http.Error ()
pause =
  pauseUnpause True

unpause : Concourse.JobIdentifier -> Task Http.Error ()
unpause =
  pauseUnpause False

pauseUnpause : Bool -> Concourse.JobIdentifier -> Task Http.Error ()
pauseUnpause pause {teamName,pipelineName,jobName} =
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
        , url = "/api/v1/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/jobs/" ++ jobName ++ "/" ++ action
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
