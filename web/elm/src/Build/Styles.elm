module Build.Styles exposing
    ( MetadataCellType(..)
    , abortButton
    , body
    , changedStepTooltip
    , durationTooltip
    , errorLog
    , header
    , historyItem
    , historyTriangle
    , imageSteps
    , initializationToggle
    , keyValuePairHeaderLabel
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
    , style "position" "relative"
    , style "-webkit-overflow-scrolling" "touch"
    ]


historyItem : BuildStatus -> Bool -> BuildStatus -> List (Html.Attribute msg)
historyItem currentBuildStatus isCurrent status =
    [ style "position" "relative"
    , style "letter-spacing" "-1px"
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


historyTriangle : String -> List (Html.Attribute msg)
historyTriangle size =
    [ style "position" "absolute"
    , style "top" "0"
    , style "right" "0"
    , style "width" "0"
    , style "height" "0"
    , style "pointer-events" "none"
    , style "border-style" "solid"
    , style "border-width" (String.join " " [ "0", size, size, "0" ])
    , style "border-color"
        (String.join " "
            [ "transparent"
            , Colors.white
            , "transparent"
            , "transparent"
            ]
        )
    ]


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


stepHeader : StepState -> List (Html.Attribute msg)
stepHeader state =
    [ style "display" "flex"
    , style "justify-content" "space-between"
    , style "border-color" <|
        case state of
            StepStateFailed ->
                Colors.failure

            StepStateErrored ->
                Colors.error

            StepStatePending ->
                "transparent"

            StepStateRunning ->
                Colors.started

            StepStateInterrupted ->
                "transparent"

            StepStateCancelled ->
                "transparent"

            StepStateSucceeded ->
                "transparent"
    ]


stepHeaderLabel : Bool -> List (Html.Attribute msg)
stepHeaderLabel changed =
    [ style "color" <|
        if changed then
            Colors.started

        else
            Colors.pending
    , style "line-height" "28px"
    , style "padding-left" "8px"
    ]


keyValuePairHeaderLabel : List (Html.Attribute msg)
keyValuePairHeaderLabel =
    [ style "line-height" "28px"
    , style "padding-left" "6px"
    ]


stepStatusIcon : List (Html.Attribute msg)
stepStatusIcon =
    [ style "background-size" "14px 14px"
    , style "position" "relative"
    ]


changedStepTooltip : List (Html.Attribute msg)
changedStepTooltip =
    style "pointer-events" "none" :: Application.Styles.disableInteraction


durationTooltip : List (Html.Attribute msg)
durationTooltip =
    style "pointer-events" "none" :: Application.Styles.disableInteraction


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


imageSteps : List (Html.Attribute msg)
imageSteps =
    [ style "padding" "10px"
    , style "background" Colors.backgroundDark
    ]


initializationToggle : Bool -> List (Html.Attribute msg)
initializationToggle expanded =
    [ style "color" <|
        if expanded then
            Colors.text

        else
            Colors.pending
    , style "background" <|
        if expanded then
            Colors.backgroundDark

        else
            "transparent"
    ]
