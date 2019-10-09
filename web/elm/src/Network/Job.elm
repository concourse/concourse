module Network.Job exposing
    ( fetchAllJobs
    , fetchJob
    , fetchJobs
    , fetchJobsRaw
    , pause
    , pauseUnpause
    , rerunJobBuild
    , triggerBuild
    , unpause
    )

import Concourse
import Http
import HttpBuilder
import Json.Decode
import Task exposing (Task)


fetchJob : Concourse.JobIdentifier -> Task Http.Error Concourse.Job
fetchJob job =
    Http.toTask <|
        Http.get ("/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName) (Concourse.decodeJob { teamName = job.teamName, pipelineName = job.pipelineName })


fetchJobs : Concourse.PipelineIdentifier -> Task Http.Error (List Concourse.Job)
fetchJobs pi =
    Http.toTask <|
        Http.get ("/api/v1/teams/" ++ pi.teamName ++ "/pipelines/" ++ pi.pipelineName ++ "/jobs") (Json.Decode.list (Concourse.decodeJob pi))


fetchAllJobs : Task Http.Error (Maybe (List Concourse.Job))
fetchAllJobs =
    Http.toTask <|
        Http.get "/api/v1/jobs" (Json.Decode.nullable <| Json.Decode.list (Concourse.decodeJob { teamName = "", pipelineName = "" }))


fetchJobsRaw : Concourse.PipelineIdentifier -> Task Http.Error Json.Decode.Value
fetchJobsRaw pi =
    Http.toTask <|
        Http.get ("/api/v1/teams/" ++ pi.teamName ++ "/pipelines/" ++ pi.pipelineName ++ "/jobs") Json.Decode.value


triggerBuild : Concourse.JobIdentifier -> Concourse.CSRFToken -> Task Http.Error Concourse.Build
triggerBuild job csrfToken =
    HttpBuilder.post ("/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName ++ "/builds")
        |> HttpBuilder.withHeader Concourse.csrfTokenHeaderName csrfToken
        |> HttpBuilder.withExpect (Http.expectJson Concourse.decodeBuild)
        |> HttpBuilder.toTask


rerunJobBuild :
    Concourse.JobBuildIdentifier
    -> Concourse.CSRFToken
    -> Task Http.Error Concourse.Build
rerunJobBuild jbi csrfToken =
    HttpBuilder.post
        ("/api/v1/teams/"
            ++ jbi.teamName
            ++ "/pipelines/"
            ++ jbi.pipelineName
            ++ "/jobs/"
            ++ jbi.jobName
            ++ "/builds/"
            ++ jbi.buildName
        )
        |> HttpBuilder.withHeader Concourse.csrfTokenHeaderName csrfToken
        |> HttpBuilder.withExpect (Http.expectJson Concourse.decodeBuild)
        |> HttpBuilder.toTask


pause : Concourse.JobIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
pause =
    pauseUnpause True


unpause : Concourse.JobIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
unpause =
    pauseUnpause False


pauseUnpause : Bool -> Concourse.JobIdentifier -> Concourse.CSRFToken -> Task Http.Error ()
pauseUnpause shouldPause { teamName, pipelineName, jobName } csrfToken =
    let
        action =
            if shouldPause then
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
