module DotNotation exposing (expand, flatten, parse, serialize)

import Concourse exposing (JsonValue(..), decodeJsonValue, encodeJsonValue)
import Dict exposing (Dict)
import Json.Decode
import Json.Encode
import Parser
    exposing
        ( (|.)
        , (|=)
        , DeadEnd
        , Parser
        , Problem(..)
        , Trailing(..)
        , andThen
        , chompUntil
        , chompWhile
        , getChompedString
        , getOffset
        , getSource
        , map
        , oneOf
        , problem
        , sequence
        , spaces
        , succeed
        , symbol
        )


type alias DotNotation =
    { path : PathSegment
    , fields : List PathSegment
    , value : JsonValue
    }


type alias PathSegment =
    String



-- Flattening


flatten : Dict String JsonValue -> List DotNotation
flatten d =
    Dict.toList d |> List.concatMap (\( path, val ) -> flattenHelper path [] val)


flattenHelper : PathSegment -> List PathSegment -> JsonValue -> List DotNotation
flattenHelper path fields value =
    case value of
        JsonObject kvPairs ->
            kvPairs |> List.concatMap (\( k, v ) -> flattenHelper path (fields ++ [ k ]) v)

        _ ->
            [ { path = path, fields = fields, value = value } ]



-- Expanding


expand : List DotNotation -> Dict String JsonValue
expand =
    List.foldl upsert Dict.empty


upsert : DotNotation -> Dict String JsonValue -> Dict String JsonValue
upsert { path, fields, value } =
    Dict.update path
        (\v ->
            case v of
                Just (JsonObject kvPairs) ->
                    case fields of
                        field :: rest ->
                            Dict.fromList kvPairs
                                |> upsert { path = field, fields = rest, value = value }
                                |> Dict.toList
                                |> List.sortBy Tuple.first
                                |> JsonObject
                                |> Just

                        [] ->
                            Just value

                _ ->
                    Just <| constructValue (List.reverse fields) value
        )


constructValue : List String -> JsonValue -> JsonValue
constructValue revFields value =
    case revFields of
        lastField :: rest ->
            let
                leaf =
                    JsonObject [ ( lastField, value ) ]
            in
            constructValue rest leaf

        [] ->
            value



-- Parsing


parse : String -> Result String DotNotation
parse =
    Parser.run parser >> Result.mapError deadEndsToString


parser : Parser DotNotation
parser =
    succeed DotNotation
        |= pathSegment
        |= oneOf
            [ symbol "=" |> map (always [])
            , sequence
                { start = "."
                , separator = "."
                , end = "="
                , spaces = spaces
                , item = pathSegment
                , trailing = Forbidden
                }
            ]
        |= jsonValue


pathSegment : Parser PathSegment
pathSegment =
    oneOf
        [ quotedPathSegment
        , unquotedPathSegment
        ]
        |> andThen
            (\s ->
                if s == "" then
                    problem "Path segment must not be empty"

                else
                    succeed s
            )


quotedPathSegment : Parser PathSegment
quotedPathSegment =
    getChompedString
        (symbol "\""
            |. chompUntil "\""
            |. symbol "\""
        )
        |> map trimQuotes


trimQuotes : String -> String
trimQuotes =
    String.slice 1 -1


unquotedPathSegment : Parser PathSegment
unquotedPathSegment =
    getChompedString <|
        chompWhile isValidPathSegmentChar


isValidPathSegmentChar : Char -> Bool
isValidPathSegmentChar c =
    (c /= '=') && (c /= '.') && (not <| isSpace c)


isSpace : Char -> Bool
isSpace c =
    (c == ' ') || (c == '\n') || (c == '\t') || (c == '\u{000D}')


jsonValue : Parser JsonValue
jsonValue =
    succeed String.dropLeft
        |= getOffset
        |= getSource
        |> andThen
            (\s ->
                case Json.Decode.decodeString decodeJsonValue s of
                    Ok v ->
                        succeed v

                    Err err ->
                        problem <| Json.Decode.errorToString err
            )



-- Serializing


serialize : DotNotation -> ( String, String )
serialize { path, fields, value } =
    let
        k =
            path
                :: fields
                |> List.map quoteIfNeeded
                |> String.join "."

        v =
            Json.Encode.encode 0 (encodeJsonValue value)
    in
    ( k, v )


quoteIfNeeded : String -> String
quoteIfNeeded s =
    if String.all isValidPathSegmentChar s then
        s

    else
        "\"" ++ s ++ "\""



------------------------------------------------------
-- Taken from https://github.com/elm/parser/pull/16 --
------------------------------------------------------


deadEndsToString : List DeadEnd -> String
deadEndsToString deadEnds =
    String.concat (List.intersperse "; " (List.map deadEndToString deadEnds))


deadEndToString : DeadEnd -> String
deadEndToString deadend =
    problemToString deadend.problem ++ " at row " ++ String.fromInt deadend.row ++ ", col " ++ String.fromInt deadend.col


problemToString : Problem -> String
problemToString p =
    case p of
        Expecting s ->
            "expecting '" ++ s ++ "'"

        ExpectingInt ->
            "expecting int"

        ExpectingHex ->
            "expecting hex"

        ExpectingOctal ->
            "expecting octal"

        ExpectingBinary ->
            "expecting binary"

        ExpectingFloat ->
            "expecting float"

        ExpectingNumber ->
            "expecting number"

        ExpectingVariable ->
            "expecting variable"

        ExpectingSymbol s ->
            "expecting symbol '" ++ s ++ "'"

        ExpectingKeyword s ->
            "expecting keyword '" ++ s ++ "'"

        ExpectingEnd ->
            "expecting end"

        UnexpectedChar ->
            "unexpected char"

        Problem s ->
            "problem " ++ s

        BadRepeat ->
            "bad repeat"
