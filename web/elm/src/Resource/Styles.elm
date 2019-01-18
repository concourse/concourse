module Resource.Styles exposing
    ( checkStatusIcon
    , commentBar
    , commentBarContent
    , commentBarHeader
    , commentBarMessageIcon
    , commentBarPinIcon
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
                "url(/public/images/pin-ic-white.svg)"

            else
                "url(/public/images/pin-ic-grey.svg)"

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
                "url(/public/images/ic-exclamation-triangle.svg)"

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


commentBar : List ( String, String )
commentBar =
    [ ( "background-color", Colors.frame )
    , ( "position", "fixed" )
    , ( "bottom", "0" )
    , ( "width", "100%" )
    , ( "height", "300px" )
    ]


commentBarContent : List ( String, String )
commentBarContent =
    [ ( "width", "700px" )
    , ( "margin", "auto" )
    , ( "padding", "20px" )
    ]


commentBarHeader : List ( String, String )
commentBarHeader =
    [ ( "display", "flex" )
    , ( "align-items", "center" )
    ]


commentBarMessageIcon : List ( String, String )
commentBarMessageIcon =
    let
        messageIconUrl =
            "url(/public/images/baseline-message.svg)"
    in
    [ ( "background-image", messageIconUrl )
    , ( "background-size", "contain" )
    , ( "width", "24px" )
    , ( "height", "24px" )
    , ( "margin-right", "10px" )
    ]


commentBarPinIcon : List ( String, String )
commentBarPinIcon =
    let
        pinIconUrl =
            "url(/public/images/pin-ic-white.svg)"
    in
    [ ( "background-image", pinIconUrl )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "width", "20px" )
    , ( "height", "20px" )
    , ( "margin-right", "10px" )
    ]
