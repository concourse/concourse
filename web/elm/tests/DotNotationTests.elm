module DotNotationTests exposing (expandTest, flattenTest, parseTest, serializeTest)

import Concourse exposing (JsonValue(..))
import Dict
import DotNotation exposing (expand, flatten, parse, serialize)
import Expect
import Json.Encode
import Test exposing (Test, describe, test)


flattenTest : Test
flattenTest =
    describe "flatten"
        [ test "simple" <|
            \_ ->
                flatten (Dict.fromList [ ( "key", JsonString "value" ) ])
                    |> Expect.equal [ { path = "key", fields = [], value = JsonString "value" } ]
        , test "nested JSON" <|
            \_ ->
                flatten
                    (Dict.fromList
                        [ ( "key1"
                          , JsonObject
                                [ ( "foo", JsonNumber 1 )
                                , ( "bar"
                                  , JsonObject
                                        [ ( "baz", JsonNumber 2 )
                                        , ( "qux", JsonNumber 3 )
                                        ]
                                  )
                                ]
                          )
                        , ( "key2", JsonNumber 4 )
                        ]
                    )
                    |> Expect.equal
                        [ { path = "key1", fields = [ "foo" ], value = JsonNumber 1 }
                        , { path = "key1", fields = [ "bar", "baz" ], value = JsonNumber 2 }
                        , { path = "key1", fields = [ "bar", "qux" ], value = JsonNumber 3 }
                        , { path = "key2", fields = [], value = JsonNumber 4 }
                        ]
        ]


expandTest : Test
expandTest =
    describe "expand"
        [ test "flat key-value pairs" <|
            \_ ->
                expand
                    [ { path = "key1", fields = [], value = JsonString "value1" }
                    , { path = "key2", fields = [], value = JsonString "value2" }
                    ]
                    |> Expect.equal
                        (Dict.fromList
                            [ ( "key1", JsonString "value1" )
                            , ( "key2", JsonString "value2" )
                            ]
                        )
        , test "nested key-value pairs" <|
            \_ ->
                expand
                    [ { path = "key", fields = [ "foo", "bar", "baz" ], value = JsonString "value1" }
                    , { path = "key", fields = [ "foo", "qux" ], value = JsonString "value2" }
                    ]
                    |> Expect.equal
                        (Dict.fromList
                            [ ( "key"
                              , JsonObject
                                    [ ( "foo"
                                      , JsonObject
                                            [ ( "bar", JsonObject [ ( "baz", JsonString "value1" ) ] )
                                            , ( "qux", JsonString "value2" )
                                            ]
                                      )
                                    ]
                              )
                            ]
                        )
        , test "sorts field names" <|
            \_ ->
                expand
                    [ { path = "key", fields = [ "banana" ], value = JsonString "value1" }
                    , { path = "key", fields = [ "apple" ], value = JsonString "value2" }
                    ]
                    |> Expect.equal
                        (Dict.fromList
                            [ ( "key"
                              , JsonObject
                                    [ ( "apple", JsonString "value2" )
                                    , ( "banana", JsonString "value1" )
                                    ]
                              )
                            ]
                        )
        , test "replacing values" <|
            \_ ->
                expand
                    [ { path = "key", fields = [ "field" ], value = JsonString "value1" }
                    , { path = "key", fields = [ "field" ], value = JsonObject [ ( "subfield1", JsonNumber 1 ) ] }
                    , { path = "key", fields = [ "field", "subfield2" ], value = JsonNumber 2 }
                    ]
                    |> Expect.equal
                        (Dict.fromList
                            [ ( "key"
                              , JsonObject
                                    [ ( "field"
                                      , JsonObject
                                            [ ( "subfield1", JsonNumber 1 )
                                            , ( "subfield2", JsonNumber 2 )
                                            ]
                                      )
                                    ]
                              )
                            ]
                        )
        ]


parseTest : Test
parseTest =
    describe "parse"
        [ test "parses simple key-value pair" <|
            \_ ->
                parse "key=\"value\""
                    |> Expect.equal
                        (Ok { path = "key", fields = [], value = JsonString "value" })
        , test "parses simple key-value pair with dot notation" <|
            \_ ->
                parse "key.field1.field2=\"value\""
                    |> Expect.equal
                        (Ok { path = "key", fields = [ "field1", "field2" ], value = JsonString "value" })
        , test "allows quoted path segments with special characters" <|
            \_ ->
                parse "\"my.key\".\"field=1\".\"field 2 \"=\"value\""
                    |> Expect.equal
                        (Ok { path = "my.key", fields = [ "field=1", "field 2 " ], value = JsonString "value" })
        , test "parses arbitrary JSON" <|
            \_ ->
                parse "key={\"hello\": 1,\"foo\": true}"
                    |> Expect.equal
                        (Ok
                            { path = "key"
                            , fields = []
                            , value =
                                JsonObject
                                    [ ( "foo", JsonRaw (Json.Encode.bool True) )
                                    , ( "hello", JsonNumber 1 )
                                    ]
                            }
                        )
        , test "errors when missing key" <|
            \_ ->
                parse "=1" |> Expect.err
        , test "errors when given empty path segment" <|
            \_ ->
                parse "\"\"=1" |> Expect.err
        , test "errors when given empty path segment in subfield" <|
            \_ ->
                parse "hello.\"\"=1" |> Expect.err
        , test "errors when missing value" <|
            \_ ->
                parse "no_colon_here" |> Expect.err
        , test "errors when quoted key is never terminated" <|
            \_ ->
                parse "\"where's the end=1" |> Expect.err
        , test "errors when JSON value is malformed" <|
            \_ ->
                parse "hello={" |> Expect.err
        ]


serializeTest : Test
serializeTest =
    describe "toString"
        [ test "serializes basic key-value pairs" <|
            \_ ->
                serialize { path = "key", fields = [], value = JsonString "value" }
                    |> Expect.equal ( "key", "\"value\"" )
        , test "with fields" <|
            \_ ->
                serialize { path = "key", fields = [ "field1", "field2" ], value = JsonString "value" }
                    |> Expect.equal ( "key.field1.field2", "\"value\"" )
        , test "quotes as necessary" <|
            \_ ->
                serialize { path = "my.key", fields = [ "field=1", " field 2 " ], value = JsonString "value" }
                    |> Expect.equal ( "\"my.key\".\"field=1\".\" field 2 \"", "\"value\"" )
        , test "encodes value as JSON" <|
            \_ ->
                serialize
                    { path = "key"
                    , fields = []
                    , value =
                        JsonObject
                            [ ( "field1", JsonNumber 1 )
                            , ( "field2", JsonString "foo" )
                            ]
                    }
                    |> Tuple.second
                    |> Expect.equal "{\"field1\":1,\"field2\":\"foo\"}"
        ]
