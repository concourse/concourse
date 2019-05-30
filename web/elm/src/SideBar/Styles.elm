module SideBar.Styles exposing
    ( arrow
    , column
    , hamburgerIcon
    , hamburgerMenu
    , iconGroup
    , pipeline
    , pipelineIcon
    , pipelineLink
    , sideBar
    , team
    , teamHeader
    , teamIcon
    , teamName
    )

import Colors
import Html
import Html.Attributes exposing (style)
import Views.Icon as Icon


sideBar : List (Html.Attribute msg)
sideBar =
    [ style "border-top" <| "1px solid " ++ Colors.frame
    , style "border-right" <| "1px solid " ++ Colors.frame
    , style "background-color" Colors.sideBar
    , style "width" "275px"
    , style "overflow-y" "auto"
    , style "height" "100%"
    , style "flex-shrink" "0"
    , style "padding-right" "10px"
    , style "box-sizing" "border-box"
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


teamIcon : { isExpanded : Bool, isCurrent : Bool, isHovered : Bool } -> Html.Html msg
teamIcon { isExpanded, isCurrent, isHovered } =
    Icon.icon
        { sizePx = 20
        , image = "baseline-people-24px.svg"
        }
        [ style "margin-left" "10px"
        , style "background-size" "contain"
        , style "flex-shrink" "0"
        , style "opacity" <|
            if isCurrent then
                "1"

            else if isHovered || isExpanded then
                "0.5"

            else
                "0.2"
        ]


arrow : { isExpanded : Bool, isHovered : Bool } -> Html.Html msg
arrow { isExpanded, isHovered } =
    Icon.icon
        { sizePx = 12
        , image =
            "baseline-keyboard-arrow-"
                ++ (if isExpanded then
                        "down"

                    else
                        "right"
                   )
                ++ "-24px.svg"
        }
        [ style "margin-left" "10px"
        , style "flex-shrink" "0"
        , style "opacity" <|
            if isExpanded then
                "1"

            else if isHovered then
                "0.5"

            else
                "0.2"
        ]


teamName :
    { isHovered : Bool, isCurrent : Bool }
    -> List (Html.Attribute msg)
teamName { isHovered, isCurrent } =
    [ style "font-size" "14px"
    , style "padding" "2.5px"
    , style "margin-left" "7.5px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "flex-grow" "1"
    , style "border" <|
        "1px solid "
            ++ (if isHovered then
                    "#525151"

                else
                    Colors.sideBar
               )
    , style "opacity" <|
        if isCurrent || isHovered then
            "1"

        else
            "0.5"
    ]
        ++ (if isHovered then
                [ style "background-color" "#302F2F"
                ]

            else
                []
           )


pipelineLink :
    { isHovered : Bool, isCurrent : Bool }
    -> List (Html.Attribute msg)
pipelineLink { isHovered, isCurrent } =
    [ style "font-size" "14px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "padding" "2.5px"
    , style "flex-grow" "1"
    , style "border" <|
        "1px solid "
            ++ (if isCurrent then
                    Colors.groupBorderSelected

                else if isHovered then
                    "#525151"

                else
                    Colors.sideBar
               )
    , style "opacity" <|
        if isCurrent || isHovered then
            "1"

        else
            "0.5"
    ]
        ++ (if isCurrent then
                [ style "background-color" "#272727" ]

            else if isHovered then
                [ style "background-color" "#3A3A3A" ]

            else
                []
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


pipelineIcon : { isCurrent : Bool, isHovered : Bool } -> List (Html.Attribute msg)
pipelineIcon { isCurrent, isHovered } =
    [ style "background-image"
        "url(/public/images/ic-breadcrumb-pipeline.svg)"
    , style "background-repeat" "no-repeat"
    , style "height" "16px"
    , style "width" "32px"
    , style "background-size" "contain"
    , style "margin-left" "28px"
    , style "flex-shrink" "0"
    , style "opacity" <|
        if isCurrent || isHovered then
            "1"

        else
            "0.2"
    ]
