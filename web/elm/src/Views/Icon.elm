module Views.Icon exposing (icon)

import Html exposing (Html)
import Html.Attributes exposing (style)


icon :
    { sizePx : Int, image : String }
    -> List (Html.Attribute msg)
    -> Html msg
icon { sizePx, image } attrs =
    (\a -> Html.div a []) <|
        [ style "background-image" ("url(/public/images/" ++ image ++ ")")
        , style "height" (String.fromInt sizePx ++ "px")
        , style "width" (String.fromInt sizePx ++ "px")
        , style "background-position" "50% 50%"
        , style "background-repeat" "no-repeat"
        ]
            ++ attrs
