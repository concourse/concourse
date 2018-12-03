module Dashboard.Styles
    exposing
        ( infoBar
        , legend
        , legendItem
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


infoBar : List ( String, String )
infoBar =
    [ ( "height", "50px" )
    , ( "position", "fixed" )
    , ( "bottom", "0" )
    , ( "background-color", Colors.frame )
    , ( "width", "100%" )
    , ( "display", "flex" )
    , ( "justify-content", "space-between" )
    , ( "align-items", "center" )
    ]


legend : List ( String, String )
legend =
    [ ( "display", "flex" )
    ]


legendItem : List ( String, String )
legendItem =
    [ ( "display", "flex" )
    , ( "text-transform", "uppercase" )
    , ( "align-items", "center" )
    , ( "color", Colors.bottomBarText )
    ]
