module PaginationTests exposing (..)

import Array
import Dict exposing (Dict)
import ElmTest exposing (..)
import Focus
import Regex
import String
import Ansi.Log
import Http

import Concourse.Pagination exposing (Pagination, Direction(..))

responseWithHeaders : Dict String String -> Http.Response
responseWithHeaders headers =
  { status = 200
  , statusText = "OK"
  , headers = headers
  , url = "https://example.com"
  , value = Http.Text ""
  }

all : Test
all =
  suite "Pagination"
    [ suite "parsing Link headers"
        [ test "with no headers present" <|
            assertEqual
              (Pagination Nothing Nothing)
              (Concourse.Pagination.parseLinks (responseWithHeaders Dict.empty))
        , let
            headers =
              Dict.fromList
                [ ("Link", "<https://example.com/previous?until=1&limit=2>; rel=\"previous\"")
                ]
          in
            test "with a Link rel=\"previous\" present" <|
              assertEqual
                (Pagination (Just { direction = Until 1, limit = 2 }) Nothing)
                (Concourse.Pagination.parseLinks (responseWithHeaders headers))
        , let
            headers =
              Dict.fromList
                [ ("Link", "<https://example.com/next?since=1&limit=2>; rel=\"next\"")
                ]
          in
            test "with a Link rel=\"next\" present" <|
              assertEqual
                (Pagination Nothing (Just { direction = Since 1, limit = 2 }))
                (Concourse.Pagination.parseLinks (responseWithHeaders headers))
        , let
            headers =
              Dict.fromList
                [ ("Link", "<https://example.com/previous?until=1&limit=2>; rel=\"previous\", <https://example.com/next?since=3&limit=4>; rel=\"next\"")
                ]
          in
            test "with a Link rel=\"previous\" and a Link rel=\"next\" present" <|
              assertEqual
                (Pagination (Just { direction = Until 1, limit = 2 }) (Just { direction = Since 3, limit = 4 }))
                (Concourse.Pagination.parseLinks (responseWithHeaders headers))
        ]
    ]
