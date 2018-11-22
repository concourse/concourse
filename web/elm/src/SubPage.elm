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
import QueryString
import Resource
import Resource.Models
import Routes
import String
import SubPage.Msgs exposing (Msg(..))
import UpdateMsg exposing (UpdateMsg)


type Model
    = WaitingModel Routes.ConcourseRoute
    | BuildModel Build.Model
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


queryGroupsForRoute : Routes.ConcourseRoute -> List String
queryGroupsForRoute route =
    QueryString.all "groups" route.queries


querySearchForRoute : Routes.ConcourseRoute -> String
querySearchForRoute route =
    QueryString.one QueryString.string "search" route.queries
        |> Maybe.withDefault ""


init : Flags -> Routes.ConcourseRoute -> ( Model, List Effect )
init flags route =
    case route.logical of
        Routes.Build teamName pipelineName jobName buildName ->
            Build.JobBuildPage
                { teamName = teamName
                , pipelineName = pipelineName
                , jobName = jobName
                , buildName = buildName
                }
                |> Build.init { csrfToken = flags.csrfToken, hash = route.hash }
                |> Tuple.mapFirst BuildModel

        Routes.OneOffBuild buildId ->
            Build.BuildPage (Result.withDefault 0 (String.toInt buildId))
                |> Build.init { csrfToken = flags.csrfToken, hash = route.hash }
                |> Tuple.mapFirst BuildModel

        Routes.Resource teamName pipelineName resourceName ->
            Resource.init
                { resourceName = resourceName
                , teamName = teamName
                , pipelineName = pipelineName
                , paging = route.page
                , csrfToken = flags.csrfToken
                }
                |> Tuple.mapFirst ResourceModel

        Routes.Job teamName pipelineName jobName ->
            Job.init
                { jobName = jobName
                , teamName = teamName
                , pipelineName = pipelineName
                , paging = route.page
                , csrfToken = flags.csrfToken
                }
                |> Tuple.mapFirst JobModel

        Routes.Pipeline teamName pipelineName ->
            Pipeline.init
                { teamName = teamName
                , pipelineName = pipelineName
                , turbulenceImgSrc = flags.turbulencePath
                , route = route
                }
                |> Tuple.mapFirst PipelineModel

        Routes.Dashboard ->
            Dashboard.init
                { turbulencePath = flags.turbulencePath
                , csrfToken = flags.csrfToken
                , search = querySearchForRoute route
                , highDensity = False
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                , route = route
                }
                |> Tuple.mapFirst DashboardModel

        Routes.DashboardHd ->
            Dashboard.init
                { turbulencePath = flags.turbulencePath
                , csrfToken = flags.csrfToken
                , search = querySearchForRoute route
                , highDensity = True
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                , route = route
                }
                |> Tuple.mapFirst DashboardModel

        Routes.FlySuccess ->
            FlySuccess.init
                { authToken = flags.authToken
                , flyPort = QueryString.one QueryString.int "fly_port" route.queries
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


urlUpdate : Routes.ConcourseRoute -> Model -> ( Model, List Effect )
urlUpdate route model =
    case ( route.logical, model ) of
        ( Routes.Pipeline team pipeline, PipelineModel mdl ) ->
            Pipeline.changeToPipelineAndGroups
                { teamName = team
                , pipelineName = pipeline
                , turbulenceImgSrc = mdl.turbulenceImgSrc
                , route = route
                }
                mdl
                |> Tuple.mapFirst PipelineModel

        ( Routes.Resource teamName pipelineName resourceName, ResourceModel mdl ) ->
            Resource.changeToResource
                { teamName = teamName
                , pipelineName = pipelineName
                , resourceName = resourceName
                , paging = route.page
                , csrfToken = mdl.csrfToken
                }
                mdl
                |> Tuple.mapFirst ResourceModel

        ( Routes.Job teamName pipelineName jobName, JobModel mdl ) ->
            Job.changeToJob
                { teamName = teamName
                , pipelineName = pipelineName
                , jobName = jobName
                , paging = route.page
                , csrfToken = mdl.csrfToken
                }
                mdl
                |> Tuple.mapFirst JobModel

        ( Routes.Build teamName pipelineName jobName buildName, BuildModel buildModel ) ->
            Build.changeToBuild
                (Build.JobBuildPage
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , jobName = jobName
                    , buildName = buildName
                    }
                )
                buildModel
                |> Tuple.mapFirst BuildModel

        _ ->
            ( model, [] )


view : Model -> Html Msg
view mdl =
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
            Resource.view model
                |> HS.toUnstyled
                |> Html.map ResourceMsg

        DashboardModel model ->
            Dashboard.view model
                |> HS.toUnstyled
                |> Html.map DashboardMsg

        WaitingModel _ ->
            Html.div [] []

        NotFoundModel model ->
            NotFound.view model

        FlySuccessModel model ->
            FlySuccess.view model
                |> Html.map FlySuccessMsg


subscriptions : Model -> Sub Msg
subscriptions mdl =
    case mdl of
        BuildModel model ->
            Build.subscriptions model
                |> Sub.map BuildMsg

        JobModel model ->
            Job.subscriptions model
                |> Sub.map JobMsg

        PipelineModel model ->
            Pipeline.subscriptions model
                |> Sub.map PipelineMsg

        ResourceModel model ->
            Resource.subscriptions model
                |> Sub.map ResourceMsg

        DashboardModel model ->
            Dashboard.subscriptions model
                |> Sub.map DashboardMsg

        WaitingModel _ ->
            Sub.none

        NotFoundModel _ ->
            Sub.none

        FlySuccessModel _ ->
            Sub.none
