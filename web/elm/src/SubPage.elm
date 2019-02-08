module SubPage exposing
    ( Model(..)
    , handleCallback
    , handleNotFound
    , init
    , subscriptions
    , update
    , urlUpdate
    , view
    )

import Build
import Build.Models
import Build.Msgs
import Callback exposing (Callback)
import Concourse
import Dashboard
import Effects exposing (Effect)
import FlySuccess
import Html exposing (Html)
import Html.Styled as HS
import Job
import NotFound
import Pipeline
import Resource
import Resource.Models
import Routes
import String
import SubPage.Msgs exposing (Msg(..))
import Subscription exposing (Subscription)
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState)


type Model
    = BuildModel Build.Models.Model
    | JobModel Job.Model
    | ResourceModel Resource.Models.Model
    | PipelineModel Pipeline.Model
    | NotFoundModel NotFound.Model
    | DashboardModel Dashboard.Model
    | FlySuccessModel FlySuccess.Model


type alias Flags =
    { csrfToken : String
    , authToken : String
    , turbulencePath : String
    , pipelineRunningKeyframes : String
    }


init : Flags -> Routes.Route -> ( Model, List Effect )
init flags route =
    case route of
        Routes.Build teamName pipelineName jobName buildName highlight ->
            Build.Models.JobBuildPage
                { teamName = teamName
                , pipelineName = pipelineName
                , jobName = jobName
                , buildName = buildName
                }
                |> Build.init
                    { csrfToken = flags.csrfToken
                    , highlight = highlight
                    }
                |> Tuple.mapFirst BuildModel

        Routes.OneOffBuild buildId highlight ->
            Build.Models.BuildPage (Result.withDefault 0 (String.toInt buildId))
                |> Build.init
                    { csrfToken = flags.csrfToken
                    , highlight = highlight
                    }
                |> Tuple.mapFirst BuildModel

        Routes.Resource teamName pipelineName resourceName page ->
            Resource.init
                { resourceName = resourceName
                , teamName = teamName
                , pipelineName = pipelineName
                , paging = page
                , csrfToken = flags.csrfToken
                }
                |> Tuple.mapFirst ResourceModel

        Routes.Job teamName pipelineName jobName page ->
            Job.init
                { jobName = jobName
                , teamName = teamName
                , pipelineName = pipelineName
                , paging = page
                , csrfToken = flags.csrfToken
                }
                |> Tuple.mapFirst JobModel

        Routes.Pipeline teamName pipelineName groups ->
            Pipeline.init
                { teamName = teamName
                , pipelineName = pipelineName
                , turbulenceImgSrc = flags.turbulencePath
                , selectedGroups = groups
                }
                |> Tuple.mapFirst PipelineModel

        Routes.Dashboard (Routes.Normal search) ->
            Dashboard.init
                { turbulencePath = flags.turbulencePath
                , csrfToken = flags.csrfToken
                , search = search |> Maybe.withDefault ""
                , highDensity = False
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                , route = route
                }
                |> Tuple.mapFirst DashboardModel

        Routes.Dashboard Routes.HighDensity ->
            Dashboard.init
                { turbulencePath = flags.turbulencePath
                , csrfToken = flags.csrfToken
                , search = ""
                , highDensity = True
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                , route = route
                }
                |> Tuple.mapFirst DashboardModel

        Routes.FlySuccess flyPort ->
            FlySuccess.init
                { authToken = flags.authToken
                , flyPort = flyPort
                }
                |> Tuple.mapFirst FlySuccessModel


handleNotFound : String -> ( Model, List Effect ) -> ( Model, List Effect )
handleNotFound notFound ( model, effects ) =
    case getUpdateMessage model of
        UpdateMsg.NotFound ->
            ( NotFoundModel { notFoundImgSrc = notFound }, [ Effects.SetTitle "Not Found " ] )

        UpdateMsg.AOK ->
            ( model, effects )


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    case model of
        BuildModel mdl ->
            Build.getUpdateMessage mdl

        JobModel mdl ->
            Job.getUpdateMessage mdl

        ResourceModel mdl ->
            Resource.getUpdateMessage mdl

        PipelineModel mdl ->
            Pipeline.getUpdateMessage mdl

        _ ->
            UpdateMsg.AOK


handleCallback :
    Concourse.CSRFToken
    -> Callback
    -> Model
    -> ( Model, List Effect )
handleCallback csrfToken callback model =
    case model of
        BuildModel buildModel ->
            Build.handleCallback callback { buildModel | csrfToken = csrfToken }
                |> Tuple.mapFirst BuildModel

        JobModel model ->
            Job.handleCallback callback { model | csrfToken = csrfToken }
                |> Tuple.mapFirst JobModel

        PipelineModel model ->
            Pipeline.handleCallback callback model
                |> Tuple.mapFirst PipelineModel

        ResourceModel model ->
            Resource.handleCallback callback { model | csrfToken = csrfToken }
                |> Tuple.mapFirst ResourceModel

        DashboardModel model ->
            Dashboard.handleCallback callback model
                |> Tuple.mapFirst DashboardModel

        FlySuccessModel model ->
            FlySuccess.handleCallback callback model
                |> Tuple.mapFirst FlySuccessModel

        _ ->
            ( model, [] )


update :
    String
    -> String
    -> Concourse.CSRFToken
    -> Msg
    -> Model
    -> ( Model, List Effect )
update turbulence notFound csrfToken msg mdl =
    case ( msg, mdl ) of
        ( NewCSRFToken c, BuildModel buildModel ) ->
            Build.update (Build.Msgs.NewCSRFToken c) buildModel
                |> Tuple.mapFirst BuildModel

        ( BuildMsg msg, BuildModel buildModel ) ->
            let
                model =
                    { buildModel | csrfToken = csrfToken }
            in
            Build.update msg model
                |> Tuple.mapFirst BuildModel
                |> handleNotFound notFound

        ( NewCSRFToken c, JobModel model ) ->
            ( JobModel { model | csrfToken = c }, [] )

        ( JobMsg message, JobModel model ) ->
            Job.update message { model | csrfToken = csrfToken }
                |> Tuple.mapFirst JobModel
                |> handleNotFound notFound

        ( PipelineMsg message, PipelineModel model ) ->
            Pipeline.update message model
                |> Tuple.mapFirst PipelineModel
                |> handleNotFound notFound

        ( NewCSRFToken c, ResourceModel model ) ->
            ( ResourceModel { model | csrfToken = c }, [] )

        ( ResourceMsg message, ResourceModel model ) ->
            Resource.update message { model | csrfToken = csrfToken }
                |> Tuple.mapFirst ResourceModel
                |> handleNotFound notFound

        ( NewCSRFToken c, DashboardModel model ) ->
            ( DashboardModel { model | csrfToken = c }, [] )

        ( DashboardMsg message, DashboardModel model ) ->
            Dashboard.update message model
                |> Tuple.mapFirst DashboardModel

        ( FlySuccessMsg message, FlySuccessModel model ) ->
            FlySuccess.update message model
                |> Tuple.mapFirst FlySuccessModel

        ( NewCSRFToken _, mdl ) ->
            ( mdl, [] )

        unknown ->
            flip always (Debug.log "impossible combination" unknown) <|
                ( mdl, [] )


urlUpdate : Routes.Route -> Model -> ( Model, List Effect )
urlUpdate route model =
    case ( route, model ) of
        ( Routes.Pipeline team pipeline groups, PipelineModel mdl ) ->
            Pipeline.changeToPipelineAndGroups
                { teamName = team
                , pipelineName = pipeline
                , turbulenceImgSrc = mdl.turbulenceImgSrc
                , selectedGroups = groups
                }
                mdl
                |> Tuple.mapFirst PipelineModel

        ( Routes.Resource teamName pipelineName resourceName page, ResourceModel mdl ) ->
            Resource.changeToResource
                { teamName = teamName
                , pipelineName = pipelineName
                , resourceName = resourceName
                , paging = page
                , csrfToken = mdl.csrfToken
                }
                mdl
                |> Tuple.mapFirst ResourceModel

        ( Routes.Job teamName pipelineName jobName page, JobModel mdl ) ->
            Job.changeToJob
                { teamName = teamName
                , pipelineName = pipelineName
                , jobName = jobName
                , paging = page
                , csrfToken = mdl.csrfToken
                }
                mdl
                |> Tuple.mapFirst JobModel

        ( Routes.Build teamName pipelineName jobName buildName highlight, BuildModel buildModel ) ->
            Build.changeToBuild
                (Build.Models.JobBuildPage
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , jobName = jobName
                    , buildName = buildName
                    }
                )
                { buildModel | highlight = highlight }
                |> Tuple.mapFirst BuildModel

        _ ->
            ( model, [] )


view : UserState -> Model -> Html Msg
view userState mdl =
    case mdl of
        BuildModel model ->
            Build.view model
                |> Html.map BuildMsg

        JobModel model ->
            Job.view model
                |> Html.map JobMsg

        PipelineModel model ->
            Pipeline.view model
                |> Html.map PipelineMsg

        ResourceModel model ->
            Resource.view userState model
                |> HS.toUnstyled
                |> Html.map ResourceMsg

        DashboardModel model ->
            Dashboard.view model
                |> HS.toUnstyled
                |> Html.map DashboardMsg

        NotFoundModel model ->
            NotFound.view model

        FlySuccessModel model ->
            FlySuccess.view model
                |> Html.map FlySuccessMsg


subscriptions : Model -> List (Subscription Msg)
subscriptions mdl =
    case mdl of
        BuildModel model ->
            Build.subscriptions model
                |> List.map (Subscription.map BuildMsg)

        JobModel model ->
            Job.subscriptions model
                |> List.map (Subscription.map JobMsg)

        PipelineModel model ->
            Pipeline.subscriptions model
                |> List.map (Subscription.map PipelineMsg)

        ResourceModel model ->
            Resource.subscriptions model
                |> List.map (Subscription.map ResourceMsg)

        DashboardModel model ->
            Dashboard.subscriptions model
                |> List.map (Subscription.map DashboardMsg)

        NotFoundModel _ ->
            []

        FlySuccessModel _ ->
            []
