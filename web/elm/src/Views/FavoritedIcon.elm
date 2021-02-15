module Views.FavoritedIcon exposing (view)

import Assets
import Html exposing (Html)
import Html.Attributes exposing (id, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Effects exposing (toHtmlID)
import Message.Message exposing (DomID(..), Message(..))
import Views.Icon as Icon


view :
    { a
        | isHovered : Bool
        , isFavorited : Bool
        , isSideBar : Bool
        , domID : DomID
    }
    -> List (Html.Attribute Message)
    -> Html Message
view params attrs =
    Icon.icon
        { sizePx = 20
        , image =
            Assets.FavoritedToggleIcon
                { isFavorited = params.isFavorited, isHovered = params.isHovered, isSideBar = params.isSideBar }
        }
        ([ style "cursor" "pointer"
         , style "background-size" "contain"
         , onClick <| Click <| params.domID
         , onMouseEnter <| Hover <| Just <| params.domID
         , onMouseLeave <| Hover Nothing
         , id <| toHtmlID params.domID
         ]
            ++ attrs
        )
