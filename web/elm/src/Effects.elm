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
import Concourse.Build
import Concourse.BuildPlan
import Concourse.BuildPrep
import Concourse.BuildResources
import Concourse.FlyToken
import Concourse.Info
import Concourse.Job
import Concourse.Pagination exposing (Page, Paginated)
import Concourse.Pipeline
import Concourse.Resource
import Concourse.User
import Dashboard.APIData
import Dashboard.Group
import Dashboard.Models
import Dom
import Favicon
import Json.Encode
import Navigation
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
            Concourse.Job.fetchJob id
                |> Task.attempt JobFetched

        FetchJobs id ->
            Concourse.Job.fetchJobsRaw id
                |> Task.attempt JobsFetched

        FetchJobBuilds id page ->
            Concourse.Build.fetchJobBuilds id page
                |> Task.attempt JobBuildsFetched

        FetchResource id ->
            Concourse.Resource.fetchResource id
                |> Task.attempt ResourceFetched

        FetchVersionedResources id paging ->
            Concourse.Resource.fetchVersionedResources id paging
                |> Task.map ((,) paging)
                |> Task.attempt VersionedResourcesFetched

        FetchResources id ->
            Concourse.Resource.fetchResourcesRaw id
                |> Task.attempt ResourcesFetched

        FetchBuildResources id ->
            Concourse.BuildResources.fetch id
                |> Task.map ((,) id)
                |> Task.attempt BuildResourcesFetched

        FetchPipeline id ->
            Concourse.Pipeline.fetchPipeline id
                |> Task.attempt PipelineFetched

        FetchVersion ->
            Concourse.Info.fetch
                |> Task.map .version
                |> Task.attempt VersionFetched

        FetchInputTo id ->
            Concourse.Resource.fetchInputTo id
                |> Task.map ((,) id)
                |> Task.attempt InputToFetched

        FetchOutputOf id ->
            Concourse.Resource.fetchOutputOf id
                |> Task.map ((,) id)
                |> Task.attempt OutputOfFetched

        FetchData ->
            Dashboard.APIData.remoteData
                |> Task.map2 (,) Time.now
                |> Task.attempt APIDataFetched

        GetCurrentTime ->
            Task.perform GotCurrentTime Time.now

        DoTriggerBuild id csrf ->
            Concourse.Job.triggerBuild id csrf
                |> Task.attempt BuildTriggered

        PauseJob id csrf ->
            Concourse.Job.pause id csrf
                |> Task.attempt PausedToggled

        UnpauseJob id csrf ->
            Concourse.Job.unpause id csrf
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
            Concourse.Resource.pinVersion version csrfToken
                |> Task.attempt VersionPinned

        DoUnpinVersion id csrfToken ->
            Concourse.Resource.unpinVersion id csrfToken
                |> Task.attempt VersionUnpinned

        DoToggleVersion action id csrfToken ->
            Concourse.Resource.enableDisableVersionedResource (action == Enable) id csrfToken
                |> Task.attempt (VersionToggled action id)

        DoCheck rid csrfToken ->
            Concourse.Resource.check rid csrfToken
                |> Task.attempt Checked

        SetPinComment rid csrfToken comment ->
            Concourse.Resource.setPinComment rid csrfToken comment
                |> Task.attempt CommentSet

        SendTokenToFly authToken flyPort ->
            Concourse.FlyToken.sendTokenToFly authToken flyPort
                |> Task.attempt TokenSentToFly

        SendTogglePipelineRequest { pipeline, csrfToken } ->
            Concourse.Pipeline.togglePause pipeline.status pipeline.teamName pipeline.name csrfToken
                |> Task.attempt (always EmptyCallback)

        ShowTooltip ( teamName, pipelineName ) ->
            tooltip ( teamName, pipelineName )

        ShowTooltipHd ( teamName, pipelineName ) ->
            tooltipHd ( teamName, pipelineName )

        SendOrderPipelinesRequest teamName pipelines csrfToken ->
            Concourse.Pipeline.order teamName (List.map .name pipelines) csrfToken
                |> Task.attempt (always EmptyCallback)

        SendLogOutRequest ->
            Task.attempt LoggedOut Concourse.User.logOut

        GetScreenSize ->
            Task.perform ScreenResized Window.size

        PinTeamNames stickyHeaderConfig ->
            pinTeamNames stickyHeaderConfig

        FetchBuild delay browsingIndex buildId ->
            Process.sleep delay
                |> Task.andThen (always <| Concourse.Build.fetch buildId)
                |> Task.map ((,) browsingIndex)
                |> Task.attempt BuildFetched

        FetchJobBuild browsingIndex jbi ->
            Concourse.Build.fetchJobBuild jbi
                |> Task.map ((,) browsingIndex)
                |> Task.attempt BuildFetched

        FetchBuildJobDetails buildJob ->
            Concourse.Job.fetchJob buildJob
                |> Task.attempt BuildJobDetailsFetched

        FetchBuildHistory job page ->
            Concourse.Build.fetchJobBuilds job page
                |> Task.attempt BuildHistoryFetched

        FetchBuildPrep delay browsingIndex buildId ->
            Process.sleep delay
                |> Task.andThen (always <| Concourse.BuildPrep.fetch buildId)
                |> Task.map ((,) browsingIndex)
                |> Task.attempt BuildPrepFetched

        FetchBuildPlanAndResources buildId ->
            Task.map2 (,) (Concourse.BuildPlan.fetch buildId) (Concourse.BuildResources.fetch buildId)
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchBuildPlan buildId ->
            Concourse.BuildPlan.fetch buildId
                |> Task.map (\p -> ( p, Concourse.BuildResources.empty ))
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchUser ->
            Concourse.User.fetchUser
                |> Task.attempt UserFetched

        SetFavIcon status ->
            Favicon.set status
                |> Task.perform (always EmptyCallback)

        DoAbortBuild buildId csrfToken ->
            Concourse.Build.abort buildId csrfToken
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
