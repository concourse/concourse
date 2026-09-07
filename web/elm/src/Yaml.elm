module Yaml exposing (fromJson)

import Json.Decode
import Json.Encode


{-| Renders an arbitrary JSON value as block-style YAML, for displaying
`user_data` -- which the API hands back verbatim, so it can be any shape.

Keys are emitted in the order they arrive rather than re-sorted here. Note
that this is not the order the pipeline author wrote: `user_data` is an
`any` on the Go side, so encoding/json has already sorted the map keys
alphabetically by the time we see them.

-}
fromJson : Json.Decode.Value -> String
fromJson value =
    String.join "\n" (lines value)


type Classified
    = Object (List ( String, Json.Decode.Value ))
    | Array (List Json.Decode.Value)
    | Scalar String
    | Block (List String)


classify : Json.Decode.Value -> Classified
classify value =
    -- arrays before objects: an array would otherwise be ambiguous with an
    -- object depending on how the decoder treats JS arrays
    case Json.Decode.decodeValue (Json.Decode.list Json.Decode.value) value of
        Ok items ->
            Array items

        Err _ ->
            case Json.Decode.decodeValue (Json.Decode.keyValuePairs Json.Decode.value) value of
                Ok kvs ->
                    Object kvs

                Err _ ->
                    case Json.Decode.decodeValue Json.Decode.string value of
                        Ok s ->
                            if String.contains "\n" s then
                                Block (blockLines s)

                            else
                                Scalar (quote s)

                        Err _ ->
                            Scalar (nonStringScalar value)


{-| Lines for a value at zero indentation; callers indent nested blocks.
-}
lines : Json.Decode.Value -> List String
lines value =
    case classify value of
        Object [] ->
            [ "{}" ]

        Object kvs ->
            List.concatMap objectEntry kvs

        Array [] ->
            [ "[]" ]

        Array items ->
            List.concatMap arrayEntry items

        Scalar s ->
            [ s ]

        Block body ->
            "|-" :: List.map (indent "  ") body


objectEntry : ( String, Json.Decode.Value ) -> List String
objectEntry ( key, value ) =
    case classify value of
        Scalar s ->
            [ quote key ++ ": " ++ s ]

        Object [] ->
            [ quote key ++ ": {}" ]

        Array [] ->
            [ quote key ++ ": []" ]

        Block body ->
            (quote key ++ ": |-") :: List.map (indent "  ") body

        _ ->
            (quote key ++ ":") :: List.map (indent "  ") (lines value)


arrayEntry : Json.Decode.Value -> List String
arrayEntry value =
    case lines value of
        [] ->
            []

        first :: rest ->
            ("- " ++ first) :: List.map (indent "  ") rest


{-| `|-` strips trailing newlines, so drop the empty lines they'd produce.
-}
blockLines : String -> List String
blockLines s =
    s
        |> String.lines
        |> List.foldr
            (\line acc ->
                if List.isEmpty acc && String.isEmpty (String.trim line) then
                    acc

                else
                    line :: acc
            )
            []


indent : String -> String -> String
indent prefix line =
    if String.isEmpty line then
        line

    else
        prefix ++ line


nonStringScalar : Json.Decode.Value -> String
nonStringScalar value =
    case Json.Decode.decodeValue Json.Decode.bool value of
        Ok True ->
            "true"

        Ok False ->
            "false"

        Err _ ->
            case Json.Decode.decodeValue Json.Decode.float value of
                Ok f ->
                    String.fromFloat f

                Err _ ->
                    -- null, and anything else the decoders above don't cover
                    Json.Encode.encode 0 value


quote : String -> String
quote s =
    if needsQuoting s then
        "\"" ++ escape s ++ "\""

    else
        s


{-| Conservative: quote anything that could be read back as another type or
as YAML syntax rather than as a plain string.
-}
needsQuoting : String -> Bool
needsQuoting s =
    String.isEmpty s
        || (s /= String.trim s)
        || List.member (String.toLower s)
            [ "true", "false", "null", "yes", "no", "on", "off", "~" ]
        || (String.toFloat s /= Nothing)
        || String.contains "\t" s
        || String.contains ": " s
        || String.contains " #" s
        || String.endsWith ":" s
        || (case String.uncons s of
                Just ( c, _ ) ->
                    List.member c indicatorChars

                Nothing ->
                    True
           )


indicatorChars : List Char
indicatorChars =
    [ '-', '?', ':', ',', '[', ']', '{', '}', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`' ]


escape : String -> String
escape =
    String.replace "\\" "\\\\" >> String.replace "\"" "\\\""
