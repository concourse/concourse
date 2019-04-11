module Views.Styles exposing
    ( breadcrumbComponent
    , breadcrumbContainer
    , breadcrumbItem
    , concourseLogo
    , pageBelowTopBar
    , pageHeaderHeight
    , pageIncludingTopBar
    , pauseToggleIcon
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
                    [ style "height" "100%" ]

                Routes.Pipeline _ ->
                    [ style "box-sizing" "border-box"
                    , style "height" "100%"
                    ]

                Routes.Dashboard _ ->
                    [ style "box-sizing" "border-box"
                    , style "display" "flex"
                    , style "padding-bottom" "50px"
                    , style "height" "100%"
                    ]

                _ ->
                    []
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


pauseToggleIcon :
    { isHovered : Bool
    , isClickable : Bool
    , margin : String
    }
    -> List (Html.Attribute msg)
pauseToggleIcon { isHovered, isClickable, margin } =
    [ style "margin" margin
    , style "opacity" <|
        if isHovered then
            "1"

        else
            "0.5"
    , style "cursor" <|
        if isClickable then
            "pointer"

        else
            "default"
    ]
