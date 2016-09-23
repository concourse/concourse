module Concourse.Login exposing (..)

import Http
import Task exposing (Task)

import Concourse

noAuth : String -> Task Http.Error Concourse.AuthToken
noAuth teamName =
  Http.get Concourse.decodeAuthToken ("/api/v1/teams/" ++ teamName ++ "/auth/token")
--
-- fetch : Concourse.BuildId -> Task Http.Error Concourse.Build
-- fetch buildId =
--   Http.get Concourse.decodeBuild ("/api/v1/builds/" ++ toString buildId)
--
-- fetchJobBuild : Concourse.JobBuildIdentifier -> Task Http.Error Concourse.Build
-- fetchJobBuild jbi =
--   let
--     url = "/api/v1/teams/" ++ jbi.teamName ++ "/pipelines/" ++ jbi.pipelineName ++ "/jobs/" ++ jbi.jobName ++ "/builds/" ++ jbi.buildName
--   in
--     Http.get Concourse.decodeBuild url
--
-- abort : Concourse.BuildId -> Task Http.Error ()
-- abort buildId =
--   let
--     post =
--       Http.send Http.defaultSettings
--         { verb = "POST"
--         , headers = []
--         , url = "/api/v1/builds/" ++ toString buildId ++ "/abort"
--         , body = Http.empty
--         }
--   in
--     Task.mapError promoteHttpError post `Task.andThen` handleResponse
--
-- fetchJobBuilds : Concourse.JobIdentifier -> Maybe Page -> Task Http.Error (Paginated Concourse.Build)
-- fetchJobBuilds job page =
--   let
--     url = "/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName ++ "/builds"
--   in
--     Concourse.Pagination.fetch Concourse.decodeBuild url page
--
-- url : Concourse.Build -> String
-- url build =
--   case build.job of
--     Nothing ->
--       "/builds/" ++ toString build.id
--
--     Just {jobName, teamName, pipelineName} ->
--       "/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/jobs/" ++ jobName ++ "/builds/" ++ build.name

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
