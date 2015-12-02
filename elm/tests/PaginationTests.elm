module PaginationTests where

import Array
import Dict exposing (Dict)
import ElmTest exposing (..)
import Focus
import Regex
import String
import Ansi.Log
import Http

import Pagination exposing (Pagination)

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
              (Pagination.parse (responseWithHeaders Dict.empty))
        , let
            headers =
              Dict.fromList
                [ ("Link", "<https://example.com/previous>; rel=\"previous\"")
                ]
          in
            test "with a Link rel=\"previous\" present" <|
              assertEqual
                (Pagination (Just "https://example.com/previous") Nothing)
                (Pagination.parse (responseWithHeaders headers))
        , let
            headers =
              Dict.fromList
                [ ("Link", "<https://example.com/next>; rel=\"next\"")
                ]
          in
            test "with a Link rel=\"next\" present" <|
              assertEqual
                (Pagination Nothing (Just "https://example.com/next"))
                (Pagination.parse (responseWithHeaders headers))
        , let
            headers =
              Dict.fromList
                [ ("Link", "<https://example.com/previous>; rel=\"previous\", <https://example.com/next>; rel=\"next\"")
                ]
          in
            test "with a Link rel=\"previous\" and a Link rel=\"next\" present" <|
              assertEqual
                (Pagination (Just "https://example.com/previous") (Just "https://example.com/next"))
                (Pagination.parse (responseWithHeaders headers))
        ]
    ]
