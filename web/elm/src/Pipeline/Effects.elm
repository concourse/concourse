port module Pipeline.Effects exposing
    ( Effect(..)
    , resetPipelineFocus
    , runEffect
    )

import Concourse
import Concourse.Info
import Concourse.Job
import Concourse.Pipeline
import Concourse.Resource
import Effects
import Json.Encode
import LoginRedirect
import Navigation
import Pipeline.Msgs exposing (Msg(..))
import Task


port resetPipelineFocus : () -> Cmd msg


port renderPipeline : ( Json.Encode.Value, Json.Encode.Value ) -> Cmd msg


type Effect
    = FetchPipeline Concourse.PipelineIdentifier
    | FetchJobs Concourse.PipelineIdentifier
    | FetchResources Concourse.PipelineIdentifier
    | FetchVersion
    | ResetPipelineFocus
    | RedirectToLogin
    | RenderPipeline Json.Encode.Value Json.Encode.Value
    | SetTitle String
    | NewUrl String


runEffect : Effect -> Cmd Msg
runEffect eff =
    case eff of
        FetchPipeline id ->
            fetchPipeline id

        FetchJobs id ->
            fetchJobs id

        FetchResources id ->
            fetchResources id

        FetchVersion ->
            fetchVersion

        ResetPipelineFocus ->
            resetPipelineFocus ()

        RedirectToLogin ->
            LoginRedirect.requestLoginRedirect ""

        RenderPipeline jobs resources ->
            renderPipeline ( jobs, resources )

        SetTitle newTitle ->
            Effects.setTitle newTitle

        NewUrl newUrl ->
            Navigation.newUrl newUrl


fetchResources : Concourse.PipelineIdentifier -> Cmd Msg
fetchResources pid =
    Task.attempt ResourcesFetched <| Concourse.Resource.fetchResourcesRaw pid


fetchJobs : Concourse.PipelineIdentifier -> Cmd Msg
fetchJobs pid =
    Task.attempt JobsFetched <| Concourse.Job.fetchJobsRaw pid


fetchVersion : Cmd Msg
fetchVersion =
    Concourse.Info.fetch
        |> Task.map .version
        |> Task.attempt VersionFetched


fetchPipeline : Concourse.PipelineIdentifier -> Cmd Msg
fetchPipeline pipelineIdentifier =
    Task.attempt PipelineFetched <|
        Concourse.Pipeline.fetchPipeline pipelineIdentifier
