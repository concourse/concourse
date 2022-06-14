module Login.Styles exposing
    ( loginComponent
    , loginContainer
    , loginItem
    , logoutButton
    )

import Colors
import Html
import Html.Attributes exposing (style)


loginComponent : List (Html.Attribute msg)
loginComponent =
    [ style "max-width" "20%"
    , style "background-color" Colors.topBarBackground
    ]


loginContainer : List (Html.Attribute msg)
loginContainer =
    [ style "position" "relative"
    , style "display" "flex"
    , style "flex-direction" "column"
    , style "border-left" <|
        "1px solid "
            ++ Colors.background
    ]


loginItem : List (Html.Attribute msg)
loginItem =
    [ style "margin" "19px 30px"
    , style "cursor" "pointer"
    , style "align-items" "center"
    , style "justify-content" "center"
    , style "flex-grow" "1"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "white-space" "nowrap"
    ]


logoutButton : List (Html.Attribute msg)
logoutButton =
    [ style "position" "absolute"
    , style "top" "55px"
    , style "left" "0px"
    , style "background-color" Colors.topBarBackground
    , style "height" "54px"
    , style "width" "100%"
    , style "border-top" <| "1px solid " ++ Colors.background
    , style "cursor" "pointer"
    , style "display" "flex"
    , style "align-items" "center"
    , style "justify-content" "center"
    , style "flex-grow" "1"
    ]
