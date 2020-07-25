module Build.Styles exposing
    ( MetadataCellType(..)
    , abortButton
    , acrossStepSubHeaderLabel
    , body
    , buttonTooltip
    , buttonTooltipArrow
    , durationTooltip
    , durationTooltipArrow
    , errorLog
    , firstOccurrenceTooltip
    , header
    , historyItem
    , metadataCell
    , metadataTable
    , retryTabList
    , stepHeader
    , stepHeaderLabel
    , stepStatusIcon
    , tab
    , triggerButton
    )

import Application.Styles
import Build.Models exposing (StepHeaderType(..))
import Build.StepTree.Models exposing (StepState(..))
import Colors
import Concourse.BuildStatus exposing (BuildStatus(..))
import Dashboard.Styles exposing (striped)
import Html
import Html.Attributes exposing (style)
import Views.Styles


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


historyItem : BuildStatus -> Bool -> BuildStatus -> List (Html.Attribute msg)
historyItem currentBuildStatus isCurrent status =
    [ style "letter-spacing" "-1px"
    , style "padding" "0 2px 0 2px"
    , style "border-top" <| "1px solid " ++ Colors.buildStatusColor isCurrent currentBuildStatus
    , style "border-right" <| "1px solid " ++ Colors.buildStatusColor False status
    , style "opacity" <|
        if isCurrent then
            "1"

        else
            "0.8"
    ]
        ++ (case status of
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
           )


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


buttonTooltipArrow : List (Html.Attribute msg)
buttonTooltipArrow =
    [ style "width" "0"
    , style "height" "0"
    , style "left" "50%"
    , style "bottom" "0"
    , style "margin-left" "-5px"
    , style "border-bottom" <| "5px solid " ++ Colors.frame
    , style "border-left" "5px solid transparent"
    , style "border-right" "5px solid transparent"
    , style "position" "absolute"
    ]


buttonTooltip : Int -> List (Html.Attribute msg)
buttonTooltip width =
    [ style "position" "absolute"
    , style "right" "0"
    , style "top" "100%"
    , style "width" <| String.fromInt width ++ "px"
    , style "color" Colors.text
    , style "background-color" Colors.frame
    , style "padding" "10px"
    , style "text-align" "right"
    , style "pointer-events" "none"
    , style "z-index" "1"

    -- ^ need a value greater than 0 (inherited from .fixed-header) since this
    -- element is earlier in the document than the build tabs
    ]
        ++ Views.Styles.defaultFont


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


stepHeaderLabel : StepHeaderType -> List (Html.Attribute msg)
stepHeaderLabel headerType =
    [ style "color" <|
        case headerType of
            StepHeaderGet True ->
                Colors.started

            _ ->
                Colors.pending
    , style "line-height" "28px"
    , style "padding-left" "6px"
    ]


acrossStepSubHeaderLabel : List (Html.Attribute msg)
acrossStepSubHeaderLabel =
    [ style "line-height" "28px"
    , style "padding-left" "6px"
    ]


stepStatusIcon : List (Html.Attribute msg)
stepStatusIcon =
    [ style "background-size" "14px 14px"
    , style "position" "relative"
    ]


firstOccurrenceTooltip : List (Html.Attribute msg)
firstOccurrenceTooltip =
    [ style "background-color" Colors.tooltipBackground
    , style "padding" "5px"
    , style "z-index" "100"
    , style "width" "6em"
    , style "pointer-events" "none"
    ]
        ++ Application.Styles.disableInteraction


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


tabList : List (Html.Attribute msg)
tabList =
    [ style "line-height" "26px"
    , style "background-color" Colors.background
    ]


retryTabList : List (Html.Attribute msg)
retryTabList =
    style "font-size" "16px"
        :: style "margin" "0"
        :: tabList


tab :
    { isHovered : Bool, isCurrent : Bool, isStarted : Bool }
    -> List (Html.Attribute msg)
tab { isHovered, isCurrent, isStarted } =
    [ style "display" "inline-block"
    , style "position" "relative"
    , style "padding" "0 5px"
    , style "font-weight" Views.Styles.fontWeightDefault
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
