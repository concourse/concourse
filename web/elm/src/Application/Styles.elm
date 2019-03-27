module Application.Styles exposing (disableInteraction)

import Html
import Html.Attributes exposing (style)


disableInteraction : List (Html.Attribute msg)
disableInteraction =
    [ style "cursor" "default"
    , style "user-select" "none"
    , style "-ms-user-select" "none"
    , style "-moz-user-select" "none"
    , style "-khtml-user-select" "none"
    , style "-webkit-user-select" "none"
    , style "-webkit-touch-callout" "none"
    ]
