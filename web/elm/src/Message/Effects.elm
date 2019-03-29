port module Message.Effects exposing
    ( Effect(..)
    , ScrollDirection(..)
    , renderPipeline
    , runEffect
    , setTitle
    , stickyHeaderConfig
    )

import Browser.Dom exposing (getViewport)
import Browser.Navigation as Navigation
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination exposing (Page, Paginated)
import Dashboard.Group.Models
import Json.Encode
import Message.Callback exposing (Callback(..))
import Message.Message exposing (VersionToggleAction(..))
import Network.Build
import Network.BuildPlan
import Network.BuildPrep
import Network.BuildResources
import Network.DashboardAPIData
import Network.FlyToken
import Network.Info
import Network.Job
import Network.Pipeline
import Network.Resource
import Network.User
import Process
import Routes
import Task
import Time
import Views.Styles


port setTitle : String -> Cmd msg


port renderPipeline : ( Json.Encode.Value, Json.Encode.Value ) -> Cmd msg


port pinTeamNames : StickyHeaderConfig -> Cmd msg


port tooltip : ( String, String ) -> Cmd msg


port tooltipHd : ( String, String ) -> Cmd msg


port resetPipelineFocus : () -> Cmd msg


port loadToken : () -> Cmd msg


port saveToken : String -> Cmd msg


port requestLoginRedirect : String -> Cmd msg


port openEventStream : { url : String, eventTypes : List String } -> Cmd msg


port closeEventStream : () -> Cmd msg


port scrollIntoView : String -> Cmd msg


port scrollElement : ( String, Float ) -> Cmd msg


port scrollToBottom : () -> Cmd msg


port scrollToTop : () -> Cmd msg


port scrollUp : () -> Cmd msg


port scrollDown : () -> Cmd msg


port checkIsVisible : String -> Cmd msg


port setFavicon : String -> Cmd msg


type alias StickyHeaderConfig =
    { pageHeaderHeight : Float
    , pageBodyClass : String
    , sectionHeaderClass : String
    , sectionClass : String
    , sectionBodyClass : String
    }


stickyHeaderConfig : StickyHeaderConfig
stickyHeaderConfig =
    { pageHeaderHeight = Views.Styles.pageHeaderHeight
    , pageBodyClass = "dashboard"
    , sectionClass = "dashboard-team-group"
    , sectionHeaderClass = "dashboard-team-header"
    , sectionBodyClass = "dashboard-team-pipelines"
    }


type Effect
    = FetchJob Concourse.JobIdentifier
    | FetchJobs Concourse.PipelineIdentifier
    | FetchJobBuilds Concourse.JobIdentifier (Maybe Page)
    | FetchResource Concourse.ResourceIdentifier
    | FetchVersionedResources Concourse.ResourceIdentifier (Maybe Page)
    | FetchResources Concourse.PipelineIdentifier
    | FetchBuildResources Concourse.BuildId
    | FetchPipeline Concourse.PipelineIdentifier
    | FetchVersion
    | FetchInputTo Concourse.VersionedResourceIdentifier
    | FetchOutputOf Concourse.VersionedResourceIdentifier
    | FetchData
    | FetchUser
    | FetchBuild Float Int Int
    | FetchJobBuild Int Concourse.JobBuildIdentifier
    | FetchBuildJobDetails Concourse.JobIdentifier
    | FetchBuildHistory Concourse.JobIdentifier (Maybe Page)
    | FetchBuildPrep Float Int Int
    | FetchBuildPlan Concourse.BuildId
    | FetchBuildPlanAndResources Concourse.BuildId
    | GetCurrentTime
    | DoTriggerBuild Concourse.JobIdentifier
    | DoAbortBuild Int
    | PauseJob Concourse.JobIdentifier
    | UnpauseJob Concourse.JobIdentifier
    | ResetPipelineFocus
    | RenderPipeline Json.Encode.Value Json.Encode.Value
    | RedirectToLogin
    | LoadExternal String
    | NavigateTo String
    | ModifyUrl String
    | SetTitle String
    | DoPinVersion Concourse.VersionedResourceIdentifier
    | DoUnpinVersion Concourse.ResourceIdentifier
    | DoToggleVersion VersionToggleAction VersionId
    | DoCheck Concourse.ResourceIdentifier
    | SetPinComment Concourse.ResourceIdentifier String
    | SendTokenToFly String Int
    | SendTogglePipelineRequest Concourse.PipelineIdentifier Bool
    | ShowTooltip ( String, String )
    | ShowTooltipHd ( String, String )
    | SendOrderPipelinesRequest String (List Dashboard.Group.Models.Pipeline)
    | SendLogOutRequest
    | GetScreenSize
    | PinTeamNames StickyHeaderConfig
    | Scroll ScrollDirection
    | SetFavIcon (Maybe Concourse.BuildStatus)
    | SaveToken String
    | LoadToken
    | OpenBuildEventStream { url : String, eventTypes : List String }
    | CloseBuildEventStream
    | CheckIsVisible String
    | Focus String
    | Blur String


type alias VersionId =
    Concourse.VersionedResourceIdentifier


type ScrollDirection
    = ToTop
    | Down
    | Up
    | ToBottom
    | Element String Float
    | ToId String


runEffect : Effect -> Navigation.Key -> Concourse.CSRFToken -> Cmd Callback
runEffect effect key csrfToken =
    case effect of
        FetchJob id ->
            Network.Job.fetchJob id
                |> Task.attempt JobFetched

        FetchJobs id ->
            Network.Job.fetchJobsRaw id
                |> Task.attempt JobsFetched

        FetchJobBuilds id page ->
            Network.Build.fetchJobBuilds id page
                |> Task.attempt JobBuildsFetched

        FetchResource id ->
            Network.Resource.fetchResource id
                |> Task.attempt ResourceFetched

        FetchVersionedResources id paging ->
            Network.Resource.fetchVersionedResources id paging
                |> Task.map (\b -> ( paging, b ))
                |> Task.attempt VersionedResourcesFetched

        FetchResources id ->
            Network.Resource.fetchResourcesRaw id
                |> Task.attempt ResourcesFetched

        FetchBuildResources id ->
            Network.BuildResources.fetch id
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt BuildResourcesFetched

        FetchPipeline id ->
            Network.Pipeline.fetchPipeline id
                |> Task.attempt PipelineFetched

        FetchVersion ->
            Network.Info.fetch
                |> Task.map .version
                |> Task.attempt VersionFetched

        FetchInputTo id ->
            Network.Resource.fetchInputTo id
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt InputToFetched

        FetchOutputOf id ->
            Network.Resource.fetchOutputOf id
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt OutputOfFetched

        FetchData ->
            Network.DashboardAPIData.remoteData
                |> Task.map2 (\a b -> ( a, b )) Time.now
                |> Task.attempt APIDataFetched

        GetCurrentTime ->
            Task.perform GotCurrentTime Time.now

        DoTriggerBuild id ->
            Network.Job.triggerBuild id csrfToken
                |> Task.attempt BuildTriggered

        PauseJob id ->
            Network.Job.pause id csrfToken
                |> Task.attempt PausedToggled

        UnpauseJob id ->
            Network.Job.unpause id csrfToken
                |> Task.attempt PausedToggled

        RedirectToLogin ->
            requestLoginRedirect ""

        LoadExternal url ->
            Navigation.load url

        NavigateTo url ->
            Navigation.pushUrl key url

        ModifyUrl url ->
            Navigation.replaceUrl key url

        ResetPipelineFocus ->
            resetPipelineFocus ()

        RenderPipeline jobs resources ->
            renderPipeline ( jobs, resources )

        SetTitle newTitle ->
            setTitle newTitle

        DoPinVersion version ->
            Network.Resource.pinVersion version csrfToken
                |> Task.attempt VersionPinned

        DoUnpinVersion id ->
            Network.Resource.unpinVersion id csrfToken
                |> Task.attempt VersionUnpinned

        DoToggleVersion action id ->
            Network.Resource.enableDisableVersionedResource (action == Enable) id csrfToken
                |> Task.attempt (VersionToggled action id)

        DoCheck rid ->
            Network.Resource.check rid csrfToken
                |> Task.attempt Checked

        SetPinComment rid comment ->
            Network.Resource.setPinComment rid csrfToken comment
                |> Task.attempt CommentSet

        SendTokenToFly authToken flyPort ->
            Network.FlyToken.sendTokenToFly authToken flyPort
                |> Task.attempt TokenSentToFly

        SendTogglePipelineRequest pipelineIdentifier isPaused ->
            Network.Pipeline.togglePause
                isPaused
                pipelineIdentifier.teamName
                pipelineIdentifier.pipelineName
                csrfToken
                |> Task.attempt (PipelineToggled pipelineIdentifier)

        ShowTooltip ( teamName, pipelineName ) ->
            tooltip ( teamName, pipelineName )

        ShowTooltipHd ( teamName, pipelineName ) ->
            tooltipHd ( teamName, pipelineName )

        SendOrderPipelinesRequest teamName pipelines ->
            Network.Pipeline.order teamName (List.map .name pipelines) csrfToken
                |> Task.attempt (always EmptyCallback)

        SendLogOutRequest ->
            Task.attempt LoggedOut Network.User.logOut

        GetScreenSize ->
            Task.perform ScreenResized getViewport

        PinTeamNames shc ->
            pinTeamNames shc

        FetchBuild delay browsingIndex buildId ->
            Process.sleep delay
                |> Task.andThen (always <| Network.Build.fetch buildId)
                |> Task.map (\b -> ( browsingIndex, b ))
                |> Task.attempt BuildFetched

        FetchJobBuild browsingIndex jbi ->
            Network.Build.fetchJobBuild jbi
                |> Task.map (\b -> ( browsingIndex, b ))
                |> Task.attempt BuildFetched

        FetchBuildJobDetails buildJob ->
            Network.Job.fetchJob buildJob
                |> Task.attempt BuildJobDetailsFetched

        FetchBuildHistory job page ->
            Network.Build.fetchJobBuilds job page
                |> Task.attempt BuildHistoryFetched

        FetchBuildPrep delay browsingIndex buildId ->
            Process.sleep delay
                |> Task.andThen (always <| Network.BuildPrep.fetch buildId)
                |> Task.map (\b -> ( browsingIndex, b ))
                |> Task.attempt BuildPrepFetched

        FetchBuildPlanAndResources buildId ->
            Task.map2 (\a b -> ( a, b )) (Network.BuildPlan.fetch buildId) (Network.BuildResources.fetch buildId)
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchBuildPlan buildId ->
            Network.BuildPlan.fetch buildId
                |> Task.map (\p -> ( p, Network.BuildResources.empty ))
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchUser ->
            Network.User.fetchUser
                |> Task.attempt UserFetched

        SetFavIcon status ->
            setFavicon (faviconName status)

        DoAbortBuild buildId ->
            Network.Build.abort buildId csrfToken
                |> Task.attempt BuildAborted

        Scroll dir ->
            scrollInDirection dir

        SaveToken tokenValue ->
            saveToken tokenValue

        LoadToken ->
            loadToken ()

        Focus id ->
            Browser.Dom.focus id
                |> Task.attempt (always EmptyCallback)

        Blur id ->
            Browser.Dom.blur id
                |> Task.attempt (always EmptyCallback)

        OpenBuildEventStream config ->
            openEventStream config

        CloseBuildEventStream ->
            closeEventStream ()

        CheckIsVisible id ->
            checkIsVisible id


scrollInDirection : ScrollDirection -> Cmd Callback
scrollInDirection dir =
    case dir of
        ToTop ->
            scrollToTop ()

        Down ->
            scrollDown ()

        Up ->
            scrollUp ()

        ToBottom ->
            scrollToBottom ()

        Element id delta ->
            scrollElement ( id, delta )

        ToId id ->
            scrollIntoView id


faviconName : Maybe Concourse.BuildStatus -> String
faviconName status =
    case status of
        Just bs ->
            "/public/images/favicon-" ++ Concourse.BuildStatus.show bs ++ ".png"

        Nothing ->
            "/public/images/favicon.png"
