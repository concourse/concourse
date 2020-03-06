module Network.Job exposing
    ( pause
    , pauseUnpause
    , rerunJobBuild
    , triggerBuild
    , unpause
    )

import Concourse
import Http
import HttpBuilder
import Task exposing (Task)


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
