module Job.Styles exposing
    ( buildResourceHeader
    , buildResourceIcon
    , icon
    , pageBelowTopBar
    , triggerButton
    , triggerTooltip
    )

import Colors
import Concourse


pageBelowTopBar : List ( String, String )
pageBelowTopBar =
    [ ( "padding-top", "54px" )
    , ( "height", "100%" )
    ]


triggerButton : Bool -> Bool -> Concourse.BuildStatus -> List ( String, String )
triggerButton buttonDisabled hovered status =
    [ ( "cursor"
      , if buttonDisabled then
            "default"

        else
            "pointer"
      )
    , ( "position", "relative" )
    , ( "background-color"
      , Colors.buildStatusColor (hovered && not buttonDisabled) status
      )
    ]
        ++ button


button : List ( String, String )
button =
    [ ( "padding", "10px" )
    , ( "border", "none" )
    , ( "outline", "none" )
    , ( "margin", "0" )
    ]


icon : Bool -> List ( String, String )
icon hovered =
    [ ( "opacity"
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
    , ( "color", Colors.buildTooltipBackground )
    , ( "font-size", "12px" )
    , ( "font-family", "Inconsolata,monospace" )
    , ( "padding", "10px" )
    , ( "text-align", "right" )
    , ( "pointer-events", "none" )
    ]


buildResourceHeader : List ( String, String )
buildResourceHeader =
    [ ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "padding-bottom", "5px" )
    ]


buildResourceIcon : List ( String, String )
buildResourceIcon =
    [ ( "background-size", "contain" )
    , ( "margin-right", "5px" )
    ]
