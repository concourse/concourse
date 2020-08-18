module Login.Styles exposing
    ( loginComponent
    , loginContainer
    , loginItem
    , loginText
    , logoutButton
    )

import Colors
import Html
import Html.Attributes exposing (style)


loginComponent : List (Html.Attribute msg)
loginComponent =
    [ style "max-width" "20%"
    , style "background-color" Colors.frame
    ]


loginContainer : List (Html.Attribute msg)
loginContainer =
    [ style "position" "relative"
    , style "display" "flex"
    , style "flex-direction" "column"
    , style "border-left" <|
        "1px solid "
            ++ Colors.background
    , style "line-height" "54px"
    ]


loginItem : List (Html.Attribute msg)
loginItem =
    [ style "padding" "0 30px"
    , style "cursor" "pointer"
    , style "display" "flex"
    , style "align-items" "center"
    , style "justify-content" "center"
    , style "flex-grow" "1"
    ]


loginText : List (Html.Attribute msg)
loginText =
    [ style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    ]


logoutButton : List (Html.Attribute msg)
logoutButton =
    [ style "position" "absolute"
    , style "top" "55px"
    , style "background-color" Colors.frame
    , style "height" "54px"
    , style "width" "100%"
    , style "border-top" <| "1px solid " ++ Colors.background
    , style "cursor" "pointer"
    , style "display" "flex"
    , style "align-items" "center"
    , style "justify-content" "center"
    , style "flex-grow" "1"
    ]
