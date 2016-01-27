
module DictView where

import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class)

view : Dict String String -> Html
view dict =
  Html.table [class "dictionary"] <|
    List.map viewPair (Dict.toList dict)

viewPair : (String, String) -> Html
viewPair (name, value) =
  Html.tr []
  [ Html.td [class "dict-key"] [Html.text name]
  , Html.td [class "dict-value"]
  [ Html.pre [] [ Html.text value ] ] ]
