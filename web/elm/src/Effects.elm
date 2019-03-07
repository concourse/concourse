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
import Concourse.BuildStatus
import Concourse.Info
import Concourse.Job
import Concourse.Pagination exposing (Page, Paginated)
import Concourse.Pipeline
import Concourse.PipelineStatus
import Concourse.Resource
import Concourse.User
import Dashboard.APIData
import Dashboard.Group
import Dashboard.Models
import Dom
import Favicon
import Http
import Json.Encode
import Navigation
import Process
import Resource.Models exposing (VersionId, VersionToggleAction(..))
import Routes
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


port openEventStream : { url : String, eventTypes : List String } -> Cmd msg


port closeEventStream : () -> Cmd msg


port scrollIntoView : String -> Cmd msg


port scrollElement : ( String, Float ) -> Cmd msg


port scrollToBottom : () -> Cmd msg


port scrollToTop : () -> Cmd msg


port scrollUp : () -> Cmd msg


port scrollDown : () -> Cmd msg


port checkIsVisible : String -> Cmd msg


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
    | DoTriggerBuild Concourse.JobIdentifier
    | DoAbortBuild Int
    | PauseJob Concourse.JobIdentifier
    | UnpauseJob Concourse.JobIdentifier
    | ResetPipelineFocus
    | RenderPipeline Json.Encode.Value Json.Encode.Value
    | RedirectToLogin
    | NavigateTo String
    | ModifyUrl String
    | SetTitle String
    | DoPinVersion Concourse.VersionedResourceIdentifier
    | DoUnpinVersion Concourse.ResourceIdentifier
    | DoToggleVersion VersionToggleAction VersionId
    | DoCheck Concourse.ResourceIdentifier
    | SetPinComment Concourse.ResourceIdentifier String
    | SendTokenToFly String Int
    | SendTogglePipelineRequest Dashboard.Models.Pipeline
    | ShowTooltip ( String, String )
    | ShowTooltipHd ( String, String )
    | SendOrderPipelinesRequest String (List Dashboard.Models.Pipeline)
    | SendLogOutRequest
    | GetScreenSize
    | PinTeamNames Dashboard.Group.StickyHeaderConfig
    | Scroll ScrollDirection
    | SetFavIcon (Maybe Concourse.BuildStatus)
    | SaveToken String
    | LoadToken
    | ForceFocus String
    | OpenBuildEventStream { url : String, eventTypes : List String }
    | CloseBuildEventStream
    | CheckIsVisible String


type ScrollDirection
    = ToTop
    | Down
    | Up
    | ToBottom
    | Element String Float
    | ToBuild Int


runEffect : Effect -> Concourse.CSRFToken -> Cmd Callback
runEffect effect csrfToken =
    case effect of
        FetchJob id ->
            fetchJob id

        FetchJobs id ->
            fetchJobs id

        FetchJobBuilds id page ->
            fetchJobBuilds id page

        FetchResource id ->
            fetchResource id

        FetchVersionedResources id paging ->
            fetchVersionedResources id paging

        FetchResources id ->
            fetchResources id

        FetchBuildResources id ->
            fetchBuildResources id

        FetchPipeline id ->
            fetchPipeline id

        FetchVersion ->
            fetchVersion

        FetchInputTo id ->
            fetchInputTo id

        FetchOutputOf id ->
            fetchOutputOf id

        FetchData ->
            fetchData

        GetCurrentTime ->
            Task.perform GotCurrentTime Time.now

        DoTriggerBuild id ->
            triggerBuild id csrfToken

        PauseJob id ->
            pauseJob id csrfToken

        UnpauseJob id ->
            unpauseJob id csrfToken

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

        DoPinVersion version ->
            Task.attempt VersionPinned <|
                Concourse.Resource.pinVersion version csrfToken

        DoUnpinVersion id ->
            Task.attempt VersionUnpinned <|
                Concourse.Resource.unpinVersion id csrfToken

        DoToggleVersion action id ->
            Concourse.Resource.enableDisableVersionedResource
                (action == Enable)
                id
                csrfToken
                |> Task.attempt (VersionToggled action id)

        DoCheck rid ->
            Task.attempt Checked <|
                Concourse.Resource.check rid csrfToken

        SetPinComment rid comment ->
            Task.attempt CommentSet <|
                Concourse.Resource.setPinComment rid csrfToken comment

        SendTokenToFly authToken flyPort ->
            sendTokenToFly authToken flyPort

        SendTogglePipelineRequest pipeline ->
            togglePipelinePaused { pipeline = pipeline, csrfToken = csrfToken }

        ShowTooltip ( teamName, pipelineName ) ->
            tooltip ( teamName, pipelineName )

        ShowTooltipHd ( teamName, pipelineName ) ->
            tooltipHd ( teamName, pipelineName )

        SendOrderPipelinesRequest teamName pipelines ->
            orderPipelines teamName pipelines csrfToken

        SendLogOutRequest ->
            Task.attempt LoggedOut Concourse.User.logOut

        GetScreenSize ->
            Task.perform ScreenResized Window.size

        PinTeamNames stickyHeaderConfig ->
            pinTeamNames stickyHeaderConfig

        FetchBuild delay browsingIndex buildId ->
            fetchBuild delay browsingIndex buildId

        FetchJobBuild browsingIndex jbi ->
            fetchJobBuild browsingIndex jbi

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

        FetchUser ->
            fetchUser

        SetFavIcon status ->
            setFavicon status

        DoAbortBuild buildId ->
            abortBuild buildId csrfToken

        Scroll dir ->
            scrollInDirection dir

        SaveToken tokenValue ->
            saveToken tokenValue

        LoadToken ->
            loadToken ()

        ForceFocus dom ->
            Dom.focus dom
                |> Task.attempt (always EmptyCallback)

        OpenBuildEventStream config ->
            openEventStream config

        CloseBuildEventStream ->
            closeEventStream ()

        CheckIsVisible id ->
            checkIsVisible id


fetchJobBuilds : Concourse.JobIdentifier -> Maybe Concourse.Pagination.Page -> Cmd Callback
fetchJobBuilds jobIdentifier page =
    Task.attempt JobBuildsFetched <|
        Concourse.Build.fetchJobBuilds jobIdentifier page


fetchJob : Concourse.JobIdentifier -> Cmd Callback
fetchJob jobIdentifier =
    Task.attempt JobFetched <|
        Concourse.Job.fetchJob jobIdentifier


fetchResource : Concourse.ResourceIdentifier -> Cmd Callback
fetchResource resourceIdentifier =
    Task.attempt ResourceFetched <|
        Concourse.Resource.fetchResource resourceIdentifier


fetchVersionedResources : Concourse.ResourceIdentifier -> Maybe Page -> Cmd Callback
fetchVersionedResources resourceIdentifier page =
    Concourse.Resource.fetchVersionedResources resourceIdentifier page
        |> Task.map ((,) page)
        |> Task.attempt VersionedResourcesFetched


fetchBuildResources : Concourse.BuildId -> Cmd Callback
fetchBuildResources buildIdentifier =
    Concourse.BuildResources.fetch buildIdentifier
        |> Task.map ((,) buildIdentifier)
        |> Task.attempt BuildResourcesFetched


fetchResources : Concourse.PipelineIdentifier -> Cmd Callback
fetchResources pid =
    Task.attempt ResourcesFetched <| Concourse.Resource.fetchResourcesRaw pid


fetchJobs : Concourse.PipelineIdentifier -> Cmd Callback
fetchJobs pid =
    Task.attempt JobsFetched <| Concourse.Job.fetchJobsRaw pid


fetchUser : Cmd Callback
fetchUser =
    Task.attempt UserFetched Concourse.User.fetchUser


fetchVersion : Cmd Callback
fetchVersion =
    Concourse.Info.fetch
        |> Task.map .version
        |> Task.attempt VersionFetched


fetchPipeline : Concourse.PipelineIdentifier -> Cmd Callback
fetchPipeline pipelineIdentifier =
    Task.attempt PipelineFetched <|
        Concourse.Pipeline.fetchPipeline pipelineIdentifier


fetchInputTo : VersionId -> Cmd Callback
fetchInputTo versionId =
    Concourse.Resource.fetchInputTo versionId
        |> Task.map ((,) versionId)
        |> Task.attempt InputToFetched


fetchOutputOf : VersionId -> Cmd Callback
fetchOutputOf versionId =
    Concourse.Resource.fetchOutputOf versionId
        |> Task.map ((,) versionId)
        |> Task.attempt OutputOfFetched


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


sendTokenToFly : String -> Int -> Cmd Callback
sendTokenToFly authToken flyPort =
    Http.request
        { method = "GET"
        , headers = []
        , url = Routes.tokenToFlyRoute authToken flyPort
        , body = Http.emptyBody
        , expect = Http.expectStringResponse (\_ -> Ok ())
        , timeout = Nothing
        , withCredentials = False
        }
        |> Http.send (\r -> TokenSentToFly (r == Ok ()))


fetchData : Cmd Callback
fetchData =
    Dashboard.APIData.remoteData
        |> Task.map2 (,) Time.now
        |> Task.attempt APIDataFetched


togglePipelinePaused : { pipeline : Dashboard.Models.Pipeline, csrfToken : Concourse.CSRFToken } -> Cmd Callback
togglePipelinePaused { pipeline, csrfToken } =
    Task.attempt (always EmptyCallback) <|
        if pipeline.status == Concourse.PipelineStatus.PipelineStatusPaused then
            Concourse.Pipeline.unpause pipeline.teamName pipeline.name csrfToken

        else
            Concourse.Pipeline.pause pipeline.teamName pipeline.name csrfToken


orderPipelines : String -> List Dashboard.Models.Pipeline -> Concourse.CSRFToken -> Cmd Callback
orderPipelines teamName pipelines csrfToken =
    Task.attempt (always EmptyCallback) <|
        Concourse.Pipeline.order teamName (List.map .name pipelines) csrfToken


fetchBuildJobDetails : Concourse.JobIdentifier -> Cmd Callback
fetchBuildJobDetails buildJob =
    Task.attempt BuildJobDetailsFetched <|
        Concourse.Job.fetchJob buildJob


fetchBuild : Time -> Int -> Int -> Cmd Callback
fetchBuild delay browsingIndex buildId =
    Process.sleep delay
        |> Task.andThen (always <| Concourse.Build.fetch buildId)
        |> Task.map ((,) browsingIndex)
        |> Task.attempt BuildFetched


fetchJobBuild : Int -> Concourse.JobBuildIdentifier -> Cmd Callback
fetchJobBuild browsingIndex jbi =
    Concourse.Build.fetchJobBuild jbi
        |> Task.map ((,) browsingIndex)
        |> Task.attempt BuildFetched


fetchBuildHistory : Concourse.JobIdentifier -> Maybe Concourse.Pagination.Page -> Cmd Callback
fetchBuildHistory job page =
    Task.attempt BuildHistoryFetched <|
        Concourse.Build.fetchJobBuilds job page


fetchBuildPrep : Time -> Int -> Int -> Cmd Callback
fetchBuildPrep delay browsingIndex buildId =
    Process.sleep delay
        |> Task.andThen (always <| Concourse.BuildPrep.fetch buildId)
        |> Task.map ((,) browsingIndex)
        |> Task.attempt BuildPrepFetched


fetchBuildPlanAndResources : Concourse.BuildId -> Cmd Callback
fetchBuildPlanAndResources buildId =
    Task.map2 (,) (Concourse.BuildPlan.fetch buildId) (Concourse.BuildResources.fetch buildId)
        |> Task.attempt (PlanAndResourcesFetched buildId)


fetchBuildPlan : Concourse.BuildId -> Cmd Callback
fetchBuildPlan buildId =
    Concourse.BuildPlan.fetch buildId
        |> Task.map (\p -> ( p, Concourse.BuildResources.empty ))
        |> Task.attempt (PlanAndResourcesFetched buildId)


setFavicon : Maybe Concourse.BuildStatus -> Cmd Callback
setFavicon status =
    let
        iconName =
            case status of
                Just status ->
                    "/public/images/favicon-" ++ Concourse.BuildStatus.show status ++ ".png"

                Nothing ->
                    "/public/images/favicon.png"
    in
    Favicon.set iconName
        |> Task.perform (always EmptyCallback)


abortBuild : Int -> Concourse.CSRFToken -> Cmd Callback
abortBuild buildId csrfToken =
    Concourse.Build.abort buildId csrfToken
        |> Task.attempt BuildAborted


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

        ToBuild id ->
            scrollIntoView <| toString id
