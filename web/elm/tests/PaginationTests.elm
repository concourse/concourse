module PaginationTests exposing (all, responseWithHeaders)

import Ansi.Log
import Array
import Concourse.Pagination exposing (Direction(..), Pagination)
import Dict exposing (Dict)
import Expect exposing (..)
import Http
import Network.Pagination
import Regex
import String
import Test exposing (..)


responseWithHeaders : Dict String String -> Http.Response String
responseWithHeaders headers =
    { status = { code = 200, message = "OK" }
    , headers = headers
    , url = "https://example.com"
    , body = ""
    }


all : Test
all =
    describe "Pagination"
        [ describe "parsing Link headers"
            [ test "with no headers present" <|
                \_ ->
                    Expect.equal
                        (Pagination Nothing Nothing)
                        (Network.Pagination.parseLinks (responseWithHeaders Dict.empty))
            , let
                headers =
                    Dict.fromList
                        [ ( "Link", "<https://example.com/previous?until=1&limit=2>; rel=\"previous\"" )
                        ]
              in
              test "with a Link rel=\"previous\" present" <|
                \_ ->
                    Expect.equal
                        (Pagination (Just { direction = Until 1, limit = 2 }) Nothing)
                        (Network.Pagination.parseLinks (responseWithHeaders headers))
            , let
                headers =
                    Dict.fromList
                        [ ( "Link", "<https://example.com/next?since=1&limit=2>; rel=\"next\"" )
                        ]
              in
              test "with a Link rel=\"next\" present" <|
                \_ ->
                    Expect.equal
                        (Pagination Nothing (Just { direction = Since 1, limit = 2 }))
                        (Network.Pagination.parseLinks (responseWithHeaders headers))
            , let
                headers =
                    Dict.fromList
                        [ ( "Link", "<https://example.com/previous?until=1&limit=2>; rel=\"previous\", <https://example.com/next?since=3&limit=4>; rel=\"next\"" )
                        ]
              in
              test "with a Link rel=\"previous\" and a Link rel=\"next\" present" <|
                \_ ->
                    Expect.equal
                        (Pagination (Just { direction = Until 1, limit = 2 }) (Just { direction = Since 3, limit = 4 }))
                        (Network.Pagination.parseLinks (responseWithHeaders headers))
            ]
        ]
