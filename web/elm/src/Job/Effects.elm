module Job.Effects exposing (Effect(..), runEffect)

import Concourse
import Concourse.Build
import Concourse.BuildResources
import Concourse.Job
import Concourse.Pagination exposing (Page)
import Effects exposing (setTitle)
import Job.Msgs exposing (Msg(..))
import LoginRedirect
import Navigation
import Task
import Time exposing (Time)


type Effect
    = FetchJob Concourse.JobIdentifier
    | FetchJobBuilds Concourse.JobIdentifier (Maybe Page)
    | FetchBuildResources Concourse.BuildId
    | GetCurrentTime
    | DoTriggerBuild Concourse.JobIdentifier String
    | PauseJob Concourse.JobIdentifier String
    | UnpauseJob Concourse.JobIdentifier String
    | RedirectToLogin
    | NavigateTo String
    | SetTitle String


runEffect : Effect -> Cmd Msg
runEffect effect =
    case effect of
        FetchJob id ->
            fetchJob id

        FetchJobBuilds id page ->
            fetchJobBuilds id page

        FetchBuildResources id ->
            fetchBuildResources id

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

        SetTitle newTitle ->
            setTitle newTitle


fetchJobBuilds :
    Concourse.JobIdentifier
    -> Maybe Concourse.Pagination.Page
    -> Cmd Msg
fetchJobBuilds jobIdentifier page =
    Task.attempt JobBuildsFetched <|
        Concourse.Build.fetchJobBuilds jobIdentifier page


fetchJob : Concourse.JobIdentifier -> Cmd Msg
fetchJob jobIdentifier =
    Task.attempt JobFetched <|
        Concourse.Job.fetchJob jobIdentifier


fetchBuildResources : Concourse.BuildId -> Cmd Msg
fetchBuildResources buildIdentifier =
    Task.attempt (BuildResourcesFetched buildIdentifier) <|
        Concourse.BuildResources.fetch buildIdentifier


getCurrentTime : Cmd Msg
getCurrentTime =
    Task.perform ClockTick Time.now


triggerBuild : Concourse.JobIdentifier -> Concourse.CSRFToken -> Cmd Msg
triggerBuild job csrfToken =
    Task.attempt BuildTriggered <|
        Concourse.Job.triggerBuild job csrfToken


pauseJob : Concourse.JobIdentifier -> Concourse.CSRFToken -> Cmd Msg
pauseJob jobIdentifier csrfToken =
    Task.attempt PausedToggled <|
        Concourse.Job.pause jobIdentifier csrfToken


unpauseJob : Concourse.JobIdentifier -> Concourse.CSRFToken -> Cmd Msg
unpauseJob jobIdentifier csrfToken =
    Task.attempt PausedToggled <|
        Concourse.Job.unpause jobIdentifier csrfToken
