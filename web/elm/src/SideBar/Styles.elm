module SideBar.Styles exposing
    ( Background(..)
    , FontWeight(..)
    , Opacity(..)
    , SidebarElementColor(..)
    , collapseIcon
    , column
    , favoriteIcon
    , iconGroup
    , instanceGroup
    , instanceGroupBadge
    , opacityAttr
    , pipeline
    , pipelineIcon
    , pipelineName
    , pipelineTextIcon
    , sectionHeader
    , sideBar
    , sideBarHandle
    , sideBarMenu
    , starPadding
    , starWidth
    , team
    , teamHeader
    , teamIcon
    , teamName
    , tooltipArrowSize
    , tooltipBody
    , tooltipOffset
    )

import Assets
import ColorValues
import Colors
import Html
import Html.Attributes as Attr exposing (style)
import Tooltip
import Views.Icon as Icon
import Views.Styles


starPadding : Float
starPadding =
    10


starWidth : Float
starWidth =
    18


tooltipArrowSize : Float
tooltipArrowSize =
    15


tooltipOffset : Float
tooltipOffset =
    3


sideBar : { r | width : Float } -> List (Html.Attribute msg)
sideBar { width } =
    [ style "border-right" <| "1px solid " ++ Colors.border
    , style "background-color" Colors.sideBarBackground
    , style "width" <| String.fromFloat width ++ "px"
    , style "overflow-y" "auto"
    , style "height" "100%"
    , style "flex-shrink" "0"
    , style "box-sizing" "border-box"
    , style "padding-bottom" "10px"
    , style "-webkit-overflow-scrolling" "touch"
    , style "position" "relative"
    ]


sideBarHandle : { r | width : Float } -> List (Html.Attribute msg)
sideBarHandle { width } =
    [ style "position" "fixed"
    , style "width" "10px"
    , style "height" "100%"
    , style "top" "0"
    , style "left" <| String.fromFloat (width - 5) ++ "px"
    , style "z-index" "2"
    , style "cursor" "col-resize"
    ]


column : List (Html.Attribute msg)
column =
    [ style "display" "flex"
    , style "flex-direction" "column"
    ]


sectionHeader : List (Html.Attribute msg)
sectionHeader =
    [ style "font-size" "14px"
    , style "overflow" "hidden"
    , style "white-space" "nowrap"
    , style "text-overflow" "ellipsis"
    , style "padding" "15px 5px 5px 10px"
    , fontWeightAttr Bold
    ]


teamHeader : { a | background : Background } -> List (Html.Attribute msg)
teamHeader { background } =
    [ style "display" "flex"
    , style "cursor" "pointer"
    , style "align-items" "center"
    , backgroundAttr background
    ]


iconGroup : List (Html.Attribute msg)
iconGroup =
    [ style "width" "54px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "justify-content" "space-between"
    , style "padding" "5px"
    , style "box-sizing" "border-box"
    , style "flex-shrink" "0"
    ]


type Opacity
    = Dim
    | GreyedOut
    | Bright


opacityAttr : Opacity -> Html.Attribute msg
opacityAttr opacity =
    style "opacity" <|
        case opacity of
            Dim ->
                "0.5"

            GreyedOut ->
                "0.7"

            Bright ->
                "1"


type SidebarElementColor
    = Grey
    | LightGrey
    | White


genericColorAttr : String -> SidebarElementColor -> Html.Attribute msg
genericColorAttr attr color =
    style attr <|
        case color of
            Grey ->
                Colors.sideBarTextDim

            LightGrey ->
                Colors.sideBarTextBright

            White ->
                Colors.white


colorAttr : SidebarElementColor -> Html.Attribute msg
colorAttr =
    genericColorAttr "color"


backgroundColorAttr : SidebarElementColor -> Html.Attribute msg
backgroundColorAttr =
    genericColorAttr "background-color"


teamIcon : Html.Html msg
teamIcon =
    Icon.icon
        { sizePx = 18
        , image = Assets.PeopleIcon
        }
        [ style "margin-left" "8px"
        , style "background-size" "contain"
        , style "flex-shrink" "0"
        ]


collapseIcon : { opacity : Opacity, asset : Assets.Asset } -> Html.Html msg
collapseIcon { asset } =
    Icon.icon
        { sizePx = 10
        , image = asset
        }
        [ style "margin-left" "10px"
        , style "flex-shrink" "0"
        ]


teamName :
    { a | color : SidebarElementColor }
    -> List (Html.Attribute msg)
teamName { color } =
    [ style "font-size" "14px"
    , style "padding" "5px 2.5px"
    , style "margin-left" "5px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "flex-grow" "1"
    , style "color" Colors.sideBarTextBright
    , colorAttr color
    , fontWeightAttr Bold
    ]


type Background
    = Dark
    | Light
    | Invisible


backgroundAttr : Background -> Html.Attribute msg
backgroundAttr background =
    style "background-color" <|
        case background of
            Dark ->
                Colors.sideBarActive

            Light ->
                Colors.sideBarHovered

            Invisible ->
                "inherit"


type FontWeight
    = Default
    | Bold


fontWeightAttr : FontWeight -> Html.Attribute msg
fontWeightAttr weight =
    style "font-weight" <|
        case weight of
            Default ->
                Views.Styles.fontWeightLight

            Bold ->
                Views.Styles.fontWeightBold


pipelineName :
    { a | color : SidebarElementColor, weight : FontWeight }
    -> List (Html.Attribute msg)
pipelineName { color, weight } =
    [ style "font-size" "14px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "padding" "5px 2.5px"
    , style "margin-left" "5px"
    , style "flex-grow" "1"
    , colorAttr color
    , fontWeightAttr weight
    ]


sideBarMenu :
    Bool
    -> List (Html.Attribute msg)
sideBarMenu isClickable =
    [ style "border-right" <| "1px solid " ++ Colors.border
    , style "opacity" "1"
    , style "padding" "16px"
    , style "cursor" <|
        if isClickable then
            "pointer"

        else
            "default"
    , style "background-color" Colors.sideBarIconBackground
    ]


team : List (Html.Attribute msg)
team =
    [ style "padding-top" "5px", style "line-height" "1.2" ] ++ column


pipeline : { a | background : Background } -> List (Html.Attribute msg)
pipeline { background } =
    [ style "display" "flex"
    , style "align-items" "center"
    , style "margin-top" "2px"
    , backgroundAttr background
    ]


instanceGroup : { a | background : Background } -> List (Html.Attribute msg)
instanceGroup =
    pipeline


pipelineIcon : Assets.Asset -> List (Html.Attribute msg)
pipelineIcon asset =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just asset
    , style "background-repeat" "no-repeat"
    , style "height" "18px"
    , style "width" "18px"
    , style "background-size" "contain"
    , style "background-position" "center"
    , style "margin-left" "28px"
    , style "flex-shrink" "0"
    ]


pipelineTextIcon : List (Html.Attribute msg)
pipelineTextIcon =
    [ style "height" "18px"
    , style "width" "18px"
    , style "margin-left" "28px"
    , style "flex-shrink" "0"
    , style "display" "flex"
    , style "justify-content" "center"
    , style "align-items" "center"
    , style "font-size" "16px"
    ]


instanceGroupBadge : { count : Int, color : SidebarElementColor } -> Html.Html msg
instanceGroupBadge { count, color } =
    let
        ( text, fontSize ) =
            if count > 99 then
                ( "99+", "10px" )

            else
                ( String.fromInt count, "12px" )
    in
    Html.div
        [ style "border-radius" "4px"
        , style "color" ColorValues.grey90
        , style "display" "flex"
        , style "align-items" "center"
        , style "justify-content" "center"
        , style "letter-spacing" "0"
        , style "height" "18px"
        , style "width" "18px"
        , style "margin-left" "28px"
        , style "flex-shrink" "0"
        , style "font-size" fontSize
        , backgroundColorAttr color
        ]
        [ Html.text text ]


favoriteIcon : { filled : Bool, isBright : Bool } -> List (Html.Attribute msg)
favoriteIcon fav =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just <|
                Assets.FavoritedToggleIcon
                    { isFavorited = fav.filled, isHovered = fav.isBright, isSideBar = True }
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "height" <| String.fromFloat starWidth ++ "px"
    , style "width" <| String.fromFloat starWidth ++ "px"
    , style "background-size" "contain"
    , style "background-origin" "content-box"
    , style "padding" <| "0 " ++ String.fromFloat starPadding ++ "px"
    , style "flex-shrink" "0"
    , style "cursor" "pointer"
    , Attr.attribute "aria-label" "Favorite Icon"
    ]


tooltipBody : List (Html.Attribute msg)
tooltipBody =
    [ style "padding-right" "12px"
    , style "padding-left" "6px"
    , style "font-size" "12px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "height" "30px"
    ]
        ++ Tooltip.colors
