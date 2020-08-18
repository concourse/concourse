module Views.FavoritedIcon exposing (view)

import Assets
import Html exposing (Html)
import Html.Attributes exposing (style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import Views.Icon as Icon


view :
    { a
        | isHovered : Bool
        , isFavorited : Bool
        , domID : DomID
    }
    -> List (Html.Attribute Message)
    -> Html Message
view params attrs =
    Icon.icon
        { sizePx = 20, image = Assets.FavoritedToggleIcon params.isFavorited }
        ([ style "opacity" <|
            if params.isHovered || params.isFavorited then
                "1"

            else
                "0.5"
         , style "cursor" "pointer"
         , style "background-size" "contain"
         , onClick <| Click <| params.domID
         , onMouseEnter <| Hover <| Just <| params.domID
         , onMouseLeave <| Hover Nothing
         ]
            ++ attrs
        )
