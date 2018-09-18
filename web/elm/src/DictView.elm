module DictView exposing (view)

import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class)


view : Dict String (Html a) -> Html a
view dict =
    Html.table [ class "dictionary" ] <|
        List.map viewPair (Dict.toList dict)


viewPair : ( String, Html a ) -> Html a
viewPair ( name, value ) =
    Html.tr []
        [ Html.td [ class "dict-key" ] [ Html.text name ]
        , Html.td [ class "dict-value" ] [ value ]
        ]
