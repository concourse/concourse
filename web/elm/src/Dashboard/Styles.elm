module Dashboard.Styles exposing (..)

import Colors
import Concourse


pipelineCardBanner :
    { status : Concourse.PipelineStatus
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

        texture : String -> List ( String, String )
        texture =
            if running then
                striped
            else
                solid

        color : String
        color =
            case status of
                Concourse.PipelineStatusSucceeded ->
                    Colors.success

                Concourse.PipelineStatusPaused ->
                    Colors.paused

                Concourse.PipelineStatusPending ->
                    Colors.pending

                Concourse.PipelineStatusFailed ->
                    Colors.failure

                Concourse.PipelineStatusErrored ->
                    Colors.error

                Concourse.PipelineStatusAborted ->
                    Colors.aborted

                Concourse.PipelineStatusRunning ->
                    Colors.pending
    in
        [ ( "height", "7px" ) ] ++ texture color
