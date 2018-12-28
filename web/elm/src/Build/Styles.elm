module Build.Styles exposing (abortButton, abortIcon, triggerButton, triggerIcon)

import Colors


triggerButton : List ( String, String )
triggerButton =
    [ ( "background-color", Colors.background )
    , ( "padding", "10px" )
    , ( "cursor", "pointer" )
    , ( "border", "none" )
    , ( "outline", "none" )
    ]


triggerIcon : Bool -> List ( String, String )
triggerIcon hovered =
    [ ( "width", "40px" )
    , ( "height", "40px" )
    , ( "background-position", "50% 50%" )
    , ( "background-image"
      , "url(/public/images/ic_add_circle_outline_white.svg)"
      )
    , ( "background-repeat", "no-repeat" )
    , ( "opacity"
      , if hovered then
            "1"

        else
            "0.5"
      )
    ]


abortButton : List ( String, String )
abortButton =
    [ ( "background-color", Colors.background )
    , ( "padding", "10px" )
    , ( "cursor", "pointer" )
    , ( "border", "none" )
    , ( "outline", "none" )
    ]


abortIcon : Bool -> List ( String, String )
abortIcon hovered =
    [ ( "width", "40px" )
    , ( "height", "40px" )
    , ( "background-position", "50% 50%" )
    , ( "background-image"
      , "url(/public/images/ic_abort_circle_outline_white.svg)"
      )
    , ( "background-repeat", "no-repeat" )
    , ( "opacity"
      , if hovered then
            "1"

        else
            "0.5"
      )
    ]
