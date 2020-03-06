module Network.Build exposing (abort)

import Concourse
import Http
import Task exposing (Task)


abort : Concourse.BuildId -> Concourse.CSRFToken -> Task Http.Error ()
abort buildId csrfToken =
    Http.toTask <|
        Http.request
            { method = "PUT"
            , url = "/api/v1/builds/" ++ String.fromInt buildId ++ "/abort"
            , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
            , body = Http.emptyBody
            , expect = Http.expectStringResponse (always (Ok ()))
            , timeout = Nothing
            , withCredentials = False
            }
