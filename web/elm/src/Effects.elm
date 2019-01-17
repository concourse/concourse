port module Effects exposing
    ( Callback(..)
    , Effect(..)
    , renderPipeline
    , resetPipelineFocus
    , runEffect
    , setTitle
    )

import Concourse
import Concourse.Build
import Concourse.BuildResources
import Concourse.Info
import Concourse.Job
import Concourse.Pagination exposing (Page, Paginated)
import Concourse.Pipeline
import Concourse.Resource
import Http
import Json.Encode
import LoginRedirect
import Navigation
import Task
import Time exposing (Time)


port setTitle : String -> Cmd msg


port resetPipelineFocus : () -> Cmd msg


port renderPipeline : ( Json.Encode.Value, Json.Encode.Value ) -> Cmd msg


type Effect
    = FetchJob Concourse.JobIdentifier
    | FetchJobs Concourse.PipelineIdentifier
    | FetchJobBuilds Concourse.JobIdentifier (Maybe Page)
    | FetchResources Concourse.PipelineIdentifier
    | FetchBuildResources Concourse.BuildId
    | FetchPipeline Concourse.PipelineIdentifier
    | FetchVersion
    | GetCurrentTime
    | DoTriggerBuild Concourse.JobIdentifier String
    | PauseJob Concourse.JobIdentifier String
    | UnpauseJob Concourse.JobIdentifier String
    | ResetPipelineFocus
    | RenderPipeline Json.Encode.Value Json.Encode.Value
    | RedirectToLogin
    | NavigateTo String
    | SetTitle String
    | NewUrl String


type Callback
    = GotCurrentTime Time
    | BuildTriggered (Result Http.Error Concourse.Build)
    | JobBuildsFetched (Result Http.Error (Paginated Concourse.Build))
    | JobFetched (Result Http.Error Concourse.Job)
    | JobsFetched (Result Http.Error Json.Encode.Value)
    | PipelineFetched (Result Http.Error Concourse.Pipeline)
    | ResourcesFetched (Result Http.Error Json.Encode.Value)
    | BuildResourcesFetched Int (Result Http.Error Concourse.BuildResources)
    | VersionFetched (Result Http.Error String)
    | PausedToggled (Result Http.Error ())


runEffect : Effect -> Cmd Callback
runEffect effect =
    case effect of
        FetchJob id ->
            fetchJob id

        FetchJobs id ->
            fetchJobs id

        FetchJobBuilds id page ->
            fetchJobBuilds id page

        FetchResources id ->
            fetchResources id

        FetchBuildResources id ->
            fetchBuildResources id

        FetchPipeline id ->
            fetchPipeline id

        FetchVersion ->
            fetchVersion

        GetCurrentTime ->
            getCurrentTime

        DoTriggerBuild id csrf ->
            triggerBuild id csrf

        PauseJob id csrf ->
            pauseJob id csrf

        UnpauseJob id csrf ->
            unpauseJob id csrf

        RedirectToLogin ->
            LoginRedirect.requestLoginRedirect ""

        NavigateTo newUrl ->
            Navigation.newUrl newUrl

        ResetPipelineFocus ->
            resetPipelineFocus ()

        RenderPipeline jobs resources ->
            renderPipeline ( jobs, resources )

        NewUrl newUrl ->
            Navigation.newUrl newUrl

        SetTitle newTitle ->
            setTitle newTitle


fetchJobBuilds :
    Concourse.JobIdentifier
    -> Maybe Concourse.Pagination.Page
    -> Cmd Callback
fetchJobBuilds jobIdentifier page =
    Task.attempt JobBuildsFetched <|
        Concourse.Build.fetchJobBuilds jobIdentifier page


fetchJob : Concourse.JobIdentifier -> Cmd Callback
fetchJob jobIdentifier =
    Task.attempt JobFetched <|
        Concourse.Job.fetchJob jobIdentifier


fetchBuildResources : Concourse.BuildId -> Cmd Callback
fetchBuildResources buildIdentifier =
    Task.attempt (BuildResourcesFetched buildIdentifier) <|
        Concourse.BuildResources.fetch buildIdentifier


fetchResources : Concourse.PipelineIdentifier -> Cmd Callback
fetchResources pid =
    Task.attempt ResourcesFetched <| Concourse.Resource.fetchResourcesRaw pid


fetchJobs : Concourse.PipelineIdentifier -> Cmd Callback
fetchJobs pid =
    Task.attempt JobsFetched <| Concourse.Job.fetchJobsRaw pid


fetchVersion : Cmd Callback
fetchVersion =
    Concourse.Info.fetch
        |> Task.map .version
        |> Task.attempt VersionFetched


fetchPipeline : Concourse.PipelineIdentifier -> Cmd Callback
fetchPipeline pipelineIdentifier =
    Task.attempt PipelineFetched <|
        Concourse.Pipeline.fetchPipeline pipelineIdentifier


getCurrentTime : Cmd Callback
getCurrentTime =
    Task.perform GotCurrentTime Time.now


triggerBuild : Concourse.JobIdentifier -> Concourse.CSRFToken -> Cmd Callback
triggerBuild job csrfToken =
    Task.attempt BuildTriggered <|
        Concourse.Job.triggerBuild job csrfToken


pauseJob : Concourse.JobIdentifier -> Concourse.CSRFToken -> Cmd Callback
pauseJob jobIdentifier csrfToken =
    Task.attempt PausedToggled <|
        Concourse.Job.pause jobIdentifier csrfToken


unpauseJob : Concourse.JobIdentifier -> Concourse.CSRFToken -> Cmd Callback
unpauseJob jobIdentifier csrfToken =
    Task.attempt PausedToggled <|
        Concourse.Job.unpause jobIdentifier csrfToken
