module Concourse.Build exposing (..)

import Http
import Task exposing (Task)
import Concourse
import Concourse.Pagination exposing (Paginated, Page)


fetch : Concourse.BuildId -> Task Http.Error Concourse.Build
fetch buildId =
    Http.toTask <| Http.get ("/api/v1/builds/" ++ toString buildId) Concourse.decodeBuild


fetchJobBuild : Concourse.JobBuildIdentifier -> Task Http.Error Concourse.Build
fetchJobBuild jbi =
    let
        url =
            "/api/v1/teams/" ++ jbi.teamName ++ "/pipelines/" ++ jbi.pipelineName ++ "/jobs/" ++ jbi.jobName ++ "/builds/" ++ jbi.buildName
    in
        Http.toTask <| Http.get url Concourse.decodeBuild


abort : Concourse.BuildId -> Concourse.CSRFToken -> Task Http.Error ()
abort buildId csrfToken =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url = "/api/v1/builds/" ++ toString buildId ++ "/abort"
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , body = Http.emptyBody
            , expect = Http.expectStringResponse (always (Ok ()))
            , timeout = Nothing
            , withCredentials = False
            }


fetchJobBuilds : Concourse.JobIdentifier -> Maybe Page -> Task Http.Error (Paginated Concourse.Build)
fetchJobBuilds job page =
    let
        url =
            "/api/v1/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName ++ "/builds"
    in
        Concourse.Pagination.fetch Concourse.decodeBuild url page


url : Concourse.Build -> String
url build =
    case build.job of
        Nothing ->
            "/builds/" ++ toString build.id

        Just { jobName, teamName, pipelineName } ->
            "/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/jobs/" ++ jobName ++ "/builds/" ++ build.name
