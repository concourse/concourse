module Job.Styles exposing
    ( pauseToggleIcon
    , triggerButton
    , triggerIcon
    , triggerTooltip
    )

import Colors
import Concourse


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


pauseToggleIcon : { paused : Bool, hovered : Bool } -> List ( String, String )
pauseToggleIcon { paused, hovered } =
    [ ( "background-image"
      , "url(/public/images/"
            ++ (if paused then
                    "ic-play-circle-outline.svg)"

                else
                    "ic-pause-circle-outline-white.svg)"
               )
      )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "width", "40px" )
    , ( "height", "40px" )
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
    , ( "color", Colors.buildTooltipBackground )
    , ( "font-size", "12px" )
    , ( "font-family", "Inconsolata,monospace" )
    , ( "padding", "10px" )
    , ( "text-align", "right" )
    , ( "pointer-events", "none" )
    ]
