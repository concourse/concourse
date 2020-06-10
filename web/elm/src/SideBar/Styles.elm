module SideBar.Styles exposing
    ( Opacity(..)
    , PipelineBackingRectangle(..)
    , TeamBackingRectangle(..)
    , collapseIcon
    , column
    , hamburgerIcon
    , hamburgerMenu
    , iconGroup
    , opacityAttr
    , pipeline
    , pipelineIcon
    , pipelineLink
    , sideBar
    , team
    , teamHeader
    , teamIcon
    , teamName
    , tooltip
    , tooltipBody
    )

import Assets
import Colors
import Html
import Html.Attributes exposing (style)
import Views.Icon as Icon


sideBar : List (Html.Attribute msg)
sideBar =
    [ style "border-right" <| "1px solid " ++ Colors.frame
    , style "background-color" Colors.sideBar
    , style "width" "275px"
    , style "overflow-y" "auto"
    , style "height" "100%"
    , style "flex-shrink" "0"
    , style "padding-right" "10px"
    , style "box-sizing" "border-box"
    , style "padding-bottom" "10px"
    , style "-webkit-overflow-scrolling" "touch"
    ]


column : List (Html.Attribute msg)
column =
    [ style "display" "flex"
    , style "flex-direction" "column"
    ]


teamHeader : List (Html.Attribute msg)
teamHeader =
    [ style "display" "flex"
    , style "cursor" "pointer"
    , style "align-items" "center"
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
                "0.3"

            GreyedOut ->
                "0.7"

            Bright ->
                "1"


teamIcon : Opacity -> Html.Html msg
teamIcon opacity =
    Icon.icon
        { sizePx = 18
        , image = Assets.PeopleIcon
        }
        [ style "margin-left" "8px"
        , style "background-size" "contain"
        , style "flex-shrink" "0"
        , opacityAttr opacity
        ]


collapseIcon : { opacity : Opacity, asset : Assets.Asset } -> Html.Html msg
collapseIcon { opacity, asset } =
    Icon.icon
        { sizePx = 10
        , image = asset
        }
        [ style "margin-left" "10px"
        , style "flex-shrink" "0"
        , opacityAttr opacity
        ]


type TeamBackingRectangle
    = TeamInvisible
    | GreyWithLightBorder


teamName :
    { a | opacity : Opacity, rectangle : TeamBackingRectangle }
    -> List (Html.Attribute msg)
teamName { opacity, rectangle } =
    [ style "font-size" "14px"
    , style "padding" "5px 2.5px"
    , style "margin-left" "5px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "flex-grow" "1"
    , opacityAttr opacity
    ]
        ++ (case rectangle of
                TeamInvisible ->
                    [ style "border" <| "1px solid " ++ Colors.sideBar
                    ]

                GreyWithLightBorder ->
                    [ style "border" "1px solid #525151"
                    , style "background-color" "#3A3A3A"
                    ]
           )


type PipelineBackingRectangle
    = Dark
    | Light
    | PipelineInvisible


pipelineLink :
    { a | opacity : Opacity, rectangle : PipelineBackingRectangle }
    -> List (Html.Attribute msg)
pipelineLink { opacity, rectangle } =
    [ style "font-size" "14px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "padding" "5px 2.5px"
    , style "margin-left" "5px"
    , style "flex-grow" "1"
    , opacityAttr opacity
    ]
        ++ (case rectangle of
                Dark ->
                    [ style "border" <| "1px solid " ++ Colors.groupBorderSelected
                    , style "background-color" Colors.sideBarActive
                    ]

                Light ->
                    [ style "border" "1px solid #525151"
                    , style "background-color" "#3A3A3A"
                    ]

                PipelineInvisible ->
                    [ style "border" <| "1px solid " ++ Colors.sideBar ]
           )


hamburgerMenu :
    { isSideBarOpen : Bool, isClickable : Bool }
    -> List (Html.Attribute msg)
hamburgerMenu { isSideBarOpen, isClickable } =
    [ style "border-right" <| "1px solid " ++ Colors.frame
    , style "opacity" "1"
    , style "cursor" <|
        if isClickable then
            "pointer"

        else
            "default"
    , style "background-color" <|
        if isSideBarOpen then
            Colors.sideBar

        else
            Colors.frame
    ]


hamburgerIcon : { isHovered : Bool, isActive : Bool } -> List (Html.Attribute msg)
hamburgerIcon { isHovered, isActive } =
    [ style "opacity" <|
        if isActive || isHovered then
            "1"

        else
            "0.5"
    ]


team : List (Html.Attribute msg)
team =
    [ style "padding-top" "5px", style "line-height" "1.2" ] ++ column


pipeline : List (Html.Attribute msg)
pipeline =
    [ style "display" "flex"
    , style "align-items" "center"
    ]


pipelineIcon : { asset : Assets.Asset, opacity : Opacity } -> List (Html.Attribute msg)
pipelineIcon { asset, opacity } =
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
    , opacityAttr opacity
    ]


tooltip : List (Html.Attribute msg)
tooltip =
    [ style "z-index" "1"
    , style "display" "flex"
    ]


tooltipBody : List (Html.Attribute msg)
tooltipBody =
    [ style "background-color" Colors.frame
    , style "padding-right" "10px"
    , style "font-size" "12px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "height" "30px"
    ]
