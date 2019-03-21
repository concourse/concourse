module TopBar.Styles exposing
    ( breadcrumbComponent
    , breadcrumbContainer
    , breadcrumbItem
    , concourseLogo
    , loginComponent
    , loginContainer
    , loginItem
    , loginText
    , logoutButton
    , pageBelowTopBar
    , pageHeaderHeight
    , pageIncludingTopBar
    , pauseToggleIcon
    , pipelinePageBelowTopBar
    , searchButton
    , topBar
    )

import Colors


pageHeaderHeight : Float
pageHeaderHeight =
    54


pageIncludingTopBar : List ( String, String )
pageIncludingTopBar =
    [ ( "-webkit-font-smoothing", "antialiased" )
    , ( "font-weight", "700" )
    , ( "height", "100%" )
    ]


pageBelowTopBar : List ( String, String )
pageBelowTopBar =
    [ ( "padding-top", "54px" )
    , ( "height", "100%" )
    , ( "padding-bottom", "50px" )
    , ( "box-sizing", "border-box" )
    , ( "display", "flex" )
    ]


pipelinePageBelowTopBar : List ( String, String )
pipelinePageBelowTopBar =
    [ ( "padding-top", "0" )
    , ( "height", "100%" )
    ]


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


searchButton : List ( String, String )
searchButton =
    [ ( "background-image", "url('public/images/ic-search-white-24px.svg')" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "12px 8px" )
    , ( "height", "32px" )
    , ( "width", "32px" )
    , ( "display", "inline-block" )
    , ( "float", "left" )
    ]


logoutButton : List ( String, String )
logoutButton =
    [ ( "position", "absolute" )
    , ( "top", "55px" )
    , ( "background-color", Colors.frame )
    , ( "height", "54px" )
    , ( "width", "100%" )
    , ( "border-top", "1px solid " ++ Colors.background )
    , ( "cursor", "pointer" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    , ( "flex-grow", "1" )
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


loginComponent : List ( String, String )
loginComponent =
    [ ( "max-width", "20%" ) ]


loginContainer : Bool -> List ( String, String )
loginContainer isPaused =
    [ ( "position", "relative" )
    , ( "display", "flex" )
    , ( "flex-direction", "column" )
    , ( "border-left"
      , "1px solid "
            ++ (if isPaused then
                    Colors.pausedTopbarSeparator

                else
                    Colors.background
               )
      )
    , ( "line-height", "54px" )
    ]


loginItem : List ( String, String )
loginItem =
    [ ( "padding", "0 30px" )
    , ( "cursor", "pointer" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    , ( "flex-grow", "1" )
    ]


loginText : List ( String, String )
loginText =
    [ ( "overflow", "hidden" )
    , ( "text-overflow", "ellipsis" )
    ]
