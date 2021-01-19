module Views.InstanceGroupBadge exposing (view)

import Html exposing (Html)
import Html.Attributes exposing (style)
import Message.Message exposing (Message)
import Views.Styles as Styles


view : String -> Int -> Html Message
view backgroundColor n =
    let
        ( text, fontSize ) =
            if n <= 99 then
                ( String.fromInt n, "14px" )

            else
                ( "99+", "11px" )
    in
    Html.div
        (Styles.instanceGroupBadge backgroundColor
            ++ [ style "font-size" fontSize ]
        )
        [ Html.text text ]
