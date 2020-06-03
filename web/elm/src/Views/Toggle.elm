module Views.Toggle exposing (toggleSwitch)

import Assets
import Html exposing (Html)
import Html.Attributes exposing (attribute, href, style)
import Message.Message exposing (DomID(..), Message(..))
import Routes


toggleSwitch :
    { on : Bool
    , hrefRoute : Routes.Route
    , text : String
    , ariaLabel : String
    , styles : List (Html.Attribute Message)
    }
    -> Html Message
toggleSwitch { ariaLabel, hrefRoute, text, styles, on } =
    Html.a
        ([ href <| Routes.toString hrefRoute
         , attribute "aria-label" ariaLabel
         , style "display" "flex"
         , style "align-items" "center"
         ]
            ++ styles
        )
        [ Html.div
            [ style "background-image" <|
                Assets.backgroundImage <|
                    Just (Assets.ToggleSwitch on)
            , style "background-size" "contain"
            , style "height" "20px"
            , style "width" "35px"
            , style "flex-shrink" "0"
            , style "margin-right" "10px"
            ]
            []
        , Html.text text
        ]
