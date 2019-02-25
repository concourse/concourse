module Dashboard.Styles exposing
    ( asciiArt
    , cardBody
    , cardFooter
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
    , pipelineStatusIcon
    , previewPlaceholder
    , resourceErrorTriangle
    , runningLegendItem
    , topCliIcon
    , welcomeCard
    , welcomeCardBody
    , welcomeCardTitle
    )

import Application.Styles
import Colors
import Concourse.Cli as Cli
import Concourse.PipelineStatus exposing (PipelineStatus(..))
import ScreenSize


statusColor : PipelineStatus -> String
statusColor status =
    case status of
        PipelineStatusPaused ->
            Colors.paused

        PipelineStatusSucceeded _ ->
            Colors.success

        PipelineStatusPending _ ->
            Colors.pending

        PipelineStatusFailed _ ->
            Colors.failure

        PipelineStatusErrored _ ->
            Colors.error

        PipelineStatusAborted _ ->
            Colors.aborted


pipelineCard : List ( String, String )
pipelineCard =
    [ ( "cursor", "move" )
    , ( "margin", "25px" )
    ]


pipelineCardBanner :
    { status : PipelineStatus
    , pipelineRunningKeyframes : String
    }
    -> List ( String, String )
pipelineCardBanner { status, pipelineRunningKeyframes } =
    let
        color =
            statusColor status

        isRunning =
            Concourse.PipelineStatus.isRunning status
    in
    [ ( "height", "7px" ) ] ++ texture pipelineRunningKeyframes isRunning color


noPipelineCardHd : List ( String, String )
noPipelineCardHd =
    [ ( "background-color", Colors.card )
    , ( "font-size", "14px" )
    , ( "width", "200px" )
    , ( "height", "60px" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    , ( "letter-spacing", "1px" )
    , ( "margin-right", "60px" )
    ]


noPipelineCardTextHd : List ( String, String )
noPipelineCardTextHd =
    [ ( "padding", "10px" )
    ]


noPipelineCardHeader : List ( String, String )
noPipelineCardHeader =
    [ ( "color", Colors.dashboardText )
    , ( "background-color", Colors.card )
    , ( "font-size", "1.5em" )
    , ( "letter-spacing", "0.1em" )
    , ( "padding", "12.5px" )
    , ( "text-align", "center" )
    , ( "-webkit-font-smoothing", "antialiased" )
    ]


pipelineCardHeader : List ( String, String )
pipelineCardHeader =
    [ ( "background-color", Colors.card )
    , ( "color", Colors.dashboardText )
    , ( "font-size", "1.5em" )
    , ( "letter-spacing", "0.1em" )
    , ( "-webkit-font-smoothing", "antialiased" )
    , ( "padding", "12.5px" )
    ]


pipelineName : List ( String, String )
pipelineName =
    [ ( "width", "245px" )
    , ( "white-space", "nowrap" )
    , ( "overflow", "hidden" )
    , ( "text-overflow", "ellipsis" )
    ]


cardBody : List ( String, String )
cardBody =
    [ ( "width", "200px" )
    , ( "height", "120px" )
    , ( "padding", "20px 36px" )
    , ( "background-color", Colors.card )
    , ( "margin", "2px 0" )
    , ( "display", "flex" )
    ]


pipelineCardBody : List ( String, String )
pipelineCardBody =
    [ ( "background-color", Colors.card )
    , ( "margin", "2px 0" )
    ]


cardFooter : List ( String, String )
cardFooter =
    [ ( "height", "47px" )
    , ( "background-color", Colors.card )
    ]


previewPlaceholder : List ( String, String )
previewPlaceholder =
    [ ( "background-color", Colors.background )
    , ( "flex-grow", "1" )
    ]


pipelineCardHd : PipelineStatus -> List ( String, String )
pipelineCardHd status =
    [ ( "display", "flex" )
    , ( "height", "60px" )
    , ( "width", "200px" )
    , ( "margin", "0 60px 4px 0" )
    , ( "position", "relative" )
    , ( "background-color"
      , case status of
            PipelineStatusSucceeded _ ->
                Colors.successFaded

            PipelineStatusFailed _ ->
                Colors.failure

            PipelineStatusErrored _ ->
                Colors.error

            _ ->
                Colors.card
      )
    , ( "font-size", "19px" )
    , ( "letter-spacing", "1px" )
    ]


pipelineCardBodyHd : List ( String, String )
pipelineCardBodyHd =
    [ ( "width", "180px" )
    , ( "white-space", "nowrap" )
    , ( "overflow", "hidden" )
    , ( "text-overflow", "ellipsis" )
    , ( "align-self", "center" )
    , ( "padding", "10px" )
    ]


pipelineCardBannerHd :
    { status : PipelineStatus
    , pipelineRunningKeyframes : String
    }
    -> List ( String, String )
pipelineCardBannerHd { status, pipelineRunningKeyframes } =
    let
        color =
            statusColor status

        isRunning =
            Concourse.PipelineStatus.isRunning status
    in
    [ ( "width", "8px" ) ]
        ++ texture pipelineRunningKeyframes isRunning color


solid : String -> List ( String, String )
solid color =
    [ ( "background-color", color ) ]


striped : String -> String -> List ( String, String )
striped pipelineRunningKeyframes color =
    [ ( "background-image"
      , withStripes color Colors.card
      )
    , ( "background-size", "106px 114px" )
    , ( "animation"
      , pipelineRunningKeyframes ++ " 3s linear infinite"
      )
    ]


withStripes : String -> String -> String
withStripes color stripeColor =
    "repeating-linear-gradient(-115deg,"
        ++ stripeColor
        ++ " 0,"
        ++ stripeColor
        ++ " 10px,"
        ++ color
        ++ " 0,"
        ++ color
        ++ " 16px)"


texture : String -> Bool -> String -> List ( String, String )
texture pipelineRunningKeyframes isRunning =
    if isRunning then
        striped pipelineRunningKeyframes

    else
        solid


pipelineCardFooter : List ( String, String )
pipelineCardFooter =
    [ ( "padding", "13.5px" )
    , ( "display", "flex" )
    , ( "justify-content", "space-between" )
    , ( "background-color", Colors.card )
    ]


pipelineStatusIcon : PipelineStatus -> List ( String, String )
pipelineStatusIcon pipelineStatus =
    let
        image =
            case pipelineStatus of
                PipelineStatusPaused ->
                    "ic-pause-blue.svg"

                PipelineStatusPending _ ->
                    "ic-pending-grey.svg"

                PipelineStatusSucceeded _ ->
                    "ic-running-green.svg"

                PipelineStatusFailed _ ->
                    "ic-failing-red.svg"

                PipelineStatusAborted _ ->
                    "ic-aborted-brown.svg"

                PipelineStatusErrored _ ->
                    "ic-error-orange.svg"
    in
    [ ( "background-image", "url(/public/images/" ++ image ++ ")" )
    , ( "height", "20px" )
    , ( "width", "20px" )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-size", "contain" )
    ]


pipelineCardTransitionAge : PipelineStatus -> List ( String, String )
pipelineCardTransitionAge status =
    [ ( "color", statusColor status )
    , ( "font-size", "18px" )
    , ( "line-height", "20px" )
    , ( "letter-spacing", "0.05em" )
    , ( "margin-left", "8px" )
    ]


infoBar :
    { hideLegend : Bool, screenSize : ScreenSize.ScreenSize }
    -> List ( String, String )
infoBar { hideLegend, screenSize } =
    [ ( "position", "fixed" )
    , ( "bottom", "0" )
    , ( "line-height", "35px" )
    , ( "padding", "7.5px 30px" )
    , ( "background-color", Colors.frame )
    , ( "width", "100%" )
    , ( "box-sizing", "border-box" )
    , ( "display", "flex" )
    , ( "justify-content"
      , if hideLegend then
            "flex-end"

        else
            "space-between"
      )
    ]
        ++ (case screenSize of
                ScreenSize.Mobile ->
                    [ ( "flex-direction", "column" ) ]

                ScreenSize.Desktop ->
                    [ ( "flex-direction", "column" ) ]

                ScreenSize.BigDesktop ->
                    []
           )


legend : List ( String, String )
legend =
    [ ( "display", "flex" )
    , ( "flex-wrap", "wrap" )
    ]


legendItem : List ( String, String )
legendItem =
    [ ( "display", "flex" )
    , ( "text-transform", "uppercase" )
    , ( "align-items", "center" )
    , ( "color", Colors.bottomBarText )
    , ( "margin-right", "20px" )
    ]


legendSeparator : List ( String, String )
legendSeparator =
    [ ( "color", Colors.bottomBarText )
    , ( "margin-right", "20px" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    ]


highDensityToggle : List ( String, String )
highDensityToggle =
    [ ( "color", Colors.bottomBarText )
    , ( "margin-right", "20px" )
    , ( "display", "flex" )
    , ( "text-transform", "uppercase" )
    , ( "align-items", "center" )
    ]


highDensityIcon : Bool -> List ( String, String )
highDensityIcon highDensity =
    [ ( "background-image"
      , if highDensity then
            "url(/public/images/ic-hd-on.svg)"

        else
            "url(/public/images/ic-hd-off.svg)"
      )
    , ( "background-size", "contain" )
    , ( "height", "20px" )
    , ( "width", "35px" )
    , ( "flex-shrink", "0" )
    , ( "margin-right", "10px" )
    ]


info : List ( String, String )
info =
    [ ( "display", "flex" )
    , ( "color", Colors.bottomBarText )
    , ( "font-size", "1.25em" )
    ]


infoItem : List ( String, String )
infoItem =
    [ ( "margin-right", "30px" )
    , ( "display", "flex" )
    , ( "align-items", "center" )
    ]


infoCliIcon : { hovered : Bool, cli : Cli.Cli } -> List ( String, String )
infoCliIcon { hovered, cli } =
    [ ( "margin-right", "10px" )
    , ( "width", "20px" )
    , ( "height", "20px" )
    , ( "background-image", Cli.iconUrl cli )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "background-size", "contain" )
    , ( "opacity"
      , if hovered then
            "1"

        else
            "0.5"
      )
    ]


topCliIcon : { hovered : Bool, cli : Cli.Cli } -> List ( String, String )
topCliIcon { hovered, cli } =
    [ ( "opacity"
      , if hovered then
            "1"

        else
            "0.5"
      )
    , ( "background-image", Cli.iconUrl cli )
    , ( "background-position", "50% 50%" )
    , ( "background-repeat", "no-repeat" )
    , ( "width", "32px" )
    , ( "height", "32px" )
    , ( "margin", "5px" )
    , ( "z-index", "1" )
    ]


welcomeCard : List ( String, String )
welcomeCard =
    [ ( "background-color", Colors.card )
    , ( "margin", "25px" )
    , ( "padding", "40px" )
    , ( "-webkit-font-smoothing", "antialiased" )
    , ( "position", "relative" )
    , ( "overflow", "hidden" )
    , ( "font-weight", "400" )
    , ( "display", "flex" )
    , ( "flex-direction", "column" )
    ]


welcomeCardBody : List ( String, String )
welcomeCardBody =
    [ ( "font-size", "16px" )
    , ( "z-index", "2" )
    ]


welcomeCardTitle : List ( String, String )
welcomeCardTitle =
    [ ( "font-size", "32px" ) ]


resourceErrorTriangle : List ( String, String )
resourceErrorTriangle =
    [ ( "position", "absolute" )
    , ( "top", "0" )
    , ( "right", "0" )
    , ( "width", "0" )
    , ( "height", "0" )
    , ( "border-top", "30px solid " ++ Colors.resourceError )
    , ( "border-left", "30px solid transparent" )
    ]


runningLegendItem : List ( String, String )
runningLegendItem =
    [ ( "background-image", "url(/public/images/ic-running-legend.svg)" )
    , ( "height", "20px" )
    , ( "width", "20px" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    ]


asciiArt : List ( String, String )
asciiArt =
    [ ( "font-size", "16px" )
    , ( "line-height", "8px" )
    , ( "position", "absolute" )
    , ( "top", "0" )
    , ( "left", "23em" )
    , ( "margin", "0" )
    , ( "white-space", "pre" )
    , ( "color", Colors.asciiArt )
    , ( "z-index", "1" )
    ]
        ++ Application.Styles.disableInteraction


noResults : List ( String, String )
noResults =
    [ ( "text-align", "center" )
    , ( "font-size", "13px" )
    , ( "margin-top", "20px" )
    ]
