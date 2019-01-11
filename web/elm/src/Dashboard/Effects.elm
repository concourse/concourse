port module Dashboard.Effects exposing (Effect(..), runEffect)

import Concourse
import Concourse.Pipeline
import Concourse.PipelineStatus as PipelineStatus exposing (PipelineStatus(..))
import Dashboard.APIData as APIData
import Dashboard.Group as Group
import Dashboard.Models as Models
import Dashboard.Msgs exposing (Msg(..))
import Dom
import Effects exposing (setTitle)
import LoginRedirect
import Navigation
import NewTopBar
import RemoteData
import Task
import Time
import Window


port pinTeamNames : Group.StickyHeaderConfig -> Cmd msg


port tooltip : ( String, String ) -> Cmd msg


port tooltipHd : ( String, String ) -> Cmd msg


type Effect
    = FetchData
    | FocusSearchInput
    | ModifyUrl String
    | NewUrl String
    | SendTogglePipelineRequest { pipeline : Models.Pipeline, csrfToken : Concourse.CSRFToken }
    | ShowTooltip ( String, String )
    | ShowTooltipHd ( String, String )
    | SendOrderPipelinesRequest String (List Models.Pipeline) Concourse.CSRFToken
    | RedirectToLogin String
    | SendLogOutRequest
    | SetTitle String
    | GetScreenSize
    | PinTeamNames Group.StickyHeaderConfig


runEffect : Effect -> Cmd Msg
runEffect effect =
    case effect of
        FetchData ->
            fetchData

        FocusSearchInput ->
            Task.attempt (always Noop) (Dom.focus "search-input-field")

        ModifyUrl url ->
            Navigation.modifyUrl url

        NewUrl url ->
            Navigation.newUrl url

        SendTogglePipelineRequest { pipeline, csrfToken } ->
            togglePipelinePaused { pipeline = pipeline, csrfToken = csrfToken }

        ShowTooltip ( teamName, pipelineName ) ->
            tooltip ( teamName, pipelineName )

        ShowTooltipHd ( teamName, pipelineName ) ->
            tooltipHd ( teamName, pipelineName )

        SendOrderPipelinesRequest teamName pipelines csrfToken ->
            orderPipelines teamName pipelines csrfToken

        RedirectToLogin s ->
            LoginRedirect.requestLoginRedirect s

        SendLogOutRequest ->
            NewTopBar.logOut

        SetTitle newTitle ->
            setTitle newTitle

        GetScreenSize ->
            Task.perform ScreenResized Window.size

        PinTeamNames stickyHeaderConfig ->
            pinTeamNames stickyHeaderConfig


fetchData : Cmd Msg
fetchData =
    APIData.remoteData
        |> Task.map2 (,) Time.now
        |> RemoteData.asCmd
        |> Cmd.map APIDataFetched


togglePipelinePaused : { pipeline : Models.Pipeline, csrfToken : Concourse.CSRFToken } -> Cmd Msg
togglePipelinePaused { pipeline, csrfToken } =
    Task.attempt (always Noop) <|
        if pipeline.status == PipelineStatus.PipelineStatusPaused then
            Concourse.Pipeline.unpause pipeline.teamName pipeline.name csrfToken

        else
            Concourse.Pipeline.pause pipeline.teamName pipeline.name csrfToken


orderPipelines : String -> List Models.Pipeline -> Concourse.CSRFToken -> Cmd Msg
orderPipelines teamName pipelines csrfToken =
    Task.attempt (always Noop) <|
        Concourse.Pipeline.order
            teamName
            (List.map .name pipelines)
            csrfToken
