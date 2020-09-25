module Views.InstanceGroupBadge exposing (view)

import Application.Models exposing (Session)
import Html exposing (Html)
import Html.Attributes exposing (style)
import Message.Message exposing (Message)
import Views.Styles as Styles


view : Int -> Html Message
view n =
    let
        ( text, fontSize ) =
            if n <= 99 then
                ( String.fromInt n, "14px" )

            else
                ( "99+", "11px" )
    in
    Html.div
        (Styles.instanceGroupBadge
            ++ [ style "font-size" fontSize ]
        )
        [ Html.text text ]
