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
    [ style "border-top" <| "1px solid " ++ Colors.background
    , style "background-color" Colors.frame
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


teamIcon : Bool -> Html.Html msg
teamIcon isHovered =
    Html.div
        []
        [ Icon.icon
            { sizePx = 20
            , image = "baseline-people-24px.svg"
            }
            [ style "opacity" <|
                if isHovered then
                    "1"

                else
                    "0.5"
            ]
        ]


arrow : { isExpanded : Bool, isHovered : Bool } -> Html.Html msg
arrow { isExpanded, isHovered } =
    Icon.icon
        { sizePx = 20
        , image =
            "baseline-keyboard-arrow-"
                ++ (if isExpanded then
                        "down"

                    else
                        "right"
                   )
                ++ "-24px.svg"
        }
        [ style "opacity" <|
            if isExpanded || isHovered then
                "1"

            else
                "0.5"
        ]


teamName :
    { isExpanded : Bool, isHovered : Bool, isCurrent : Bool }
    -> List (Html.Attribute msg)
teamName { isExpanded, isHovered, isCurrent } =
    [ style "font-size" "18px"
    , style "padding" "2.5px"
    , style "margin" "2.5px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "border" <|
        "1px solid "
            ++ (if isCurrent then
                    Colors.groupBorderSelected

                else
                    Colors.frame
               )
    , style "opacity" <|
        if isCurrent || isExpanded || isHovered then
            "1"

        else
            "0.5"
    ]


pipelineLink :
    { isHovered : Bool, isCurrent : Bool }
    -> List (Html.Attribute msg)
pipelineLink { isHovered, isCurrent } =
    [ style "font-size" "18px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "padding" "2.5px"
    , style "border" <|
        "1px solid "
            ++ (if isCurrent then
                    Colors.groupBorderSelected

                else if isHovered then
                    "#525151"

                else
                    Colors.frame
               )
    , style "opacity" <|
        if isCurrent || isHovered then
            "1"

        else
            "0.5"
    ]
        ++ (if isHovered then
                [ style "background-color" "#2f2e2e" ]

            else
                []
           )


hamburgerMenu :
    { isSideBarOpen : Bool, isPaused : Bool, isClickable : Bool }
    -> List (Html.Attribute msg)
hamburgerMenu { isSideBarOpen, isPaused, isClickable } =
    [ style "border-right" <|
        "1px solid "
            ++ (if isPaused then
                    Colors.pausedTopbarSeparator

                else
                    Colors.background
               )
    , style "opacity" "1"
    , style "cursor" <|
        if isClickable then
            "pointer"

        else
            "default"
    , style "background-color" <|
        if isPaused then
            Colors.paused

        else if isSideBarOpen then
            "#333333"

        else
            Colors.frame
    ]


hamburgerIcon : Bool -> List (Html.Attribute msg)
hamburgerIcon hovered =
    [ style "opacity" <|
        if hovered then
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
    , style "padding" "2.5px"
    ]


pipelineIcon : List (Html.Attribute msg)
pipelineIcon =
    [ style "background-image"
        "url(/public/images/ic-breadcrumb-pipeline.svg)"
    , style "background-repeat" "no-repeat"
    , style "height" "16px"
    , style "width" "32px"
    , style "background-size" "contain"
    , style "margin-left" "22px"
    , style "flex-shrink" "0"
    , style "opacity" "0.4"
    ]
