module Api exposing (get)

import Api.Endpoints exposing (Endpoint, toUrl)
import Http
import Json.Decode exposing (Decoder)
import Task exposing (Task)


get : Endpoint -> Decoder a -> Task Http.Error a
get endpoint decoder =
    Http.request
        { method = "GET"
        , url = toUrl endpoint
        , body = Http.emptyBody
        , headers = []
        , expect = Http.expectJson decoder
        , timeout = Nothing
        , withCredentials = False
        }
        |> Http.toTask
