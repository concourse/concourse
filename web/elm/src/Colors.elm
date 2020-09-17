module Colors exposing
    ( aborted
    , abortedFaded
    , asciiArt
    , background
    , backgroundDark
    , bottomBarText
    , buildStatusColor
    , buildTooltipText
    , buttonDisabledGrey
    , card
    , cliIconHover
    , dashboardText
    , dropdownFaded
    , dropdownUnselectedText
    , error
    , errorFaded
    , errorLog
    , failure
    , failureFaded
    , flySuccessButtonHover
    , flySuccessCard
    , flySuccessTokenCopied
    , frame
    , groupBackground
    , groupBorderHovered
    , groupBorderSelected
    , groupBorderUnselected
    , groupsBarBackground
    , inputOutline
    , paginationHover
    , paused
    , pausedTopbarSeparator
    , pending
    , pendingFaded
    , pinHighlight
    , pinIconHover
    , pinTools
    , pinned
    , resourceError
    , retryTabText
    , secondaryTopBar
    , sectionHeader
    , sideBar
    , sideBarActive
    , sideBarHovered
    , started
    , startedFaded
    , statusColor
    , success
    , successFaded
    , text
    , tooltipBackground
    , unknown
    , white
    )

import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.PipelineStatus exposing (PipelineStatus(..))


frame : String
frame =
    "#1e1d1d"


sectionHeader : String
sectionHeader =
    "#1e1d1d"


dashboardText : String
dashboardText =
    "#ffffff"


bottomBarText : String
bottomBarText =
    "#868585"


pinned : String
pinned =
    "#5c3bd1"


tooltipBackground : String
tooltipBackground =
    "#9b9b9b"


pinIconHover : String
pinIconHover =
    "#1e1d1d"


pinTools : String
pinTools =
    "#2e2c2c"


white : String
white =
    "#ffffff"


background : String
background =
    "#3d3c3c"


backgroundDark : String
backgroundDark =
    "#323232"


started : String
started =
    "#fad43b"


startedFaded : String
startedFaded =
    "#f1c40f"


success : String
success =
    "#11c560"


successFaded : String
successFaded =
    "#419867"


paused : String
paused =
    "#3498db"


pending : String
pending =
    "#9b9b9b"


pendingFaded : String
pendingFaded =
    "#7a7373"


unknown : String
unknown =
    "#9b9b9b"


failure : String
failure =
    "#ed4b35"


failureFaded : String
failureFaded =
    "#bd3826"


error : String
error =
    "#f5a623"


errorFaded : String
errorFaded =
    "#ec9910"


aborted : String
aborted =
    "#8b572a"


abortedFaded : String
abortedFaded =
    "#6a401c"


card : String
card =
    "#2a2929"


secondaryTopBar : String
secondaryTopBar =
    "#2a2929"


flySuccessCard : String
flySuccessCard =
    "#323030"


flySuccessButtonHover : String
flySuccessButtonHover =
    "#242424"


flySuccessTokenCopied : String
flySuccessTokenCopied =
    "#196ac8"


resourceError : String
resourceError =
    "#e67e22"


cliIconHover : String
cliIconHover =
    "#ffffff"


text : String
text =
    "#e6e7e8"


asciiArt : String
asciiArt =
    "#888888"


paginationHover : String
paginationHover =
    "#504b4b"


inputOutline : String
inputOutline =
    "#504b4b"


groupsBarBackground : String
groupsBarBackground =
    "#2b2a2a"


buildTooltipText : String
buildTooltipText =
    "#ecf0f1"


pausedTopbarSeparator : String
pausedTopbarSeparator =
    "rgba(255, 255, 255, 0.5)"


dropdownFaded : String
dropdownFaded =
    "#2e2e2e"


dropdownUnselectedText : String
dropdownUnselectedText =
    "#9b9b9b"


pinHighlight : String
pinHighlight =
    "rgba(255, 255, 255, 0.3)"


groupBorderSelected : String
groupBorderSelected =
    "#979797"


groupBorderUnselected : String
groupBorderUnselected =
    "#2b2a2a"


groupBorderHovered : String
groupBorderHovered =
    "#fff2"


groupBackground : String
groupBackground =
    "rgba(151, 151, 151, 0.1)"


sideBar : String
sideBar =
    "#333333"


sideBarActive : String
sideBarActive =
    "#272727"


sideBarHovered : String
sideBarHovered =
    "#444444"


errorLog : String
errorLog =
    "#e74c3c"


retryTabText : String
retryTabText =
    "#f5f5f5"


statusColor : PipelineStatus -> String
statusColor status =
    case status of
        PipelineStatusPaused ->
            paused

        PipelineStatusArchived ->
            background

        PipelineStatusSucceeded _ ->
            success

        PipelineStatusPending _ ->
            pending

        PipelineStatusFailed _ ->
            failure

        PipelineStatusErrored _ ->
            error

        PipelineStatusAborted _ ->
            aborted


buildStatusColor : Bool -> BuildStatus -> String
buildStatusColor isBright status =
    if isBright then
        case status of
            BuildStatusStarted ->
                started

            BuildStatusPending ->
                pending

            BuildStatusSucceeded ->
                success

            BuildStatusFailed ->
                failure

            BuildStatusErrored ->
                error

            BuildStatusAborted ->
                aborted

    else
        case status of
            BuildStatusStarted ->
                startedFaded

            BuildStatusPending ->
                pendingFaded

            BuildStatusSucceeded ->
                successFaded

            BuildStatusFailed ->
                failureFaded

            BuildStatusErrored ->
                errorFaded

            BuildStatusAborted ->
                abortedFaded


buttonDisabledGrey : String
buttonDisabledGrey =
    "#979797"
