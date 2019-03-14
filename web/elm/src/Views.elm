module Views exposing (icon)

import Html exposing (Html)
import Html.Attributes exposing (style)


icon :
    { sizePx : Int, image : String }
    -> List (Html.Attribute msg)
    -> Html msg
icon { sizePx, image } attrs =
    flip Html.div [] <|
        [ style
            [ ( "background-image", "url(/public/images/" ++ image ++ ")" )
            , ( "height", toString sizePx ++ "px" )
            , ( "width", toString sizePx ++ "px" )
            , ( "background-position", "50% 50%" )
            , ( "background-repeat", "no-repeat" )
            , ( "background-size", "contain" )
            ]
        ]
            ++ attrs
