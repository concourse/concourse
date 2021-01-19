module Colors exposing
    ( aborted
    , abortedFaded
    , asciiArt
    , background
    , backgroundDark
    , border
    , bottomBarText
    , buildStatusColor
    , buildTooltipText
    , buttonDisabledGrey
    , card
    , cliIconHover
    , dashboardPipelineHeaderText
    , dashboardText
    , dropdownFaded
    , dropdownItemInputText
    , dropdownItemSelectedBackground
    , dropdownItemSelectedText
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
    , hamburgerClosedBackground
    , infoBarBackground
    , inputOutline
    , instanceGroupBanner
    , noPipelinesPlaceholderBackground
    , paginationHover
    , paused
    , pausedFaded
    , pending
    , pendingFaded
    , pinIconHover
    , pinMenuBackground
    , pinMenuHover
    , pinTools
    , pinned
    , resourceError
    , retryTabText
    , secondaryTopBar
    , sectionHeader
    , showArchivedButtonBorder
    , sideBar
    , sideBarActive
    , sideBarBackground
    , sideBarHovered
    , sideBarTextBright
    , sideBarTextDim
    , started
    , startedFaded
    , statusColor
    , success
    , successFaded
    , text
    , tooltipBackground
    , tooltipText
    , topBarBackground
    , unknown
    , welcomeCardText
    , white
    )

import ColorValues
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.PipelineStatus exposing (PipelineStatus(..))



----


frame : String
frame =
    "#1e1d1d"


topBarBackground : String
topBarBackground =
    ColorValues.grey100


infoBarBackground : String
infoBarBackground =
    ColorValues.grey100


hamburgerClosedBackground : String
hamburgerClosedBackground =
    ColorValues.grey100


border : String
border =
    ColorValues.black


dropdownItemSelectedBackground : String
dropdownItemSelectedBackground =
    ColorValues.grey90



----


sectionHeader : String
sectionHeader =
    "#1e1d1d"



----


dashboardText : String
dashboardText =
    ColorValues.white


dashboardPipelineHeaderText : String
dashboardPipelineHeaderText =
    ColorValues.grey20


dropdownItemInputText : String
dropdownItemInputText =
    ColorValues.grey30


dropdownItemSelectedText : String
dropdownItemSelectedText =
    ColorValues.grey30



----


bottomBarText : String
bottomBarText =
    ColorValues.grey40



----


pinned : String
pinned =
    "#5c3bd1"



----


tooltipBackground : String
tooltipBackground =
    ColorValues.grey20


tooltipText : String
tooltipText =
    ColorValues.grey80



----


pinIconHover : String
pinIconHover =
    "#1e1d1d"



----


pinTools : String
pinTools =
    "#2e2c2c"


pinMenuBackground : String
pinMenuBackground =
    ColorValues.grey90


pinMenuHover : String
pinMenuHover =
    ColorValues.grey100



----


white : String
white =
    ColorValues.white



----


background : String
background =
    "#3d3c3c"


noPipelinesPlaceholderBackground : String
noPipelinesPlaceholderBackground =
    ColorValues.grey80


showArchivedButtonBorder : String
showArchivedButtonBorder =
    ColorValues.grey90



----


backgroundDark : String
backgroundDark =
    ColorValues.grey80



----


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


pausedFaded : String
pausedFaded =
    "#2776ab"


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



-----


card : String
card =
    ColorValues.grey90



-----


instanceGroupBanner : String
instanceGroupBanner =
    "#4d4d4d"


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



-----


resourceError : String
resourceError =
    "#e67e22"



-----


cliIconHover : String
cliIconHover =
    ColorValues.white



-----


text : String
text =
    "#e6e7e8"



-----


asciiArt : String
asciiArt =
    ColorValues.grey50



-----


paginationHover : String
paginationHover =
    "#504b4b"



----


inputOutline : String
inputOutline =
    ColorValues.grey60



-----


groupsBarBackground : String
groupsBarBackground =
    "#2b2a2a"



----


buildTooltipText : String
buildTooltipText =
    "#ecf0f1"



----


dropdownFaded : String
dropdownFaded =
    ColorValues.grey80



----


dropdownUnselectedText : String
dropdownUnselectedText =
    ColorValues.grey40



----


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



----


sideBar : String
sideBar =
    "#333333"


sideBarBackground : String
sideBarBackground =
    ColorValues.grey90



----


sideBarActive : String
sideBarActive =
    ColorValues.grey100


sideBarHovered : String
sideBarHovered =
    ColorValues.grey80



-----


errorLog : String
errorLog =
    "#e74c3c"


retryTabText : String
retryTabText =
    "#f5f5f5"



-----


statusColor : Bool -> PipelineStatus -> String
statusColor isBright status =
    if isBright then
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

    else
        case status of
            PipelineStatusPaused ->
                pausedFaded

            PipelineStatusArchived ->
                backgroundDark

            PipelineStatusSucceeded _ ->
                successFaded

            PipelineStatusPending _ ->
                pendingFaded

            PipelineStatusFailed _ ->
                failureFaded

            PipelineStatusErrored _ ->
                errorFaded

            PipelineStatusAborted _ ->
                abortedFaded


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



-----


buttonDisabledGrey : String
buttonDisabledGrey =
    "#979797"



-----


sideBarTextDim : String
sideBarTextDim =
    ColorValues.grey30


sideBarTextBright : String
sideBarTextBright =
    ColorValues.grey20



-----


welcomeCardText : String
welcomeCardText =
    ColorValues.grey30
