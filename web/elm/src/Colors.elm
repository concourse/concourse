module Colors exposing
    ( aborted
    , abortedFaded
    , abortedTextFaded
    , asciiArt
    , background
    , backgroundDark
    , black
    , border
    , bottomBarText
    , buildStatusColor
    , buildTabBorderColor
    , buildTabTextColor
    , buildTitleTextColor
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
    , errorTextFaded
    , failure
    , failureFaded
    , failureTextFaded
    , flySuccessButtonHover
    , flySuccessCard
    , flySuccessTokenCopied
    , frame
    , groupBackground
    , groupBorderHovered
    , groupBorderSelected
    , groupBorderUnselected
    , groupsBarBackground
    , infoBarBackground
    , inputOutline
    , instanceGroupBanner
    , metadataKeyBackground
    , metadataValueBackground
    , noPipelinesPlaceholderBackground
    , paginationHover
    , paused
    , pausedFaded
    , pausedTextFaded
    , pending
    , pendingFaded
    , pendingTextFaded
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
    , sideBarIconBackground
    , sideBarTextBright
    , sideBarTextDim
    , started
    , startedFaded
    , startedTextFaded
    , statusColor
    , success
    , successFaded
    , successTextFaded
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


sideBarIconBackground : String
sideBarIconBackground =
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


black : String
black =
    ColorValues.black



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


startedTextFaded : String
startedTextFaded =
    ColorValues.grey20


success : String
success =
    ColorValues.success40


successFaded : String
successFaded =
    ColorValues.success70


successTextFaded : String
successTextFaded =
    ColorValues.success20


paused : String
paused =
    ColorValues.paused40


pausedFaded : String
pausedFaded =
    ColorValues.paused70


pausedTextFaded : String
pausedTextFaded =
    ColorValues.paused20


pending : String
pending =
    ColorValues.grey40


pendingFaded : String
pendingFaded =
    ColorValues.grey60


pendingTextFaded : String
pendingTextFaded =
    ColorValues.grey20


unknown : String
unknown =
    ColorValues.grey40


failure : String
failure =
    ColorValues.failure50


failureFaded : String
failureFaded =
    ColorValues.failure70


failureTextFaded : String
failureTextFaded =
    ColorValues.failure10


error : String
error =
    ColorValues.error40


errorFaded : String
errorFaded =
    ColorValues.error50


errorTextFaded : String
errorTextFaded =
    ColorValues.error20


aborted : String
aborted =
    ColorValues.error70


abortedFaded : String
abortedFaded =
    ColorValues.error80


abortedTextFaded : String
abortedTextFaded =
    ColorValues.error50



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


metadataKeyBackground : String
metadataKeyBackground =
    ColorValues.grey70


metadataValueBackground : String
metadataValueBackground =
    ColorValues.grey90



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


buildTabBorderColor : Bool -> BuildStatus -> String
buildTabBorderColor isBright status =
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
        ColorValues.grey100


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


buildTitleTextColor : String
buildTitleTextColor =
    ColorValues.grey100


buildTabTextColor : Bool -> BuildStatus -> String
buildTabTextColor isCurrent status =
    if isCurrent then
        case status of
            BuildStatusStarted ->
                ColorValues.grey100

            BuildStatusPending ->
                ColorValues.grey100

            _ ->
                white

    else
        case status of
            BuildStatusStarted ->
                startedTextFaded

            BuildStatusPending ->
                pendingTextFaded

            BuildStatusSucceeded ->
                successTextFaded

            BuildStatusFailed ->
                failureTextFaded

            BuildStatusErrored ->
                errorTextFaded

            BuildStatusAborted ->
                abortedTextFaded



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
