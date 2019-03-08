module Network.Pipeline exposing (fetchPipeline, fetchPipelines, order, togglePause)

import Concourse
import Concourse.PipelineStatus
import Http
import Json.Decode
import Json.Encode
import Task exposing (Task)


order : String -> List String -> Concourse.CSRFToken -> Task Http.Error ()
order teamName pipelineNames csrfToken =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url = "/api/v1/teams/" ++ teamName ++ "/pipelines/ordering"
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , expect = Http.expectStringResponse (always (Ok ()))
            , body = Http.jsonBody (Json.Encode.list (List.map Json.Encode.string pipelineNames))
            , timeout = Nothing
            , withCredentials = False
            }


fetchPipeline : Concourse.PipelineIdentifier -> Task Http.Error Concourse.Pipeline
fetchPipeline { teamName, pipelineName } =
    Http.toTask <|
        Http.get
            ("/api/v1/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName)
            Concourse.decodePipeline


fetchPipelines : Task Http.Error (List Concourse.Pipeline)
fetchPipelines =
    Http.toTask <|
        Http.get
            "/api/v1/pipelines"
            (Json.Decode.list Concourse.decodePipeline)


togglePause :
    Concourse.PipelineStatus.PipelineStatus
    -> String
    -> String
    -> Concourse.CSRFToken
    -> Task Http.Error ()
togglePause status =
    if status == Concourse.PipelineStatus.PipelineStatusPaused then
        putAction "unpause"

    else
        putAction "pause"


putAction : String -> String -> String -> Concourse.CSRFToken -> Task Http.Error ()
putAction action teamName pipelineName csrfToken =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url = "/api/v1/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/" ++ action
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , expect = Http.expectStringResponse (always (Ok ()))
            , body = Http.emptyBody
            , timeout = Nothing
            , withCredentials = False
            }
