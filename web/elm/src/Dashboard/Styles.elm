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
        , pipelineCardFooter
        , pipelineStatusIcon
        , pipelineCardTransitionAge
        )

import Colors
import Concourse.PipelineStatus exposing (PipelineStatus(..))


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
    , running : Bool
    , pipelineRunningKeyframes : String
    }
    -> List ( String, String )
pipelineCardBanner { status, running, pipelineRunningKeyframes } =
    let
        solid : String -> List ( String, String )
        solid color =
            [ ( "background-color", color ) ]

        striped : String -> List ( String, String )
        striped color =
            [ ( "background-image"
              , withStripes color Colors.runningStripes
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

        texture : Bool -> String -> List ( String, String )
        texture isRunning =
            if isRunning then
                striped
            else
                solid

        color =
            statusColor status

        isRunning =
            Concourse.PipelineStatus.isRunning status
    in
        [ ( "height", "7px" ) ] ++ texture isRunning color


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


infoBar : Int -> List ( String, String )
infoBar screenWidth =
    let
        styleList : List ( a, Bool ) -> List a
        styleList =
            List.filter Tuple.second >> List.map Tuple.first
    in
        ([ ( "position", "fixed" )
         , ( "bottom", "0" )
         , ( "line-height", "35px" )
         , ( "padding", "7.5px 30px" )
         , ( "background-color", Colors.frame )
         , ( "width", "100%" )
         , ( "box-sizing", "border-box" )
         , ( "display", "flex" )
         , ( "justify-content", "space-between" )
         ]
            ++ styleList
                [ ( ( "flex-direction", "column" ), screenWidth <= 1230 )
                ]
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
