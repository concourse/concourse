module Pipeline.Styles exposing
    ( cliIcon
    , groupItem
    , groupsBar
    , groupsList
    , pauseToggle
    , pinBadge
    , pinDropdownCursor
    , pinHoverHighlight
    , pinIcon
    , pinIconContainer
    , pinIconDropdown
    , pinText
    )

import Colors
import Concourse.Cli as Cli
import Html
import Html.Attributes exposing (style)


groupsBar : List (Html.Attribute msg)
groupsBar =
    [ style "background-color" Colors.groupsBarBackground
    , style "color" Colors.dashboardText
    ]


groupsList : List (Html.Attribute msg)
groupsList =
    [ style "flex-grow" "1"
    , style "display" "flex"
    , style "flex-flow" "row wrap"
    , style "padding" "5px"
    , style "list-style" "none"
    ]


groupItem : { selected : Bool, hovered : Bool } -> List (Html.Attribute msg)
groupItem { selected, hovered } =
    [ style "font-size" "14px"
    , style "background" Colors.groupBackground
    , style "margin" "5px"
    , style "padding" "10px"
    ]
        ++ (if selected then
                [ style "opacity" "1"
                , style "border" <| "1px solid " ++ Colors.groupBorderSelected
                ]

            else if hovered then
                [ style "opacity" "0.6"
                , style "border" <| "1px solid " ++ Colors.groupBorderHovered
                ]

            else
                [ style "opacity" "0.6"
                , style "border" <| "1px solid " ++ Colors.groupBorderUnselected
                ]
           )


pinHoverHighlight : List (Html.Attribute msg)
pinHoverHighlight =
    [ style "border-width" "5px"
    , style "border-style" "solid"
    , style "border-color" <| "transparent transparent " ++ Colors.white ++ " transparent"
    , style "position" "absolute"
    , style "top" "100%"
    , style "right" "50%"
    , style "margin-right" "-5px"
    , style "margin-top" "-10px"
    ]


pinText : List (Html.Attribute msg)
pinText =
    [ style "font-weight" "700" ]


pinDropdownCursor : List (Html.Attribute msg)
pinDropdownCursor =
    [ style "cursor" "pointer" ]


pinIconDropdown : List (Html.Attribute msg)
pinIconDropdown =
    [ style "background-color" Colors.white
    , style "color" Colors.pinIconHover
    , style "position" "absolute"
    , style "top" "100%"
    , style "right" "0"
    , style "white-space" "nowrap"
    , style "list-style-type" "none"
    , style "padding" "10px"
    , style "margin-top" "0"
    , style "z-index" "1"
    ]


pinIcon : List (Html.Attribute msg)
pinIcon =
    [ style "background-image" "url(/public/images/pin-ic-white.svg)"
    , style "width" "40px"
    , style "height" "40px"
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "position" "relative"
    ]


pinBadge : List (Html.Attribute msg)
pinBadge =
    [ style "background-color" Colors.pinned
    , style "border-radius" "50%"
    , style "width" "15px"
    , style "height" "15px"
    , style "position" "absolute"
    , style "top" "3px"
    , style "right" "3px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "justify-content" "center"
    ]


pinIconContainer : Bool -> List (Html.Attribute msg)
pinIconContainer showBackground =
    [ style "margin-right" "15px"
    , style "top" "10px"
    , style "position" "relative"
    , style "height" "40px"
    , style "display" "flex"
    , style "max-width" "20%"
    ]
        ++ (if showBackground then
                [ style "background-color" Colors.pinHighlight
                , style "border-radius" "50%"
                ]

            else
                []
           )


pauseToggle : Bool -> List (Html.Attribute msg)
pauseToggle isPaused =
    [ style "border-left" <|
        if isPaused then
            "1px solid rgba(255, 255, 255, 0.5)"

        else
            "1px solid #3d3c3c"
    ]


cliIcon : Cli.Cli -> List (Html.Attribute msg)
cliIcon cli =
    [ style "width" "12px"
    , style "height" "12px"
    , style "background-image" <| Cli.iconUrl cli
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "background-size" "contain"
    , style "display" "inline-block"
    ]
