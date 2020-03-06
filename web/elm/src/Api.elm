module Api exposing (Method(..), get, paginatedGet, request)

import Api.Endpoints exposing (Endpoint, toUrl)
import Concourse.Pagination exposing (Paginated)
import Http
import Json.Decode exposing (Decoder)
import Network.Pagination exposing (parsePagination)
import Task exposing (Task)


type Method
    = Get


methodToString : Method -> String
methodToString m =
    case m of
        Get ->
            "GET"


request :
    { endpoint : Endpoint
    , method : Method
    , headers : List Http.Header
    , body : Http.Body
    , expect : Http.Expect a
    }
    -> Task Http.Error a
request { endpoint, method, headers, body, expect } =
    Http.request
        { method = methodToString method
        , headers = headers
        , url = toUrl endpoint
        , body = body
        , expect = expect
        , timeout = Nothing
        , withCredentials = False
        }
        |> Http.toTask


get : Endpoint -> Decoder a -> Task Http.Error a
get endpoint decoder =
    request
        { method = Get
        , headers = []
        , endpoint = endpoint
        , body = Http.emptyBody
        , expect = Http.expectJson decoder
        }


paginatedGet : Endpoint -> Decoder a -> Task Http.Error (Paginated a)
paginatedGet endpoint decoder =
    request
        { method = Get
        , headers = []
        , endpoint = endpoint
        , body = Http.emptyBody
        , expect = Http.expectStringResponse (parsePagination decoder)
        }
