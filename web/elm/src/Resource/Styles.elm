module Resource.Styles exposing
    ( body
    , checkBarStatus
    , checkButton
    , checkButtonIcon
    , checkStatusIcon
    , commentBar
    , commentBarIconContainer
    , commentBarMessageIcon
    , commentSaveButton
    , commentText
    , commentTextArea
    , editButton
    , editSaveWrapper
    , enabledCheckbox
    , headerBar
    , headerHeight
    , headerLastCheckedSection
    , headerResourceName
    , pagination
    , pinBar
    , pinBarTooltip
    , pinBarViewVersion
    , pinButton
    , pinButtonTooltip
    , pinIcon
    , pinTools
    , versionHeader
    )

import Assets
import Colors
import Html
import Html.Attributes exposing (rows, style)
import Pinned
import Resource.Models as Models
import Views.Styles


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
    , style "align-items" "flex-start"
    , style "position" "relative"
    , style "background-color" Colors.pinTools
    , style "border" <| "1px solid" ++ pinBarBorderColor
    , style "flex" "1"
    ]


pinIcon :
    { clickable : Bool
    , hover : Bool
    }
    -> List (Html.Attribute msg)
pinIcon { clickable, hover } =
    let
        cursorType =
            if clickable then
                "pointer"

            else
                "default"

        backgroundColor =
            if hover then
                Colors.pinIconHover

            else
                "transparent"
    in
    [ style "margin" "4px 5px 5px 5px"
    , style "cursor" cursorType
    , style "background-color" backgroundColor
    , style "padding" "6px"
    , style "background-size" "contain"
    , style "background-origin" "content-box"
    , style "min-width" "14px"
    , style "min-height" "14px"
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


pinBarViewVersion : List (Html.Attribute msg)
pinBarViewVersion =
    [ style "margin" "8px 8px 8px 0" ]


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


commentBarMessageIcon : List (Html.Attribute msg)
commentBarMessageIcon =
    [ style "background-size" "contain"
    , style "margin" "10px"
    , style "flex-shrink" "0"
    , style "background-origin" "content-box"
    ]


commentTextArea : List (Html.Attribute msg)
commentTextArea =
    [ style "box-sizing" "border-box"
    , style "flex-grow" "1"
    , style "resize" "none"
    , style "outline" "none"
    , style "border" "none"
    , style "color" Colors.text
    , style "background-color" "transparent"
    , style "max-height" "150px"
    , style "margin" "8px 0"
    , rows 1
    ]
        ++ Views.Styles.defaultFont


commentText : List (Html.Attribute msg)
commentText =
    [ style "flex-grow" "1"
    , style "margin" "0"
    , style "outline" "0"
    , style "padding" "8px 0"
    , style "max-height" "150px"
    , style "overflow-y" "scroll"
    ]


commentSaveButton :
    { isHovered : Bool, commentChanged : Bool, pinCommentLoading : Bool }
    -> List (Html.Attribute msg)
commentSaveButton { commentChanged, isHovered, pinCommentLoading } =
    [ style "border" <|
        "1px solid "
            ++ (if commentChanged && not pinCommentLoading then
                    Colors.white

                else
                    Colors.buttonDisabledGrey
               )
    , style "background-color" <|
        if isHovered && commentChanged && not pinCommentLoading then
            Colors.frame

        else
            "transparent"
    , style "color" <|
        if commentChanged && not pinCommentLoading then
            Colors.text

        else
            Colors.buttonDisabledGrey
    , style "padding" "5px 10px"
    , style "margin" "5px 5px 7px 7px"
    , style "outline" "none"
    , style "transition" "border 200ms ease, color 200ms ease"
    , style "cursor" <|
        if commentChanged && not pinCommentLoading then
            "pointer"

        else
            "default"
    ]
        ++ Views.Styles.defaultFont


commentBarIconContainer : Bool -> List (Html.Attribute msg)
commentBarIconContainer isEditing =
    [ style "display" "flex"
    , style "align-items" "flex-start"
    , style "flex-grow" "1"
    , style "background-color" <|
        if isEditing then
            Colors.pinned

        else
            Colors.pinTools
    ]


editButton : Bool -> List (Html.Attribute msg)
editButton isHovered =
    [ style "padding" "5px"
    , style "margin" "5px"
    , style "cursor" "pointer"
    , style "background-color" <|
        if isHovered then
            Colors.sectionHeader

        else
            Colors.pinTools
    , style "background-origin" "content-box"
    , style "background-size" "contain"
    ]


editSaveWrapper : List (Html.Attribute msg)
editSaveWrapper =
    [ style "width" "60px"
    , style "display" "flex"
    , style "justify-content" "flex-end"
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
    [ style "margin-left" "18px"
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
