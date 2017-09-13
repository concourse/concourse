-- TODO: explicit exposing


module Concourse.Job exposing (..)

import Http
import HttpBuilder
import Task exposing (Task)
import Json.Decode
import Concourse


fetchJob : Concourse.JobIdentifier -> Task Http.Error Concourse.Job
fetchJob job =
    Http.toTask <|
        flip Http.get
            (Concourse.decodeJob { teamName = job.teamName, pipelineName = job.pipelineName })
            ("/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName)


fetchJobs : Concourse.PipelineIdentifier -> Task Http.Error (List Concourse.Job)
fetchJobs pi =
    Http.toTask <|
        flip Http.get
            (Json.Decode.list (Concourse.decodeJob pi))
            ("/api/v1/teams/" ++ pi.teamName ++ "/pipelines/" ++ pi.pipelineName ++ "/jobs")


fetchJobsWithTransitionBuilds : Concourse.PipelineIdentifier -> Task Http.Error (List Concourse.Job)
fetchJobsWithTransitionBuilds pi =
    Http.toTask <|
        flip Http.get
            (Json.Decode.list (Concourse.decodeJob pi))
            ("/api/v1/teams/" ++ pi.teamName ++ "/pipelines/" ++ pi.pipelineName ++ "/jobs" ++ "?include=transitionBuilds")


fetchJobsRaw : Concourse.PipelineIdentifier -> Task Http.Error Json.Decode.Value
fetchJobsRaw pi =
    Http.toTask <|
        flip Http.get
            Json.Decode.value
            ("/api/v1/teams/" ++ pi.teamName ++ "/pipelines/" ++ pi.pipelineName ++ "/jobs")


triggerBuild : Concourse.JobIdentifier -> Concourse.CSRFToken -> Task Http.Error Concourse.Build
triggerBuild job csrfToken =
    HttpBuilder.post ("/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName ++ "/builds")
        |> HttpBuilder.withHeader Concourse.csrfTokenHeaderName csrfToken
        |> HttpBuilder.withExpect (Http.expectJson (Concourse.decodeBuild))
        |> HttpBuilder.toTask


pause : Concourse.JobIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
pause =
    pauseUnpause True


unpause : Concourse.JobIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
unpause =
    pauseUnpause False


pauseUnpause : Bool -> Concourse.JobIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
pauseUnpause pause { teamName, pipelineName, jobName } csrfToken =
    let
        action =
            if pause then
                "pause"
            else
                "unpause"
    in
        Http.toTask <|
            Http.request
                { method = "PUT"
                , url = "/api/v1/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/jobs/" ++ jobName ++ "/" ++ action
                , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
                , body = Http.emptyBody
                , expect = Http.expectStringResponse (\_ -> Ok ())
                , timeout = Nothing
                , withCredentials = False
                }
