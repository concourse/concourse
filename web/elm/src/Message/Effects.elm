port module Message.Effects exposing
    ( Effect(..)
    , ScrollDirection(..)
    , renderPipeline
    , renderSvgIcon
    , runEffect
    , stickyHeaderConfig
    , toHtmlID
    )

import Base64
import Browser.Dom exposing (Viewport, getViewport, getViewportOf, setViewportOf)
import Browser.Navigation as Navigation
import Concourse
import Concourse.BuildStatus exposing (BuildStatus)
import Concourse.Pagination exposing (Page)
import Dashboard.Group.Models
import Http
import Json.Decode
import Json.Encode
import Maybe exposing (Maybe)
import Message.Callback
    exposing
        ( ApiEntity(..)
        , Callback(..)
        , Route(..)
        , TooltipPolicy(..)
        )
import Message.Message
    exposing
        ( DomID(..)
        , VersionToggleAction(..)
        , VisibilityAction(..)
        )
import Network.Build
import Network.BuildPlan
import Network.BuildPrep
import Network.BuildResources
import Network.Info
import Network.Job
import Network.Pipeline
import Network.Resource
import Network.Team
import Network.User
import Process
import Routes
import Task
import Time
import Url.Builder
import Views.Styles


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


port checkIsVisible : String -> Cmd msg


port setFavicon : String -> Cmd msg


port rawHttpRequest : String -> Cmd msg


port renderSvgIcon : String -> Cmd msg


port loadSideBarState : () -> Cmd msg


port saveSideBarState : Bool -> Cmd msg


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
    = ApiCall Route
    | FetchJobBuilds Concourse.JobIdentifier (Maybe Page)
    | FetchResource Concourse.ResourceIdentifier
    | FetchCheck Int
    | FetchVersionedResources Concourse.ResourceIdentifier (Maybe Page)
    | FetchResources Concourse.PipelineIdentifier
    | FetchBuildResources Concourse.BuildId
    | FetchPipeline Concourse.PipelineIdentifier
    | FetchPipelines String
    | FetchClusterInfo
    | FetchInputTo Concourse.VersionedResourceIdentifier
    | FetchOutputOf Concourse.VersionedResourceIdentifier
    | FetchAllTeams
    | FetchUser
    | FetchBuild Float Int
    | FetchJobBuild Concourse.JobBuildIdentifier
    | FetchBuildHistory Concourse.JobIdentifier (Maybe Page)
    | FetchBuildPrep Float Int
    | FetchBuildPlan Concourse.BuildId
    | FetchBuildPlanAndResources Concourse.BuildId
    | FetchAllPipelines
    | FetchAllResources
    | FetchAllJobs
    | GetCurrentTime
    | GetCurrentTimeZone
    | DoTriggerBuild Concourse.JobIdentifier
    | RerunJobBuild Concourse.JobBuildIdentifier
    | DoAbortBuild Int
    | PauseJob Concourse.JobIdentifier
    | UnpauseJob Concourse.JobIdentifier
    | ResetPipelineFocus
    | RenderPipeline Json.Encode.Value Json.Encode.Value
    | RedirectToLogin
    | LoadExternal String
    | NavigateTo String
    | ModifyUrl String
    | DoPinVersion Concourse.VersionedResourceIdentifier
    | DoUnpinVersion Concourse.ResourceIdentifier
    | DoToggleVersion VersionToggleAction VersionId
    | DoCheck Concourse.ResourceIdentifier
    | SetPinComment Concourse.ResourceIdentifier String
    | SendTokenToFly String Int
    | SendTogglePipelineRequest Concourse.PipelineIdentifier Bool
    | ShowTooltip ( String, String )
    | ShowTooltipHd ( String, String )
    | SendOrderPipelinesRequest String (List String)
    | SendLogOutRequest
    | GetScreenSize
    | PinTeamNames StickyHeaderConfig
    | Scroll ScrollDirection String
    | SetFavIcon (Maybe BuildStatus)
    | SaveToken String
    | LoadToken
    | OpenBuildEventStream { url : String, eventTypes : List String }
    | CloseBuildEventStream
    | CheckIsVisible String
    | Focus String
    | Blur String
    | RenderSvgIcon String
    | ChangeVisibility VisibilityAction Concourse.PipelineIdentifier
    | LoadSideBarState
    | SaveSideBarState Bool
    | GetViewportOf DomID TooltipPolicy
    | GetElement DomID


type alias VersionId =
    Concourse.VersionedResourceIdentifier


type ScrollDirection
    = ToTop
    | Down
    | Up
    | ToBottom
    | Sideways Float
    | ToId String


toUrl : Route -> String
toUrl route =
    case route of
        RouteJob { teamName, pipelineName, jobName } ->
            Url.Builder.absolute
                [ "api"
                , "v1"
                , "teams"
                , teamName
                , "pipelines"
                , pipelineName
                , "jobs"
                , jobName
                ]
                []

        RouteJobs { teamName, pipelineName } ->
            Url.Builder.absolute
                [ "api"
                , "v1"
                , "teams"
                , teamName
                , "pipelines"
                , pipelineName
                , "jobs"
                ]
                []


method : Route -> String
method route =
    case route of
        _ ->
            "GET"


expect : Route -> Http.Expect ApiEntity
expect route =
    case route of
        RouteJob { teamName, pipelineName } ->
            Http.expectJson <|
                Json.Decode.map Job <|
                    Concourse.decodeJob
                        { teamName = teamName, pipelineName = pipelineName }

        RouteJobs { teamName, pipelineName } ->
            Http.expectJson <|
                Json.Decode.map Jobs <|
                    Json.Decode.value


runEffect : Effect -> Navigation.Key -> Concourse.CSRFToken -> Cmd Callback
runEffect effect key csrfToken =
    case effect of
        ApiCall route ->
            Http.request
                { method = method route
                , url = toUrl route
                , headers =
                    [ Http.header Concourse.csrfTokenHeaderName csrfToken ]
                , body = Http.emptyBody
                , expect = expect route
                , timeout = Nothing
                , withCredentials = False
                }
                |> Http.toTask
                |> Task.attempt (ApiResponse route)

        FetchJobBuilds id page ->
            Network.Build.fetchJobBuilds id page
                |> Task.attempt JobBuildsFetched

        FetchResource id ->
            Network.Resource.fetchResource id
                |> Task.attempt ResourceFetched

        FetchCheck id ->
            Network.Resource.fetchCheck id
                |> Task.attempt Checked

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

        FetchPipelines team ->
            Network.Pipeline.fetchPipelinesForTeam team
                |> Task.attempt PipelinesFetched

        FetchAllResources ->
            Network.Resource.fetchAllResources
                |> Task.map (Maybe.withDefault [])
                |> Task.attempt AllResourcesFetched

        FetchAllJobs ->
            Network.Job.fetchAllJobs
                |> Task.map (Maybe.withDefault [])
                |> Task.attempt AllJobsFetched

        FetchClusterInfo ->
            Network.Info.fetch
                |> Task.attempt ClusterInfoFetched

        FetchInputTo id ->
            Network.Resource.fetchInputTo id
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt InputToFetched

        FetchOutputOf id ->
            Network.Resource.fetchOutputOf id
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt OutputOfFetched

        FetchAllTeams ->
            Network.Team.fetchTeams
                |> Task.attempt AllTeamsFetched

        FetchAllPipelines ->
            Network.Pipeline.fetchPipelines
                |> Task.attempt AllPipelinesFetched

        GetCurrentTime ->
            Task.perform GotCurrentTime Time.now

        GetCurrentTimeZone ->
            Task.perform GotCurrentTimeZone Time.here

        DoTriggerBuild id ->
            Network.Job.triggerBuild id csrfToken
                |> Task.attempt BuildTriggered

        RerunJobBuild id ->
            Network.Job.rerunJobBuild id csrfToken
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
            rawHttpRequest <| Routes.tokenToFlyRoute authToken flyPort

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

        SendOrderPipelinesRequest teamName pipelineNames ->
            Network.Pipeline.order teamName pipelineNames csrfToken
                |> Task.attempt (PipelinesOrdered teamName)

        SendLogOutRequest ->
            Task.attempt LoggedOut Network.User.logOut

        GetScreenSize ->
            Task.perform ScreenResized getViewport

        PinTeamNames shc ->
            pinTeamNames shc

        FetchBuild delay buildId ->
            Process.sleep delay
                |> Task.andThen (always <| Network.Build.fetch buildId)
                |> Task.attempt BuildFetched

        FetchJobBuild jbi ->
            Network.Build.fetchJobBuild jbi
                |> Task.attempt BuildFetched

        FetchBuildHistory job page ->
            Network.Build.fetchJobBuilds job page
                |> Task.attempt BuildHistoryFetched

        FetchBuildPrep delay buildId ->
            Process.sleep delay
                |> Task.andThen (always <| Network.BuildPrep.fetch buildId)
                |> Task.attempt (BuildPrepFetched buildId)

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

        Scroll ToTop id ->
            scroll id id (always 0) (always 0)

        Scroll Down id ->
            scroll id id (always 0) (.viewport >> .y >> (+) 60)

        Scroll Up id ->
            scroll id id (always 0) (.viewport >> .y >> (+) -60)

        Scroll ToBottom id ->
            scroll id id (always 0) (.scene >> .height)

        Scroll (Sideways delta) id ->
            scroll id id (.viewport >> .x >> (+) -delta) (always 0)

        Scroll (ToId id) idOfThingToScroll ->
            scroll id idOfThingToScroll (.viewport >> .x) (.viewport >> .y)

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

        RenderSvgIcon icon ->
            renderSvgIcon icon

        ChangeVisibility action pipelineId ->
            Network.Pipeline.changeVisibility
                action
                pipelineId.teamName
                pipelineId.pipelineName
                csrfToken
                |> Task.attempt (VisibilityChanged action pipelineId)

        LoadSideBarState ->
            loadSideBarState ()

        SaveSideBarState isOpen ->
            saveSideBarState isOpen

        GetViewportOf domID tooltipPolicy ->
            Browser.Dom.getViewportOf (toHtmlID domID)
                |> Task.attempt (GotViewport domID tooltipPolicy)

        GetElement domID ->
            Browser.Dom.getElement (toHtmlID domID)
                |> Task.attempt GotElement


toHtmlID : DomID -> String
toHtmlID domId =
    case domId of
        SideBarTeam t ->
            Base64.encode t

        SideBarPipeline p ->
            Base64.encode p.teamName ++ "_" ++ Base64.encode p.pipelineName

        FirstOccurrenceGetStepLabel stepID ->
            stepID ++ "_first_occurrence"

        StepState stepID ->
            stepID ++ "_state"

        Dashboard ->
            "dashboard"

        DashboardGroup teamName ->
            teamName

        _ ->
            ""


scroll :
    String
    -> String
    -> (Viewport -> Float)
    -> (Viewport -> Float)
    -> Cmd Callback
scroll srcId idOfThingToScroll getX getY =
    getViewportOf srcId
        |> Task.andThen
            (\info ->
                setViewportOf
                    idOfThingToScroll
                    (getX info)
                    (getY info)
            )
        |> Task.attempt (\_ -> EmptyCallback)


faviconName : Maybe BuildStatus -> String
faviconName status =
    case status of
        Just bs ->
            "/public/images/favicon-" ++ Concourse.BuildStatus.show bs ++ ".png"

        Nothing ->
            "/public/images/favicon.png"
