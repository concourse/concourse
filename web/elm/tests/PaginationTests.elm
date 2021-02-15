module PaginationTests exposing (all, responseWithHeaders)

import Ansi.Log
import Api.Pagination
import Array
import Concourse.Pagination exposing (Direction(..), Pagination)
import Dict exposing (Dict)
import Expect exposing (..)
import Http
import String
import Test exposing (..)


all : Test
all =
    describe "Pagination"
        [ describe "parsing Link headers"
            [ test "with no headers present" <|
                \_ ->
                    responseWithHeaders Dict.empty
                        |> Api.Pagination.parseLinks
                        |> Expect.equal
                            { previousPage = Nothing
                            , nextPage = Nothing
                            }
            , test "with a Link rel=\"previous\" present" <|
                \_ ->
                    responseWithHeaders withPreviousLink
                        |> Api.Pagination.parseLinks
                        |> Expect.equal
                            { previousPage =
                                Just { direction = From 1, limit = 2 }
                            , nextPage = Nothing
                            }
            , test "with a Link rel=\"next\" present" <|
                \_ ->
                    responseWithHeaders withNextLink
                        |> Api.Pagination.parseLinks
                        |> Expect.equal
                            { previousPage = Nothing
                            , nextPage =
                                Just { direction = To 1, limit = 2 }
                            }
            , test "with a Link rel=\"previous\" and a Link rel=\"next\" present" <|
                \_ ->
                    responseWithHeaders withPreviousAndNextLink
                        |> Api.Pagination.parseLinks
                        |> Expect.equal
                            { previousPage =
                                Just { direction = From 3, limit = 2 }
                            , nextPage =
                                Just { direction = To 1, limit = 4 }
                            }
            , test "with malformed link header" <|
                \_ ->
                    responseWithHeaders withMalformedLink
                        |> Api.Pagination.parseLinks
                        |> Expect.equal
                            { previousPage = Nothing
                            , nextPage = Nothing
                            }
            ]
        ]


responseWithHeaders : Dict String String -> Http.Response String
responseWithHeaders headers =
    { status = { code = 200, message = "OK" }
    , headers = headers
    , url = "https://example.com"
    , body = ""
    }


withPreviousLink : Dict String String
withPreviousLink =
    Dict.fromList
        [ ( "Link"
          , "<https://example.com/previous?from=1&limit=2>; rel=\"previous\""
          )
        ]


withNextLink : Dict String String
withNextLink =
    Dict.fromList
        [ ( "Link"
          , "<https://example.com/next?to=1&limit=2>; rel=\"next\""
          )
        ]


withPreviousAndNextLink : Dict String String
withPreviousAndNextLink =
    Dict.fromList
        [ ( "link"
          , "<https://example.com/previous?from=3&limit=2>; rel=\"previous\""
                ++ ", <https://example.com/next?to=1&limit=4>; rel=\"next\""
          )
        ]


withMalformedLink : Dict String String
withMalformedLink =
    Dict.fromList [ ( "Link", "banana" ) ]
