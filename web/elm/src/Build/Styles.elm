module Build.Styles exposing
    ( StepHeaderIcon(..)
    , abortButton
    , abortIcon
    , stepHeader
    , stepHeaderIcon
    , stepStatusIcon
    , triggerButton
    , triggerIcon
    , triggerTooltip
    )

import Colors


triggerButton : Bool -> List ( String, String )
triggerButton buttonDisabled =
    [ ( "background-color", Colors.background )
    , ( "padding", "10px" )
    , ( "cursor"
      , if buttonDisabled then
            "default"

        else
            "pointer"
      )
    , ( "border", "none" )
    , ( "outline", "none" )
    , ( "position", "relative" )
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


triggerTooltip : List ( String, String )
triggerTooltip =
    [ ( "position", "absolute" )
    , ( "right", "100%" )
    , ( "top", "15px" )
    , ( "width", "300px" )
    , ( "color", "#ecf0f1" )
    , ( "font-size", "12px" )
    , ( "font-family", "Inconsolata,monospace" )
    , ( "padding", "10px" )
    , ( "text-align", "right" )
    , ( "pointer-events", "none" )
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


stepHeader : List ( String, String )
stepHeader =
    [ ( "display", "flex" )
    , ( "justify-content", "space-between" )
    ]


type StepHeaderIcon
    = ArrowUp
    | ArrowDown
    | Terminal


stepHeaderIcon : StepHeaderIcon -> List ( String, String )
stepHeaderIcon icon =
    let
        image =
            case icon of
                ArrowDown ->
                    "arrow_downward"

                ArrowUp ->
                    "arrow_upward"

                Terminal ->
                    "terminal"
    in
    [ ( "height", "28px" )
    , ( "width", "28px" )
    , ( "background-image"
      , "url(/public/images/ic_" ++ image ++ ".svg)"
      )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "background-size", "14px 14px" )
    ]


stepStatusIcon : String -> List ( String, String )
stepStatusIcon image =
    [ ( "height", "28px" )
    , ( "width", "28px" )
    , ( "background-image"
      , "url(/public/images/" ++ image ++ ".svg)"
      )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "background-size", "14px 14px" )
    ]
