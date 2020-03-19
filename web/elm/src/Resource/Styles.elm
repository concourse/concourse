module Resource.Styles exposing
    ( body
    , checkBarStatus
    , checkButton
    , checkButtonIcon
    , checkStatusIcon
    , commentBar
    , commentBarContent
    , commentBarIconContainer
    , commentBarMessageIcon
    , commentSaveButton
    , commentText
    , commentTextArea
    , enabledCheckbox
    , headerBar
    , headerHeight
    , headerLastCheckedSection
    , headerResourceName
    , pagination
    , pinBar
    , pinBarTooltip
    , pinButton
    , pinButtonTooltip
    , pinIcon
    , pinTools
    , versionHeader
    )

import Assets
import Colors
import Html
import Html.Attributes exposing (style)
import Pinned
import Resource.Models as Models


headerHeight : Int
headerHeight =
    60


pinBar : Bool -> List (Html.Attribute msg)
pinBar isPinned =
    let
        pinBarBorderColor =
            if isPinned then
                Colors.pinned

            else
                Colors.background
    in
    [ style "display" "flex"
    , style "align-items" "center"
    , style "position" "relative"
    , style "background-color" Colors.pinTools
    , style "border" <| "1px solid" ++ pinBarBorderColor
    , style "flex" "1"
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


pinTools : Bool -> List (Html.Attribute msg)
pinTools isPinned =
    let
        pinToolsBorderColor =
            if isPinned then
                Colors.pinned

            else
                Colors.background
    in
    [ style "background-color" Colors.pinTools
    , style "min-height" "28px"
    , style "margin-bottom" "24px"
    , style "display" "flex"
    , style "align-items" "stretch"
    , style "border" <| "1px solid " ++ pinToolsBorderColor
    , style "box-sizing" "border-box"
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
        Assets.backgroundImage <|
            case enabled of
                Models.Enabled ->
                    Just Assets.CheckmarkIcon

                Models.Changing ->
                    Nothing

                Models.Disabled ->
                    Nothing
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

            Pinned.NotThePinnedVersion ->
                "pointer"

            Pinned.PinnedStatically _ ->
                "default"

            Pinned.Disabled ->
                "default"

            Pinned.InTransition ->
                "default"
    , style "background-image" <|
        Assets.backgroundImage <|
            case pinState of
                Pinned.InTransition ->
                    Nothing

                _ ->
                    Just Assets.PinIconWhite
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


commentBar : Bool -> List (Html.Attribute msg)
commentBar isPinned =
    let
        commentBarBorderColor =
            if isPinned then
                Colors.pinned

            else
                Colors.background
    in
    [ style "background-color" Colors.pinTools
    , style "min-height" "25px"
    , style "display" "flex"
    , style "flex" "1"
    , style "border" <| "1px solid" ++ commentBarBorderColor
    ]


commentBarContent : List (Html.Attribute msg)
commentBarContent =
    [ style "display" "flex"
    , style "flex-direction" "column"
    ]


commentBarMessageIcon : List (Html.Attribute msg)
commentBarMessageIcon =
    [ style "background-size" "contain"
    , style "margin" "4px 10px"
    ]


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
    [ style "flex-grow" "1"
    , style "overflow-y" "auto"
    , style "margin" "0"
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
    , style "display" "flex"
    , style "align-items" "stretch"
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


body : List (Html.Attribute msg)
body =
    [ style "padding" "10px"
    , style "overflow-y" "auto"
    , style "flex-grow" "1"
    ]


pagination : List (Html.Attribute msg)
pagination =
    [ style "display" "flex"
    , style "align-items" "stretch"
    , style "margin-left" "auto"
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
