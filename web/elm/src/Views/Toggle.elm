module Views.Toggle exposing (TextDirection(..), toggleSwitch)

import Assets
import Html exposing (Html)
import Html.Attributes exposing (attribute, href, style, type_)
import Html.Events exposing (onClick)
import Message.Message exposing (DomID(..), Message(..))
import Routes


type TextDirection
    = Left
    | Right


toggleSwitch :
    { on : Bool
    , hrefRoute : Maybe Routes.Route
    , onToggle : Message
    , text : String
    , textDirection : TextDirection
    , ariaLabel : String
    , styles : List (Html.Attribute Message)
    }
    -> Html Message
toggleSwitch { ariaLabel, hrefRoute, onToggle, text, textDirection, styles, on } =
    let
        textElem =
            Html.text text

        iconElem =
            Html.div
                [ style "background-image" <|
                    Assets.backgroundImage <|
                        Just (Assets.ToggleSwitch on)
                , style "background-size" "contain"
                , style "height" "20px"
                , style "width" "35px"
                , style "flex-shrink" "0"
                , case textDirection of
                    Left ->
                        style "margin-left" "10px"

                    Right ->
                        style "margin-right" "10px"
                ]
                []
    in
    case hrefRoute of
        Just route ->
            Html.a
                ([ href <| Routes.toString route
                 , attribute "aria-label" ariaLabel
                 , style "display" "flex"
                 , style "align-items" "center"
                 , style "flex-direction" <|
                    case textDirection of
                        Right ->
                            "row"

                        Left ->
                            "row-reverse"
                 ]
                    ++ styles
                )
                [ iconElem, textElem ]

        Nothing ->
            Html.button
                ([ type_ "button"
                 , attribute "aria-label" ariaLabel
                 , style "cursor" "pointer"
                 , style "-webkit-appearance" "none"
                 , style "appearance" "none"
                 , style "background" "transparent"
                 , style "border" "0"
                 , style "padding" "0"
                 , style "margin" "0"
                 , style "font" "inherit"
                 , style "color" "inherit"
                 , style "display" "flex"
                 , style "align-items" "center"
                 , style "flex-direction" <|
                    case textDirection of
                        Right ->
                            "row"

                        Left ->
                            "row-reverse"
                 , onClick onToggle
                 ]
                    ++ styles
                )
                [ iconElem, textElem ]
