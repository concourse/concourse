module Resource.Styles exposing
    ( checkStatusIcon
    , commentBar
    , commentBarContent
    , commentBarHeader
    , commentBarMessageIcon
    , commentBarPinIcon
    , enabledCheckbox
    , pinBar
    , pinBarTooltip
    , pinButton
    , pinIcon
    , versionHeader
    )

import Colors
import Pinned
import Resource.Models as Models


pinBar : { isPinned : Bool } -> List ( String, String )
pinBar { isPinned } =
    let
        borderColor =
            if isPinned then
                Colors.pinned

            else
                "#3d3c3c"
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
    , ( "background-color", Colors.pinTooltip )
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
    ]


commentBarContent : List ( String, String )
commentBarContent =
    [ ( "width", "700px" )
    , ( "margin", "auto" )
    , ( "padding", "20px" )
    ]


commentBarHeader : List ( String, String )
commentBarHeader =
    [ ( "display", "flex" )
    , ( "align-items", "center" )
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
