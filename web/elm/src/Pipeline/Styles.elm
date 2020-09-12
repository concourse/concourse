module Pipeline.Styles exposing
    ( cliIcon
    , favoritedIcon
    , groupItem
    , groupsBar
    , pauseToggle
    , pipelineBackground
    )

import Assets
import Colors
import Concourse.Cli as Cli
import Html
import Html.Attributes exposing (style)


groupsBar : List (Html.Attribute msg)
groupsBar =
    [ style "background-color" Colors.groupsBarBackground
    , style "color" Colors.dashboardText
    , style "display" "flex"
    , style "flex-flow" "row wrap"
    , style "padding" "5px"
    ]


groupItem : { selected : Bool, hovered : Bool } -> List (Html.Attribute msg)
groupItem { selected, hovered } =
    [ style "font-size" "14px"
    , style "background" Colors.groupBackground
    , style "margin" "5px"
    , style "padding" "10px"
    ]
        ++ (if selected then
                [ style "opacity" "1"
                , style "border" <| "1px solid " ++ Colors.groupBorderSelected
                ]

            else if hovered then
                [ style "opacity" "0.6"
                , style "border" <| "1px solid " ++ Colors.groupBorderHovered
                ]

            else
                [ style "opacity" "0.6"
                , style "border" <| "1px solid " ++ Colors.groupBorderUnselected
                ]
           )


favoritedIcon : List (Html.Attribute msg)
favoritedIcon =
    [ style "border-left" <|
        "1px solid "
            ++ Colors.background
    , style "background-color" Colors.frame
    ]


pauseToggle : List (Html.Attribute msg)
pauseToggle =
    [ style "border-left" <|
        "1px solid "
            ++ Colors.background
    , style "background-color" Colors.frame
    ]


cliIcon : Cli.Cli -> List (Html.Attribute msg)
cliIcon cli =
    [ style "width" "12px"
    , style "height" "12px"
    , style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.CliIcon cli)
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "background-size" "contain"
    , style "display" "inline-block"
    ]


pipelineBackground : String -> List (Html.Attribute msg)
pipelineBackground bg =
    [ style "background-image" <|
        "url(\""
            ++ bg
            ++ "\")"
    , style "background-repeat" "no-repeat"
    , style "background-size" "cover"
    , style "background-position" "center"
    , style "opacity" "30%"
    , style "filter" "grayscale(1)"
    , style "width" "100%"
    , style "height" "100%"
    , style "position" "absolute"
    ]
