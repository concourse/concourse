port module Message.Effects exposing
    ( Effect(..)
    , renderPipeline
    , renderSvgIcon
    , runEffect
    , stickyHeaderConfig
    , toHtmlID
    )

import Api
import Api.Endpoints as Endpoints
import Base64
import Browser.Dom exposing (Element, getElement, getViewport, getViewportOf, setViewportOf)
import Browser.Navigation as Navigation
import Concourse
import Concourse.BuildStatus exposing (BuildStatus)
import Concourse.Pagination exposing (Page)
import Http
import Json.Decode
import Json.Encode
import Maybe exposing (Maybe)
import Message.Callback exposing (Callback(..), TooltipPolicy(..))
import Message.Message
    exposing
        ( DomID(..)
        , VersionToggleAction(..)
        , VisibilityAction(..)
        )
import Message.ScrollDirection exposing (ScrollDirection(..))
import Network.Build
import Network.Job
import Network.Pipeline
import Network.Resource
import Process
import Routes
import Task
import Time
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


runEffect : Effect -> Navigation.Key -> Concourse.CSRFToken -> Cmd Callback
runEffect effect key csrfToken =
    case effect of
        FetchJob id ->
            Api.get (Endpoints.Job id) Concourse.decodeJob
                |> Task.attempt JobFetched

        FetchJobs id ->
            Api.get (Endpoints.Jobs id) Json.Decode.value
                |> Task.attempt JobsFetched

        FetchJobBuilds id page ->
            Api.paginatedGet (Endpoints.JobBuilds id page) Concourse.decodeBuild
                |> Task.attempt JobBuildsFetched

        FetchResource id ->
            Api.get (Endpoints.Resource id) Concourse.decodeResource
                |> Task.attempt ResourceFetched

        FetchCheck id ->
            Api.get (Endpoints.Check id) Concourse.decodeCheck
                |> Task.attempt Checked

        FetchVersionedResources id paging ->
            Api.paginatedGet (Endpoints.ResourceVersions id paging)
                Concourse.decodeVersionedResource
                |> Task.map (\b -> ( paging, b ))
                |> Task.attempt VersionedResourcesFetched

        FetchResources id ->
            Api.get (Endpoints.Resources id) Json.Decode.value
                |> Task.attempt ResourcesFetched

        FetchBuildResources id ->
            Api.get (Endpoints.BuildResources id) Concourse.decodeBuildResources
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt BuildResourcesFetched

        FetchPipeline id ->
            Api.get (Endpoints.Pipeline id) Concourse.decodePipeline
                |> Task.attempt PipelineFetched

        FetchPipelines team ->
            Api.get (Endpoints.TeamPipelines team)
                (Json.Decode.list Concourse.decodePipeline)
                |> Task.attempt PipelinesFetched

        FetchAllResources ->
            Api.get Endpoints.AllResources
                (Json.Decode.nullable <|
                    Json.Decode.list Concourse.decodeResource
                )
                |> Task.map (Maybe.withDefault [])
                |> Task.attempt AllResourcesFetched

        FetchAllJobs ->
            Api.get Endpoints.AllJobs
                (Json.Decode.nullable <|
                    Json.Decode.list Concourse.decodeJob
                )
                |> Task.map (Maybe.withDefault [])
                |> Task.attempt AllJobsFetched

        FetchClusterInfo ->
            Api.get Endpoints.ClusterInfo Concourse.decodeInfo
                |> Task.attempt ClusterInfoFetched

        FetchInputTo id ->
            Api.get (Endpoints.ResourceVersionInputTo id)
                (Json.Decode.list Concourse.decodeBuild)
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt InputToFetched

        FetchOutputOf id ->
            Api.get (Endpoints.ResourceVersionOutputOf id)
                (Json.Decode.list Concourse.decodeBuild)
                |> Task.map (\b -> ( id, b ))
                |> Task.attempt OutputOfFetched

        FetchAllTeams ->
            Api.get Endpoints.AllTeams
                (Json.Decode.list Concourse.decodeTeam)
                |> Task.attempt AllTeamsFetched

        FetchAllPipelines ->
            Api.get Endpoints.AllPipelines
                (Json.Decode.list Concourse.decodePipeline)
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
            Api.request
                { endpoint = Endpoints.Logout
                , method = Api.Get
                , expect = Api.ignoreResponse
                , body = Http.emptyBody
                , headers = []
                }
                |> Task.attempt LoggedOut

        GetScreenSize ->
            Task.perform ScreenResized getViewport

        PinTeamNames shc ->
            pinTeamNames shc

        FetchBuild delay buildId ->
            Process.sleep delay
                |> Task.andThen
                    (always <|
                        Api.get (Endpoints.Build buildId) Concourse.decodeBuild
                    )
                |> Task.attempt BuildFetched

        FetchJobBuild jbi ->
            Api.get (Endpoints.JobBuild jbi) Concourse.decodeBuild
                |> Task.attempt BuildFetched

        FetchBuildJobDetails buildJob ->
            Api.get (Endpoints.Job buildJob) Concourse.decodeJob
                |> Task.attempt BuildJobDetailsFetched

        FetchBuildHistory job page ->
            Api.paginatedGet (Endpoints.JobBuilds job page) Concourse.decodeBuild
                |> Task.attempt BuildHistoryFetched

        FetchBuildPrep delay buildId ->
            Process.sleep delay
                |> Task.andThen
                    (always <|
                        Api.get (Endpoints.BuildPrep buildId)
                            Concourse.decodeBuildPrep
                    )
                |> Task.attempt (BuildPrepFetched buildId)

        FetchBuildPlanAndResources buildId ->
            Task.map2 (\a b -> ( a, b ))
                (Api.get (Endpoints.BuildPlan buildId) Concourse.decodeBuildPlan)
                (Api.get (Endpoints.BuildResources buildId) Concourse.decodeBuildResources)
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchBuildPlan buildId ->
            Api.get (Endpoints.BuildPlan buildId) Concourse.decodeBuildPlan
                |> Task.map (\p -> ( p, Concourse.emptyBuildResources ))
                |> Task.attempt (PlanAndResourcesFetched buildId)

        FetchUser ->
            Api.get Endpoints.UserInfo Concourse.decodeUser
                |> Task.attempt UserFetched

        SetFavIcon status ->
            setFavicon (faviconName status)

        DoAbortBuild buildId ->
            Network.Build.abort buildId csrfToken
                |> Task.attempt BuildAborted

        Scroll direction id ->
            scroll direction id

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


scrollToIdPadding : Float
scrollToIdPadding =
    60


scroll : ScrollDirection -> String -> Cmd Callback
scroll direction id =
    (case direction of
        ToTop ->
            scrollCoords id id (always 0) (always 0)

        Down ->
            scrollCoords id id (always 0) (.srcElem >> .viewport >> .y >> (+) 60)

        Up ->
            scrollCoords id id (always 0) (.srcElem >> .viewport >> .y >> (+) -60)

        ToBottom ->
            scrollCoords id id (always 0) (.parentElem >> .scene >> .height)

        Sideways delta ->
            scrollCoords id id (.srcElem >> .viewport >> .x >> (+) -delta) (always 0)

        ToId toId ->
            scrollCoords toId
                id
                (\{ srcElem, parentElem } ->
                    parentElem.viewport.x + srcElem.element.x - parentElem.element.x - scrollToIdPadding
                )
                (\{ srcElem, parentElem } ->
                    parentElem.viewport.y + srcElem.element.y - parentElem.element.y - scrollToIdPadding
                )
    )
        |> Task.attempt (\_ -> ScrollCompleted direction id)


scrollCoords :
    String
    -> String
    -> ({ srcElem : Element, parentElem : Element } -> Float)
    -> ({ srcElem : Element, parentElem : Element } -> Float)
    -> Task.Task Browser.Dom.Error ()
scrollCoords srcId idOfThingToScroll getX getY =
    Task.sequence [ getElement srcId, getElement idOfThingToScroll ]
        |> Task.andThen
            (\elems ->
                getViewportOf idOfThingToScroll
                    |> Task.andThen
                        (\parentViewport ->
                            Task.succeed
                                { elems = elems
                                , parentViewport = parentViewport
                                }
                        )
            )
        |> Task.andThen
            (\{ elems, parentViewport } ->
                case elems of
                    [ srcInfo, parentInfo ] ->
                        let
                            info =
                                { srcElem = srcInfo

                                -- https://github.com/elm/browser/issues/86
                                , parentElem =
                                    { parentInfo
                                        | viewport = parentViewport.viewport
                                        , scene = parentViewport.scene
                                    }
                                }
                        in
                        setViewportOf
                            idOfThingToScroll
                            (getX info)
                            (getY info)

                    _ ->
                        Task.fail <|
                            Browser.Dom.NotFound "unexpected number of elements"
            )


faviconName : Maybe BuildStatus -> String
faviconName status =
    case status of
        Just bs ->
            "/public/images/favicon-" ++ Concourse.BuildStatus.show bs ++ ".png"

        Nothing ->
            "/public/images/favicon.png"
