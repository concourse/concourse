module TagTests exposing (all, tag, white)

import Dashboard.Group.Tag as Tag
import Expect exposing (Expectation)
import Fuzz
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (style)


tag : Fuzz.Fuzzer Tag.Tag
tag =
    Fuzz.frequency
        [ ( 1, Fuzz.constant Tag.Owner )
        , ( 1, Fuzz.constant Tag.Member )
        , ( 1, Fuzz.constant Tag.PipelineOperator )
        , ( 1, Fuzz.constant Tag.Viewer )
        ]


white : String
white =
    "#ffffff"


all : Test
all =
    describe "dashboard team role pill" <|
        let
            hasStyle : List ( String, String ) -> Tag.Tag -> Expectation
            hasStyle styles =
                Tag.view False
                    >> Query.fromHtml
                    >> Query.has (List.map (\( k, v ) -> style k v) styles)
        in
        [ fuzz tag "has a white border" <|
            hasStyle [ ( "border", "1px solid " ++ white ) ]
        , fuzz tag "is an inline-block" <|
            hasStyle [ ( "display", "inline-block" ) ]
        , fuzz tag "has very small font" <|
            hasStyle [ ( "font-size", "0.7em" ) ]
        , fuzz tag "letters are spaced apart" <|
            hasStyle [ ( "letter-spacing", "0.2em" ) ]
        , fuzz tag "has a bit of padding above and below" <|
            hasStyle
                [ ( "padding", "0.5em" )
                , ( "line-height", "0.9em" )
                ]
        , fuzz tag "text is horizontally centered in the white box" <|
            hasStyle [ ( "text-align", "center" ) ]
        ]
