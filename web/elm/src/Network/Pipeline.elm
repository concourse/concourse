module Network.Pipeline exposing
    ( changeVisibility
    , order
    , togglePause
    )

import Concourse
import Http
import Json.Encode
import Message.Message exposing (VisibilityAction(..))
import Task exposing (Task)


order : String -> List String -> Concourse.CSRFToken -> Task Http.Error ()
order teamName pipelineNames csrfToken =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url = "/api/v1/teams/" ++ teamName ++ "/pipelines/ordering"
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , expect = Http.expectStringResponse (always (Ok ()))
            , body = Http.jsonBody (Json.Encode.list Json.Encode.string pipelineNames)
            , timeout = Nothing
            , withCredentials = False
            }


togglePause :
    Bool
    -> String
    -> String
    -> Concourse.CSRFToken
    -> Task Http.Error ()
togglePause isPaused =
    if isPaused then
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


changeVisibility :
    VisibilityAction
    -> String
    -> String
    -> Concourse.CSRFToken
    -> Task Http.Error ()
changeVisibility action teamName pipelineName csrfToken =
    let
        endpoint =
            case action of
                Hide ->
                    "/hide"

                Expose ->
                    "/expose"
    in
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url =
                "/api/v1/teams/"
                    ++ teamName
                    ++ "/pipelines/"
                    ++ pipelineName
                    ++ endpoint
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , expect = Http.expectStringResponse (always (Ok ()))
            , body = Http.emptyBody
            , timeout = Nothing
            , withCredentials = False
            }
