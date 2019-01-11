port module Effects exposing
    ( Callback(..)
    , Effect(..)
    , runEffect
    , setTitle
    )

import Concourse
import Concourse.Build
import Concourse.BuildResources
import Concourse.Job
import Concourse.Pagination exposing (Page, Paginated)
import Http
import LoginRedirect
import Navigation
import Task
import Time exposing (Time)


port setTitle : String -> Cmd msg


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


type Callback
    = GotCurrentTime Time
    | BuildTriggered (Result Http.Error Concourse.Build)
    | JobBuildsFetched (Result Http.Error (Paginated Concourse.Build))
    | JobFetched (Result Http.Error Concourse.Job)
    | BuildResourcesFetched Int (Result Http.Error Concourse.BuildResources)
    | PausedToggled (Result Http.Error ())


runEffect : Effect -> Cmd Callback
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
