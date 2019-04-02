module Resource.Styles exposing
    ( body
    , checkBarStatus
    , checkButton
    , checkButtonIcon
    , checkStatusIcon
    , commentBar
    , commentBarContent
    , commentBarHeader
    , commentBarIconContainer
    , commentBarMessageIcon
    , commentBarPinIcon
    , commentSaveButton
    , commentText
    , commentTextArea
    , enabledCheckbox
    , headerBar
    , headerHeight
    , headerLastCheckedSection
    , headerResourceName
    , pageBelowTopBar
    , pagination
    , pinBar
    , pinBarTooltip
    , pinButton
    , pinButtonTooltip
    , pinIcon
    , versionHeader
    )

import Colors
import Html
import Html.Attributes exposing (style)
import Pinned
import Resource.Models as Models
import Views.Styles


headerHeight : Int
headerHeight =
    60


commentBarHeight : Int
commentBarHeight =
    300


bodyPadding : Int
bodyPadding =
    10


pageBelowTopBar : List (Html.Attribute msg)
pageBelowTopBar =
    [ style "padding-top" "54px"
    , style "height" "100%"
    , style "display" "block"
    ]


pinBar : Bool -> List (Html.Attribute msg)
pinBar isPinned =
    let
        pinBarBorderColor =
            if isPinned then
                Colors.pinned

            else
                Colors.background
    in
    [ style "flex-grow" "1"
    , style "margin" "10px"
    , style "padding-left" "7px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "position" "relative"
    , style "border" <| "1px solid " ++ pinBarBorderColor
    ]


pinIcon :
    { isPinnedDynamically : Bool
    , hover : Bool
    }
    -> List (Html.Attribute msg)
pinIcon { isPinnedDynamically, hover } =
    let
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
    [ style "margin-right" "10px"
    , style "cursor" cursorType
    , style "background-color" backgroundColor
    ]


pinBarTooltip : List (Html.Attribute msg)
pinBarTooltip =
    [ style "position" "absolute"
    , style "top" "-10px"
    , style "left" "30px"
    , style "background-color" Colors.tooltipBackground
    , style "padding" "5px"
    , style "z-index" "2"
    ]


checkStatusIcon : List (Html.Attribute msg)
checkStatusIcon =
    [ style "background-size" "14px 14px" ]


enabledCheckbox :
    { a
        | enabled : Models.VersionEnabledState
        , pinState : Pinned.VersionPinState
    }
    -> List (Html.Attribute msg)
enabledCheckbox { enabled, pinState } =
    [ style "margin-right" "5px"
    , style "width" "25px"
    , style "height" "25px"
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "cursor" "pointer"
    , style "border" <| "1px solid " ++ borderColor pinState
    , style "background-color" Colors.sectionHeader
    , style "background-image" <|
        case enabled of
            Models.Enabled ->
                "url(/public/images/checkmark-ic.svg)"

            Models.Changing ->
                "none"

            Models.Disabled ->
                "none"
    ]


pinButton : Pinned.VersionPinState -> List (Html.Attribute msg)
pinButton pinState =
    [ style "background-color" Colors.sectionHeader
    , style "border" <| "1px solid " ++ borderColor pinState
    , style "margin-right" "5px"
    , style "width" "25px"
    , style "height" "25px"
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "position" "relative"
    , style "cursor" <|
        case pinState of
            Pinned.Enabled ->
                "pointer"

            Pinned.PinnedDynamically ->
                "pointer"

            Pinned.PinnedStatically _ ->
                "default"

            Pinned.Disabled ->
                "default"

            Pinned.InTransition ->
                "default"
    , style "background-image" <|
        case pinState of
            Pinned.InTransition ->
                "none"

            _ ->
                "url(/public/images/pin-ic-white.svg)"
    ]


pinButtonTooltip : List (Html.Attribute msg)
pinButtonTooltip =
    [ style "position" "absolute"
    , style "bottom" "25px"
    , style "background-color" Colors.tooltipBackground
    , style "z-index" "2"
    , style "padding" "5px"
    , style "width" "170px"
    ]


versionHeader : Pinned.VersionPinState -> List (Html.Attribute msg)
versionHeader pinnedState =
    [ style "background-color" Colors.sectionHeader
    , style "border" <| "1px solid " ++ borderColor pinnedState
    , style "padding-left" "10px"
    , style "cursor" "pointer"
    , style "flex-grow" "1"
    , style "display" "flex"
    , style "align-items" "center"
    ]


borderColor : Pinned.VersionPinState -> String
borderColor pinnedState =
    case pinnedState of
        Pinned.PinnedStatically _ ->
            Colors.pinned

        Pinned.PinnedDynamically ->
            Colors.pinned

        _ ->
            Colors.sectionHeader


commentBar : List (Html.Attribute msg)
commentBar =
    [ style "background-color" Colors.frame
    , style "position" "fixed"
    , style "bottom" "0"
    , style "width" "100%"
    , style "height" "300px"
    , style "display" "flex"
    , style "justify-content" "center"
    ]


commentBarContent : List (Html.Attribute msg)
commentBarContent =
    [ style "width" "700px"
    , style "padding" "20px 0"
    , style "display" "flex"
    , style "flex-direction" "column"
    ]


commentBarHeader : List (Html.Attribute msg)
commentBarHeader =
    [ style "display" "flex"
    , style "flex-shrink" "0"
    , style "align-items" "flex-start"
    ]


commentBarMessageIcon : List (Html.Attribute msg)
commentBarMessageIcon =
    [ style "background-size" "contain"
    , style "margin-right" "10px"
    ]


commentBarPinIcon : List (Html.Attribute msg)
commentBarPinIcon =
    [ style "margin-right" "10px" ]


commentTextArea : List (Html.Attribute msg)
commentTextArea =
    [ style "background-color" "transparent"
    , style "color" Colors.text
    , style "outline" "none"
    , style "border" <| "1px solid " ++ Colors.background
    , style "font-size" "12px"
    , style "font-family" "Inconsolata, monospace"
    , style "font-weight" "700"
    , style "resize" "none"
    , style "margin" "10px 0"
    , style "flex-grow" "1"
    , style "padding" "10px"
    ]


commentText : List (Html.Attribute msg)
commentText =
    [ style "margin" "10px 0"
    , style "flex-grow" "1"
    , style "overflow-y" "auto"
    , style "padding" "10px"
    ]


commentSaveButton :
    { commentChanged : Bool, isHovered : Bool }
    -> List (Html.Attribute msg)
commentSaveButton { commentChanged, isHovered } =
    [ style "font-size" "12px"
    , style "font-family" "Inconsolata, monospace"
    , style "font-weight" "700"
    , style "border" <|
        "1px solid "
            ++ (if commentChanged then
                    Colors.comment

                else
                    Colors.background
               )
    , style "background-color" <|
        if isHovered then
            Colors.comment

        else
            "transparent"
    , style "color" Colors.text
    , style "padding" "5px 10px"
    , style "align-self" "flex-end"
    , style "outline" "none"
    , style "cursor" <|
        if isHovered then
            "pointer"

        else
            "default"
    ]


commentBarIconContainer : List (Html.Attribute msg)
commentBarIconContainer =
    [ style "display" "flex"
    , style "align-items" "center"
    ]


headerBar : List (Html.Attribute msg)
headerBar =
    [ style "height" <| String.fromInt headerHeight ++ "px"
    , style "position" "fixed"
    , style "top" <| String.fromFloat Views.Styles.pageHeaderHeight ++ "px"
    , style "display" "flex"
    , style "align-items" "stretch"
    , style "width" "100%"
    , style "z-index" "1"
    , style "background-color" Colors.secondaryTopBar
    ]


headerResourceName : List (Html.Attribute msg)
headerResourceName =
    [ style "font-weight" "700"
    , style "margin-left" "18px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "justify-content" "center"
    ]


headerLastCheckedSection : List (Html.Attribute msg)
headerLastCheckedSection =
    [ style "display" "flex"
    , style "align-items" "center"
    , style "justify-content" "center"
    , style "margin-left" "24px"
    ]


body : Bool -> List (Html.Attribute msg)
body hasCommentBar =
    [ style "padding-top" <| String.fromInt (headerHeight + bodyPadding) ++ "px"
    , style "padding-left" <| String.fromInt bodyPadding ++ "px"
    , style "padding-right" <| String.fromInt bodyPadding ++ "px"
    , style "padding-bottom" <|
        if hasCommentBar then
            String.fromInt commentBarHeight ++ "px"

        else
            String.fromInt bodyPadding ++ "px"
    ]


pagination : List (Html.Attribute msg)
pagination =
    [ style "display" "flex"
    , style "align-items" "stretch"
    ]


checkBarStatus : List (Html.Attribute msg)
checkBarStatus =
    [ style "display" "flex"
    , style "justify-content" "space-between"
    , style "align-items" "center"
    , style "flex-grow" "1"
    , style "height" "28px"
    , style "background" Colors.sectionHeader
    , style "padding-left" "5px"
    ]


checkButton : Bool -> List (Html.Attribute msg)
checkButton isClickable =
    [ style "height" "28px"
    , style "width" "28px"
    , style "background-color" Colors.sectionHeader
    , style "margin-right" "5px"
    , style "cursor" <|
        if isClickable then
            "pointer"

        else
            "default"
    ]


checkButtonIcon : Bool -> List (Html.Attribute msg)
checkButtonIcon isHighlighted =
    [ style "margin" "4px"
    , style "background-size" "contain"
    , style "opacity" <|
        if isHighlighted then
            "1"

        else
            "0.5"
    ]
