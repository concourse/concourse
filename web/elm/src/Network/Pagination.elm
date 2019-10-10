module Network.Pagination exposing (fetch, parseLinks)

import Concourse.Pagination
    exposing
        ( Direction(..)
        , Page
        , Paginated
        , Pagination
        )
import Dict
import Http
import Json.Decode
import List.Extra
import Maybe.Extra exposing (orElse)
import Parser
    exposing
        ( (|.)
        , (|=)
        , Parser
        , backtrackable
        , chompWhile
        , getChompedString
        , keyword
        , map
        , oneOf
        , run
        , spaces
        , succeed
        , symbol
        )
import String
import Task exposing (Task)
import Url
import Url.Builder
import Url.Parser exposing (parse, query)
import Url.Parser.Query as Query


fetch :
    Json.Decode.Decoder a
    -> List String
    -> Maybe Page
    -> Task Http.Error (Paginated a)
fetch decoder segments p =
    Http.toTask <|
        Http.request
            { method = "GET"
            , headers = []
            , url = Url.Builder.absolute segments (params p)
            , body = Http.emptyBody
            , expect = Http.expectStringResponse (parsePagination decoder)
            , timeout = Nothing
            , withCredentials = False
            }


params : Maybe Page -> List Url.Builder.QueryParameter
params p =
    case p of
        Just { direction, limit } ->
            [ Url.Builder.int "limit" limit
            , case direction of
                Since since ->
                    Url.Builder.int "since" since

                Until until ->
                    Url.Builder.int "until" until

                From from ->
                    Url.Builder.int "from" from

                To to ->
                    Url.Builder.int "to" to
            ]

        Nothing ->
            []


parsePagination :
    Json.Decode.Decoder a
    -> Http.Response String
    -> Result String (Paginated a)
parsePagination decoder response =
    response.body
        |> Json.Decode.decodeString (Json.Decode.list decoder)
        |> Result.mapError Json.Decode.errorToString
        |> Result.map
            (\content ->
                { content = content, pagination = parseLinks response }
            )


parseLinks : Http.Response String -> Pagination
parseLinks =
    .headers
        >> Dict.toList
        >> List.Extra.find (Tuple.first >> String.toLower >> (==) "link")
        >> Maybe.map Tuple.second
        >> Maybe.andThen (run pagination >> Result.toMaybe)
        >> Maybe.withDefault { previousPage = Nothing, nextPage = Nothing }


pagination : Parser Pagination
pagination =
    let
        entry rel =
            backtrackable <|
                succeed parsePage
                    |. symbol "<"
                    |= getChompedString (chompWhile <| (/=) '>')
                    |. symbol ">"
                    |. symbol ";"
                    |. spaces
                    |. keyword "rel"
                    |. symbol "="
                    |. symbol "\""
                    |. keyword rel
                    |. symbol "\""
    in
    oneOf
        [ succeed (\p n -> { previousPage = p, nextPage = n })
            |= entry previousRel
            |. symbol ","
            |. spaces
            |= entry nextRel
        , succeed (\n p -> { previousPage = p, nextPage = n })
            |= entry nextRel
            |. symbol ","
            |. spaces
            |= entry previousRel
        , succeed (\p -> { previousPage = p, nextPage = Nothing })
            |= entry previousRel
        , succeed (\n -> { previousPage = Nothing, nextPage = n })
            |= entry nextRel
        ]


previousRel : String
previousRel =
    "previous"


nextRel : String
nextRel =
    "next"


parsePage : String -> Maybe Page
parsePage url =
    let
        tryParam param =
            url
                |> Url.fromString
                -- for some reason, the `query` function returns parsers that
                -- only work when the path is empty. This is probably a bug:
                -- https://github.com/elm/url/issues/17
                |> Maybe.map (\u -> { u | path = "" })
                |> Maybe.andThen (parse <| query <| Query.int param)
                |> Maybe.withDefault Nothing

        tryDirection dir =
            tryParam
                >> Maybe.map
                    (\n ->
                        { direction = dir n
                        , limit = tryParam "limit" |> Maybe.withDefault 0
                        }
                    )
    in
    tryDirection Until "until"
        |> orElse (tryDirection Since "since")
        |> orElse (tryDirection From "from")
        |> orElse (tryDirection To "to")
