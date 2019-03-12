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
    , pagination
    , pinBar
    , pinBarTooltip
    , pinButton
    , pinButtonTooltip
    , pinIcon
    , versionHeader
    )

import Colors
import Pinned
import Resource.Models as Models
import TopBar.Styles


headerHeight : Int
headerHeight =
    60


commentBarHeight : Int
commentBarHeight =
    300


bodyPadding : Int
bodyPadding =
    10


pinBar : { isPinned : Bool } -> List ( String, String )
pinBar { isPinned } =
    let
        borderColor =
            if isPinned then
                Colors.pinned

            else
                Colors.background
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
    , ( "background-color", Colors.tooltipBackground )
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


enabledCheckbox :
    { a
        | enabled : Models.VersionEnabledState
        , pinState : Pinned.VersionPinState
    }
    -> List ( String, String )
enabledCheckbox { enabled, pinState } =
    [ ( "margin-right", "5px" )
    , ( "width", "25px" )
    , ( "height", "25px" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "cursor", "pointer" )
    , ( "border", "1px solid " ++ borderColor pinState )
    , ( "background-color", Colors.sectionHeader )
    , ( "background-image"
      , case enabled of
            Models.Enabled ->
                "url(/public/images/checkmark-ic.svg)"

            Models.Changing ->
                "none"

            Models.Disabled ->
                "none"
      )
    ]


pinButton : Pinned.VersionPinState -> List ( String, String )
pinButton pinState =
    [ ( "background-color", Colors.sectionHeader )
    , ( "border", "1px solid " ++ borderColor pinState )
    , ( "margin-right", "5px" )
    , ( "width", "25px" )
    , ( "height", "25px" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "position", "relative" )
    , ( "cursor"
      , case pinState of
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
      )
    , ( "background-image"
      , case pinState of
            Pinned.InTransition ->
                "none"

            _ ->
                "url(/public/images/pin-ic-white.svg)"
      )
    ]


pinButtonTooltip : List ( String, String )
pinButtonTooltip =
    [ ( "position", "absolute" )
    , ( "bottom", "25px" )
    , ( "background-color", Colors.tooltipBackground )
    , ( "z-index", "2" )
    , ( "padding", "5px" )
    , ( "width", "170px" )
    ]


versionHeader : Pinned.VersionPinState -> List ( String, String )
versionHeader pinnedState =
    [ ( "background-color", Colors.sectionHeader )
    , ( "border", "1px solid " ++ borderColor pinnedState )
    , ( "padding-left", "10px" )
    , ( "cursor", "pointer" )
    , ( "flex-grow", "1" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
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


commentBar : List ( String, String )
commentBar =
    [ ( "background-color", Colors.frame )
    , ( "position", "fixed" )
    , ( "bottom", "0" )
    , ( "width", "100%" )
    , ( "height", "300px" )
    , ( "display", "flex" )
    , ( "justify-content", "center" )
    ]


commentBarContent : List ( String, String )
commentBarContent =
    [ ( "width", "700px" )
    , ( "padding", "20px 0" )
    , ( "display", "flex" )
    , ( "flex-direction", "column" )
    ]


commentBarHeader : List ( String, String )
commentBarHeader =
    [ ( "display", "flex" )
    , ( "flex-shrink", "0" )
    , ( "align-items", "flex-start" )
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


commentTextArea : List ( String, String )
commentTextArea =
    [ ( "background-color", "transparent" )
    , ( "color", Colors.text )
    , ( "outline", "none" )
    , ( "border", "1px solid " ++ Colors.background )
    , ( "font-size", "12px" )
    , ( "font-family", "Inconsolata, monospace" )
    , ( "font-weight", "700" )
    , ( "resize", "none" )
    , ( "margin", "10px 0" )
    , ( "flex-grow", "1" )
    , ( "padding", "10px" )
    ]


commentText : List ( String, String )
commentText =
    [ ( "margin", "10px 0" )
    , ( "flex-grow", "1" )
    , ( "overflow-y", "auto" )
    , ( "padding", "10px" )
    ]


commentSaveButton :
    { commentChanged : Bool, isHovered : Bool }
    -> List ( String, String )
commentSaveButton { commentChanged, isHovered } =
    [ ( "font-size", "12px" )
    , ( "font-family", "Inconsolata, monospace" )
    , ( "font-weight", "700" )
    , ( "border"
      , "1px solid "
            ++ (if commentChanged then
                    Colors.comment

                else
                    Colors.background
               )
      )
    , ( "background-color"
      , if isHovered then
            Colors.comment

        else
            "transparent"
      )
    , ( "color", Colors.text )
    , ( "padding", "5px 10px" )
    , ( "align-self", "flex-end" )
    , ( "outline", "none" )
    , ( "cursor"
      , if isHovered then
            "pointer"

        else
            "default"
      )
    ]


commentBarIconContainer : List ( String, String )
commentBarIconContainer =
    [ ( "display", "flex" )
    , ( "align-items", "center" )
    ]


headerBar : List ( String, String )
headerBar =
    [ ( "height", toString headerHeight ++ "px" )
    , ( "position", "fixed" )
    , ( "top", toString TopBar.Styles.pageHeaderHeight ++ "px" )
    , ( "display", "flex" )
    , ( "align-items", "stretch" )
    , ( "width", "100%" )
    , ( "z-index", "1" )
    , ( "background-color", Colors.secondaryTopBar )
    ]


headerResourceName : List ( String, String )
headerResourceName =
    [ ( "font-weight", "700" )
    , ( "margin-left", "18px" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    ]


headerLastCheckedSection : List ( String, String )
headerLastCheckedSection =
    [ ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    , ( "margin-left", "24px" )
    ]


body : Bool -> List ( String, String )
body hasCommentBar =
    [ ( "padding-top", toString (headerHeight + bodyPadding) ++ "px" )
    , ( "padding-left", toString bodyPadding ++ "px" )
    , ( "padding-right", toString bodyPadding ++ "px" )
    , ( "padding-bottom"
      , if hasCommentBar then
            toString commentBarHeight ++ "px"

        else
            toString bodyPadding ++ "px"
      )
    ]


pagination : List ( String, String )
pagination =
    [ ( "display", "flex" )
    , ( "align-items", "stretch" )
    ]


checkBarStatus : List ( String, String )
checkBarStatus =
    [ ( "display", "flex" )
    , ( "justify-content", "space-between" )
    , ( "align-items", "center" )
    , ( "flex-grow", "1" )
    , ( "height", "28px" )
    , ( "background", Colors.sectionHeader )
    , ( "padding-left", "5px" )
    ]


checkButton : Bool -> List ( String, String )
checkButton isClickable =
    [ ( "height", "28px" )
    , ( "width", "28px" )
    , ( "background-color", Colors.sectionHeader )
    , ( "margin-right", "5px" )
    , ( "cursor"
      , if isClickable then
            "pointer"

        else
            "default"
      )
    ]


checkButtonIcon : Bool -> List ( String, String )
checkButtonIcon isHighlighted =
    [ ( "height", "20px" )
    , ( "width", "20px" )
    , ( "margin", "4px" )
    , ( "background-image", "url(/public/images/baseline-refresh-24px.svg)" )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-size", "contain" )
    , ( "opacity"
      , if isHighlighted then
            "1"

        else
            "0.5"
      )
    ]
