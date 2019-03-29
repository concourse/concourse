module Views.DictView exposing (view)

import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class)


view : List (Html.Attribute a) -> Dict String (Html a) -> Html a
view attrs dict =
    Html.table (class "dictionary" :: attrs) <|
        List.map viewPair (Dict.toList dict)


viewPair : ( String, Html a ) -> Html a
viewPair ( name, value ) =
    Html.tr []
        [ Html.td [ class "dict-key" ] [ Html.text name ]
        , Html.td [ class "dict-value" ] [ value ]
        ]
