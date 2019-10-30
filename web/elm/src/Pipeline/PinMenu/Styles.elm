module Pipeline.PinMenu.Styles exposing
    ( pinBadge
    , pinIcon
    , pinIconDropdown
    , title
    )

import Colors
import Html
import Html.Attributes exposing (style)
import Pipeline.PinMenu.Views
    exposing
        ( Background(..)
        , Brightness(..)
        , Distance(..)
        , Position(..)
        )


pinIcon : { a | iconStyle : Brightness, background : Background } -> List (Html.Attribute msg)
pinIcon { iconStyle, background } =
    [ style "position" "relative"
    , style "display" "flex"
    , style "max-width" "20%"
    , style "background-image" "url(/public/images/pin-ic-white.svg)"
    , style "border-left" <| "1px solid " ++ Colors.background
    , style "width" "54px"
    , style "height" "54px"
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "position" "relative"
    , style "opacity" <|
        case iconStyle of
            Bright ->
                "1"

            Dim ->
                "0.5"
    ]
        ++ (case background of
                Light ->
                    [ style "background-color" Colors.secondaryTopBar ]

                Dark ->
                    []
           )


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


pinIconDropdown :
    { a
        | background : String
        , position : Position
        , paddingPx : Int
    }
    -> List (Html.Attribute msg)
pinIconDropdown { background, position, paddingPx } =
    case position of
        TopRight top right ->
            [ style "background-color" background
            , style "color" Colors.pinIconHover
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
            , style "padding" <| String.fromInt paddingPx ++ "px"
            , style "margin-top" "0"
            , style "z-index" "1"
            ]


title : { a | fontWeight : Int, color : String } -> List (Html.Attribute msg)
title { fontWeight, color } =
    [ style "font-weight" <| String.fromInt fontWeight
    , style "color" color
    ]
