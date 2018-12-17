module NewTopBar.Styles exposing (..)

import Css exposing (..)
import ScreenSize exposing (ScreenSize(..))
import SearchBar exposing (SearchBar(..))


pageHeaderHeight : Float
pageHeaderHeight =
    54


topBar : List Style
topBar =
    [ position fixed
    , top zero
    , width <| pct 100
    , zIndex <| int 999
    , displayFlex
    , justifyContent spaceBetween
    , backgroundColor <| hex "1e1d1d"
    ]


concourseLogo : List Style
concourseLogo =
    [ backgroundImage <| url "public/images/concourse_logo_white.svg"
    , backgroundSize2 (px 42) (px 42)
    , backgroundPosition center
    , backgroundRepeat noRepeat
    , display inlineBlock
    , height <| px pageHeaderHeight
    , width <| px 54
    ]


middleSection :
    { a
        | searchBar : SearchBar
        , screenSize : ScreenSize
        , highDensity : Bool
    }
    -> List Style
middleSection { searchBar, screenSize, highDensity } =
    let
        flexLayout =
            if highDensity then
                []
            else
                case searchBar of
                    Expanded _ ->
                        case screenSize of
                            Mobile ->
                                [ alignItems stretch ]

                            Desktop ->
                                [ alignItems center ]

                            BigDesktop ->
                                [ alignItems center ]

                    Collapsed ->
                        [ alignItems flexStart ]
    in
        [ displayFlex
        , flexDirection column
        , flexGrow <| num 1
        , justifyContent center
        , padding <| px 12
        , position relative
        ]
            ++ flexLayout


searchForm : List Style
searchForm =
    [ position relative
    , displayFlex
    , flexDirection column
    , alignItems stretch
    ]


searchInput : ScreenSize -> List Style
searchInput screenSize =
    let
        widthStyles =
            case screenSize of
                Mobile ->
                    []

                Desktop ->
                    [ width <| px 220 ]

                BigDesktop ->
                    [ width <| px 220 ]
    in
        [ backgroundColor transparent
        , backgroundImage <| url "public/images/ic_search_white_24px.svg"
        , backgroundRepeat noRepeat
        , backgroundPosition2 (px 12) (px 8)
        , border3 (px 1) solid (hex "504b4b")
        , color <| hex "fff"
        , fontSize <| em 1.15
        , fontFamilies [ "Inconsolata", .value monospace ]
        , height <| px 30
        , padding2 zero <| px 42
        , focus
            [ border3 (px 1) solid (hex "504b4b")
            , outline zero
            ]
        ]
            ++ widthStyles


searchClearButton : Bool -> List Style
searchClearButton active =
    let
        opacityValue =
            if active then
                1
            else
                0.2
    in
        [ backgroundImage <| url "public/images/ic_close_white_24px.svg"
        , backgroundPosition2 (px 10) (px 10)
        , backgroundRepeat noRepeat
        , border zero
        , cursor pointer
        , color <| hex "504b4b"
        , cursor pointer
        , position absolute
        , right zero
        , padding <| px 17
        , opacity <| num opacityValue
        ]


searchOptionsList : ScreenSize -> List Style
searchOptionsList screenSize =
    case screenSize of
        Mobile ->
            [ margin zero ]

        Desktop ->
            [ position absolute
            , top <| px 32
            ]

        BigDesktop ->
            [ position absolute
            , top <| px 32
            ]


searchOption : { screenSize : ScreenSize, active : Bool } -> List Style
searchOption { screenSize, active } =
    let
        widthStyles =
            case screenSize of
                Mobile ->
                    []

                Desktop ->
                    [ width <| px 220 ]

                BigDesktop ->
                    [ width <| px 220 ]

        activeStyles =
            if active then
                [ backgroundColor <| hex "1e1d1d"
                , color <| hex "fff"
                ]
            else
                []

        layout =
            [ marginTop <| px -1
            , border3 (px 1) solid (hex "504b4b")
            , textAlign left
            , lineHeight <| px 30
            , padding2 zero <| px 42
            ]

        styling =
            [ listStyleType none
            , backgroundColor <| hex "2e2e2e"
            , fontSize <| em 1.15
            , cursor pointer
            , color <| hex "9b9b9b"
            ]
    in
        layout ++ styling ++ widthStyles ++ activeStyles


searchButton : List Style
searchButton =
    [ backgroundImage (url "public/images/ic_search_white_24px.svg")
    , backgroundRepeat noRepeat
    , backgroundPosition2 (px 13) (px 9)
    , height (px 32)
    , width (px 32)
    , display inlineBlock
    , float left
    ]


userInfo : List Style
userInfo =
    [ position relative
    , maxWidth (pct 20)
    , displayFlex
    , flexDirection column
    , borderLeft3 (px 1) solid (hex "3d3c3c")
    ]


menuItem : List Style
menuItem =
    [ cursor pointer
    , displayFlex
    , alignItems center
    , justifyContent center
    , flexGrow (num 1)
    ]


menuButton : List Style
menuButton =
    [ padding2 zero (px 30)
    ]
        ++ menuItem


userName : List Style
userName =
    [ overflow hidden
    , textOverflow ellipsis
    ]


logoutButton : List Style
logoutButton =
    [ position absolute
    , top (px <| pageHeaderHeight + 1)
    , backgroundColor (hex "1e1d1d")
    , height (px pageHeaderHeight)
    , width (pct 100)
    , borderTop3 (px 1) solid (hex "3d3c3c")
    , hover [ backgroundColor (hex "2a2929") ]
    ]
        ++ menuItem
