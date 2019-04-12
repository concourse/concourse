module PaginationTests exposing (all, responseWithHeaders)

import Ansi.Log
import Array
import Concourse.Pagination exposing (Direction(..), Pagination)
import Dict exposing (Dict)
import Expect exposing (..)
import Http
import Network.Pagination
import String
import Test exposing (..)


all : Test
all =
    describe "Pagination"
        [ describe "parsing Link headers"
            [ test "with no headers present" <|
                \_ ->
                    responseWithHeaders Dict.empty
                        |> Network.Pagination.parseLinks
                        |> Expect.equal
                            { previousPage = Nothing
                            , nextPage = Nothing
                            }
            , test "with a Link rel=\"previous\" present" <|
                \_ ->
                    responseWithHeaders withPreviousLink
                        |> Network.Pagination.parseLinks
                        |> Expect.equal
                            { previousPage =
                                Just { direction = Until 1, limit = 2 }
                            , nextPage = Nothing
                            }
            , test "with a Link rel=\"next\" present" <|
                \_ ->
                    responseWithHeaders withNextLink
                        |> Network.Pagination.parseLinks
                        |> Expect.equal
                            { previousPage = Nothing
                            , nextPage =
                                Just { direction = Since 1, limit = 2 }
                            }
            , test "with a Link rel=\"previous\" and a Link rel=\"next\" present" <|
                \_ ->
                    responseWithHeaders withPreviousAndNextLink
                        |> Network.Pagination.parseLinks
                        |> Expect.equal
                            { previousPage =
                                Just { direction = Until 1, limit = 2 }
                            , nextPage =
                                Just { direction = Since 3, limit = 4 }
                            }
            , test "with malformed link header" <|
                \_ ->
                    responseWithHeaders withMalformedLink
                        |> Network.Pagination.parseLinks
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
          , "<https://example.com/previous?until=1&limit=2>; rel=\"previous\""
          )
        ]


withNextLink : Dict String String
withNextLink =
    Dict.fromList
        [ ( "Link"
          , "<https://example.com/next?since=1&limit=2>; rel=\"next\""
          )
        ]


withPreviousAndNextLink : Dict String String
withPreviousAndNextLink =
    Dict.fromList
        [ ( "link"
          , "<https://example.com/previous?until=1&limit=2>; rel=\"previous\""
                ++ ", <https://example.com/next?since=3&limit=4>; rel=\"next\""
          )
        ]


withMalformedLink : Dict String String
withMalformedLink =
    Dict.fromList [ ( "Link", "banana" ) ]
