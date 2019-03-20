module Pipeline.Styles exposing
    ( groupItem
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


groupsBar : List ( String, String )
groupsBar =
    [ ( "background-color", Colors.groupsBarBackground )
    , ( "color", Colors.dashboardText )
    , ( "margin-top", "54px" )
    ]


groupsList : List ( String, String )
groupsList =
    [ ( "flex-grow", "1" )
    , ( "display", "flex" )
    , ( "flex-flow", "row wrap" )
    , ( "padding", "5px" )
    , ( "list-style", "none" )
    ]


groupItem : Bool -> List ( String, String )
groupItem selected =
    [ ( "font-size", "14px" )
    , ( "background", Colors.groupBackground )
    , ( "margin", "5px" )
    , ( "padding", "10px" )
    ]
        ++ (if selected then
                [ ( "opacity", "1" )
                , ( "border", "1px solid " ++ Colors.selectedGroupBorder )
                ]

            else
                [ ( "opacity", "0.6" )
                , ( "border", "1px solid " ++ Colors.unselectedGroupBorder )
                ]
           )


pinHoverHighlight : List ( String, String )
pinHoverHighlight =
    [ ( "border-width", "5px" )
    , ( "border-style", "solid" )
    , ( "border-color", "transparent transparent " ++ Colors.white ++ " transparent" )
    , ( "position", "absolute" )
    , ( "top", "100%" )
    , ( "right", "50%" )
    , ( "margin-right", "-5px" )
    , ( "margin-top", "-10px" )
    ]


pinText : List ( String, String )
pinText =
    [ ( "font-weight", "700" ) ]


pinDropdownCursor : List ( String, String )
pinDropdownCursor =
    [ ( "cursor", "pointer" ) ]


pinIconDropdown : List ( String, String )
pinIconDropdown =
    [ ( "background-color", Colors.white )
    , ( "color", Colors.pinIconHover )
    , ( "position", "absolute" )
    , ( "top", "100%" )
    , ( "right", "0" )
    , ( "white-space", "nowrap" )
    , ( "list-style-type", "none" )
    , ( "padding", "10px" )
    , ( "margin-top", "0" )
    , ( "z-index", "1" )
    ]


pinIcon : List ( String, String )
pinIcon =
    [ ( "background-image", "url(/public/images/pin-ic-white.svg)" )
    , ( "width", "40px" )
    , ( "height", "40px" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "position", "relative" )
    ]


pinBadge : List ( String, String )
pinBadge =
    [ ( "background-color", Colors.pinned )
    , ( "border-radius", "50%" )
    , ( "width", "15px" )
    , ( "height", "15px" )
    , ( "position", "absolute" )
    , ( "top", "3px" )
    , ( "right", "3px" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    ]


pinIconContainer : Bool -> List ( String, String )
pinIconContainer showBackground =
    [ ( "margin-right", "15px" )
    , ( "top", "10px" )
    , ( "position", "relative" )
    , ( "height", "40px" )
    , ( "display", "flex" )
    , ( "max-width", "20%" )
    ]
        ++ (if showBackground then
                [ ( "background-color", Colors.pinHighlight )
                , ( "border-radius", "50%" )
                ]

            else
                []
           )


pauseToggle : Bool -> List ( String, String )
pauseToggle isPaused =
    [ ( "border-left"
      , if isPaused then
            "1px solid rgba(255, 255, 255, 0.5)"

        else
            "1px solid #3d3c3c"
      )
    ]
