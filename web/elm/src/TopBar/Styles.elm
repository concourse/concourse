module TopBar.Styles exposing
    ( breadcrumbComponent
    , breadcrumbContainer
    , breadcrumbItem
    , concourseLogo
    , dropdownContainer
    , dropdownItem
    , loginComponent
    , loginContainer
    , loginItem
    , loginText
    , logoutButton
    , pageBelowTopBar
    , pageHeaderHeight
    , pageIncludingTopBar
    , pausePipelineButton
    , pinBadge
    , pinDropdownCursor
    , pinHoverHighlight
    , pinIcon
    , pinIconContainer
    , pinIconDropdown
    , pinText
    , pipelinePageBelowTopBar
    , searchButton
    , searchClearButton
    , searchContainer
    , searchInput
    , showSearchContainer
    , topBar
    )

import Colors
import ScreenSize exposing (ScreenSize(..))


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


showSearchContainer :
    { a
        | screenSize : ScreenSize
        , highDensity : Bool
    }
    -> List ( String, String )
showSearchContainer { screenSize, highDensity } =
    let
        flexLayout =
            if highDensity then
                []

            else
                [ ( "align-items", "flex-start" ) ]
    in
    [ ( "display", "flex" )
    , ( "flex-direction", "column" )
    , ( "flex-grow", "1" )
    , ( "justify-content", "center" )
    , ( "padding", "12px" )
    , ( "position", "relative" )
    ]
        ++ flexLayout


searchContainer : ScreenSize -> List ( String, String )
searchContainer screenSize =
    [ ( "display", "flex" )
    , ( "flex-direction", "column" )
    , ( "margin", "12px" )
    , ( "position", "relative" )
    , ( "align-items", "stretch" )
    ]
        ++ (case screenSize of
                Mobile ->
                    [ ( "flex-grow", "1" ) ]

                _ ->
                    []
           )


searchInput : ScreenSize -> List ( String, String )
searchInput screenSize =
    let
        widthStyles =
            case screenSize of
                Mobile ->
                    []

                Desktop ->
                    [ ( "width", "220px" ) ]

                BigDesktop ->
                    [ ( "width", "220px" ) ]
    in
    [ ( "background-color", "transparent" )
    , ( "background-image", "url('public/images/ic-search-white-24px.svg')" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "12px 8px" )
    , ( "height", "30px" )
    , ( "padding", "0 42px" )
    , ( "border", "1px solid " ++ Colors.inputOutline )
    , ( "color", Colors.dashboardText )
    , ( "font-size", "1.15em" )
    , ( "font-family", "Inconsolata, monospace" )
    , ( "outline", "0" )
    ]
        ++ widthStyles


searchClearButton : Bool -> List ( String, String )
searchClearButton active =
    let
        opacityValue =
            if active then
                "1"

            else
                "0.2"
    in
    [ ( "background-image", "url('public/images/ic-close-white-24px.svg')" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "10px 10px" )
    , ( "border", "0" )
    , ( "color", Colors.inputOutline )
    , ( "position", "absolute" )
    , ( "right", "0" )
    , ( "padding", "17px" )
    , ( "opacity", opacityValue )
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


pausePipelineButton : Bool -> List ( String, String )
pausePipelineButton isPaused =
    [ ( "background-image"
      , if isPaused then
            "url(/public/images/ic-play-white.svg)"

        else
            "url(/public/images/ic-pause-white.svg)"
      )
    , ( "display", "flex" )
    , ( "flex-direction", "column" )
    , ( "position", "relative" )
    , ( "padding", "10px" )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "border-left"
      , if isPaused then
            "1px solid rgba(255, 255, 255, 0.5)"

        else
            "1px solid #3d3c3c"
      )
    , ( "width", "34px" )
    , ( "height", "34px" )
    , ( "cursor", "pointer" )
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


dropdownContainer : ScreenSize -> List ( String, String )
dropdownContainer screenSize =
    [ ( "top", "100%" )
    , ( "margin", "0" )
    , ( "width", "100%" )
    ]
        ++ (case screenSize of
                Mobile ->
                    []

                _ ->
                    [ ( "position", "absolute" ) ]
           )


dropdownItem : Bool -> List ( String, String )
dropdownItem isSelected =
    let
        coloration =
            if isSelected then
                [ ( "background-color", Colors.frame )
                , ( "color", Colors.dashboardText )
                ]

            else
                [ ( "background-color", Colors.dropdownFaded )
                , ( "color", Colors.dropdownUnselectedText )
                ]
    in
    [ ( "padding", "0 42px" )
    , ( "line-height", "30px" )
    , ( "list-style-type", "none" )
    , ( "border", "1px solid " ++ Colors.inputOutline )
    , ( "margin-top", "-1px" )
    , ( "font-size", "1.15em" )
    , ( "cursor", "pointer" )
    ]
        ++ coloration


pinIconContainer : Bool -> List ( String, String )
pinIconContainer showBackground =
    [ ( "margin-right", "15px" )
    , ( "top", "10px" )
    , ( "position", "relative" )
    , ( "height", "40px" )
    , ( "display", "flex" )
    , ( "max-width", "20%" )
    ]
        ++ (if showBackground then
                [ ( "background-color", Colors.pinHighlight )
                , ( "border-radius", "50%" )
                ]

            else
                []
           )


pinIcon : List ( String, String )
pinIcon =
    [ ( "background-image", "url(/public/images/pin-ic-white.svg)" )
    , ( "width", "40px" )
    , ( "height", "40px" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "position", "relative" )
    ]


pinBadge : List ( String, String )
pinBadge =
    [ ( "background-color", Colors.pinned )
    , ( "border-radius", "50%" )
    , ( "width", "15px" )
    , ( "height", "15px" )
    , ( "position", "absolute" )
    , ( "top", "3px" )
    , ( "right", "3px" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    ]


pinIconDropdown : List ( String, String )
pinIconDropdown =
    [ ( "background-color", Colors.white )
    , ( "color", Colors.pinIconHover )
    , ( "position", "absolute" )
    , ( "top", "100%" )
    , ( "right", "0" )
    , ( "white-space", "nowrap" )
    , ( "list-style-type", "none" )
    , ( "padding", "10px" )
    , ( "margin-top", "0" )
    , ( "z-index", "1" )
    ]


pinHoverHighlight : List ( String, String )
pinHoverHighlight =
    [ ( "border-width", "5px" )
    , ( "border-style", "solid" )
    , ( "border-color", "transparent transparent " ++ Colors.white ++ " transparent" )
    , ( "position", "absolute" )
    , ( "top", "100%" )
    , ( "right", "50%" )
    , ( "margin-right", "-5px" )
    , ( "margin-top", "-10px" )
    ]


pinDropdownCursor : List ( String, String )
pinDropdownCursor =
    [ ( "cursor", "pointer" ) ]


pinText : List ( String, String )
pinText =
    [ ( "font-weight", "700" ) ]
