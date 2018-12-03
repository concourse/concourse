module Dashboard.Styles exposing (..)

import Colors
import Concourse.PipelineStatus exposing (PipelineStatus(..))


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
    in
        [ ( "height", "7px" ) ]
            ++ case status of
                PipelineStatusPaused ->
                    solid Colors.paused

                PipelineStatusSucceeded isRunning ->
                    texture isRunning Colors.success

                PipelineStatusPending isRunning ->
                    texture isRunning Colors.pending

                PipelineStatusFailed isRunning ->
                    texture isRunning Colors.failure

                PipelineStatusErrored isRunning ->
                    texture isRunning Colors.error

                PipelineStatusAborted isRunning ->
                    texture isRunning Colors.aborted
