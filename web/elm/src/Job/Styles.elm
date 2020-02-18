module Job.Styles exposing
    ( buildResourceHeader
    , buildResourceIcon
    , icon
    , noBuildsMessage
    , triggerButton
    , triggerTooltip
    )

import Colors
import Concourse.BuildStatus exposing (BuildStatus)
import Html
import Html.Attributes exposing (style)


triggerButton : Bool -> Bool -> BuildStatus -> List (Html.Attribute msg)
triggerButton buttonDisabled hovered status =
    [ style "cursor" <|
        if buttonDisabled then
            "default"

        else
            "pointer"
    , style "position" "relative"
    , style "background-color" <|
        Colors.buildStatusColor (hovered && not buttonDisabled) status
    ]
        ++ button


button : List (Html.Attribute msg)
button =
    [ style "padding" "10px"
    , style "border" "none"
    , style "outline" "none"
    , style "margin" "0"
    ]


icon : Bool -> List (Html.Attribute msg)
icon hovered =
    [ style "opacity" <|
        if hovered then
            "1"

        else
            "0.5"
    ]


triggerTooltip : List (Html.Attribute msg)
triggerTooltip =
    [ style "position" "absolute"
    , style "right" "100%"
    , style "top" "15px"
    , style "width" "300px"
    , style "color" Colors.buildTooltipText
    , style "font-size" "12px"
    , style "font-family" "Inconsolata,monospace"
    , style "padding" "10px"
    , style "text-align" "right"
    , style "pointer-events" "none"
    ]


buildResourceHeader : List (Html.Attribute msg)
buildResourceHeader =
    [ style "display" "flex"
    , style "align-items" "center"
    , style "padding-bottom" "5px"
    ]


buildResourceIcon : List (Html.Attribute msg)
buildResourceIcon =
    [ style "background-size" "contain"
    , style "margin-right" "5px"
    ]


noBuildsMessage : List (Html.Attribute msg)
noBuildsMessage =
    [ style "font-size" "16px"
    , style "padding" "10px 0 0 30px"
    ]
