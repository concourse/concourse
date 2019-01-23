module Build.Styles exposing
    ( abortButton
    , abortIcon
    , firstOccurrenceTooltip
    , stepHeader
    , stepHeaderIcon
    , stepStatusIcon
    , triggerButton
    , triggerIcon
    , triggerTooltip
    )

import Build.Models exposing (StepHeaderType(..))
import Colors


triggerButton : Bool -> List ( String, String )
triggerButton buttonDisabled =
    [ ( "cursor"
      , if buttonDisabled then
            "default"

        else
            "pointer"
      )
    , ( "position", "relative" )
    ]
        ++ button


abortButton : List ( String, String )
abortButton =
    ( "cursor", "pointer" ) :: button


button : List ( String, String )
button =
    [ ( "background-color", Colors.background )
    , ( "padding", "10px" )
    , ( "border", "none" )
    , ( "outline", "none" )
    , ( "margin", "0" )
    ]


triggerIcon : Bool -> List ( String, String )
triggerIcon hovered =
    [ ( "width", "40px" )
    , ( "height", "40px" )
    , ( "background-position", "50% 50%" )
    , ( "background-image"
      , "url(/public/images/ic-add-circle-outline-white.svg)"
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


abortIcon : Bool -> List ( String, String )
abortIcon hovered =
    [ ( "width", "40px" )
    , ( "height", "40px" )
    , ( "background-position", "50% 50%" )
    , ( "background-image"
      , "url(/public/images/ic-abort-circle-outline-white.svg)"
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


stepHeaderIcon : StepHeaderType -> List ( String, String )
stepHeaderIcon icon =
    let
        image =
            case icon of
                StepHeaderGet False ->
                    "arrow-downward"

                StepHeaderGet True ->
                    "arrow-downward-yellow"

                StepHeaderPut ->
                    "arrow-upward"

                StepHeaderTask ->
                    "terminal"
    in
    [ ( "height", "28px" )
    , ( "width", "28px" )
    , ( "background-image"
      , "url(/public/images/ic-" ++ image ++ ".svg)"
      )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "background-size", "14px 14px" )
    , ( "position", "relative" )
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


firstOccurrenceTooltip : List ( String, String )
firstOccurrenceTooltip =
    [ ( "position", "absolute" )
    , ( "left", "100%" )
    , ( "bottom", "50%" )
    , ( "background-color", Colors.tooltipBackground )
    , ( "padding", "10px" )
    , ( "z-index", "100" )
    , ( "width", "6em" )
    ]
