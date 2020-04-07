module Views.Icon exposing (icon, iconWithTooltip)

import Assets
import Html exposing (Html)
import Html.Attributes exposing (style)


icon :
    { sizePx : Int, image : Assets.Asset }
    -> List (Html.Attribute msg)
    -> Html msg
icon { sizePx, image } attrs =
    iconWithTooltip { sizePx = sizePx, image = image } attrs []


iconWithTooltip :
    { sizePx : Int, image : Assets.Asset }
    -> List (Html.Attribute msg)
    -> List (Html msg)
    -> Html msg
iconWithTooltip { sizePx, image } attrs tooltipContent =
    Html.div
        ([ style "background-image" <|
            Assets.backgroundImage <|
                Just image
         , style "height" (String.fromInt sizePx ++ "px")
         , style "width" (String.fromInt sizePx ++ "px")
         , style "background-position" "50% 50%"
         , style "background-repeat" "no-repeat"
         ]
            ++ attrs
        )
        tooltipContent
