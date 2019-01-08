module Resource.Styles exposing
    ( checkStatusIcon
    , pinBar
    , pinBarTooltip
    , pinIcon
    )

import Colors


pinBar : { isPinned : Bool } -> List ( String, String )
pinBar { isPinned } =
    let
        borderColor =
            if isPinned then
                Colors.pinned

            else
                "#3d3c3c"
    in
    [ ( "flex-grow", "1" )
    , ( "margin", "10px" )
    , ( "padding-left", "7px" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "position", "relative" )
    , ( "border", "1px solid " ++ borderColor )
    ]


pinIcon :
    { isPinned : Bool
    , isPinnedDynamically : Bool
    , hover : Bool
    }
    -> List ( String, String )
pinIcon { isPinned, isPinnedDynamically, hover } =
    let
        backgroundImage =
            if isPinned then
                "url(/public/images/pin_ic_white.svg)"

            else
                "url(/public/images/pin_ic_grey.svg)"

        cursorType =
            if isPinnedDynamically then
                "pointer"

            else
                "default"

        backgroundColor =
            if hover then
                Colors.pinIconHover

            else
                "transparent"
    in
    [ ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "height", "25px" )
    , ( "width", "25px" )
    , ( "margin-right", "10px" )
    , ( "background-image", backgroundImage )
    , ( "cursor", cursorType )
    , ( "background-color", backgroundColor )
    ]


pinBarTooltip : List ( String, String )
pinBarTooltip =
    [ ( "position", "absolute" )
    , ( "top", "-10px" )
    , ( "left", "30px" )
    , ( "background-color", Colors.pinBarTooltip )
    , ( "padding", "5px" )
    , ( "z-index", "2" )
    ]


checkStatusIcon : Bool -> List ( String, String )
checkStatusIcon failingToCheck =
    let
        icon =
            if failingToCheck then
                "url(/public/images/ic_exclamation-triangle.svg)"

            else
                "url(/public/images/ic-success-check.svg)"
    in
    [ ( "background-image", icon )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "width", "28px" )
    , ( "height", "28px" )
    , ( "background-size", "14px 14px" )
    ]
