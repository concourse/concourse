module Dashboard.Styles
    exposing
        ( infoBar
        , legend
        , legendItem
        , legendSeparator
        , highDensityToggle
        , highDensityIcon
        , info
        , infoItem
        , infoCliIcon
        , pipelineCardBanner
        , pipelineCardHd
        , pipelineCardBannerHd
        , pipelineCardBodyHd
        , pipelineCardFooter
        , pipelineStatusIcon
        , pipelineCardTransitionAge
        )

import Colors
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


pipelineCardHd : List ( String, String )
pipelineCardHd =
    [ ( "display", "flex" )
    , ( "height", "60px" )
    , ( "width", "200px" )
    , ( "margin", "0 60px 4px 0" )
    , ( "position", "relative" )
    ]


pipelineCardBodyHd : PipelineStatus -> List ( String, String )
pipelineCardBodyHd status =
    case status of
        PipelineStatusSucceeded _ ->
            [ ( "background-color", Colors.successFaded ) ]

        PipelineStatusFailed _ ->
            [ ( "background-color", Colors.failure ) ]

        PipelineStatusErrored _ ->
            [ ( "background-color", Colors.error ) ]

        _ ->
            []


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
        [ ( "width", "8px" )
        , ( "background-size", "35px" )
        ]
            ++ texture pipelineRunningKeyframes isRunning color


solid : String -> List ( String, String )
solid color =
    [ ( "background-color", color ) ]


striped : String -> String -> List ( String, String )
striped pipelineRunningKeyframes color =
    [ ( "background-image"
      , withStripes color Colors.card
      )
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
    [ ( "border-top", "2px solid " ++ Colors.dashboardBackground )
    , ( "padding", "13.5px" )
    , ( "display", "flex" )
    , ( "justify-content", "space-between" )
    ]


pipelineStatusIcon : PipelineStatus -> List ( String, String )
pipelineStatusIcon pipelineStatus =
    let
        image =
            case pipelineStatus of
                PipelineStatusPaused ->
                    "ic_pause_blue.svg"

                PipelineStatusPending _ ->
                    "ic_pending_grey.svg"

                PipelineStatusSucceeded _ ->
                    "ic_running_green.svg"

                PipelineStatusFailed _ ->
                    "ic_failing_red.svg"

                PipelineStatusAborted _ ->
                    "ic_aborted_brown.svg"

                PipelineStatusErrored _ ->
                    "ic_error_orange.svg"
    in
        [ ( "background-image", "url(public/images/" ++ image ++ ")" )
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


infoBar : ScreenSize.ScreenSize -> List ( String, String )
infoBar screenSize =
    [ ( "position", "fixed" )
    , ( "bottom", "0" )
    , ( "line-height", "35px" )
    , ( "padding", "7.5px 30px" )
    , ( "background-color", Colors.frame )
    , ( "width", "100%" )
    , ( "box-sizing", "border-box" )
    , ( "display", "flex" )
    , ( "justify-content", "space-between" )
    ]
        ++ case screenSize of
            ScreenSize.Mobile ->
                [ ( "flex-direction", "column" ) ]

            ScreenSize.Desktop ->
                [ ( "flex-direction", "column" ) ]

            ScreenSize.BigDesktop ->
                []


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
            "url(public/images/ic_hd_on.svg)"
        else
            "url(public/images/ic_hd_off.svg)"
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


infoCliIcon : Bool -> List ( String, String )
infoCliIcon hovered =
    [ ( "margin-right", "10px" )
    , ( "font-size", "1.2em" )
    , ( "color"
      , if hovered then
            Colors.cliIconHover
        else
            Colors.bottomBarText
      )
    ]
