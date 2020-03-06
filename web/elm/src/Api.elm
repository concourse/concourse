module Api exposing (get, paginatedGet)

import Api.Endpoints exposing (Endpoint, toUrl)
import Concourse.Pagination exposing (Paginated)
import Http
import Json.Decode exposing (Decoder)
import Network.Pagination exposing (parsePagination)
import Task exposing (Task)


get : Endpoint -> Decoder a -> Task Http.Error a
get endpoint decoder =
    Http.request
        { method = "GET"
        , headers = []
        , url = toUrl endpoint
        , body = Http.emptyBody
        , expect = Http.expectJson decoder
        , timeout = Nothing
        , withCredentials = False
        }
        |> Http.toTask


paginatedGet : Endpoint -> Decoder a -> Task Http.Error (Paginated a)
paginatedGet endpoint decoder =
    Http.request
        { method = "GET"
        , headers = []
        , url = toUrl endpoint
        , body = Http.emptyBody
        , expect = Http.expectStringResponse (parsePagination decoder)
        , timeout = Nothing
        , withCredentials = False
        }
        |> Http.toTask
