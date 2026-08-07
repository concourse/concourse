module YamlTests exposing (all)

import Expect
import Json.Decode
import Test exposing (Test, describe, test)
import Yaml


{-| Parsing from a JSON string keeps these cases readable and exercises the
same path the API response takes.
-}
render : String -> String
render json =
    case Json.Decode.decodeString Json.Decode.value json of
        Ok value ->
            Yaml.fromJson value

        Err _ ->
            "<invalid json in test>"


all : Test
all =
    describe "Yaml.fromJson"
        [ describe "scalars"
            [ test "string" <|
                \_ -> render "\"hello\"" |> Expect.equal "hello"
            , test "integer renders without a decimal point" <|
                \_ -> render "42" |> Expect.equal "42"
            , test "float" <|
                \_ -> render "1.5" |> Expect.equal "1.5"
            , test "bool" <|
                \_ -> render "true" |> Expect.equal "true"
            , test "null" <|
                \_ -> render "null" |> Expect.equal "null"
            ]
        , describe "quoting"
            [ test "quotes a string that would otherwise read as a bool" <|
                \_ -> render "\"true\"" |> Expect.equal "\"true\""
            , test "quotes a string that would otherwise read as a number" <|
                \_ -> render "\"42\"" |> Expect.equal "\"42\""
            , test "quotes the empty string" <|
                \_ -> render "\"\"" |> Expect.equal "\"\""
            , test "quotes a string with leading whitespace" <|
                \_ -> render "\" padded\"" |> Expect.equal "\" padded\""
            , test "quotes a string starting with an indicator character" <|
                \_ -> render "\"- item\"" |> Expect.equal "\"- item\""
            , test "quotes a string containing a key separator" <|
                \_ -> render "\"a: b\"" |> Expect.equal "\"a: b\""
            , test "quotes a string containing a comment marker" <|
                \_ -> render "\"a #b\"" |> Expect.equal "\"a #b\""
            , test "escapes embedded quotes and backslashes once quoting kicks in" <|
                \_ ->
                    -- leading '-' forces quoting; the quotes and backslash
                    -- inside then have to be escaped
                    render "\"- say \\\"hi\\\" \\\\ here\""
                        |> Expect.equal "\"- say \\\"hi\\\" \\\\ here\""
            , test "leaves interior quotes alone in a plain scalar" <|
                \_ ->
                    render "\"say \\\"hi\\\" here\""
                        |> Expect.equal "say \"hi\" here"
            , test "leaves an ordinary string unquoted" <|
                \_ -> render "\"just words\"" |> Expect.equal "just words"
            ]
        , describe "objects"
            [ test "emits keys in the order received rather than re-sorting" <|
                \_ ->
                    render "{\"zebra\": 1, \"apple\": 2, \"mango\": 3}"
                        |> Expect.equal "zebra: 1\napple: 2\nmango: 3"
            , test "empty object" <|
                \_ -> render "{}" |> Expect.equal "{}"
            , test "nested object is indented under its key" <|
                \_ ->
                    render "{\"outer\": {\"inner\": \"v\"}}"
                        |> Expect.equal "outer:\n  inner: v"
            , test "empty nested object stays inline" <|
                \_ ->
                    render "{\"outer\": {}}" |> Expect.equal "outer: {}"
            , test "quotes a key that needs it" <|
                \_ ->
                    render "{\"a: b\": 1}" |> Expect.equal "\"a: b\": 1"
            ]
        , describe "arrays"
            [ test "list of scalars" <|
                \_ ->
                    render "[1, 2, 3]" |> Expect.equal "- 1\n- 2\n- 3"
            , test "empty array" <|
                \_ -> render "[]" |> Expect.equal "[]"
            , test "empty array under a key stays inline" <|
                \_ -> render "{\"a\": []}" |> Expect.equal "a: []"
            , test "array of objects" <|
                \_ ->
                    render "[{\"a\": 1, \"b\": 2}, {\"a\": 3, \"b\": 4}]"
                        |> Expect.equal "- a: 1\n  b: 2\n- a: 3\n  b: 4"
            , test "array under a key" <|
                \_ ->
                    render "{\"items\": [\"x\", \"y\"]}"
                        |> Expect.equal "items:\n  - x\n  - y"
            , test "nested arrays" <|
                \_ ->
                    render "[[1, 2], [3]]"
                        |> Expect.equal "- - 1\n  - 2\n- - 3"
            ]
        , describe "multi-line strings"
            [ test "renders as a literal block scalar" <|
                \_ ->
                    render "{\"notes\": \"line one\\nline two\"}"
                        |> Expect.equal "notes: |-\n  line one\n  line two"
            , test "drops the trailing blank lines the block scalar would strip" <|
                \_ ->
                    render "{\"notes\": \"only line\\n\"}"
                        |> Expect.equal "notes: |-\n  only line"
            , test "at the top level" <|
                \_ ->
                    render "\"a\\nb\"" |> Expect.equal "|-\n  a\n  b"
            ]
        , test "a realistic user_data object" <|
            \_ ->
                render
                    """
                    { "owner": "platform-team"
                    , "slack": "#platform"
                    , "runbooks": ["https://example.com/a", "https://example.com/b"]
                    , "tier": 1
                    , "oncall": {"primary": "alice", "secondary": "bob"}
                    }
                    """
                    |> Expect.equal
                        (String.join "\n"
                            [ "owner: platform-team"
                            , "slack: \"#platform\""
                            , "runbooks:"
                            , "  - https://example.com/a"
                            , "  - https://example.com/b"
                            , "tier: 1"
                            , "oncall:"
                            , "  primary: alice"
                            , "  secondary: bob"
                            ]
                        )
        ]
