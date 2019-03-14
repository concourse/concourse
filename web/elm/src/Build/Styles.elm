module Build.Styles exposing
    ( abortButton
    , abortIcon
    , firstOccurrenceTooltip
    , firstOccurrenceTooltipArrow
    , header
    , historyItem
    , stepHeader
    , stepHeaderIcon
    , stepStatusIcon
    , triggerButton
    , triggerIcon
    , triggerTooltip
    )

import Application.Styles
import Build.Models exposing (StepHeaderType(..))
import Colors
import Concourse
import Dashboard.Styles exposing (striped)


header : Concourse.BuildStatus -> List ( String, String )
header status =
    [ ( "display", "flex" )
    , ( "justify-content", "space-between" )
    , ( "background"
      , case status of
            Concourse.BuildStatusStarted ->
                Colors.startedFaded

            Concourse.BuildStatusPending ->
                Colors.pending

            Concourse.BuildStatusSucceeded ->
                Colors.success

            Concourse.BuildStatusFailed ->
                Colors.failure

            Concourse.BuildStatusErrored ->
                Colors.error

            Concourse.BuildStatusAborted ->
                Colors.aborted
      )
    ]


historyItem : Concourse.BuildStatus -> List ( String, String )
historyItem status =
    case status of
        Concourse.BuildStatusStarted ->
            striped
                { pipelineRunningKeyframes = "pipeline-running"
                , thickColor = Colors.startedFaded
                , thinColor = Colors.started
                }

        Concourse.BuildStatusPending ->
            [ ( "background", Colors.pending ) ]

        Concourse.BuildStatusSucceeded ->
            [ ( "background", Colors.success ) ]

        Concourse.BuildStatusFailed ->
            [ ( "background", Colors.failure ) ]

        Concourse.BuildStatusErrored ->
            [ ( "background", Colors.error ) ]

        Concourse.BuildStatusAborted ->
            [ ( "background", Colors.aborted ) ]


triggerButton : Bool -> Bool -> Concourse.BuildStatus -> List ( String, String )
triggerButton buttonDisabled hovered status =
    [ ( "cursor"
      , if buttonDisabled then
            "default"

        else
            "pointer"
      )
    , ( "position", "relative" )
    , ( "background-color"
      , Colors.buildStatusColor (not hovered || buttonDisabled) status
      )
    ]
        ++ button


abortButton : Bool -> List ( String, String )
abortButton isHovered =
    [ ( "cursor", "pointer" )
    , ( "background-color"
      , if isHovered then
            Colors.failureFaded

        else
            Colors.failure
      )
    ]
        ++ button


button : List ( String, String )
button =
    [ ( "padding", "10px" )
    , ( "outline", "none" )
    , ( "margin", "0" )
    , ( "border-width", "0 0 0 1px" )
    , ( "border-color", Colors.background )
    , ( "border-style", "solid" )
    ]


triggerIcon : Bool -> List ( String, String )
triggerIcon hovered =
    [ ( "width", "40px" )
    , ( "height", "40px" )
    , ( "background-position", "50% 50%" )
    , ( "background-image"
      , "url(/public/images/ic-add-circle-outline-white.svg)"
      )
    , ( "background-repeat", "no-repeat" )
    ]


triggerTooltip : List ( String, String )
triggerTooltip =
    [ ( "position", "absolute" )
    , ( "right", "100%" )
    , ( "top", "15px" )
    , ( "width", "300px" )
    , ( "color", Colors.buildTooltipBackground )
    , ( "font-size", "12px" )
    , ( "font-family", "Inconsolata,monospace" )
    , ( "padding", "10px" )
    , ( "text-align", "right" )
    , ( "pointer-events", "none" )
    ]


abortIcon : Bool -> List ( String, String )
abortIcon hovered =
    [ ( "width", "40px" )
    , ( "height", "40px" )
    , ( "background-position", "50% 50%" )
    , ( "background-image"
      , "url(/public/images/ic-abort-circle-outline-white.svg)"
      )
    , ( "background-repeat", "no-repeat" )
    ]


stepHeader : List ( String, String )
stepHeader =
    [ ( "display", "flex" )
    , ( "justify-content", "space-between" )
    ]


stepHeaderIcon : StepHeaderType -> List ( String, String )
stepHeaderIcon icon =
    let
        image =
            case icon of
                StepHeaderGet False ->
                    "arrow-downward"

                StepHeaderGet True ->
                    "arrow-downward-yellow"

                StepHeaderPut ->
                    "arrow-upward"

                StepHeaderTask ->
                    "terminal"
    in
    [ ( "height", "28px" )
    , ( "width", "28px" )
    , ( "background-image"
      , "url(/public/images/ic-" ++ image ++ ".svg)"
      )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "background-size", "14px 14px" )
    , ( "position", "relative" )
    ]


stepStatusIcon : String -> List ( String, String )
stepStatusIcon image =
    [ ( "height", "28px" )
    , ( "width", "28px" )
    , ( "background-image"
      , "url(/public/images/" ++ image ++ ".svg)"
      )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "background-size", "14px 14px" )
    ]


firstOccurrenceTooltip : List ( String, String )
firstOccurrenceTooltip =
    [ ( "position", "absolute" )
    , ( "left", "0" )
    , ( "bottom", "100%" )
    , ( "background-color", Colors.tooltipBackground )
    , ( "padding", "5px" )
    , ( "z-index", "100" )
    , ( "width", "6em" )
    , ( "pointer-events", "none" )
    ]
        ++ Application.Styles.disableInteraction


firstOccurrenceTooltipArrow : List ( String, String )
firstOccurrenceTooltipArrow =
    [ ( "width", "0" )
    , ( "height", "0" )
    , ( "left", "50%" )
    , ( "margin-left", "-5px" )
    , ( "border-top", "5px solid " ++ Colors.tooltipBackground )
    , ( "border-left", "5px solid transparent" )
    , ( "border-right", "5px solid transparent" )
    , ( "position", "absolute" )
    ]
