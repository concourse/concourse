module NewTopBar.Styles exposing
    ( breadcrumbComponentCSS
    , breadcrumbContainerCSS
    , concourseLogoCSS
    , dropdownContainerCSS
    , dropdownItemCSS
    , loginContainerCSS
    , loginItemCSS
    , logoutButtonCSS
    , pageHeaderHeight
    , searchButtonCSS
    , searchClearButtonCSS
    , searchContainerCSS
    , searchInputCSS
    , showSearchContainerCSS
    , topBarCSS
    )

import ScreenSize exposing (ScreenSize(..))
import SearchBar exposing (SearchBar(..))


pageHeaderHeight : Float
pageHeaderHeight =
    54


topBarCSS : List ( String, String )
topBarCSS =
    [ ( "position", "fixed" )
    , ( "top", "0" )
    , ( "width", "100%" )
    , ( "z-index", "999" )
    , ( "display", "flex" )
    , ( "justify-content", "space-between" )
    , ( "background-color", "#1e1d1d" )
    , ( "font-weight", "700" )
    ]


showSearchContainerCSS :
    { a
        | searchBar : SearchBar
        , screenSize : ScreenSize
        , highDensity : Bool
    }
    -> List ( String, String )
showSearchContainerCSS { searchBar, screenSize, highDensity } =
    let
        flexLayout =
            if highDensity then
                []

            else
                [ ( "align-items"
                  , case searchBar of
                        Expanded _ ->
                            case screenSize of
                                Mobile ->
                                    "stretch"

                                Desktop ->
                                    "center"

                                BigDesktop ->
                                    "center"

                        Collapsed ->
                            "flexStart"
                  )
                ]
    in
    [ ( "display", "flex" )
    , ( "flex-direction", "column" )
    , ( "flex-grow", "1" )
    , ( "justify-content", "center" )
    , ( "padding", "12px" )
    , ( "position", "relative" )
    ]
        ++ flexLayout


searchContainerCSS : ScreenSize -> List ( String, String )
searchContainerCSS screenSize =
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


searchInputCSS : ScreenSize -> List ( String, String )
searchInputCSS screenSize =
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
    , ( "border", "1px solid #504b4b" )
    , ( "color", "#fff" )
    , ( "font-size", "1.15em" )
    , ( "font-family", "Inconsolata, monospace" )
    , ( "outline", "0" )
    ]
        ++ widthStyles


searchClearButtonCSS : Bool -> List ( String, String )
searchClearButtonCSS active =
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
    , ( "color", "#504b4b" )
    , ( "position", "absolute" )
    , ( "right", "0" )
    , ( "padding", "17px" )
    , ( "opacity", opacityValue )
    ]


searchButtonCSS : List ( String, String )
searchButtonCSS =
    [ ( "background-image", "url('public/images/ic-search-white-24px.svg')" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "12px 8px" )
    , ( "height", "32px" )
    , ( "width", "32px" )
    , ( "display", "inline-block" )
    , ( "float", "left" )
    ]


logoutButtonCSS : List ( String, String )
logoutButtonCSS =
    [ ( "position", "absolute" )
    , ( "top", "55px" )
    , ( "background-color", "#1e1d1d" )
    , ( "height", "54px" )
    , ( "width", "100%" )
    , ( "border-top", "1px solid #3d3c3c" )
    , ( "cursor", "pointer" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    , ( "flex-grow", "1" )
    ]


concourseLogoCSS : List ( String, String )
concourseLogoCSS =
    [ ( "background-image", "url(/public/images/concourse-logo-white.svg)" )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-size", "42px 42px" )
    , ( "display", "inline-block" )
    , ( "width", "54px" )
    , ( "height", "54px" )
    ]


breadcrumbComponentCSS : String -> List ( String, String )
breadcrumbComponentCSS componentType =
    [ ( "background-image", "url(/public/images/ic-breadcrumb-" ++ componentType ++ ".svg)" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-size", "contain" )
    , ( "display", "inline-block" )
    , ( "vertical-align", "middle" )
    , ( "height", "16px" )
    , ( "width", "32px" )
    , ( "margin-right", "10px" )
    ]


breadcrumbContainerCSS : List ( String, String )
breadcrumbContainerCSS =
    [ ( "display", "inline-block" )
    , ( "vertical-align", "middle" )
    , ( "font-size", "18px" )
    , ( "padding", "0 10px" )
    , ( "line-height", "54px" )
    ]


loginContainerCSS : List ( String, String )
loginContainerCSS =
    [ ( "position", "relative" )
    , ( "display", "flex" )
    , ( "flex-direction", "column" )
    , ( "border-left", "1px solid #3d3c3c" )
    , ( "line-height", "56px" )
    ]


loginItemCSS : List ( String, String )
loginItemCSS =
    [ ( "padding", "0 30px" )
    , ( "cursor", "pointer" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "justify-content", "center" )
    , ( "flex-grow", "1" )
    ]


dropdownContainerCSS : ScreenSize -> List ( String, String )
dropdownContainerCSS screenSize =
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


dropdownItemCSS : List ( String, String )
dropdownItemCSS =
    [ ( "padding", "0 42px" )
    , ( "background-color", "#2e2e2e" )
    , ( "line-height", "30px" )
    , ( "list-style-type", "none" )
    , ( "border", "1px solid #504b4b" )
    , ( "margin-top", "-1px" )
    , ( "color", "#9b9b9b" )
    , ( "font-size", "1.15em" )
    , ( "cursor", "pointer" )
    ]
