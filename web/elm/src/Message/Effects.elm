port module Message.Effects exposing
    ( Effect(..)
    , pipelinesSectionName
    , renderPipeline
    , renderSvgIcon
    , runEffect
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
        ( CommentBarButtonKind(..)
        , DomID(..)
        , PipelinesSection(..)
        , VersionToggleAction(..)
        , VisibilityAction(..)
        )
import Message.ScrollDirection exposing (ScrollDirection(..))
import Message.Storage
    exposing
        ( deleteFromCache
        , favoritedInstanceGroupsKey
        , favoritedPipelinesKey
        , jobsKey
        , loadFromCache
        , loadFromLocalStorage
        , pipelinesKey
        , saveToCache
        , saveToLocalStorage
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


port renderCausality : String -> Cmd msg


port pinTeamNames : StickyHeaderConfig -> Cmd msg


port resetPipelineFocus : () -> Cmd msg


port requestLoginRedirect : String -> Cmd msg


port openEventStream : { url : String, eventTypes : List String } -> Cmd msg


port closeEventStream : () -> Cmd msg


port checkIsVisible : String -> Cmd msg


port setFavicon : String -> Cmd msg


port rawHttpRequest : String -> Cmd msg


port renderSvgIcon : String -> Cmd msg


port syncTextareaHeight : String -> Cmd msg


port syncStickyBuildLogHeaders : () -> Cmd msg


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
    | FetchJobBuilds Concourse.JobIdentifier Page
    | FetchResource Concourse.ResourceIdentifier
    | FetchCheck Int
    | FetchVersionedResource Concourse.VersionedResourceIdentifier
    | FetchVersionedResources Concourse.ResourceIdentifier Page
    | FetchVersionedResourceId Concourse.ResourceIdentifier Concourse.Version
    | FetchResources Concourse.PipelineIdentifier
    | FetchBuildResources Concourse.BuildId
    | FetchPipeline Concourse.PipelineIdentifier
    | FetchPipelines String
    | FetchClusterInfo
    | FetchInputTo Concourse.VersionedResourceIdentifier
    | FetchDownstreamCausality Concourse.VersionedResourceIdentifier
    | FetchOutputOf Concourse.VersionedResourceIdentifier
    | FetchUpstreamCausality Concourse.VersionedResourceIdentifier
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
    | SetBuildComment Int String
    | DoAbortBuild Int
    | PauseJob Concourse.JobIdentifier
    | UnpauseJob Concourse.JobIdentifier
    | ResetPipelineFocus
    | RenderPipeline (List Concourse.Job) (List Concourse.Resource)
    | RenderCausality String
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
    | SendOrderPipelinesRequest Concourse.TeamName (List Concourse.PipelineName)
    | SendOrderPipelinesWithinGroupRequest Concourse.InstanceGroupIdentifier (List Concourse.InstanceVars)
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
    | SyncStickyBuildLogHeaders
    | SaveFavoritedPipelines (Set DatabaseID)
    | LoadFavoritedPipelines
    | SaveFavoritedInstanceGroups (Set ( Concourse.TeamName, Concourse.PipelineName ))
    | LoadFavoritedInstanceGroups


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
                |> Api.expectJson (Json.Decode.list Concourse.decodeJob)
                |> Api.request
                |> Task.attempt JobsFetched

        FetchJobBuilds id page ->
            Api.paginatedGet
                (Endpoints.JobBuildsList |> Endpoints.Job id)
                (Just page)
                []
                Concourse.decodeBuild
                |> Api.request
                |> Task.map (\b -> ( page, b ))
                |> Task.attempt JobBuildsFetched

        FetchResource id ->
            Api.get (Endpoints.BaseResource |> Endpoints.Resource id)
                |> Api.expectJson Concourse.decodeResource
                |> Api.request
                |> Task.attempt ResourceFetched

        FetchCheck id ->
            Api.get (Endpoints.Build id Endpoints.BaseBuild)
                |> Api.expectJson Concourse.decodeBuild
                |> Api.request
                |> Task.attempt Checked

        FetchVersionedResourceId id version ->
            Api.paginatedGet
                (Endpoints.ResourceVersionsList |> Endpoints.Resource id)
                Nothing
                (Routes.versionQueryParams version)
                Concourse.decodeVersionedResource
                |> Api.request
                |> Task.map (\b -> List.head b.content)
                |> Task.attempt VersionedResourceIdFetched

        FetchVersionedResources id page ->
            Api.paginatedGet
                (Endpoints.ResourceVersionsList |> Endpoints.Resource id)
                (Just page)
                []
                Concourse.decodeVersionedResource
                |> Api.request
                |> Task.map (\b -> ( page, b ))
                |> Task.attempt VersionedResourcesFetched

        FetchVersionedResource id ->
            Api.get
                (Endpoints.BaseResourceVersion |> Endpoints.ResourceVersion id)
                |> Api.expectJson Concourse.decodeVersionedResource
                |> Api.request
                |> Task.attempt VersionedResourceFetched

        FetchResources id ->
            Api.get
                (Endpoints.PipelineResourcesList |> Endpoints.Pipeline id)
                |> Api.expectJson (Json.Decode.list Concourse.decodeResource)
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

        FetchDownstreamCausality id ->
            Api.get
                (Endpoints.DownstreamCausality |> Endpoints.ResourceVersion id)
                |> Api.expectJson Concourse.decodeCausality
                |> Api.request
                |> Task.map (\b -> ( Concourse.Downstream, Just b ))
                |> Task.attempt CausalityFetched

        FetchOutputOf id ->
            Api.get
                (Endpoints.ResourceVersionOutputOf |> Endpoints.ResourceVersion id)
                |> Api.expectJson (Json.Decode.list Concourse.decodeBuild)
                |> Api.request
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt OutputOfFetched

        FetchUpstreamCausality id ->
            Api.get
                (Endpoints.UpstreamCasuality |> Endpoints.ResourceVersion id)
                |> Api.expectJson Concourse.decodeCausality
                |> Api.request
                |> Task.map (\b -> ( Concourse.Upstream, Just b ))
                |> Task.attempt CausalityFetched

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

        SetBuildComment buildId comment ->
            Api.put (Endpoints.SetComment |> Endpoints.Build buildId) csrfToken
                |> Api.withJsonBody
                    (Json.Encode.object
                        [ ( "comment"
                          , Json.Encode.string comment
                          )
                        ]
                    )
                |> Api.request
                |> Task.attempt (BuildCommentSet buildId comment)

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

        RenderCausality dot ->
            renderCausality dot

        RenderPipeline jobs resources ->
            renderPipeline
                ( Json.Encode.list Concourse.encodeJob jobs
                , Json.Encode.list Concourse.encodeResource resources
                )

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
                |> Api.expectJson Concourse.decodeBuild
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

        SendOrderPipelinesRequest teamName pipelineNames ->
            Api.put
                (Endpoints.OrderTeamPipelines |> Endpoints.Team teamName)
                csrfToken
                |> Api.withJsonBody
                    (Json.Encode.list Json.Encode.string pipelineNames)
                |> Api.request
                |> Task.attempt (PipelinesOrdered teamName)

        SendOrderPipelinesWithinGroupRequest id instanceVars ->
            Api.put
                (Endpoints.OrderInstanceGroupPipelines |> Endpoints.InstanceGroup id)
                csrfToken
                |> Api.withJsonBody
                    (Json.Encode.list Concourse.encodeInstanceVars instanceVars)
                |> Api.request
                |> Task.attempt (PipelinesOrdered id.teamName)

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
                []
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
                    |> Api.expectJson Concourse.decodeBuildPlanResponse
                    |> Api.request
                )
                (Api.get (Endpoints.BuildResourcesList |> Endpoints.Build buildId)
                    |> Api.expectJson Concourse.decodeBuildResources
                    |> Api.request
                )
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchBuildPlan buildId ->
            Api.get (Endpoints.BuildPlan |> Endpoints.Build buildId)
                |> Api.expectJson Concourse.decodeBuildPlanResponse
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
            saveToLocalStorage ( sideBarStateKey, encodeSideBarState state )

        LoadSideBarState ->
            loadFromLocalStorage sideBarStateKey

        SaveCachedJobs jobs ->
            saveToCache ( jobsKey, jobs |> Json.Encode.list encodeJob )

        LoadCachedJobs ->
            loadFromCache jobsKey

        DeleteCachedJobs ->
            deleteFromCache jobsKey

        SaveCachedPipelines pipelines ->
            saveToCache ( pipelinesKey, pipelines |> Json.Encode.list encodePipeline )

        LoadCachedPipelines ->
            loadFromCache pipelinesKey

        DeleteCachedPipelines ->
            deleteFromCache pipelinesKey

        SaveFavoritedPipelines pipelineIDs ->
            saveToLocalStorage
                ( favoritedPipelinesKey
                , pipelineIDs |> Json.Encode.set Json.Encode.int
                )

        LoadFavoritedPipelines ->
            loadFromLocalStorage favoritedPipelinesKey

        SaveFavoritedInstanceGroups igs ->
            saveToLocalStorage
                ( favoritedInstanceGroupsKey
                , igs
                    |> Json.Encode.set
                        (\( teamName, name ) ->
                            Concourse.encodeInstanceGroupId { teamName = teamName, name = name }
                        )
                )

        LoadFavoritedInstanceGroups ->
            loadFromLocalStorage favoritedInstanceGroupsKey

        SaveCachedTeams teams ->
            saveToCache ( teamsKey, teams |> Json.Encode.list encodeTeam )

        LoadCachedTeams ->
            loadFromCache teamsKey

        DeleteCachedTeams ->
            deleteFromCache teamsKey

        GetViewportOf domID ->
            Browser.Dom.getViewportOf (toHtmlID domID)
                |> Task.attempt (GotViewport domID)

        GetElement domID ->
            Browser.Dom.getElement (toHtmlID domID)
                |> Task.attempt GotElement

        SyncTextareaHeight domID ->
            syncTextareaHeight (toHtmlID domID)

        SyncStickyBuildLogHeaders ->
            syncStickyBuildLogHeaders ()


pipelinesSectionName : PipelinesSection -> String
pipelinesSectionName section =
    case section of
        FavoritesSection ->
            "Favorites"

        AllPipelinesSection ->
            "AllPipelines"


toHtmlID : DomID -> String
toHtmlID domId =
    case domId of
        SideBarTeam section t ->
            pipelinesSectionName section ++ "_" ++ Base64.encode t

        SideBarPipeline section id ->
            pipelinesSectionName section ++ "_" ++ String.fromInt id

        SideBarInstancedPipeline section id ->
            -- This can be the same as SideBarPipeline because they are
            -- mutually exclusive
            pipelinesSectionName section ++ "_" ++ String.fromInt id

        SideBarInstanceGroup section teamName groupName ->
            pipelinesSectionName section
                ++ "_"
                ++ Base64.encode teamName
                ++ "_"
                ++ Base64.encode groupName

        PipelineStatusIcon section p ->
            pipelinesSectionName section
                ++ "_"
                ++ encodePipelineId p
                ++ "_status"

        VisibilityButton section p ->
            pipelinesSectionName section
                ++ "_"
                ++ encodePipelineId p
                ++ "_visibility"

        PipelineCardFavoritedIcon section p ->
            pipelinesSectionName section
                ++ "_"
                ++ encodePipelineId p
                ++ "_favorite"

        InstanceGroupCardFavoritedIcon section { teamName, name } ->
            pipelinesSectionName section
                ++ "_"
                ++ Base64.encode teamName
                ++ "_"
                ++ Base64.encode name
                ++ "_favorite"

        PipelineCardPauseToggle section p ->
            pipelinesSectionName section
                ++ "_"
                ++ encodePipelineId p
                ++ "_toggle_pause"

        PipelineCardName section p ->
            pipelinesSectionName section
                ++ "_"
                ++ encodePipelineId p
                ++ "_name"

        UserDisplayName _ ->
            "user-id"

        PipelineCardNameHD p ->
            "HD_"
                ++ encodePipelineId p
                ++ "_name"

        InstanceGroupCardName section teamName groupName ->
            pipelinesSectionName section
                ++ "_"
                ++ Base64.encode teamName
                ++ "_"
                ++ Base64.encode groupName
                ++ "_name"

        InstanceGroupCardNameHD teamName groupName ->
            "HD_"
                ++ Base64.encode teamName
                ++ "_"
                ++ Base64.encode groupName
                ++ "_name"

        PipelineCardInstanceVar section p varName _ ->
            pipelinesSectionName section
                ++ "_"
                ++ encodePipelineId p
                ++ "_var_"
                ++ Base64.encode varName

        PipelineCardInstanceVars section p _ ->
            pipelinesSectionName section
                ++ "_"
                ++ encodePipelineId p
                ++ "_vars"

        PipelinePreview section p ->
            "pipeline_preview_"
                ++ pipelinesSectionName section
                ++ "_"
                ++ encodePipelineId p

        JobPreview section p jobName ->
            "job_preview_"
                ++ pipelinesSectionName section
                ++ "_"
                ++ encodePipelineId p
                ++ "_jobs_"
                ++ jobName

        ChangedStepLabel stepID _ ->
            stepID ++ "_changed"

        StepState stepID ->
            stepID ++ "_state"

        StepInitialization stepID ->
            stepID ++ "_image"

        StepVersion stepID ->
            stepID ++ "_version"

        SideBarIcon ->
            "sidebar-icon"

        Dashboard ->
            "dashboard"

        DashboardGroup teamName ->
            teamName

        ResourceCommentTextarea ->
            "resource_comment"

        TopBarFavoritedIcon _ ->
            "top-bar-favorited-icon"

        TopBarPauseToggle _ ->
            "top-bar-pause-toggle"

        TopBarPinIcon ->
            "top-bar-pin-icon"

        CommentBar id ->
            "comment-bar-" ++ toHtmlID id

        CommentBarButton kind id ->
            "comment-bar-"
                ++ (case kind of
                        Edit ->
                            "edit"

                        Save ->
                            "save"
                   )
                ++ "-button-"
                ++ toHtmlID id

        ToggleBuildCommentButton ->
            "toggle-build-comment-button"

        BuildComment ->
            "build-comment"

        AbortBuildButton ->
            "abort-build-button"

        RerunBuildButton ->
            "rerun-build-button"

        TriggerBuildButton ->
            "trigger-build-button"

        ToggleJobButton ->
            "toggle-job-button"

        CheckButton _ ->
            "check-button"

        BuildTab id _ ->
            String.fromInt id

        PinIcon ->
            "pin-icon"

        EditButton ->
            "edit-button"

        PinButton id ->
            "pin-button_" ++ String.fromInt id.versionID

        VersionToggle id ->
            "version-toggle_" ++ String.fromInt id.versionID

        PinBar ->
            "pin-bar"

        JobName ->
            "job-name"

        JobBuildLink name ->
            "job-build-" ++ Base64.encode name

        NextPageButton ->
            "next-page"

        PreviousPageButton ->
            "previous-page"

        InputsTo id ->
            "view-all-inputs-" ++ String.fromInt id.versionID

        OutputsOf id ->
            "view-all-outputs" ++ String.fromInt id.versionID

        _ ->
            ""


encodePipelineId : Concourse.DatabaseID -> String
encodePipelineId id =
    String.fromInt id


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
