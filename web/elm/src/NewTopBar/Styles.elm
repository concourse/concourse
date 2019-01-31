module NewTopBar.Styles exposing
    ( breadcrumbComponent
    , breadcrumbContainer
    , concourseLogo
    , dropdownContainer
    , dropdownItem
    , loginComponent
    , loginContainer
    , loginItem
    , loginText
    , logoutButton
    , pageHeaderHeight
    , searchButton
    , searchClearButton
    , searchContainer
    , searchInput
    , showSearchContainer
    , topBar
    )

import ScreenSize exposing (ScreenSize(..))
import SearchBar exposing (SearchBar(..))


pageHeaderHeight : Float
pageHeaderHeight =
    54


topBar : List ( String, String )
topBar =
    [ ( "position", "fixed" )
    , ( "top", "0" )
    , ( "width", "100%" )
    , ( "z-index", "999" )
    , ( "display", "flex" )
    , ( "justify-content", "space-between" )
    , ( "background-color", "#1e1d1d" )
    , ( "font-weight", "700" )
    ]


showSearchContainer :
    { a
        | searchBar : SearchBar
        , screenSize : ScreenSize
        , highDensity : Bool
    }
    -> List ( String, String )
showSearchContainer { searchBar, screenSize, highDensity } =
    let
        flexLayout =
            if highDensity then
                []

            else
                [ ( "align-items"
                  , case searchBar of
                        Visible _ ->
                            case screenSize of
                                Mobile ->
                                    "stretch"

                                Desktop ->
                                    "center"

                                BigDesktop ->
                                    "center"

                        Minified ->
                            "flexStart"

                        Gone ->
                            Debug.log "attempting to show search container when search is gone" "center"
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
    , ( "border", "1px solid #504b4b" )
    , ( "color", "#fff" )
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
    , ( "color", "#504b4b" )
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


breadcrumbContainer : List ( String, String )
breadcrumbContainer =
    [ ( "display", "inline-block" )
    , ( "vertical-align", "middle" )
    , ( "font-size", "18px" )
    , ( "padding", "0 10px" )
    , ( "line-height", "54px" )
    ]


loginComponent : List ( String, String )
loginComponent =
    [ ( "max-width", "20%" ) ]


loginContainer : List ( String, String )
loginContainer =
    [ ( "position", "relative" )
    , ( "display", "flex" )
    , ( "flex-direction", "column" )
    , ( "border-left", "1px solid #3d3c3c" )
    , ( "line-height", "56px" )
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
                [ ( "background-color", "#1e1d1d" )
                , ( "color", "#fff" )
                ]

            else
                [ ( "background-color", "#2e2e2e" )
                , ( "color", "#9b9b9b" )
                ]
    in
    [ ( "padding", "0 42px" )
    , ( "line-height", "30px" )
    , ( "list-style-type", "none" )
    , ( "border", "1px solid #504b4b" )
    , ( "margin-top", "-1px" )
    , ( "font-size", "1.15em" )
    , ( "cursor", "pointer" )
    ]
        ++ coloration
