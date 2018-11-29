module Dashboard.Styles exposing (..)

import Colors
import Concourse


pipelineCardBanner :
    { status : Concourse.PipelineStatus
    , running : Bool
    }
    -> List ( String, String )
pipelineCardBanner { status, running } =
    let
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
    in
        [ ( "height", "7px" ) ]
            ++ case ( status, running ) of
                ( Concourse.PipelineStatusSucceeded, False ) ->
                    [ ( "background-color", Colors.success ) ]

                ( Concourse.PipelineStatusSucceeded, True ) ->
                    [ ( "background-image", withStripes Colors.success Colors.runningStripes )
                    , ( "animation", "pipeline-running 3s linear infinite" )
                    ]

                ( Concourse.PipelineStatusPaused, _ ) ->
                    [ ( "background-color", Colors.paused ) ]

                _ ->
                    []
