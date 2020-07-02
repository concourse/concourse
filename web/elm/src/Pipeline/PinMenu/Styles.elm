module Pipeline.PinMenu.Styles exposing
    ( pinBadge
    , pinIcon
    , pinIconBackground
    , pinIconDropdown
    , pinIconDropdownItem
    , title
    )

import Assets
import Colors
import Html
import Html.Attributes exposing (style)
import Pipeline.PinMenu.Views
    exposing
        ( Background(..)
        , Distance(..)
        , Position(..)
        )
import SideBar.Styles as SS
import Views.Styles


pinIconBackground :
    { a | background : Background, clickable : Bool }
    -> List (Html.Attribute msg)
pinIconBackground { background, clickable } =
    [ style "position" "relative"
    , style "border-left" <| "1px solid " ++ Colors.background
    , style "border-bottom" <| "1px solid " ++ Colors.frame
    , style "width" "54px"
    , style "height" "54px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "justify-content" "center"
    , style "cursor" <|
        if clickable then
            "pointer"

        else
            "default"
    , style "background-color" <|
        case background of
            Light ->
                Colors.sideBar

            Dark ->
                Colors.frame
    ]


pinIcon : { a | opacity : SS.Opacity } -> List (Html.Attribute msg)
pinIcon { opacity } =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just Assets.PinIconWhite
    , style "width" "18px"
    , style "height" "18px"
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "background-size" "contain"
    , style "position" "relative"
    , SS.opacityAttr opacity
    ]


pinBadge :
    { a
        | color : String
        , diameterPx : Int
        , position : Position
    }
    -> List (Html.Attribute msg)
pinBadge { color, diameterPx, position } =
    case position of
        TopRight top right ->
            [ style "background-color" color
            , style "border-radius" "50%"
            , style "width" <| String.fromInt diameterPx ++ "px"
            , style "height" <| String.fromInt diameterPx ++ "px"
            , style "position" "absolute"
            , style "top" <|
                case top of
                    Px px ->
                        String.fromInt px ++ "px"

                    Percent pct ->
                        String.fromInt pct ++ "%"
            , style "right" <|
                case right of
                    Px px ->
                        String.fromInt px ++ "px"

                    Percent pct ->
                        String.fromInt pct ++ "%"
            , style "display" "flex"
            , style "align-items" "center"
            , style "justify-content" "center"
            ]


pinIconDropdown : { a | position : Position } -> List (Html.Attribute msg)
pinIconDropdown { position } =
    case position of
        TopRight top right ->
            [ style "color" Colors.pinIconHover
            , style "position" "absolute"
            , style "top" <|
                case top of
                    Px px ->
                        String.fromInt px ++ "px"

                    Percent pct ->
                        String.fromInt pct ++ "%"
            , style "right" <|
                case right of
                    Px px ->
                        String.fromInt px ++ "px"

                    Percent pct ->
                        String.fromInt pct ++ "%"
            , style "white-space" "nowrap"
            , style "list-style-type" "none"
            , style "margin-top" "1px"
            , style "margin-right" "-2px"
            , style "z-index" "1"
            ]


pinIconDropdownItem :
    { a | paddingPx : Int, background : String }
    -> List (Html.Attribute msg)
pinIconDropdownItem { paddingPx, background } =
    [ style "padding" <| String.fromInt paddingPx ++ "px"
    , style "background-color" background
    , style "cursor" "pointer"
    , style "font-weight" Views.Styles.fontWeightLight
    , style "border-width" "0 1px 1px 1px"
    , style "border-style" "solid"
    , style "border-color" Colors.frame
    ]


title : { a | fontWeight : String, color : String } -> List (Html.Attribute msg)
title { fontWeight, color } =
    [ style "font-weight" fontWeight
    , style "color" color
    ]
