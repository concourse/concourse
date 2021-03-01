module Dashboard.Styles exposing
    ( asciiArt
    , cardFooter
    , cardTooltip
    , content
    , dropdownContainer
    , dropdownItem
    , emptyCardBody
    , highDensityToggle
    , info
    , infoBar
    , infoCliIcon
    , infoItem
    , inlineInstanceVar
    , instanceGroupCard
    , instanceGroupCardBadge
    , instanceGroupCardBanner
    , instanceGroupCardBannerHd
    , instanceGroupCardBody
    , instanceGroupCardBodyHd
    , instanceGroupCardFooter
    , instanceGroupCardHd
    , instanceGroupCardHeader
    , instanceGroupCardNameHd
    , instanceGroupCardPipelineBox
    , instanceGroupName
    , instanceVar
    , jobPreview
    , jobPreviewLink
    , jobPreviewTooltip
    , legend
    , legendItem
    , legendSeparator
    , loadingView
    , noInstanceVars
    , noPipelineCard
    , noPipelineCardHd
    , noPipelineCardHeader
    , noPipelineCardTextHd
    , noResults
    , pipelineCard
    , pipelineCardBanner
    , pipelineCardBannerArchived
    , pipelineCardBannerArchivedHd
    , pipelineCardBannerHd
    , pipelineCardBannerStale
    , pipelineCardBannerStaleHd
    , pipelineCardBody
    , pipelineCardBodyHd
    , pipelineCardFooter
    , pipelineCardHd
    , pipelineCardHeader
    , pipelineCardTransitionAge
    , pipelineCardTransitionAgeStale
    , pipelineName
    , pipelinePreviewGrid
    , pipelinePreviewTooltip
    , pipelineSectionHeader
    , pipelineStatusIcon
    , previewPlaceholder
    , resourceErrorTriangle
    , searchButton
    , searchClearButton
    , searchContainer
    , searchInput
    , showArchivedToggle
    , showSearchContainer
    , striped
    , teamNameHd
    , topBarContent
    , topCliIcon
    , visibilityToggle
    , welcomeCard
    , welcomeCardBody
    , welcomeCardTitle
    )

import Application.Styles
import Assets
import ColorValues
import Colors
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Cli as Cli
import Concourse.PipelineStatus exposing (PipelineStatus(..))
import Dashboard.Grid.Constants as GridConstants
import Html
import Html.Attributes exposing (style)
import ScreenSize exposing (ScreenSize(..))
import Tooltip
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
    , style "-webkit-overflow-scrolling" "touch"
    , style "flex-direction" <|
        if highDensity then
            "column"

        else
            "row"
    ]


pipelineCard : List (Html.Attribute msg)
pipelineCard =
    [ style "height" "100%"
    , style "display" "flex"
    , style "flex-direction" "column"
    ]


instanceGroupCard : List (Html.Attribute msg)
instanceGroupCard =
    pipelineCard


pipelineCardBanner :
    { status : PipelineStatus
    , pipelineRunningKeyframes : String
    }
    -> List (Html.Attribute msg)
pipelineCardBanner { status, pipelineRunningKeyframes } =
    let
        color =
            Colors.statusColor True status

        isRunning =
            Concourse.PipelineStatus.isRunning status
    in
    style "height" "7px" :: texture pipelineRunningKeyframes isRunning color


pipelineCardBannerStale : List (Html.Attribute msg)
pipelineCardBannerStale =
    [ style "height" "7px"
    , style "background-color" Colors.unknown
    ]


pipelineCardBannerArchived : List (Html.Attribute msg)
pipelineCardBannerArchived =
    [ style "height" "7px"
    , style "background-color" Colors.backgroundDark
    ]


pipelineStatusIcon : List (Html.Attribute msg)
pipelineStatusIcon =
    [ style "background-size" "contain" ]


noPipelineCard : List (Html.Attribute msg)
noPipelineCard =
    [ style "display" "flex"
    , style "flex-direction" "column"
    , style "width" <| String.fromInt GridConstants.cardWidth ++ "px"
    , style "height" <| String.fromInt (GridConstants.cardBodyHeight + GridConstants.cardHeaderHeight 1) ++ "px"
    , style "margin-left" <| String.fromInt GridConstants.padding ++ "px"
    ]


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
    [ style "color" Colors.dashboardPipelineHeaderText
    , style "background-color" Colors.card
    , style "font-size" "1.5em"
    , style "letter-spacing" "0.1em"
    , style "padding" "12.5px"
    , style "text-align" "center"
    ]


pipelineCardHeader : Float -> List (Html.Attribute msg)
pipelineCardHeader height =
    [ style "background-color" Colors.card
    , style "color" Colors.dashboardPipelineHeaderText
    , style "font-size" "1.5em"
    , style "letter-spacing" "0.1em"
    , style "padding" <| String.fromInt GridConstants.cardHeaderPadding ++ "px"
    , style "height" <| String.fromFloat height ++ "px"
    , style "box-sizing" "border-box"
    ]


instanceGroupCardHeader : Float -> List (Html.Attribute msg)
instanceGroupCardHeader =
    pipelineCardHeader


pipelineName : List (Html.Attribute msg)
pipelineName =
    [ style "width" "240px"
    , style "line-height" <| String.fromInt GridConstants.cardHeaderRowLineHeight ++ "px"
    , style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    ]


instanceVar : List (Html.Attribute msg)
instanceVar =
    pipelineName ++ [ style "letter-spacing" "0.05em" ]


inlineInstanceVar : List (Html.Attribute msg)
inlineInstanceVar =
    [ style "padding-right" "8px" ]


noInstanceVars : List (Html.Attribute msg)
noInstanceVars =
    instanceVar ++ [ style "color" Colors.pending ]


instanceGroupName : List (Html.Attribute msg)
instanceGroupName =
    pipelineName
        ++ [ style "display" "flex"
           , style "align-items" "center"
           ]


emptyCardBody : List (Html.Attribute msg)
emptyCardBody =
    [ style "padding" "20px 36px"
    , style "background-color" Colors.card
    , style "margin" "2px 0"
    , style "display" "flex"
    , style "flex-grow" "1"
    ]


pipelineCardBody : List (Html.Attribute msg)
pipelineCardBody =
    [ style "background-color" Colors.card
    , style "margin" "2px 0"
    , style "flex-grow" "1"
    , style "display" "flex"
    ]


instanceGroupCardBadge : List (Html.Attribute msg)
instanceGroupCardBadge =
    [ style "background" "#f2f2f2"
    , style "border-radius" "4px"
    , style "color" "#222"
    , style "display" "flex"
    , style "letter-spacing" "0"
    , style "margin-right" "8px"
    , style "width" "20px"
    , style "height" "20px"
    , style "flex-shrink" "0"
    , style "align-items" "center"
    , style "justify-content" "center"
    ]


instanceGroupCardBody : List (Html.Attribute msg)
instanceGroupCardBody =
    [ style "background-color" Colors.card
    , style "padding" "20px 36px"
    , style "margin" "2px 0"
    , style "flex-grow" "1"
    , style "display" "flex"
    , style "flex-direction" "column"
    ]


instanceGroupCardPipelineBox : String -> Bool -> PipelineStatus -> List (Html.Attribute msg)
instanceGroupCardPipelineBox pipelineRunningKeyframes isHovered status =
    let
        color =
            Colors.statusColor (not isHovered) status

        isRunning =
            Concourse.PipelineStatus.isRunning status
    in
    [ style "margin" "2px"
    , style "background-color" color
    , style "flex-grow" "1"
    , style "display" "flex"
    ]
        ++ (if isRunning then
                striped
                    { pipelineRunningKeyframes = pipelineRunningKeyframes
                    , thickColor = Colors.statusColor False status
                    , thinColor = Colors.statusColor True status
                    }

            else
                []
           )


pipelinePreviewGrid : List (Html.Attribute msg)
pipelinePreviewGrid =
    [ style "box-sizing" "border-box"
    , style "width" "100%"
    ]


cardFooter : List (Html.Attribute msg)
cardFooter =
    [ style "height" "47px"
    , style "background-color" Colors.card
    ]


previewPlaceholder : List (Html.Attribute msg)
previewPlaceholder =
    [ style "background-color" Colors.noPipelinesPlaceholderBackground
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


instanceGroupCardBodyHd : List (Html.Attribute msg)
instanceGroupCardBodyHd =
    [ style "padding" "10px"
    , style "display" "flex"
    , style "align-items" "center"
    , style "min-width" "0"
    ]


instanceGroupCardNameHd : List (Html.Attribute msg)
instanceGroupCardNameHd =
    [ style "white-space" "nowrap"
    , style "overflow" "hidden"
    , style "text-overflow" "ellipsis"
    ]


instanceGroupCardHd : List (Html.Attribute msg)
instanceGroupCardHd =
    [ style "display" "flex"
    , style "height" "60px"
    , style "width" "200px"
    , style "margin" "0 60px 4px 0"
    , style "position" "relative"
    , style "background-color" Colors.card
    , style "font-size" "19px"
    , style "letter-spacing" "1px"
    ]


instanceGroupCardBannerHd : List (Html.Attribute msg)
instanceGroupCardBannerHd =
    [ style "width" "8px"
    , style "background-color" Colors.card
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
            Colors.statusColor True status

        isRunning =
            Concourse.PipelineStatus.isRunning status
    in
    style "width" "8px" :: texture pipelineRunningKeyframes isRunning color


pipelineCardBannerStaleHd : List (Html.Attribute msg)
pipelineCardBannerStaleHd =
    [ style "width" "8px"
    , style "background-color" Colors.unknown
    ]


pipelineCardBannerArchivedHd : List (Html.Attribute msg)
pipelineCardBannerArchivedHd =
    [ style "width" "8px"
    , style "background-color" Colors.backgroundDark
    ]


instanceGroupCardBanner : List (Html.Attribute msg)
instanceGroupCardBanner =
    [ style "height" "7px"
    , style "background-color" Colors.instanceGroupBanner
    ]


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
        ++ thinColor
        ++ " 0px,"
        ++ thickColor
        ++ " 1px,"
        ++ thickColor
        ++ " 10px,"
        ++ thinColor
        ++ " 11px,"
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
    [ style "padding" <| String.fromInt GridConstants.cardHeaderPadding ++ "px"
    , style "display" "flex"
    , style "justify-content" "space-between"
    , style "background-color" Colors.card
    ]


instanceGroupCardFooter : List (Html.Attribute msg)
instanceGroupCardFooter =
    [ style "padding" "13.5px"
    , style "display" "flex"
    , style "justify-content" "flex-end"
    , style "background-color" Colors.card
    ]


pipelineCardTransitionAge : PipelineStatus -> List (Html.Attribute msg)
pipelineCardTransitionAge status =
    [ style "color" <| Colors.statusColor True status
    , style "font-size" "18px"
    , style "line-height" "20px"
    , style "letter-spacing" "0.05em"
    , style "margin-left" "8px"
    ]


pipelineCardTransitionAgeStale : List (Html.Attribute msg)
pipelineCardTransitionAgeStale =
    [ style "color" Colors.unknown
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
    , style "z-index" "2"
    , style "bottom" "0"
    , style "line-height" "35px"
    , style "padding" "7.5px 30px"
    , style "background-color" Colors.infoBarBackground
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
    , style "text-transform" "uppercase"
    ]


showArchivedToggle : List (Html.Attribute msg)
showArchivedToggle =
    [ style "margin-right" "10px"
    , style "padding-left" "10px"
    , style "border-left" <| "1px solid " ++ Colors.showArchivedButtonBorder
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
    , style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.CliIcon cli)
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
    , style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.CliIcon cli)
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
    , style "position" "relative"
    , style "overflow" "hidden"
    , style "font-weight" Views.Styles.fontWeightLight
    , style "display" "flex"
    , style "flex-direction" "column"
    ]


welcomeCardBody : List (Html.Attribute msg)
welcomeCardBody =
    [ style "font-size" "16px"
    , style "z-index" "2"
    , style "color" Colors.welcomeCardText
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


topBarContent : List (Html.Attribute msg)
topBarContent =
    [ style "display" "flex"
    , style "flex-grow" "1"
    , style "justify-content" "center"
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


searchInput : ScreenSize -> Bool -> List (Html.Attribute msg)
searchInput screenSize hasQuery =
    let
        widthStyles =
            case screenSize of
                Mobile ->
                    []

                Desktop ->
                    [ style "width" "251px" ]

                BigDesktop ->
                    [ style "width" "251px" ]

        borderColor =
            if hasQuery then
                ColorValues.grey30

            else
                ColorValues.grey60

        bgImage =
            if hasQuery then
                Just Assets.SearchIconWhite

            else
                Just Assets.SearchIconGrey
    in
    [ style "background-color" ColorValues.grey90
    , style "background-image" <|
        Assets.backgroundImage <|
            bgImage
    , style "background-repeat" "no-repeat"
    , style "background-position" "12px 8px"
    , style "height" "30px"
    , style "min-height" "30px"
    , style "padding" "0 42px"
    , style "border" <| "1px solid " ++ borderColor
    , style "color" Colors.white
    , style "font-size" "12px"
    , style "font-family" Views.Styles.fontFamilyDefault
    , style "outline" "0"
    ]
        ++ widthStyles


searchClearButton : List (Html.Attribute msg)
searchClearButton =
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just Assets.CloseIcon
    , style "background-repeat" "no-repeat"
    , style "background-position" "10px 10px"
    , style "border" "0"
    , style "color" "transparent"
    , style "position" "absolute"
    , style "right" "0"
    , style "padding" "17px"
    ]


dropdownItem : Bool -> Bool -> List (Html.Attribute msg)
dropdownItem isSelected hasQuery =
    let
        coloration =
            if isSelected then
                [ style "background-color" Colors.dropdownItemSelectedBackground
                , style "color" Colors.dropdownItemSelectedText
                ]

            else
                [ style "background-color" Colors.dropdownFaded
                , style "color" Colors.dropdownUnselectedText
                ]

        borderColor =
            if hasQuery then
                ColorValues.grey30

            else
                ColorValues.grey60
    in
    [ style "padding" "0 42px"
    , style "line-height" "30px"
    , style "list-style-type" "none"
    , style "border" <| "1px solid " ++ borderColor
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
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just Assets.SearchIconGrey
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
    [ style "background-image" <|
        Assets.backgroundImage <|
            Just (Assets.VisibilityToggleIcon public)
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


cardTooltip : List (Html.Attribute msg)
cardTooltip =
    [ style "padding" "6px 12px 6px 6px"
    , style "height" "30px"
    , style "box-sizing" "border-box"
    , style "display" "flex"
    , style "align-items" "center"
    ]
        ++ Tooltip.colors


jobPreviewTooltip : List (Html.Attribute msg)
jobPreviewTooltip =
    cardTooltip


pipelinePreviewTooltip : List (Html.Attribute msg)
pipelinePreviewTooltip =
    cardTooltip


jobPreview : Concourse.Job -> Bool -> List (Html.Attribute msg)
jobPreview job isHovered =
    [ style "flex-grow" "1"
    , style "display" "flex"
    , style "margin" "2px"
    ]
        ++ (if job.paused then
                [ style "background-color" <|
                    Colors.statusColor (not isHovered) PipelineStatusPaused
                ]

            else
                let
                    finishedBuildStatus =
                        job.finishedBuild
                            |> Maybe.map .status
                            |> Maybe.withDefault BuildStatusPending

                    isRunning =
                        job.nextBuild /= Nothing

                    color =
                        Colors.buildStatusColor
                            (not isHovered)
                            finishedBuildStatus
                in
                if isRunning then
                    striped
                        { pipelineRunningKeyframes = "pipeline-running"
                        , thickColor =
                            Colors.buildStatusColor
                                False
                                finishedBuildStatus
                        , thinColor =
                            Colors.buildStatusColor
                                True
                                finishedBuildStatus
                        }

                else
                    solid color
           )


jobPreviewLink : List (Html.Attribute msg)
jobPreviewLink =
    [ style "flex-grow" "1" ]


loadingView : List (Html.Attribute msg)
loadingView =
    [ style "display" "flex"
    , style "justify-content" "center"
    , style "align-items" "center"
    , style "width" "100%"
    , style "height" "100%"
    ]


pipelineSectionHeader : List (Html.Attribute msg)
pipelineSectionHeader =
    [ style "font-size" "22px"
    , style "font-weight" Views.Styles.fontWeightBold
    , style "padding" <| String.fromInt GridConstants.padding ++ "px"
    ]
