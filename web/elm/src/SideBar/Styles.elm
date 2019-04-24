module SideBar.Styles exposing
    ( arrow
    , column
    , hamburgerMenu
    , iconGroup
    , pipeline
    , sideBar
    , teamHeader
    , teamIcon
    , teamName
    )

import Colors
import Html
import Html.Attributes exposing (style)
import Views.Icon as Icon


sideBar : List (Html.Attribute msg)
sideBar =
    [ style "border-top" <| "1px solid " ++ Colors.background
    , style "background-color" Colors.frame
    , style "max-width" "38%"
    , style "overflow-y" "auto"
    , style "height" "100%"
    , style "flex-shrink" "0"
    , style "padding-right" "10px"
    , style "box-sizing" "border-box"
    ]


column : List (Html.Attribute msg)
column =
    [ style "display" "flex"
    , style "flex-direction" "column"
    ]


teamHeader : List (Html.Attribute msg)
teamHeader =
    [ style "display" "flex"
    , style "cursor" "pointer"
    ]


iconGroup : List (Html.Attribute msg)
iconGroup =
    [ style "width" "54px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "justify-content" "space-between"
    , style "padding" "5px"
    , style "box-sizing" "border-box"
    ]


teamIcon : Bool -> Html.Html msg
teamIcon isHovered =
    Html.div
        []
        [ Icon.icon
            { sizePx = 20
            , image = "baseline-people-24px.svg"
            }
            [ style "opacity" <|
                if isHovered then
                    "1"

                else
                    "0.5"
            ]
        ]


arrow : { isExpanded : Bool, isHovered : Bool } -> Html.Html msg
arrow { isExpanded, isHovered } =
    Icon.icon
        { sizePx = 20
        , image =
            "baseline-keyboard-arrow-"
                ++ (if isExpanded then
                        "down"

                    else
                        "right"
                   )
                ++ "-24px.svg"
        }
        [ style "opacity" <|
            if isExpanded || isHovered then
                "1"

            else
                "0.5"
        ]


teamName : { isExpanded : Bool, isHovered : Bool } -> List (Html.Attribute msg)
teamName { isExpanded, isHovered } =
    [ style "font-size" "18px"
    , style "padding" "5px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "opacity" <|
        if isExpanded || isHovered then
            "1"

        else
            "0.5"
    ]


pipeline : Bool -> List (Html.Attribute msg)
pipeline isHovered =
    [ style "margin-left" "54px"
    , style "padding" "5px"
    , style "font-size" "18px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "border" <|
        "1px solid "
            ++ (if isHovered then
                    "#525151"

                else
                    Colors.frame
               )
    , style "opacity" <|
        if isHovered then
            "1"

        else
            "0.5"
    ]
        ++ (if isHovered then
                [ style "background-color" "#2f2e2e" ]

            else
                []
           )


hamburgerMenu : { hovered : Bool, clicked : Bool } -> List (Html.Attribute msg)
hamburgerMenu { hovered, clicked } =
    [ style "cursor" "pointer"
    , style "opacity" <|
        if hovered then
            "1"

        else
            "0.5"
    , style "background-color" <|
        if clicked then
            "#333333"

        else
            Colors.frame
    ]
