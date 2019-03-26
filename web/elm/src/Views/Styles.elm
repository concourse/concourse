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
import Routes


pageHeaderHeight : Float
pageHeaderHeight =
    54


pageIncludingTopBar : List ( String, String )
pageIncludingTopBar =
    [ ( "-webkit-font-smoothing", "antialiased" )
    , ( "font-weight", "700" )
    , ( "height", "100%" )
    ]


pageBelowTopBar : Routes.Route -> List ( String, String )
pageBelowTopBar route =
    [ ( "padding-top", "54px" )
    , ( "height", "100%" )
    ]
        ++ (case route of
                Routes.Pipeline _ ->
                    [ ( "box-sizing", "border-box" ) ]

                Routes.Dashboard _ ->
                    [ ( "box-sizing", "border-box" )
                    , ( "display", "flex" )
                    , ( "padding-bottom", "50px" )
                    ]

                _ ->
                    []
           )


topBar : Bool -> List ( String, String )
topBar isPaused =
    [ ( "position", "fixed" )
    , ( "top", "0" )
    , ( "width", "100%" )
    , ( "z-index", "999" )
    , ( "display", "flex" )
    , ( "justify-content", "space-between" )
    , ( "font-weight", "700" )
    , ( "background-color"
      , if isPaused then
            Colors.paused

        else
            Colors.frame
      )
    ]


concourseLogo : List ( String, String )
concourseLogo =
    [ ( "background-image", "url(/public/images/concourse-logo-white.svg)" )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-size", "42px 42px" )
    , ( "display", "inline-block" )
    , ( "width", "54px" )
    , ( "height", "54px" )
    ]


breadcrumbContainer : List ( String, String )
breadcrumbContainer =
    [ ( "flex-grow", "1" ) ]


breadcrumbComponent : String -> List ( String, String )
breadcrumbComponent componentType =
    [ ( "background-image", "url(/public/images/ic-breadcrumb-" ++ componentType ++ ".svg)" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-size", "contain" )
    , ( "display", "inline-block" )
    , ( "vertical-align", "middle" )
    , ( "height", "16px" )
    , ( "width", "32px" )
    , ( "margin-right", "10px" )
    ]


breadcrumbItem : Bool -> List ( String, String )
breadcrumbItem clickable =
    [ ( "display", "inline-block" )
    , ( "vertical-align", "middle" )
    , ( "font-size", "18px" )
    , ( "padding", "0 10px" )
    , ( "line-height", "54px" )
    , ( "cursor"
      , if clickable then
            "pointer"

        else
            "default"
      )
    ]


pauseToggleIcon :
    { isHovered : Bool
    , isClickable : Bool
    , margin : String
    }
    -> List ( String, String )
pauseToggleIcon { isHovered, isClickable, margin } =
    [ ( "margin", margin )
    , ( "opacity"
      , if isHovered then
            "1"

        else
            "0.5"
      )
    , ( "cursor"
      , if isClickable then
            "pointer"

        else
            "default"
      )
    ]
