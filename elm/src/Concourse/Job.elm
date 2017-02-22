-- TODO: explicit exposing


module Concourse.Job exposing (..)

import Http
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


fetchJobsRaw : Concourse.PipelineIdentifier -> Task Http.Error Json.Decode.Value
fetchJobsRaw pi =
    Http.toTask <|
        flip Http.get
            Json.Decode.value
            ("/api/v1/teams/" ++ pi.teamName ++ "/pipelines/" ++ pi.pipelineName ++ "/jobs")


triggerBuild : Concourse.JobIdentifier -> Task Http.Error Concourse.Build
triggerBuild job =
    let
        url =
            "/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName ++ "/builds"
    in
        Http.toTask <| Http.post url Http.emptyBody Concourse.decodeBuild


pause : Concourse.JobIdentifier -> Task Http.Error ()
pause =
    pauseUnpause True


unpause : Concourse.JobIdentifier -> Task Http.Error ()
unpause =
    pauseUnpause False


pauseUnpause : Bool -> Concourse.JobIdentifier -> Task Http.Error ()
pauseUnpause pause { teamName, pipelineName, jobName } =
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
                , headers = []
                , body = Http.emptyBody
                , expect = Http.expectStringResponse (\_ -> Ok ())
                , timeout = Nothing
                , withCredentials = False
                }
