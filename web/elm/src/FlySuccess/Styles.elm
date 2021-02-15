module FlySuccess.Styles exposing
    ( body
    , button
    , card
    , input
    , paragraph
    , title
    )

import Colors
import FlySuccess.Models exposing (ButtonState(..), InputState(..), isClicked)
import Html
import Html.Attributes exposing (style)
import Views.Styles


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
    , style "font-weight" Views.Styles.fontWeightLight
    ]


title : List (Html.Attribute msg)
title =
    [ style "font-size" "18px"
    , style "margin" "0"
    , style "font-weight" Views.Styles.fontWeightDefault
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


input : InputState -> List (Html.Attribute msg)
input inputState =
    [ style "border" <|
        "1px solid "
            ++ Colors.text
    , style "display" "flex"
    , style "justify-content" "center"
    , style "align-items" "center"
    , style "margin" "15px 0"
    , style "padding" "10px 10px"
    , style "width" "192px"
    , style "text-align" "center"
    , style "background-color" Colors.flySuccessCard
    , style "background-color" <|
        case inputState of
            InputUnhovered ->
                Colors.flySuccessCard

            InputHovered ->
                Colors.flySuccessButtonHover
    , style "color" Colors.text
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
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
