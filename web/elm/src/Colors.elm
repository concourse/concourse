module Colors exposing
    ( aborted
    , asciiArt
    , background
    , bottomBarText
    , buildStatusColor
    , card
    , cliIconHover
    , comment
    , dashboardText
    , error
    , failure
    , failureFaded
    , flySuccessButtonHover
    , flySuccessCard
    , flySuccessTokenCopied
    , frame
    , groupsBarBackground
    , paginationHover
    , paused
    , pending
    , pinIconHover
    , pinned
    , resourceError
    , secondaryTopBar
    , sectionHeader
    , started
    , startedFaded
    , statusColor
    , success
    , successFaded
    , text
    , tooltipBackground
    )

import Concourse exposing (BuildStatus(..))
import Concourse.PipelineStatus exposing (PipelineStatus(..))


frame : String
frame =
    "#1e1d1d"


sectionHeader : String
sectionHeader =
    "#1e1d1d"


dashboardText : String
dashboardText =
    "#fff"


bottomBarText : String
bottomBarText =
    "#868585"


pinned : String
pinned =
    "#5C3BD1"


tooltipBackground : String
tooltipBackground =
    "#9b9b9b"


pinIconHover : String
pinIconHover =
    "#1e1d1d"


background : String
background =
    "#3d3c3c"


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
    "#4a90e2"


pending : String
pending =
    "#9b9b9b"


pendingFaded : String
pendingFaded =
    "#7a7373"


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
    "#196AC8"


resourceError : String
resourceError =
    "#e67e22"


cliIconHover : String
cliIconHover =
    "#fff"


text : String
text =
    "#e6e7e8"


asciiArt : String
asciiArt =
    "#888888"


paginationHover : String
paginationHover =
    "#504b4b"


comment : String
comment =
    "#196AC8"


groupsBarBackground : String
groupsBarBackground =
    "#2b2a2a"


statusColor : PipelineStatus -> String
statusColor status =
    case status of
        PipelineStatusPaused ->
            paused

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
