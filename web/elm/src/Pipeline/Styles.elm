module Pipeline.Styles exposing
    ( consoleIcon
    , docsIcon
    , favoritedIcon
    , groupItem
    , groupsBar
    , pauseToggle
    , pipelineBackground
    )

import Assets
import Colors
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
    , style "background-color" Colors.topBarBackground
    ]


pauseToggle : List (Html.Attribute msg)
pauseToggle =
    [ style "border-left" <|
        "1px solid "
            ++ Colors.background
    , style "background-color" Colors.topBarBackground
    ]


docsIcon : List (Html.Attribute msg)
docsIcon =
    [ style "margin-right" "5px"
    , style "width" "12px"
    , style "height" "12px"
    , style "background-image" <|
        Assets.backgroundImage <|
            Just Assets.FileDocument
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "background-size" "contain"
    , style "display" "inline-block"
    ]


consoleIcon : List (Html.Attribute msg)
consoleIcon =
    [ style "margin-right" "5px"
    , style "width" "12px"
    , style "height" "12px"
    , style "background-image" <|
        Assets.backgroundImage <|
            Just Assets.Console
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "background-size" "contain"
    , style "display" "inline-block"
    ]


pipelineBackground : { image : String, filter : Maybe String } -> List (Html.Attribute msg)
pipelineBackground { image, filter } =
    [ style "background-image" <|
        "url(\""
            ++ image
            ++ "\")"
    , style "background-repeat" "no-repeat"
    , style "background-size" "cover"
    , style "background-position" "center"
    , style "filter" (Maybe.withDefault "grayscale(100%) opacity(30%)" filter)
    , style "width" "100%"
    , style "height" "100%"
    , style "position" "absolute"
    ]
