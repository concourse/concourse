module Dashboard.Styles exposing
    ( asciiArt
    , cardBody
    , cardFooter
    , concourseLogo
    , content
    , dropdownContainer
    , dropdownItem
    , highDensityIcon
    , highDensityToggle
    , info
    , infoBar
    , infoCliIcon
    , infoItem
    , legend
    , legendItem
    , legendSeparator
    , noPipelineCardHd
    , noPipelineCardHeader
    , noPipelineCardTextHd
    , noResults
    , pipelineCard
    , pipelineCardBanner
    , pipelineCardBannerHd
    , pipelineCardBody
    , pipelineCardBodyHd
    , pipelineCardFooter
    , pipelineCardHd
    , pipelineCardHeader
    , pipelineCardTransitionAge
    , pipelineName
    , previewPlaceholder
    , resourceErrorTriangle
    , searchButton
    , searchClearButton
    , searchContainer
    , searchInput
    , showSearchContainer
    , striped
    , teamNameHd
    , topCliIcon
    , visibilityToggle
    , visibilityTooltip
    , welcomeCard
    , welcomeCardBody
    , welcomeCardTitle
    )

import Application.Styles
import Colors
import Concourse.Cli as Cli
import Concourse.PipelineStatus exposing (PipelineStatus(..))
import Html
import Html.Attributes exposing (style)
import ScreenSize exposing (ScreenSize(..))
import Views.Styles


content : Bool -> List (Html.Attribute msg)
content highDensity =
    [ style "align-content" "flex-start"
    , style "display" <|
        if highDensity then
            "flex"

        else
            "initial"
    , style "flex-flow" "column wrap"
    , style "padding" <|
        if highDensity then
            "60px"

        else
            "0"
    , style "flex-grow" "1"
    , style "overflow-y" "auto"
    , style "height" "100%"
    , style "width" "100%"
    , style "box-sizing" "border-box"
    , style "flex-direction" <|
        if highDensity then
            "column"

        else
            "row"
    ]


pipelineCard : List (Html.Attribute msg)
pipelineCard =
    [ style "cursor" "move"
    , style "margin" "25px"
    ]


pipelineCardBanner :
    { status : PipelineStatus
    , pipelineRunningKeyframes : String
    }
    -> List (Html.Attribute msg)
pipelineCardBanner { status, pipelineRunningKeyframes } =
    let
        color =
            Colors.statusColor status

        isRunning =
            Concourse.PipelineStatus.isRunning status
    in
    style "height" "7px" :: texture pipelineRunningKeyframes isRunning color


noPipelineCardHd : List (Html.Attribute msg)
noPipelineCardHd =
    [ style "background-color" Colors.card
    , style "font-size" "14px"
    , style "width" "200px"
    , style "height" "60px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "letter-spacing" "1px"
    , style "margin-right" "60px"
    ]


noPipelineCardTextHd : List (Html.Attribute msg)
noPipelineCardTextHd =
    [ style "padding" "10px"
    ]


noPipelineCardHeader : List (Html.Attribute msg)
noPipelineCardHeader =
    [ style "color" Colors.dashboardText
    , style "background-color" Colors.card
    , style "font-size" "1.5em"
    , style "letter-spacing" "0.1em"
    , style "padding" "12.5px"
    , style "text-align" "center"
    , style "-webkit-font-smoothing" "antialiased"
    ]


pipelineCardHeader : List (Html.Attribute msg)
pipelineCardHeader =
    [ style "background-color" Colors.card
    , style "color" Colors.dashboardText
    , style "font-size" "1.5em"
    , style "letter-spacing" "0.1em"
    , style "-webkit-font-smoothing" "antialiased"
    , style "padding" "12.5px"
    ]


pipelineName : List (Html.Attribute msg)
pipelineName =
    [ style "width" "245px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    ]


cardBody : List (Html.Attribute msg)
cardBody =
    [ style "width" "200px"
    , style "height" "120px"
    , style "padding" "20px 36px"
    , style "background-color" Colors.card
    , style "margin" "2px 0"
    , style "display" "flex"
    ]


pipelineCardBody : List (Html.Attribute msg)
pipelineCardBody =
    [ style "background-color" Colors.card
    , style "margin" "2px 0"
    ]


cardFooter : List (Html.Attribute msg)
cardFooter =
    [ style "height" "47px"
    , style "background-color" Colors.card
    ]


previewPlaceholder : List (Html.Attribute msg)
previewPlaceholder =
    [ style "background-color" Colors.background
    , style "flex-grow" "1"
    ]


teamNameHd : List (Html.Attribute msg)
teamNameHd =
    [ style "letter-spacing" ".2em"
    ]


pipelineCardHd : PipelineStatus -> List (Html.Attribute msg)
pipelineCardHd status =
    [ style "display" "flex"
    , style "height" "60px"
    , style "width" "200px"
    , style "margin" "0 60px 4px 0"
    , style "position" "relative"
    , style "background-color" <|
        case status of
            PipelineStatusSucceeded _ ->
                Colors.successFaded

            PipelineStatusFailed _ ->
                Colors.failure

            PipelineStatusErrored _ ->
                Colors.error

            _ ->
                Colors.card
    , style "font-size" "19px"
    , style "letter-spacing" "1px"
    ]


pipelineCardBodyHd : List (Html.Attribute msg)
pipelineCardBodyHd =
    [ style "width" "180px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    , style "align-self" "center"
    , style "padding" "10px"
    ]


pipelineCardBannerHd :
    { status : PipelineStatus
    , pipelineRunningKeyframes : String
    }
    -> List (Html.Attribute msg)
pipelineCardBannerHd { status, pipelineRunningKeyframes } =
    let
        color =
            Colors.statusColor status

        isRunning =
            Concourse.PipelineStatus.isRunning status
    in
    style "width" "8px" :: texture pipelineRunningKeyframes isRunning color


solid : String -> List (Html.Attribute msg)
solid color =
    [ style "background-color" color ]


striped :
    { pipelineRunningKeyframes : String
    , thickColor : String
    , thinColor : String
    }
    -> List (Html.Attribute msg)
striped { pipelineRunningKeyframes, thickColor, thinColor } =
    [ style "background-image" <| withStripes thickColor thinColor
    , style "background-size" "106px 114px"
    , style "animation" <| pipelineRunningKeyframes ++ " 3s linear infinite"
    ]


withStripes : String -> String -> String
withStripes thickColor thinColor =
    "repeating-linear-gradient(-115deg,"
        ++ thickColor
        ++ " 0,"
        ++ thickColor
        ++ " 10px,"
        ++ thinColor
        ++ " 0,"
        ++ thinColor
        ++ " 16px)"


texture : String -> Bool -> String -> List (Html.Attribute msg)
texture pipelineRunningKeyframes isRunning color =
    if isRunning then
        striped
            { pipelineRunningKeyframes = pipelineRunningKeyframes
            , thickColor = Colors.card
            , thinColor = color
            }

    else
        solid color


pipelineCardFooter : List (Html.Attribute msg)
pipelineCardFooter =
    [ style "padding" "13.5px"
    , style "display" "flex"
    , style "justify-content" "space-between"
    , style "background-color" Colors.card
    ]


pipelineCardTransitionAge : PipelineStatus -> List (Html.Attribute msg)
pipelineCardTransitionAge status =
    [ style "color" <| Colors.statusColor status
    , style "font-size" "18px"
    , style "line-height" "20px"
    , style "letter-spacing" "0.05em"
    , style "margin-left" "8px"
    ]


infoBar :
    { hideLegend : Bool, screenSize : ScreenSize.ScreenSize }
    -> List (Html.Attribute msg)
infoBar { hideLegend, screenSize } =
    [ style "position" "fixed"
    , style "bottom" "0"
    , style "line-height" "35px"
    , style "padding" "7.5px 30px"
    , style "background-color" Colors.frame
    , style "width" "100%"
    , style "box-sizing" "border-box"
    , style "display" "flex"
    , style "justify-content" <|
        if hideLegend then
            "flex-end"

        else
            "space-between"
    ]
        ++ (case screenSize of
                ScreenSize.Mobile ->
                    [ style "flex-direction" "column" ]

                ScreenSize.Desktop ->
                    [ style "flex-direction" "column" ]

                ScreenSize.BigDesktop ->
                    []
           )


legend : List (Html.Attribute msg)
legend =
    [ style "display" "flex"
    , style "flex-wrap" "wrap"
    ]


legendItem : List (Html.Attribute msg)
legendItem =
    [ style "display" "flex"
    , style "text-transform" "uppercase"
    , style "align-items" "center"
    , style "color" Colors.bottomBarText
    , style "margin-right" "20px"
    ]


legendSeparator : List (Html.Attribute msg)
legendSeparator =
    [ style "color" Colors.bottomBarText
    , style "margin-right" "20px"
    , style "display" "flex"
    , style "align-items" "center"
    ]


highDensityToggle : List (Html.Attribute msg)
highDensityToggle =
    [ style "color" Colors.bottomBarText
    , style "margin-right" "20px"
    , style "display" "flex"
    , style "text-transform" "uppercase"
    , style "align-items" "center"
    ]


highDensityIcon : Bool -> List (Html.Attribute msg)
highDensityIcon highDensity =
    [ style "background-image" <|
        if highDensity then
            "url(/public/images/ic-hd-on.svg)"

        else
            "url(/public/images/ic-hd-off.svg)"
    , style "background-size" "contain"
    , style "height" "20px"
    , style "width" "35px"
    , style "flex-shrink" "0"
    , style "margin-right" "10px"
    ]


info : List (Html.Attribute msg)
info =
    [ style "display" "flex"
    , style "color" Colors.bottomBarText
    , style "font-size" "1.25em"
    ]


infoItem : List (Html.Attribute msg)
infoItem =
    [ style "margin-right" "30px"
    , style "display" "flex"
    , style "align-items" "center"
    ]


infoCliIcon : { hovered : Bool, cli : Cli.Cli } -> List (Html.Attribute msg)
infoCliIcon { hovered, cli } =
    [ style "margin-right" "10px"
    , style "width" "20px"
    , style "height" "20px"
    , style "background-image" <| Cli.iconUrl cli
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "background-size" "contain"
    , style "opacity" <|
        if hovered then
            "1"

        else
            "0.5"
    ]


topCliIcon : { hovered : Bool, cli : Cli.Cli } -> List (Html.Attribute msg)
topCliIcon { hovered, cli } =
    [ style "opacity" <|
        if hovered then
            "1"

        else
            "0.5"
    , style "background-image" <| Cli.iconUrl cli
    , style "background-position" "50% 50%"
    , style "background-repeat" "no-repeat"
    , style "width" "32px"
    , style "height" "32px"
    , style "margin" "5px"
    , style "z-index" "1"
    ]


welcomeCard : List (Html.Attribute msg)
welcomeCard =
    [ style "background-color" Colors.card
    , style "margin" "25px"
    , style "padding" "40px"
    , style "-webkit-font-smoothing" "antialiased"
    , style "position" "relative"
    , style "overflow" "hidden"
    , style "font-weight" "400"
    , style "display" "flex"
    , style "flex-direction" "column"
    ]


welcomeCardBody : List (Html.Attribute msg)
welcomeCardBody =
    [ style "font-size" "16px"
    , style "z-index" "2"
    ]


welcomeCardTitle : List (Html.Attribute msg)
welcomeCardTitle =
    [ style "font-size" "32px" ]


resourceErrorTriangle : List (Html.Attribute msg)
resourceErrorTriangle =
    [ style "position" "absolute"
    , style "top" "0"
    , style "right" "0"
    , style "width" "0"
    , style "height" "0"
    , style "border-top" <| "30px solid " ++ Colors.resourceError
    , style "border-left" "30px solid transparent"
    ]


asciiArt : List (Html.Attribute msg)
asciiArt =
    [ style "font-size" "16px"
    , style "line-height" "8px"
    , style "position" "absolute"
    , style "top" "0"
    , style "left" "23em"
    , style "margin" "0"
    , style "white-space" "pre"
    , style "color" Colors.asciiArt
    , style "z-index" "1"
    ]
        ++ Application.Styles.disableInteraction


noResults : List (Html.Attribute msg)
noResults =
    [ style "text-align" "center"
    , style "font-size" "13px"
    , style "margin-top" "20px"
    ]


searchContainer : ScreenSize -> List (Html.Attribute msg)
searchContainer screenSize =
    [ style "display" "flex"
    , style "flex-direction" "column"
    , style "margin" "12px"
    , style "position" "relative"
    , style "align-items" "stretch"
    ]
        ++ (case screenSize of
                Mobile ->
                    [ style "flex-grow" "1" ]

                _ ->
                    []
           )


searchInput : ScreenSize -> List (Html.Attribute msg)
searchInput screenSize =
    let
        widthStyles =
            case screenSize of
                Mobile ->
                    []

                Desktop ->
                    [ style "width" "220px" ]

                BigDesktop ->
                    [ style "width" "220px" ]
    in
    [ style "background-color" "transparent"
    , style "background-image" "url('public/images/ic-search-white-24px.svg')"
    , style "background-repeat" "no-repeat"
    , style "background-position" "12px 8px"
    , style "height" "30px"
    , style "padding" "0 42px"
    , style "border" <| "1px solid " ++ Colors.inputOutline
    , style "color" Colors.dashboardText
    , style "font-size" "1.15em"
    , style "font-family" "Inconsolata, monospace"
    , style "outline" "0"
    ]
        ++ widthStyles


searchClearButton : Bool -> List (Html.Attribute msg)
searchClearButton active =
    let
        opacityValue =
            if active then
                "1"

            else
                "0.2"
    in
    [ style "background-image" "url('public/images/ic-close-white-24px.svg')"
    , style "background-repeat" "no-repeat"
    , style "background-position" "10px 10px"
    , style "border" "0"
    , style "color" Colors.inputOutline
    , style "position" "absolute"
    , style "right" "0"
    , style "padding" "17px"
    , style "opacity" opacityValue
    ]


dropdownItem : Bool -> List (Html.Attribute msg)
dropdownItem isSelected =
    let
        coloration =
            if isSelected then
                [ style "background-color" Colors.frame
                , style "color" Colors.dashboardText
                ]

            else
                [ style "background-color" Colors.dropdownFaded
                , style "color" Colors.dropdownUnselectedText
                ]
    in
    [ style "padding" "0 42px"
    , style "line-height" "30px"
    , style "list-style-type" "none"
    , style "border" <| "1px solid " ++ Colors.inputOutline
    , style "margin-top" "-1px"
    , style "font-size" "1.15em"
    , style "cursor" "pointer"
    ]
        ++ coloration


dropdownContainer : ScreenSize -> List (Html.Attribute msg)
dropdownContainer screenSize =
    [ style "top" "100%"
    , style "margin" "0"
    , style "width" "100%"
    ]
        ++ (case screenSize of
                Mobile ->
                    []

                _ ->
                    [ style "position" "absolute" ]
           )


showSearchContainer :
    { a
        | screenSize : ScreenSize
        , highDensity : Bool
    }
    -> List (Html.Attribute msg)
showSearchContainer { highDensity } =
    let
        flexLayout =
            if highDensity then
                []

            else
                [ style "align-items" "flex-start" ]
    in
    [ style "display" "flex"
    , style "flex-direction" "column"
    , style "flex-grow" "1"
    , style "justify-content" "center"
    , style "padding" "12px"
    , style "position" "relative"
    ]
        ++ flexLayout


searchButton : List (Html.Attribute msg)
searchButton =
    [ style "background-image" "url('public/images/ic-search-white-24px.svg')"
    , style "background-repeat" "no-repeat"
    , style "background-position" "12px 8px"
    , style "height" "32px"
    , style "width" "32px"
    , style "display" "inline-block"
    , style "float" "left"
    ]


visibilityToggle :
    { public : Bool
    , isClickable : Bool
    , isHovered : Bool
    }
    -> List (Html.Attribute msg)
visibilityToggle { public, isClickable, isHovered } =
    let
        image =
            if public then
                "baseline-visibility-24px.svg"

            else
                "baseline-visibility-off-24px.svg"
    in
    [ style "background-image" ("url(/public/images/" ++ image ++ ")")
    , style "height" "20px"
    , style "width" "20px"
    , style "background-position" "50% 50%"
    , style "background-repeat" "no-repeat"
    , style "position" "relative"
    , style "background-size" "contain"
    , style "cursor" <|
        if isClickable then
            "pointer"

        else
            "default"
    , style "opacity" <|
        if isClickable && isHovered then
            "1"

        else
            "0.5"
    ]


visibilityTooltip : List (Html.Attribute msg)
visibilityTooltip =
    [ style "position" "absolute"
    , style "background-color" Colors.tooltipBackground
    , style "bottom" "100%"
    , style "white-space" "nowrap"
    , style "padding" "2.5px"
    , style "margin-bottom" "5px"
    , style "right" "-150%"
    ]


concourseLogo : List (Html.Attribute msg)
concourseLogo =
    style "border-left" ("1px solid " ++ Colors.background)
        :: Views.Styles.concourseLogo
