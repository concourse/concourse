module Concourse.Pipeline exposing (fetchPipeline, fetchPipelines, pause, unpause, order)

import Http
import Json.Encode
import Json.Decode
import Task exposing (Task)
import Concourse


order : String -> List String -> Task Http.Error ()
order teamName pipelineNames =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url = "/api/v1/teams/" ++ teamName ++ "/pipelines/ordering"
            , headers = []
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


pause : String -> String -> Task Http.Error ()
pause =
    putAction "pause"


unpause : String -> String -> Task Http.Error ()
unpause =
    putAction "unpause"


putAction : String -> String -> String -> Task Http.Error ()
putAction action teamName pipelineName =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url = "/api/v1/teams/" ++ teamName ++ "/pipelines/" ++ pipelineName ++ "/" ++ action
            , headers = []
            , expect = Http.expectStringResponse (always (Ok ()))
            , body = Http.emptyBody
            , timeout = Nothing
            , withCredentials = False
            }
