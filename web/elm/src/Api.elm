module Api exposing
    ( Request
    , expectJson
    , get
    , ignoreResponse
    , paginatedGet
    , post
    , put
    , request
    , withJsonBody
    )

import Api.Endpoints exposing (Endpoint, toString)
import Api.Pagination exposing (parsePagination)
import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Http
import Json.Decode exposing (Decoder)
import Json.Encode
import Task exposing (Task)
import Url.Builder


type alias Request a =
    { endpoint : Endpoint
    , query : List Url.Builder.QueryParameter
    , method : String
    , headers : List Http.Header
    , body : Http.Body
    , expect : Http.Expect a
    }


request : Request a -> Task Http.Error a
request { endpoint, method, headers, body, expect, query } =
    Http.request
        { method = method
        , headers = headers
        , url = endpoint |> toString query
        , body = body
        , expect = expect
        , timeout = Nothing
        , withCredentials = False
        }
        |> Http.toTask


get : Endpoint -> Request ()
get endpoint =
    { method = "GET"
    , headers = []
    , endpoint = endpoint
    , query = []
    , body = Http.emptyBody
    , expect = ignoreResponse
    }


paginatedGet : Endpoint -> Maybe Page -> Decoder a -> Request (Paginated a)
paginatedGet endpoint page decoder =
    { method = "GET"
    , headers = []
    , endpoint = endpoint
    , query = Api.Pagination.params page
    , body = Http.emptyBody
    , expect = Http.expectStringResponse (parsePagination decoder)
    }


post : Endpoint -> Concourse.CSRFToken -> Request ()
post endpoint csrfToken =
    { method = "POST"
    , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
    , endpoint = endpoint
    , query = []
    , body = Http.emptyBody
    , expect = ignoreResponse
    }


put : Endpoint -> Concourse.CSRFToken -> Request ()
put endpoint csrfToken =
    { method = "PUT"
    , headers = [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
    , endpoint = endpoint
    , query = []
    , body = Http.emptyBody
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


withJsonBody : Json.Encode.Value -> Request a -> Request a
withJsonBody value r =
    { r | body = Http.jsonBody value }


ignoreResponse : Http.Expect ()
ignoreResponse =
    Http.expectStringResponse <| always <| Ok ()
