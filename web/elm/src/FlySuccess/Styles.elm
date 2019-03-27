module FlySuccess.Styles exposing
    ( body
    , button
    , card
    , paragraph
    , title
    )

import Colors
import FlySuccess.Models exposing (ButtonState(..), isClicked)
import Html
import Html.Attributes exposing (style)


card : List (Html.Attribute msg)
card =
    [ style "background-color" Colors.flySuccessCard
    , style "padding" "30px"
    , style "width" "330px"
    , style "margin" "50px auto"
    , style "display" "flex"
    , style "flex-direction" "column"
    , style "align-items" "center"
    , style "text-align" "center"
    , style "-webkit-font-smoothing" "antialiased"
    , style "font-weight" "400"
    ]


title : List (Html.Attribute msg)
title =
    [ style "font-size" "18px"
    , style "margin" "0"
    , style "font-weight" "700"
    ]


body : List (Html.Attribute msg)
body =
    [ style "font-size" "14px"
    , style "margin" "10px 0"
    , style "display" "flex"
    , style "flex-direction" "column"
    , style "align-items" "center"
    ]


paragraph : List (Html.Attribute msg)
paragraph =
    [ style "margin" "5px 0"
    ]


button : ButtonState -> List (Html.Attribute msg)
button buttonState =
    [ style "border" <|
        "1px solid "
            ++ (if isClicked buttonState then
                    Colors.flySuccessTokenCopied

                else
                    Colors.text
               )
    , style "display" "flex"
    , style "justify-content" "center"
    , style "align-items" "center"
    , style "margin" "15px 0"
    , style "padding" "10px 0"
    , style "width" "212px"
    , style "cursor" <|
        if isClicked buttonState then
            "default"

        else
            "pointer"
    , style "text-align" "center"
    , style "background-color" <|
        case buttonState of
            Unhovered ->
                Colors.flySuccessCard

            Hovered ->
                Colors.flySuccessButtonHover

            Clicked ->
                Colors.flySuccessTokenCopied
    ]
