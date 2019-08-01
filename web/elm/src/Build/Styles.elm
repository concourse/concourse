module Build.Styles exposing
    ( abortButton
    , body
    , durationTooltip
    , durationTooltipArrow
    , errorLog
    , firstOccurrenceTooltip
    , firstOccurrenceTooltipArrow
    , header
    , historyItem
    , retryTab
    , retryTabList
    , stepHeader
    , stepHeaderIcon
    , stepStatusIcon
    , triggerButton
    , triggerTooltip
    )

import Application.Styles
import Build.Models exposing (StepHeaderType(..))
import Build.StepTree.Models exposing (StepState(..))
import Colors
import Concourse
import Dashboard.Styles exposing (striped)
import Html
import Html.Attributes exposing (style)


header : Concourse.BuildStatus -> List (Html.Attribute msg)
header status =
    [ style "display" "flex"
    , style "justify-content" "space-between"
    , style "height" "60px"
    , style "background" <|
        case status of
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
    ]


body : List (Html.Attribute msg)
body =
    [ style "overflow-y" "auto"
    , style "outline" "none"
    , style "-webkit-overflow-scrolling" "touch"
    ]


historyItem : Concourse.BuildStatus -> List (Html.Attribute msg)
historyItem status =
    case status of
        Concourse.BuildStatusStarted ->
            striped
                { pipelineRunningKeyframes = "pipeline-running"
                , thickColor = Colors.startedFaded
                , thinColor = Colors.started
                }

        Concourse.BuildStatusPending ->
            [ style "background" Colors.pending ]

        Concourse.BuildStatusSucceeded ->
            [ style "background" Colors.success ]

        Concourse.BuildStatusFailed ->
            [ style "background" Colors.failure ]

        Concourse.BuildStatusErrored ->
            [ style "background" Colors.error ]

        Concourse.BuildStatusAborted ->
            [ style "background" Colors.aborted ]


triggerButton : Bool -> Bool -> Concourse.BuildStatus -> List (Html.Attribute msg)
triggerButton buttonDisabled hovered status =
    [ style "cursor" <|
        if buttonDisabled then
            "default"

        else
            "pointer"
    , style "position" "relative"
    , style "background-color" <|
        Colors.buildStatusColor (not hovered || buttonDisabled) status
    ]
        ++ button


abortButton : Bool -> List (Html.Attribute msg)
abortButton isHovered =
    [ style "cursor" "pointer"
    , style "background-color" <|
        if isHovered then
            Colors.failureFaded

        else
            Colors.failure
    ]
        ++ button


button : List (Html.Attribute msg)
button =
    [ style "padding" "10px"
    , style "outline" "none"
    , style "margin" "0"
    , style "border-width" "0 0 0 1px"
    , style "border-color" Colors.background
    , style "border-style" "solid"
    ]


triggerTooltip : List (Html.Attribute msg)
triggerTooltip =
    [ style "position" "absolute"
    , style "right" "100%"
    , style "top" "15px"
    , style "width" "300px"
    , style "color" Colors.buildTooltipBackground
    , style "font-size" "12px"
    , style "font-family" "Inconsolata,monospace"
    , style "padding" "10px"
    , style "text-align" "right"
    , style "pointer-events" "none"
    ]


stepHeader : StepState -> List (Html.Attribute msg)
stepHeader state =
    [ style "display" "flex"
    , style "justify-content" "space-between"
    , style "border" <|
        "1px solid "
            ++ (case state of
                    StepStateFailed ->
                        Colors.failure

                    StepStateErrored ->
                        Colors.error

                    StepStatePending ->
                        Colors.frame

                    StepStateRunning ->
                        Colors.frame

                    StepStateInterrupted ->
                        Colors.frame

                    StepStateCancelled ->
                        Colors.frame

                    StepStateSucceeded ->
                        Colors.frame
               )
    ]


stepHeaderIcon : StepHeaderType -> List (Html.Attribute msg)
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
    [ style "height" "28px"
    , style "width" "28px"
    , style "background-image" <|
        "url(/public/images/ic-"
            ++ image
            ++ ".svg)"
    , style "background-repeat" "no-repeat"
    , style "background-position" "50% 50%"
    , style "background-size" "14px 14px"
    , style "position" "relative"
    ]


stepStatusIcon : List (Html.Attribute msg)
stepStatusIcon =
    [ style "background-size" "14px 14px"
    , style "position" "relative"
    ]


firstOccurrenceTooltip : List (Html.Attribute msg)
firstOccurrenceTooltip =
    [ style "position" "fixed"
    , style "transform" "translate(0, -100%)"
    , style "background-color" Colors.tooltipBackground
    , style "padding" "5px"
    , style "z-index" "100"
    , style "width" "6em"
    , style "pointer-events" "none"
    ]
        ++ Application.Styles.disableInteraction


firstOccurrenceTooltipArrow : List (Html.Attribute msg)
firstOccurrenceTooltipArrow =
    [ style "width" "0"
    , style "height" "0"
    , style "left" "50%"
    , style "margin-left" "-5px"
    , style "border-top" <| "5px solid " ++ Colors.tooltipBackground
    , style "border-left" "5px solid transparent"
    , style "border-right" "5px solid transparent"
    , style "position" "absolute"
    ]


durationTooltip : List (Html.Attribute msg)
durationTooltip =
    [ style "position" "fixed"
    , style "transform" "translate(0, -100%)"
    , style "background-color" Colors.tooltipBackground
    , style "padding" "5px"
    , style "z-index" "100"
    , style "pointer-events" "none"
    ]
        ++ Application.Styles.disableInteraction


durationTooltipArrow : List (Html.Attribute msg)
durationTooltipArrow =
    [ style "width" "0"
    , style "height" "0"
    , style "left" "50%"
    , style "top" "0px"
    , style "margin-left" "-5px"
    , style "border-top" <| "5px solid " ++ Colors.tooltipBackground
    , style "border-left" "5px solid transparent"
    , style "border-right" "5px solid transparent"
    , style "position" "absolute"
    ]


errorLog : List (Html.Attribute msg)
errorLog =
    [ style "color" Colors.errorLog
    , style "background-color" Colors.frame
    , style "padding" "5px 10px"
    ]


retryTabList : List (Html.Attribute msg)
retryTabList =
    [ style "margin" "0"
    , style "font-size" "16px"
    , style "line-height" "26px"
    , style "background-color" Colors.background
    ]


retryTab :
    { isHovered : Bool, isCurrent : Bool, isStarted : Bool }
    -> List (Html.Attribute msg)
retryTab { isHovered, isCurrent, isStarted } =
    [ style "display" "inline-block"
    , style "padding" "0 5px"
    , style "font-weight" "700"
    , style "cursor" "pointer"
    , style "color" Colors.retryTabText
    , style "background-color" <|
        if isHovered || isCurrent then
            Colors.paginationHover

        else
            Colors.background
    , style "opacity" <|
        if isStarted then
            "1"

        else
            "0.5"
    ]
