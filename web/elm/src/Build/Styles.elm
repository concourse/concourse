module Build.Styles exposing
    ( MetadataCellType(..)
    , abortButton
    , body
    , durationTooltip
    , durationTooltipArrow
    , errorLog
    , firstOccurrenceTooltip
    , firstOccurrenceTooltipArrow
    , header
    , historyItem
    , metadataCell
    , metadataTable
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
import Concourse.BuildStatus exposing (BuildStatus(..))
import Dashboard.Styles exposing (striped)
import Html
import Html.Attributes exposing (style)


header : BuildStatus -> List (Html.Attribute msg)
header status =
    [ style "display" "flex"
    , style "justify-content" "space-between"
    , style "height" "60px"
    , style "background" <|
        case status of
            BuildStatusStarted ->
                Colors.startedFaded

            BuildStatusPending ->
                Colors.pending

            BuildStatusSucceeded ->
                Colors.success

            BuildStatusFailed ->
                Colors.failure

            BuildStatusErrored ->
                Colors.error

            BuildStatusAborted ->
                Colors.aborted
    ]


body : List (Html.Attribute msg)
body =
    [ style "overflow-y" "auto"
    , style "outline" "none"
    , style "-webkit-overflow-scrolling" "touch"
    ]


historyItem : BuildStatus -> List (Html.Attribute msg)
historyItem status =
    case status of
        BuildStatusStarted ->
            striped
                { pipelineRunningKeyframes = "pipeline-running"
                , thickColor = Colors.startedFaded
                , thinColor = Colors.started
                }

        BuildStatusPending ->
            [ style "background" Colors.pending ]

        BuildStatusSucceeded ->
            [ style "background" Colors.success ]

        BuildStatusFailed ->
            [ style "background" Colors.failure ]

        BuildStatusErrored ->
            [ style "background" Colors.error ]

        BuildStatusAborted ->
            [ style "background" Colors.aborted ]


triggerButton : Bool -> Bool -> BuildStatus -> List (Html.Attribute msg)
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
                        Colors.started

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

                StepHeaderSetPipeline ->
                    "breadcrumb-pipeline"
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


firstOccurrenceTooltip : Float -> Float -> List (Html.Attribute msg)
firstOccurrenceTooltip bottom left =
    [ style "position" "fixed"
    , style "left" <| String.fromFloat left ++ "px"
    , style "bottom" <| String.fromFloat bottom ++ "px"
    , style "background-color" Colors.tooltipBackground
    , style "padding" "5px"
    , style "z-index" "100"
    , style "width" "6em"
    , style "pointer-events" "none"
    ]
        ++ Application.Styles.disableInteraction


firstOccurrenceTooltipArrow : Float -> Float -> Float -> List (Html.Attribute msg)
firstOccurrenceTooltipArrow bottom left width =
    [ style "position" "fixed"
    , style "left" <| String.fromFloat (left + width / 2) ++ "px"
    , style "bottom" <| String.fromFloat bottom ++ "px"
    , style "margin-bottom" "-5px"
    , style "margin-left" "-5px"
    , style "width" "0"
    , style "height" "0"
    , style "border-top" <| "5px solid " ++ Colors.tooltipBackground
    , style "border-left" "5px solid transparent"
    , style "border-right" "5px solid transparent"
    , style "z-index" "100"
    ]


durationTooltip : List (Html.Attribute msg)
durationTooltip =
    [ style "position" "absolute"
    , style "right" "0"
    , style "bottom" "100%"
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


type MetadataCellType
    = Key
    | Value


metadataTable : List (Html.Attribute msg)
metadataTable =
    [ style "border-collapse" "collapse"
    , style "margin-bottom" "5px"
    ]


metadataCell : MetadataCellType -> List (Html.Attribute msg)
metadataCell cell =
    case cell of
        Key ->
            [ style "text-align" "left"
            , style "vertical-align" "top"
            , style "background-color" "rgb(45,45,45)"
            , style "border-bottom" "5px solid rgb(45,45,45)"
            , style "padding" "5px"
            ]

        Value ->
            [ style "text-align" "left"
            , style "vertical-align" "top"
            , style "background-color" "rgb(30,30,30)"
            , style "border-bottom" "5px solid rgb(45,45,45)"
            , style "padding" "5px"
            ]
