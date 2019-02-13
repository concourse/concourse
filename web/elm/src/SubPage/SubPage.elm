module SubPage.SubPage exposing
    ( Model(..)
    , handleCallback
    , handleNotFound
    , init
    , subscriptions
    , update
    , urlUpdate
    , view
    )

import Build.Build as Build
import Build.Models
import Build.Msgs
import Callback exposing (Callback)
import Concourse
import Dashboard.Dashboard as Dashboard
import Effects exposing (Effect)
import FlySuccess.FlySuccess as FlySuccess
import Html exposing (Html)
import Html.Styled as HS
import Job.Job as Job
import NotFound
import Pipeline.Pipeline as Pipeline
import Resource.Models
import Resource.Resource as Resource
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
        Routes.Build { id, highlight } ->
            Build.init
                { csrfToken = flags.csrfToken
                , highlight = highlight
                , pageType = Build.Models.JobBuildPage id
                }
                |> Tuple.mapFirst BuildModel

        Routes.OneOffBuild { id, highlight } ->
            Build.init
                { csrfToken = flags.csrfToken
                , highlight = highlight
                , pageType = Build.Models.OneOffBuildPage id
                }
                |> Tuple.mapFirst BuildModel

        Routes.Resource { id, page } ->
            Resource.init
                { resourceId = id
                , paging = page
                , csrfToken = flags.csrfToken
                }
                |> Tuple.mapFirst ResourceModel

        Routes.Job { id, page } ->
            Job.init
                { jobId = id
                , paging = page
                , csrfToken = flags.csrfToken
                }
                |> Tuple.mapFirst JobModel

        Routes.Pipeline { id, groups } ->
            Pipeline.init
                { pipelineLocator = id
                , turbulenceImgSrc = flags.turbulencePath
                , selectedGroups = groups
                }
                |> Tuple.mapFirst PipelineModel

        Routes.Dashboard { searchType } ->
            Dashboard.init
                { turbulencePath = flags.turbulencePath
                , csrfToken = flags.csrfToken
                , searchType = searchType
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                }
                |> Tuple.mapFirst DashboardModel

        Routes.FlySuccess { flyPort } ->
            FlySuccess.init
                { authToken = flags.authToken
                , flyPort = flyPort
                }
                |> Tuple.mapFirst FlySuccessModel


handleNotFound : String -> Routes.Route -> ( Model, List Effect ) -> ( Model, List Effect )
handleNotFound notFound route ( model, effects ) =
    case getUpdateMessage model of
        UpdateMsg.NotFound ->
            NotFound.init { notFoundImgSrc = notFound, route = route }
                |> Tuple.mapFirst NotFoundModel

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

        NotFoundModel model ->
            NotFound.handleCallback callback model
                |> Tuple.mapFirst NotFoundModel


update :
    String
    -> String
    -> Concourse.CSRFToken
    -> Routes.Route
    -> Msg
    -> Model
    -> ( Model, List Effect )
update turbulence notFound csrfToken route msg mdl =
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
                |> handleNotFound notFound route

        ( NewCSRFToken c, JobModel model ) ->
            ( JobModel { model | csrfToken = c }, [] )

        ( JobMsg message, JobModel model ) ->
            Job.update message { model | csrfToken = csrfToken }
                |> Tuple.mapFirst JobModel
                |> handleNotFound notFound route

        ( PipelineMsg message, PipelineModel model ) ->
            Pipeline.update message model
                |> Tuple.mapFirst PipelineModel
                |> handleNotFound notFound route

        ( NewCSRFToken c, ResourceModel model ) ->
            ( ResourceModel { model | csrfToken = c }, [] )

        ( ResourceMsg message, ResourceModel model ) ->
            Resource.update message { model | csrfToken = csrfToken }
                |> Tuple.mapFirst ResourceModel
                |> handleNotFound notFound route

        ( NewCSRFToken c, DashboardModel model ) ->
            ( DashboardModel { model | csrfToken = c }, [] )

        ( DashboardMsg message, DashboardModel model ) ->
            Dashboard.update message model
                |> Tuple.mapFirst DashboardModel

        ( FlySuccessMsg message, FlySuccessModel model ) ->
            FlySuccess.update message model
                |> Tuple.mapFirst FlySuccessModel

        ( NotFoundMsg message, NotFoundModel model ) ->
            NotFound.update message model
                |> Tuple.mapFirst NotFoundModel

        ( NewCSRFToken _, mdl ) ->
            ( mdl, [] )

        unknown ->
            flip always (Debug.log "impossible combination" unknown) <|
                ( mdl, [] )


urlUpdate : Routes.Route -> Model -> ( Model, List Effect )
urlUpdate route model =
    case ( route, model ) of
        ( Routes.Pipeline { id, groups }, PipelineModel mdl ) ->
            mdl
                |> Pipeline.changeToPipelineAndGroups
                    { pipelineLocator = id
                    , turbulenceImgSrc = mdl.turbulenceImgSrc
                    , selectedGroups = groups
                    }
                |> Tuple.mapFirst PipelineModel

        ( Routes.Resource { id, page }, ResourceModel mdl ) ->
            mdl
                |> Resource.changeToResource
                    { resourceId = id
                    , paging = page
                    , csrfToken = mdl.csrfToken
                    }
                |> Tuple.mapFirst ResourceModel

        ( Routes.Job { id, page }, JobModel mdl ) ->
            mdl
                |> Job.changeToJob
                    { jobId = id
                    , paging = page
                    , csrfToken = mdl.csrfToken
                    }
                |> Tuple.mapFirst JobModel

        ( Routes.Build { id, highlight }, BuildModel buildModel ) ->
            { buildModel | highlight = highlight }
                |> Build.changeToBuild (Build.Models.JobBuildPage id)
                |> Tuple.mapFirst BuildModel

        _ ->
            ( model, [] )


view : UserState -> Model -> Html Msg
view userState mdl =
    case mdl of
        BuildModel model ->
            Build.view userState model
                |> Html.map BuildMsg

        JobModel model ->
            Job.view userState model
                |> Html.map JobMsg

        PipelineModel model ->
            Pipeline.view userState model
                |> Html.map PipelineMsg

        ResourceModel model ->
            Resource.view userState model
                |> HS.toUnstyled
                |> Html.map ResourceMsg

        DashboardModel model ->
            Dashboard.view userState model
                |> HS.toUnstyled
                |> Html.map DashboardMsg

        NotFoundModel model ->
            NotFound.view userState model
                |> Html.map NotFoundMsg

        FlySuccessModel model ->
            FlySuccess.view userState model
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
