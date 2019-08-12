module Views.Styles exposing
    ( TooltipPosition(..)
    , breadcrumbComponent
    , breadcrumbContainer
    , breadcrumbItem
    , concourseLogo
    , pageBelowTopBar
    , pageHeaderHeight
    , pageIncludingTopBar
    , pauseToggle
    , pauseToggleIcon
    , pauseToggleTooltip
    , topBar
    )

import Colors
import Html
import Html.Attributes exposing (style)
import Routes


pageHeaderHeight : Float
pageHeaderHeight =
    54


pageIncludingTopBar : List (Html.Attribute msg)
pageIncludingTopBar =
    [ style "-webkit-font-smoothing" "antialiased"
    , style "font-weight" "700"
    , style "height" "100%"
    ]


pageBelowTopBar : Routes.Route -> List (Html.Attribute msg)
pageBelowTopBar route =
    style "padding-top" "54px"
        :: (case route of
                Routes.FlySuccess _ ->
                    [ style "height" "100%" ]

                Routes.Resource _ ->
                    [ style "box-sizing" "border-box"
                    , style "height" "100%"
                    , style "display" "flex"
                    ]

                Routes.Pipeline _ ->
                    [ style "box-sizing" "border-box"
                    , style "height" "100%"
                    , style "display" "flex"
                    ]

                Routes.Dashboard _ ->
                    [ style "box-sizing" "border-box"
                    , style "display" "flex"
                    , style "height" "100%"
                    , style "padding-bottom" "50px"
                    ]

                Routes.Build _ ->
                    [ style "box-sizing" "border-box"
                    , style "height" "100%"
                    , style "display" "flex"
                    ]

                Routes.OneOffBuild _ ->
                    [ style "box-sizing" "border-box"
                    , style "height" "100%"
                    , style "display" "flex"
                    ]

                Routes.Job _ ->
                    [ style "box-sizing" "border-box"
                    , style "height" "100%"
                    , style "display" "flex"
                    ]
           )


topBar : Bool -> List (Html.Attribute msg)
topBar isPaused =
    [ style "position" "fixed"
    , style "top" "0"
    , style "width" "100%"
    , style "height" "54px"
    , style "z-index" "999"
    , style "display" "flex"
    , style "justify-content" "space-between"
    , style "font-weight" "700"
    , style "background-color" <|
        if isPaused then
            Colors.paused

        else
            Colors.frame
    , style "border-bottom" <| "1px solid " ++ Colors.frame
    ]


concourseLogo : List (Html.Attribute msg)
concourseLogo =
    [ style "background-image" "url(/public/images/concourse-logo-white.svg)"
    , style "background-position" "50% 50%"
    , style "background-repeat" "no-repeat"
    , style "background-size" "42px 42px"
    , style "display" "inline-block"
    , style "width" "54px"
    , style "height" "54px"
    ]


breadcrumbContainer : List (Html.Attribute msg)
breadcrumbContainer =
    [ style "flex-grow" "1" ]


breadcrumbComponent : String -> List (Html.Attribute msg)
breadcrumbComponent componentType =
    [ style "background-image" <| "url(/public/images/ic-breadcrumb-" ++ componentType ++ ".svg)"
    , style "background-repeat" "no-repeat"
    , style "background-size" "contain"
    , style "display" "inline-block"
    , style "vertical-align" "middle"
    , style "height" "16px"
    , style "width" "32px"
    , style "margin-right" "10px"
    ]


breadcrumbItem : Bool -> List (Html.Attribute msg)
breadcrumbItem clickable =
    [ style "display" "inline-block"
    , style "vertical-align" "middle"
    , style "font-size" "18px"
    , style "padding" "0 10px"
    , style "line-height" "54px"
    , style "cursor" <|
        if clickable then
            "pointer"

        else
            "default"
    ]


pauseToggle : String -> List (Html.Attribute msg)
pauseToggle margin =
    [ style "position" "relative"
    , style "margin" margin
    ]


pauseToggleIcon :
    { isHovered : Bool
    , isClickable : Bool
    }
    -> List (Html.Attribute msg)
pauseToggleIcon { isHovered, isClickable } =
    [ style "opacity" <|
        if not isClickable then
            "0.2"

        else if isHovered then
            "1"

        else
            "0.5"
    , style "cursor" <|
        if isClickable then
            "pointer"

        else
            "default"
    ]


type TooltipPosition
    = Above
    | Below


pauseToggleTooltip : TooltipPosition -> List (Html.Attribute msg)
pauseToggleTooltip ttp =
    [ style "background-color" "#9b9b9b"
    , style "position" "absolute"
    , style
        (case ttp of
            Above ->
                "bottom"

            Below ->
                "top"
        )
        "100%"
    , style "white-space" "nowrap"
    , style "padding" "2.5px"
    , style "margin-bottom" "5px"
    , style "right" "-150%"
    , style "z-index" "1"
    ]
