port module Message.Effects exposing
    ( Effect(..)
    , renderPipeline
    , renderSvgIcon
    , runEffect
    , sideBarSectionName
    , stickyHeaderConfig
    , toHtmlID
    )

import Api
import Api.Endpoints as Endpoints
import Assets
import Base64
import Browser.Dom exposing (Viewport, getElement, getViewport, getViewportOf, setViewportOf)
import Browser.Navigation as Navigation
import Concourse exposing (DatabaseID, encodeJob, encodePipeline, encodeTeam)
import Concourse.BuildStatus exposing (BuildStatus)
import Concourse.Pagination exposing (Page)
import Json.Decode
import Json.Encode
import Maybe exposing (Maybe)
import Message.Callback exposing (Callback(..))
import Message.Message
    exposing
        ( DomID(..)
        , SideBarSection(..)
        , VersionToggleAction(..)
        , VisibilityAction(..)
        )
import Message.ScrollDirection exposing (ScrollDirection(..))
import Message.Storage
    exposing
        ( deleteFromLocalStorage
        , favoritedPipelinesKey
        , jobsKey
        , loadFromLocalStorage
        , loadFromSessionStorage
        , pipelinesKey
        , saveToLocalStorage
        , saveToSessionStorage
        , sideBarStateKey
        , teamsKey
        , tokenKey
        )
import Process
import Routes
import Set exposing (Set)
import SideBar.State exposing (SideBarState, encodeSideBarState)
import Task
import Time
import Views.Styles


port renderPipeline : ( Json.Encode.Value, Json.Encode.Value ) -> Cmd msg


port pinTeamNames : StickyHeaderConfig -> Cmd msg


port tooltip : ( String, String ) -> Cmd msg


port tooltipHd : ( String, String ) -> Cmd msg


port resetPipelineFocus : () -> Cmd msg


port requestLoginRedirect : String -> Cmd msg


port openEventStream : { url : String, eventTypes : List String } -> Cmd msg


port closeEventStream : () -> Cmd msg


port checkIsVisible : String -> Cmd msg


port setFavicon : String -> Cmd msg


port rawHttpRequest : String -> Cmd msg


port renderSvgIcon : String -> Cmd msg


port syncTextareaHeight : String -> Cmd msg


port scrollToId : ( String, String ) -> Cmd msg


type alias StickyHeaderConfig =
    { pageHeaderHeight : Float
    , pageBodyClass : String
    , sectionHeaderClass : String
    , sectionClass : String
    , sectionBodyClass : String
    }


type alias DatabaseID =
    Int


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
    | FetchBuildJobDetails Concourse.JobIdentifier
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
    | OpenBuildEventStream { url : String, eventTypes : List String }
    | CloseBuildEventStream
    | CheckIsVisible String
    | Focus String
    | Blur String
    | RenderSvgIcon String
    | ChangeVisibility VisibilityAction Concourse.PipelineIdentifier
    | SaveToken String
    | LoadToken
    | SaveSideBarState SideBarState
    | LoadSideBarState
    | SaveCachedJobs (List Concourse.Job)
    | LoadCachedJobs
    | DeleteCachedJobs
    | SaveCachedPipelines (List Concourse.Pipeline)
    | LoadCachedPipelines
    | DeleteCachedPipelines
    | SaveCachedTeams (List Concourse.Team)
    | LoadCachedTeams
    | DeleteCachedTeams
    | GetViewportOf DomID
    | GetElement DomID
    | SyncTextareaHeight DomID
    | SaveFavoritedPipelines (Set DatabaseID)
    | LoadFavoritedPipelines


type alias VersionId =
    Concourse.VersionedResourceIdentifier


runEffect : Effect -> Navigation.Key -> Concourse.CSRFToken -> Cmd Callback
runEffect effect key csrfToken =
    case effect of
        FetchJob id ->
            Api.get (Endpoints.BaseJob |> Endpoints.Job id)
                |> Api.expectJson Concourse.decodeJob
                |> Api.request
                |> Task.attempt JobFetched

        FetchJobs id ->
            Api.get
                (Endpoints.PipelineJobsList |> Endpoints.Pipeline id)
                |> Api.expectJson Json.Decode.value
                |> Api.request
                |> Task.attempt JobsFetched

        FetchJobBuilds id page ->
            Api.paginatedGet
                (Endpoints.JobBuildsList |> Endpoints.Job id)
                page
                Concourse.decodeBuild
                |> Api.request
                |> Task.attempt JobBuildsFetched

        FetchResource id ->
            Api.get (Endpoints.BaseResource |> Endpoints.Resource id)
                |> Api.expectJson Concourse.decodeResource
                |> Api.request
                |> Task.attempt ResourceFetched

        FetchCheck id ->
            Api.get (Endpoints.Check id)
                |> Api.expectJson Concourse.decodeCheck
                |> Api.request
                |> Task.attempt Checked

        FetchVersionedResources id paging ->
            Api.paginatedGet
                (Endpoints.ResourceVersionsList |> Endpoints.Resource id)
                paging
                Concourse.decodeVersionedResource
                |> Api.request
                |> Task.map (\b -> ( paging, b ))
                |> Task.attempt VersionedResourcesFetched

        FetchResources id ->
            Api.get
                (Endpoints.PipelineResourcesList |> Endpoints.Pipeline id)
                |> Api.expectJson Json.Decode.value
                |> Api.request
                |> Task.attempt ResourcesFetched

        FetchBuildResources id ->
            Api.get
                (Endpoints.BuildResourcesList |> Endpoints.Build id)
                |> Api.expectJson Concourse.decodeBuildResources
                |> Api.request
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt BuildResourcesFetched

        FetchPipeline id ->
            Api.get (Endpoints.BasePipeline |> Endpoints.Pipeline id)
                |> Api.expectJson Concourse.decodePipeline
                |> Api.request
                |> Task.attempt PipelineFetched

        FetchPipelines team ->
            Api.get (Endpoints.TeamPipelinesList |> Endpoints.Team team)
                |> Api.expectJson (Json.Decode.list Concourse.decodePipeline)
                |> Api.request
                |> Task.attempt PipelinesFetched

        FetchAllResources ->
            Api.get Endpoints.ResourcesList
                |> Api.expectJson
                    (Json.Decode.nullable <|
                        Json.Decode.list Concourse.decodeResource
                    )
                |> Api.request
                |> Task.map (Maybe.withDefault [])
                |> Task.attempt AllResourcesFetched

        FetchAllJobs ->
            Api.get Endpoints.JobsList
                |> Api.expectJson
                    (Json.Decode.nullable <|
                        Json.Decode.list Concourse.decodeJob
                    )
                |> Api.request
                |> Task.map (Maybe.withDefault [])
                |> Task.attempt AllJobsFetched

        FetchClusterInfo ->
            Api.get Endpoints.ClusterInfo
                |> Api.expectJson Concourse.decodeInfo
                |> Api.request
                |> Task.attempt ClusterInfoFetched

        FetchInputTo id ->
            Api.get
                (Endpoints.ResourceVersionInputTo |> Endpoints.ResourceVersion id)
                |> Api.expectJson (Json.Decode.list Concourse.decodeBuild)
                |> Api.request
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt InputToFetched

        FetchOutputOf id ->
            Api.get
                (Endpoints.ResourceVersionOutputOf |> Endpoints.ResourceVersion id)
                |> Api.expectJson (Json.Decode.list Concourse.decodeBuild)
                |> Api.request
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt OutputOfFetched

        FetchAllTeams ->
            Api.get Endpoints.TeamsList
                |> Api.expectJson (Json.Decode.list Concourse.decodeTeam)
                |> Api.request
                |> Task.attempt AllTeamsFetched

        FetchAllPipelines ->
            Api.get Endpoints.PipelinesList
                |> Api.expectJson (Json.Decode.list Concourse.decodePipeline)
                |> Api.request
                |> Task.attempt AllPipelinesFetched

        GetCurrentTime ->
            Task.perform GotCurrentTime Time.now

        GetCurrentTimeZone ->
            Task.perform GotCurrentTimeZone Time.here

        DoTriggerBuild id ->
            Api.post
                (Endpoints.JobBuildsList |> Endpoints.Job id)
                csrfToken
                |> Api.expectJson Concourse.decodeBuild
                |> Api.request
                |> Task.attempt BuildTriggered

        RerunJobBuild id ->
            Api.post (Endpoints.JobBuild id) csrfToken
                |> Api.expectJson Concourse.decodeBuild
                |> Api.request
                |> Task.attempt BuildTriggered

        PauseJob id ->
            Api.put
                (Endpoints.PauseJob |> Endpoints.Job id)
                csrfToken
                |> Api.request
                |> Task.attempt PausedToggled

        UnpauseJob id ->
            Api.put
                (Endpoints.UnpauseJob |> Endpoints.Job id)
                csrfToken
                |> Api.request
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

        DoPinVersion id ->
            Api.put
                (Endpoints.PinResourceVersion |> Endpoints.ResourceVersion id)
                csrfToken
                |> Api.request
                |> Task.attempt VersionPinned

        DoUnpinVersion id ->
            Api.put
                (Endpoints.UnpinResource |> Endpoints.Resource id)
                csrfToken
                |> Api.request
                |> Task.attempt VersionUnpinned

        DoToggleVersion action id ->
            let
                endpoint =
                    Endpoints.ResourceVersion id <|
                        case action of
                            Enable ->
                                Endpoints.EnableResourceVersion

                            Disable ->
                                Endpoints.DisableResourceVersion
            in
            Api.put endpoint csrfToken
                |> Api.request
                |> Task.attempt (VersionToggled action id)

        DoCheck rid ->
            Api.post
                (Endpoints.CheckResource |> Endpoints.Resource rid)
                csrfToken
                |> Api.withJsonBody
                    (Json.Encode.object [ ( "from", Json.Encode.null ) ])
                |> Api.expectJson Concourse.decodeCheck
                |> Api.request
                |> Task.attempt Checked

        SetPinComment rid comment ->
            Api.put
                (Endpoints.PinResourceComment |> Endpoints.Resource rid)
                csrfToken
                |> Api.withJsonBody
                    (Json.Encode.object
                        [ ( "pin_comment"
                          , Json.Encode.string comment
                          )
                        ]
                    )
                |> Api.request
                |> Task.attempt CommentSet

        SendTokenToFly authToken flyPort ->
            rawHttpRequest <| Routes.tokenToFlyRoute authToken flyPort

        SendTogglePipelineRequest id isPaused ->
            let
                endpoint =
                    Endpoints.Pipeline id <|
                        if isPaused then
                            Endpoints.UnpausePipeline

                        else
                            Endpoints.PausePipeline
            in
            Api.put endpoint csrfToken
                |> Api.request
                |> Task.attempt (PipelineToggled id)

        ShowTooltip ( teamName, pipelineName ) ->
            tooltip ( teamName, pipelineName )

        ShowTooltipHd ( teamName, pipelineName ) ->
            tooltipHd ( teamName, pipelineName )

        SendOrderPipelinesRequest teamName pipelineNames ->
            Api.put
                (Endpoints.OrderTeamPipelines |> Endpoints.Team teamName)
                csrfToken
                |> Api.withJsonBody
                    (Json.Encode.list Json.Encode.string pipelineNames)
                |> Api.request
                |> Task.attempt (PipelinesOrdered teamName)

        SendLogOutRequest ->
            Api.get Endpoints.Logout
                |> Api.request
                |> Task.attempt LoggedOut

        GetScreenSize ->
            Task.perform ScreenResized getViewport

        PinTeamNames shc ->
            pinTeamNames shc

        FetchBuild delay buildId ->
            Process.sleep delay
                |> Task.andThen
                    (always
                        (Api.get (Endpoints.BaseBuild |> Endpoints.Build buildId)
                            |> Api.expectJson Concourse.decodeBuild
                            |> Api.request
                        )
                    )
                |> Task.attempt BuildFetched

        FetchJobBuild jbi ->
            Api.get (Endpoints.JobBuild jbi)
                |> Api.expectJson Concourse.decodeBuild
                |> Api.request
                |> Task.attempt BuildFetched

        FetchBuildJobDetails buildJob ->
            Api.get (Endpoints.BaseJob |> Endpoints.Job buildJob)
                |> Api.expectJson Concourse.decodeJob
                |> Api.request
                |> Task.attempt BuildJobDetailsFetched

        FetchBuildHistory job page ->
            Api.paginatedGet
                (Endpoints.JobBuildsList |> Endpoints.Job job)
                page
                Concourse.decodeBuild
                |> Api.request
                |> Task.attempt BuildHistoryFetched

        FetchBuildPrep delay buildId ->
            Process.sleep delay
                |> Task.andThen
                    (always
                        (Api.get
                            (Endpoints.BuildPrep |> Endpoints.Build buildId)
                            |> Api.expectJson Concourse.decodeBuildPrep
                            |> Api.request
                        )
                    )
                |> Task.attempt (BuildPrepFetched buildId)

        FetchBuildPlanAndResources buildId ->
            Task.map2 (\a b -> ( a, b ))
                (Api.get (Endpoints.BuildPlan |> Endpoints.Build buildId)
                    |> Api.expectJson Concourse.decodeBuildPlan
                    |> Api.request
                )
                (Api.get (Endpoints.BuildResourcesList |> Endpoints.Build buildId)
                    |> Api.expectJson Concourse.decodeBuildResources
                    |> Api.request
                )
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchBuildPlan buildId ->
            Api.get (Endpoints.BuildPlan |> Endpoints.Build buildId)
                |> Api.expectJson Concourse.decodeBuildPlan
                |> Api.request
                |> Task.map (\p -> ( p, Concourse.emptyBuildResources ))
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchUser ->
            Api.get Endpoints.UserInfo
                |> Api.expectJson Concourse.decodeUser
                |> Api.request
                |> Task.attempt UserFetched

        SetFavIcon status ->
            status
                |> Assets.BuildFavicon
                |> Assets.toString
                |> setFavicon

        DoAbortBuild buildId ->
            Api.put (Endpoints.AbortBuild |> Endpoints.Build buildId) csrfToken
                |> Api.request
                |> Task.attempt BuildAborted

        Scroll direction id ->
            scroll direction id

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
            let
                endpoint =
                    Endpoints.Pipeline pipelineId <|
                        case action of
                            Hide ->
                                Endpoints.HidePipeline

                            Expose ->
                                Endpoints.ExposePipeline
            in
            Api.put endpoint csrfToken
                |> Api.request
                |> Task.attempt (VisibilityChanged action pipelineId)

        SaveToken token ->
            saveToLocalStorage ( tokenKey, Json.Encode.string token )

        LoadToken ->
            loadFromLocalStorage tokenKey

        SaveSideBarState state ->
            saveToSessionStorage ( sideBarStateKey, encodeSideBarState state )

        LoadSideBarState ->
            loadFromSessionStorage sideBarStateKey

        SaveCachedJobs jobs ->
            saveToLocalStorage ( jobsKey, jobs |> Json.Encode.list encodeJob )

        LoadCachedJobs ->
            loadFromLocalStorage jobsKey

        DeleteCachedJobs ->
            deleteFromLocalStorage jobsKey

        SaveCachedPipelines pipelines ->
            saveToLocalStorage ( pipelinesKey, pipelines |> Json.Encode.list encodePipeline )

        LoadCachedPipelines ->
            loadFromLocalStorage pipelinesKey

        DeleteCachedPipelines ->
            deleteFromLocalStorage pipelinesKey

        SaveFavoritedPipelines pipelineIDs ->
            saveToLocalStorage
                ( favoritedPipelinesKey
                , pipelineIDs |> Json.Encode.set Json.Encode.int
                )

        LoadFavoritedPipelines ->
            loadFromLocalStorage favoritedPipelinesKey

        SaveCachedTeams teams ->
            saveToLocalStorage ( teamsKey, teams |> Json.Encode.list encodeTeam )

        LoadCachedTeams ->
            loadFromLocalStorage teamsKey

        DeleteCachedTeams ->
            deleteFromLocalStorage teamsKey

        GetViewportOf domID ->
            Browser.Dom.getViewportOf (toHtmlID domID)
                |> Task.attempt (GotViewport domID)

        GetElement domID ->
            Browser.Dom.getElement (toHtmlID domID)
                |> Task.attempt GotElement

        SyncTextareaHeight domID ->
            syncTextareaHeight (toHtmlID domID)


sideBarSectionName : SideBarSection -> String
sideBarSectionName section =
    case section of
        Favorites ->
            "Favorites"

        AllPipelines ->
            "AllPipelines"


toHtmlID : DomID -> String
toHtmlID domId =
    case domId of
        SideBarTeam section t ->
            sideBarSectionName section ++ "_" ++ Base64.encode t

        SideBarPipeline section p ->
            sideBarSectionName section ++ "_" ++ Base64.encode p.teamName ++ "_" ++ Base64.encode p.pipelineName

        PipelineStatusIcon p ->
            Base64.encode p.teamName
                ++ "_"
                ++ Base64.encode p.pipelineName
                ++ "_status"

        VisibilityButton p ->
            Base64.encode p.teamName
                ++ "_"
                ++ Base64.encode p.pipelineName
                ++ "_visibility"

        FirstOccurrenceGetStepLabel stepID ->
            stepID ++ "_first_occurrence"

        StepState stepID ->
            stepID ++ "_state"

        Dashboard ->
            "dashboard"

        DashboardGroup teamName ->
            teamName

        ResourceCommentTextarea ->
            "resource_comment"

        _ ->
            ""


scroll : ScrollDirection -> String -> Cmd Callback
scroll direction id =
    case direction of
        ToTop ->
            scrollCoords id (always 0) (always 0)
                |> Task.attempt (\_ -> EmptyCallback)

        Down ->
            scrollCoords id (always 0) (.viewport >> .y >> (+) 60)
                |> Task.attempt (\_ -> EmptyCallback)

        Up ->
            scrollCoords id (always 0) (.viewport >> .y >> (+) -60)
                |> Task.attempt (\_ -> EmptyCallback)

        ToBottom ->
            scrollCoords id (always 0) (.scene >> .height)
                |> Task.attempt (\_ -> EmptyCallback)

        Sideways delta ->
            scrollCoords id (.viewport >> .x >> (+) -delta) (always 0)
                |> Task.attempt (\_ -> EmptyCallback)

        ToId toId ->
            scrollToId ( id, toId )


scrollCoords :
    String
    -> (Viewport -> Float)
    -> (Viewport -> Float)
    -> Task.Task Browser.Dom.Error ()
scrollCoords id getX getY =
    getViewportOf id
        |> Task.andThen
            (\viewport ->
                setViewportOf
                    id
                    (getX viewport)
                    (getY viewport)
            )
