module Views.Icon exposing (icon)

import Assets
import Html exposing (Html)
import Html.Attributes exposing (style)


icon :
    { sizePx : Int, image : Assets.Asset }
    -> List (Html.Attribute msg)
    -> Html msg
icon { sizePx, image } attrs =
    Html.div
        ([ style "background-image" <|
            Assets.backgroundImage <|
                Just image
         , style "height" (String.fromInt sizePx ++ "px")
         , style "width" (String.fromInt sizePx ++ "px")
         , style "background-position" "50% 50%"
         , style "background-repeat" "no-repeat"
         , style "background-size" "cover"
         ]
            ++ attrs
        )
        []
