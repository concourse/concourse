module PipelineInfo.Styles exposing
    ( body
    , empty
    , markdown
    , page
    , yaml
    )

import Colors
import Html
import Html.Attributes exposing (style)


page : List (Html.Attribute msg)
page =
    [ style "flex-grow" "1"
    , style "overflow-y" "auto"
    , style "background-color" Colors.backgroundDark
    ]


body : List (Html.Attribute msg)
body =
    [ style "max-width" "800px"
    , style "padding" "20px"
    , style "color" Colors.dashboardText
    , style "font-size" "14px"
    , style "line-height" "1.5"

    -- the markdown and YAML blocks are siblings; a gap keeps them apart
    -- without either needing a margin that would double up on the padding
    -- when it's the only child
    , style "display" "flex"
    , style "flex-direction" "column"
    , style "gap" "20px"
    ]


markdown : List (Html.Attribute msg)
markdown =
    [ style "word-wrap" "break-word" ]


yaml : List (Html.Attribute msg)
yaml =
    [ style "background-color" Colors.groupsBarBackground
    , style "padding" "15px"
    , style "margin" "0"
    , style "overflow-x" "auto"
    , style "font-family" "Courier, monospace"
    , style "white-space" "pre"
    ]


empty : List (Html.Attribute msg)
empty =
    [ style "color" Colors.dashboardText
    , style "opacity" "0.5"
    ]
