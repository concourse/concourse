module Api exposing
    ( Method(..)
    , Request
    , expectJson
    , get
    , ignoreResponse
    , paginatedGet
    , post
    , request
    )

import Api.Endpoints exposing (Endpoint, toPath)
import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Http
import Json.Decode exposing (Decoder)
import Network.Pagination exposing (parsePagination)
import Task exposing (Task)
import Url.Builder


type Method
    = Get
    | Post


type alias Request a =
    { endpoint : Endpoint
    , query : List Url.Builder.QueryParameter
    , method : Method
    , headers : List Http.Header
    , body : Http.Body
    , expect : Http.Expect a
    }


methodToString : Method -> String
methodToString m =
    case m of
        Get ->
            "GET"

        Post ->
            "POST"


request : Request a -> Task Http.Error a
request { endpoint, method, headers, body, expect, query } =
    Http.request
        { method = methodToString method
        , headers = headers
        , url = Url.Builder.absolute (toPath endpoint) query
        , body = body
        , expect = expect
        , timeout = Nothing
        , withCredentials = False
        }
        |> Http.toTask


get : Endpoint -> Request ()
get endpoint =
    { method = Get
    , headers = []
    , endpoint = endpoint
    , query = []
    , body = Http.emptyBody
    , expect = ignoreResponse
    }


paginatedGet : Endpoint -> Maybe Page -> Decoder a -> Request (Paginated a)
paginatedGet endpoint page decoder =
    { method = Get
    , headers = []
    , endpoint = endpoint
    , query = Network.Pagination.params page
    , body = Http.emptyBody
    , expect = Http.expectStringResponse (parsePagination decoder)
    }


post : Endpoint -> Concourse.CSRFToken -> Http.Body -> Request ()
post endpoint csrfToken body =
    { method = Post
    , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
    , endpoint = endpoint
    , query = []
    , body = body
    , expect = ignoreResponse
    }


expectJson : Decoder b -> Request a -> Request b
expectJson decoder r =
    { method = r.method
    , headers = r.headers
    , endpoint = r.endpoint
    , query = r.query
    , body = r.body
    , expect = Http.expectJson decoder
    }


ignoreResponse : Http.Expect ()
ignoreResponse =
    Http.expectStringResponse <| always <| Ok ()
