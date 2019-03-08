port module Effects exposing
    ( Effect(..)
    , LayoutDispatch(..)
    , ScrollDirection(..)
    , renderPipeline
    , runEffect
    , setTitle
    )

import Callback exposing (Callback(..))
import Concourse
import Concourse.Pagination exposing (Page, Paginated)
import Dashboard.Group
import Dashboard.Models
import Dom
import Favicon
import Json.Encode
import Navigation
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
import Resource.Models exposing (VersionId, VersionToggleAction(..))
import Scroll
import Task
import Time exposing (Time)
import Window


port setTitle : String -> Cmd msg


port renderPipeline : ( Json.Encode.Value, Json.Encode.Value ) -> Cmd msg


port pinTeamNames : Dashboard.Group.StickyHeaderConfig -> Cmd msg


port tooltip : ( String, String ) -> Cmd msg


port tooltipHd : ( String, String ) -> Cmd msg


port resetPipelineFocus : () -> Cmd msg


port loadToken : () -> Cmd msg


port saveToken : String -> Cmd msg


port requestLoginRedirect : String -> Cmd msg


type LayoutDispatch
    = SubPage Int
    | Layout


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
    | FetchBuild Time Int Int
    | FetchJobBuild Int Concourse.JobBuildIdentifier
    | FetchBuildJobDetails Concourse.JobIdentifier
    | FetchBuildHistory Concourse.JobIdentifier (Maybe Page)
    | FetchBuildPrep Time Int Int
    | FetchBuildPlan Concourse.BuildId
    | FetchBuildPlanAndResources Concourse.BuildId
    | GetCurrentTime
    | DoTriggerBuild Concourse.JobIdentifier String
    | DoAbortBuild Int Concourse.CSRFToken
    | PauseJob Concourse.JobIdentifier String
    | UnpauseJob Concourse.JobIdentifier String
    | ResetPipelineFocus
    | RenderPipeline Json.Encode.Value Json.Encode.Value
    | RedirectToLogin
    | NavigateTo String
    | ModifyUrl String
    | SetTitle String
    | DoPinVersion Concourse.VersionedResourceIdentifier Concourse.CSRFToken
    | DoUnpinVersion Concourse.ResourceIdentifier Concourse.CSRFToken
    | DoToggleVersion VersionToggleAction VersionId Concourse.CSRFToken
    | DoCheck Concourse.ResourceIdentifier Concourse.CSRFToken
    | SetPinComment Concourse.ResourceIdentifier Concourse.CSRFToken String
    | SendTokenToFly String Int
    | SendTogglePipelineRequest { pipeline : Dashboard.Models.Pipeline, csrfToken : Concourse.CSRFToken }
    | ShowTooltip ( String, String )
    | ShowTooltipHd ( String, String )
    | SendOrderPipelinesRequest String (List Dashboard.Models.Pipeline) Concourse.CSRFToken
    | SendLogOutRequest
    | GetScreenSize
    | PinTeamNames Dashboard.Group.StickyHeaderConfig
    | Scroll ScrollDirection
    | SetFavIcon (Maybe Concourse.BuildStatus)
    | SaveToken String
    | LoadToken
    | ForceFocus String


type ScrollDirection
    = ToWindowTop
    | Down
    | Up
    | ToWindowBottom
    | Builds Float
    | ToCurrentBuild


runEffect : Effect -> Cmd Callback
runEffect effect =
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
                |> Task.map ((,) paging)
                |> Task.attempt VersionedResourcesFetched

        FetchResources id ->
            Network.Resource.fetchResourcesRaw id
                |> Task.attempt ResourcesFetched

        FetchBuildResources id ->
            Network.BuildResources.fetch id
                |> Task.map ((,) id)
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
                |> Task.map ((,) id)
                |> Task.attempt InputToFetched

        FetchOutputOf id ->
            Network.Resource.fetchOutputOf id
                |> Task.map ((,) id)
                |> Task.attempt OutputOfFetched

        FetchData ->
            Network.DashboardAPIData.remoteData
                |> Task.map2 (,) Time.now
                |> Task.attempt APIDataFetched

        GetCurrentTime ->
            Task.perform GotCurrentTime Time.now

        DoTriggerBuild id csrf ->
            Network.Job.triggerBuild id csrf
                |> Task.attempt BuildTriggered

        PauseJob id csrf ->
            Network.Job.pause id csrf
                |> Task.attempt PausedToggled

        UnpauseJob id csrf ->
            Network.Job.unpause id csrf
                |> Task.attempt PausedToggled

        RedirectToLogin ->
            requestLoginRedirect ""

        NavigateTo url ->
            Navigation.newUrl url

        ModifyUrl url ->
            Navigation.modifyUrl url

        ResetPipelineFocus ->
            resetPipelineFocus ()

        RenderPipeline jobs resources ->
            renderPipeline ( jobs, resources )

        SetTitle newTitle ->
            setTitle newTitle

        DoPinVersion version csrfToken ->
            Network.Resource.pinVersion version csrfToken
                |> Task.attempt VersionPinned

        DoUnpinVersion id csrfToken ->
            Network.Resource.unpinVersion id csrfToken
                |> Task.attempt VersionUnpinned

        DoToggleVersion action id csrfToken ->
            Network.Resource.enableDisableVersionedResource (action == Enable) id csrfToken
                |> Task.attempt (VersionToggled action id)

        DoCheck rid csrfToken ->
            Network.Resource.check rid csrfToken
                |> Task.attempt Checked

        SetPinComment rid csrfToken comment ->
            Network.Resource.setPinComment rid csrfToken comment
                |> Task.attempt CommentSet

        SendTokenToFly authToken flyPort ->
            Network.FlyToken.sendTokenToFly authToken flyPort
                |> Task.attempt TokenSentToFly

        SendTogglePipelineRequest { pipeline, csrfToken } ->
            Network.Pipeline.togglePause pipeline.status pipeline.teamName pipeline.name csrfToken
                |> Task.attempt (always EmptyCallback)

        ShowTooltip ( teamName, pipelineName ) ->
            tooltip ( teamName, pipelineName )

        ShowTooltipHd ( teamName, pipelineName ) ->
            tooltipHd ( teamName, pipelineName )

        SendOrderPipelinesRequest teamName pipelines csrfToken ->
            Network.Pipeline.order teamName (List.map .name pipelines) csrfToken
                |> Task.attempt (always EmptyCallback)

        SendLogOutRequest ->
            Task.attempt LoggedOut Network.User.logOut

        GetScreenSize ->
            Task.perform ScreenResized Window.size

        PinTeamNames stickyHeaderConfig ->
            pinTeamNames stickyHeaderConfig

        FetchBuild delay browsingIndex buildId ->
            Process.sleep delay
                |> Task.andThen (always <| Network.Build.fetch buildId)
                |> Task.map ((,) browsingIndex)
                |> Task.attempt BuildFetched

        FetchJobBuild browsingIndex jbi ->
            Network.Build.fetchJobBuild jbi
                |> Task.map ((,) browsingIndex)
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
                |> Task.map ((,) browsingIndex)
                |> Task.attempt BuildPrepFetched

        FetchBuildPlanAndResources buildId ->
            Task.map2 (,) (Network.BuildPlan.fetch buildId) (Network.BuildResources.fetch buildId)
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchBuildPlan buildId ->
            Network.BuildPlan.fetch buildId
                |> Task.map (\p -> ( p, Network.BuildResources.empty ))
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchUser ->
            Network.User.fetchUser
                |> Task.attempt UserFetched

        SetFavIcon status ->
            Favicon.set status
                |> Task.perform (always EmptyCallback)

        DoAbortBuild buildId csrfToken ->
            Network.Build.abort buildId csrfToken
                |> Task.attempt BuildAborted

        Scroll dir ->
            Task.perform (always EmptyCallback) (scrollInDirection dir)

        SaveToken tokenValue ->
            saveToken tokenValue

        LoadToken ->
            loadToken ()

        ForceFocus dom ->
            Dom.focus dom
                |> Task.attempt (always EmptyCallback)


scrollInDirection : ScrollDirection -> Task.Task x ()
scrollInDirection dir =
    case dir of
        ToWindowTop ->
            Scroll.toWindowTop

        Down ->
            Scroll.scrollDown

        Up ->
            Scroll.scrollUp

        ToWindowBottom ->
            Scroll.toWindowBottom

        Builds delta ->
            Scroll.scroll "builds" delta

        ToCurrentBuild ->
            Scroll.scrollIntoView "#builds .current"
