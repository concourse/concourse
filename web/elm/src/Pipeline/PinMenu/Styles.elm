module Pipeline.PinMenu.Styles exposing
    ( pinBadge
    , pinIcon
    , pinIconBackground
    , pinIconDropdown
    , pinIconDropdownItem
    , title
    )

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


pinIconBackground : { a | background : Background } -> List (Html.Attribute msg)
pinIconBackground { background } =
    [ style "position" "relative"
    , style "border-left" <| "1px solid " ++ Colors.background
    , style "background-color" <|
        case background of
            Light ->
                Colors.sideBar

            Dark ->
                Colors.frame
    ]


pinIcon : { a | opacity : SS.Opacity, clickable : Bool } -> List (Html.Attribute msg)
pinIcon { opacity, clickable } =
    [ style "background-image" "url(/public/images/pin-ic-white.svg)"
    , style "width" "54px"
    , style "height" "54px"
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "position" "relative"
    , style "cursor" <|
        if clickable then
            "pointer"

        else
            "default"
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
            , style "margin-top" "0"
            , style "z-index" "1"
            ]


pinIconDropdownItem :
    { a | paddingPx : Int, background : String }
    -> List (Html.Attribute msg)
pinIconDropdownItem { paddingPx, background } =
    [ style "padding" <| String.fromInt paddingPx ++ "px"
    , style "background-color" background
    , style "cursor" "pointer"
    , style "font-weight" "400"
    , style "border" <| "1px solid " ++ Colors.groupBorderSelected
    ]


title : { a | fontWeight : Int, color : String } -> List (Html.Attribute msg)
title { fontWeight, color } =
    [ style "font-weight" <| String.fromInt fontWeight
    , style "color" color
    ]
