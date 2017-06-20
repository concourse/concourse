module PaginationTests exposing (..)

import Array
import Dict exposing (Dict)
import Test exposing (..)
import Expect exposing (..)
import Focus
import Regex
import String
import Ansi.Log
import Http
import Concourse.Pagination exposing (Pagination, Direction(..))


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
                        (Concourse.Pagination.parseLinks (responseWithHeaders Dict.empty))
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
                            (Concourse.Pagination.parseLinks (responseWithHeaders headers))
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
                            (Concourse.Pagination.parseLinks (responseWithHeaders headers))
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
                            (Concourse.Pagination.parseLinks (responseWithHeaders headers))
            ]
        ]
