module Concourse.Pipeline exposing (fetchPipeline, fetchPipelines, pause, unpause, order)

import Http
import Json.Encode
import Json.Decode
import Task exposing (Task)
import Concourse


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


pause : String -> String -> Concourse.CSRFToken -> Task Http.Error ()
pause =
    putAction "pause"


unpause : String -> String -> Concourse.CSRFToken -> Task Http.Error ()
unpause =
    putAction "unpause"


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
