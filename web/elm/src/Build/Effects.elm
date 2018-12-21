module Build.Effects exposing (Effect(..), toCmd)

import Build.Msgs exposing (Msg(..))
import Concourse
import Concourse.Build
import Concourse.BuildPlan
import Concourse.BuildPrep
import Concourse.BuildResources
import Concourse.BuildStatus
import Concourse.Job
import Concourse.Pagination exposing (Page)
import Favicon
import LoginRedirect
import Navigation
import Process
import Scroll
import Task
import Time exposing (Time)


type Effect
    = FetchBuild Time Int Int
    | FetchJobBuild Int Concourse.JobBuildIdentifier
    | FetchBuildJobDetails Concourse.JobIdentifier
    | FetchBuildHistory Concourse.JobIdentifier (Maybe Page)
    | FetchBuildPrep Time Int Int
    | FetchBuildPlan Int
    | FetchBuildPlanAndResources Int
    | GetCurrentTime
    | SetFavIcon Concourse.BuildStatus
    | SetTitle (String -> Cmd Msg) String
    | NewUrl String
    | ModifyUrl String
    | DoTriggerBuild Concourse.JobIdentifier Concourse.CSRFToken
    | RedirectToLogin String
    | DoAbortBuild Int Concourse.CSRFToken
    | ScrollToCurrentBuildInHistory
    | DoScrollBuilds Float
    | ScrollToWindowTop
    | ScrollDown
    | ScrollUp
    | ScrollToWindowBottom


toCmd : Effect -> Cmd Msg
toCmd effect =
    case effect of
        FetchBuild delay browsingIndex buildId ->
            fetchBuild delay browsingIndex buildId

        FetchJobBuild browsingIndex jbi ->
            fetchJobBuild browsingIndex jbi

        GetCurrentTime ->
            getCurrentTime

        FetchBuildJobDetails buildJob ->
            fetchBuildJobDetails buildJob

        FetchBuildHistory job page ->
            fetchBuildHistory job page

        FetchBuildPrep delay browsingIndex buildId ->
            fetchBuildPrep delay browsingIndex buildId

        FetchBuildPlanAndResources buildId ->
            fetchBuildPlanAndResources buildId

        FetchBuildPlan buildId ->
            fetchBuildPlan buildId

        SetFavIcon status ->
            setFavicon status

        SetTitle titleFunc title ->
            titleFunc title

        NewUrl url ->
            Navigation.newUrl url

        ModifyUrl url ->
            Navigation.modifyUrl url

        DoTriggerBuild job csrfToken ->
            triggerBuild job csrfToken

        RedirectToLogin url ->
            LoginRedirect.requestLoginRedirect url

        DoAbortBuild buildId csrfToken ->
            abortBuild buildId csrfToken

        ScrollToCurrentBuildInHistory ->
            scrollToCurrentBuildInHistory

        DoScrollBuilds delta ->
            scrollBuilds delta

        ScrollToWindowTop ->
            Task.perform (always Noop) Scroll.toWindowTop

        ScrollDown ->
            Task.perform (always Noop) Scroll.scrollDown

        ScrollUp ->
            Task.perform (always Noop) Scroll.scrollUp

        ScrollToWindowBottom ->
            Task.perform (always Noop) Scroll.toWindowBottom


fetchBuildJobDetails : Concourse.JobIdentifier -> Cmd Msg
fetchBuildJobDetails buildJob =
    Task.attempt BuildJobDetailsFetched <|
        Concourse.Job.fetchJob buildJob


getCurrentTime : Cmd Msg
getCurrentTime =
    Task.perform ClockTick Time.now


fetchBuild : Time -> Int -> Int -> Cmd Msg
fetchBuild delay browsingIndex buildId =
    Task.attempt (BuildFetched browsingIndex)
        (Process.sleep delay
            |> Task.andThen (always <| Concourse.Build.fetch buildId)
        )


fetchJobBuild : Int -> Concourse.JobBuildIdentifier -> Cmd Msg
fetchJobBuild browsingIndex jbi =
    Task.attempt (BuildFetched browsingIndex) <|
        Concourse.Build.fetchJobBuild jbi


fetchBuildHistory :
    Concourse.JobIdentifier
    -> Maybe Concourse.Pagination.Page
    -> Cmd Msg
fetchBuildHistory job page =
    Task.attempt BuildHistoryFetched <|
        Concourse.Build.fetchJobBuilds job page


fetchBuildPrep : Time -> Int -> Int -> Cmd Msg
fetchBuildPrep delay browsingIndex buildId =
    Task.attempt (BuildPrepFetched browsingIndex)
        (Process.sleep delay
            |> Task.andThen (always <| Concourse.BuildPrep.fetch buildId)
        )


fetchBuildPlanAndResources : Int -> Cmd Msg
fetchBuildPlanAndResources buildId =
    Task.attempt PlanAndResourcesFetched <|
        Task.map2 (,) (Concourse.BuildPlan.fetch buildId) (Concourse.BuildResources.fetch buildId)


fetchBuildPlan : Int -> Cmd Msg
fetchBuildPlan buildId =
    Task.attempt PlanAndResourcesFetched <|
        Task.map (flip (,) Concourse.BuildResources.empty) (Concourse.BuildPlan.fetch buildId)


setFavicon : Concourse.BuildStatus -> Cmd Msg
setFavicon status =
    Task.perform (always Noop) <|
        Favicon.set ("/public/images/favicon-" ++ Concourse.BuildStatus.show status ++ ".png")


triggerBuild : Concourse.JobIdentifier -> Concourse.CSRFToken -> Cmd Msg
triggerBuild buildJob csrfToken =
    Task.attempt BuildTriggered <|
        Concourse.Job.triggerBuild buildJob csrfToken


abortBuild : Int -> Concourse.CSRFToken -> Cmd Msg
abortBuild buildId csrfToken =
    Task.attempt BuildAborted <|
        Concourse.Build.abort buildId csrfToken


scrollToCurrentBuildInHistory : Cmd Msg
scrollToCurrentBuildInHistory =
    Task.perform (always Noop) <|
        Scroll.scrollIntoView "#builds .current"


scrollBuilds : Float -> Cmd Msg
scrollBuilds delta =
    Task.perform (always Noop) <|
        Scroll.scroll "builds" delta
