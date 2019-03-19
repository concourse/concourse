module SubPage.SubPage exposing
    ( Model(..)
    , handleCallback
    , handleDelivery
    , handleNotFound
    , init
    , subscriptions
    , update
    , urlUpdate
    , view
    )

import Build.Build as Build
import Build.Models
import Dashboard.Dashboard as Dashboard
import Dashboard.Models
import FlySuccess.FlySuccess as FlySuccess
import FlySuccess.Models
import Html exposing (Html)
import Job.Job as Job
import Message.Callback exposing (Callback)
import Message.Effects exposing (Effect)
import Message.Message exposing (Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..), Subscription)
import NotFound.Model
import NotFound.NotFound as NotFound
import Pipeline.Pipeline as Pipeline
import Resource.Models
import Resource.Resource as Resource
import Routes
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState)


type Model
    = BuildModel Build.Models.Model
    | JobModel Job.Model
    | ResourceModel Resource.Models.Model
    | PipelineModel Pipeline.Model
    | NotFoundModel NotFound.Model.Model
    | DashboardModel Dashboard.Models.Model
    | FlySuccessModel FlySuccess.Models.Model


type alias Flags =
    { authToken : String
    , turbulencePath : String
    , pipelineRunningKeyframes : String
    }


init : Flags -> Routes.Route -> ( Model, List Effect )
init flags route =
    case route of
        Routes.Build { id, highlight } ->
            Build.init
                { highlight = highlight
                , pageType = Build.Models.JobBuildPage id
                }
                |> Tuple.mapFirst BuildModel

        Routes.OneOffBuild { id, highlight } ->
            Build.init
                { highlight = highlight
                , pageType = Build.Models.OneOffBuildPage id
                }
                |> Tuple.mapFirst BuildModel

        Routes.Resource { id, page } ->
            Resource.init
                { resourceId = id
                , paging = page
                }
                |> Tuple.mapFirst ResourceModel

        Routes.Job { id, page } ->
            Job.init
                { jobId = id
                , paging = page
                }
                |> Tuple.mapFirst JobModel

        Routes.Pipeline { id, groups } ->
            Pipeline.init
                { pipelineLocator = id
                , turbulenceImgSrc = flags.turbulencePath
                , selectedGroups = groups
                }
                |> Tuple.mapFirst PipelineModel

        Routes.Dashboard searchType ->
            Dashboard.init
                { turbulencePath = flags.turbulencePath
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


handleCallback : Callback -> Model -> ( Model, List Effect )
handleCallback callback model =
    case model of
        BuildModel model ->
            Build.handleCallback callback ( model, [] )
                |> Tuple.mapFirst BuildModel

        JobModel model ->
            Job.handleCallback callback ( model, [] )
                |> Tuple.mapFirst JobModel

        PipelineModel model ->
            Pipeline.handleCallback callback ( model, [] )
                |> Tuple.mapFirst PipelineModel

        ResourceModel model ->
            Resource.handleCallback callback ( model, [] )
                |> Tuple.mapFirst ResourceModel

        DashboardModel model ->
            Dashboard.handleCallback callback ( model, [] )
                |> Tuple.mapFirst DashboardModel

        FlySuccessModel model ->
            FlySuccess.handleCallback callback ( model, [] )
                |> Tuple.mapFirst FlySuccessModel

        NotFoundModel model ->
            NotFound.handleCallback callback ( model, [] )
                |> Tuple.mapFirst NotFoundModel


handleDelivery :
    String
    -> Routes.Route
    -> Delivery
    -> Model
    -> ( Model, List Effect )
handleDelivery notFound route delivery model =
    case model of
        JobModel model ->
            Job.handleDelivery delivery ( model, [] )
                |> Tuple.mapFirst JobModel
                |> handleNotFound notFound route

        DashboardModel model ->
            Dashboard.handleDelivery delivery ( model, [] )
                |> Tuple.mapFirst DashboardModel

        PipelineModel model ->
            Pipeline.handleDelivery delivery ( model, [] )
                |> Tuple.mapFirst PipelineModel
                |> handleNotFound notFound route

        ResourceModel model ->
            Resource.handleDelivery delivery ( model, [] )
                |> Tuple.mapFirst ResourceModel
                |> handleNotFound notFound route

        BuildModel model ->
            Build.handleDelivery delivery ( model, [] )
                |> Tuple.mapFirst BuildModel
                |> handleNotFound notFound route

        FlySuccessModel _ ->
            ( model, [] )

        NotFoundModel _ ->
            ( model, [] )


update :
    String
    -> Routes.Route
    -> Message
    -> Model
    -> ( Model, List Effect )
update notFound route msg mdl =
    case mdl of
        BuildModel model ->
            Build.update msg ( model, [] )
                |> Tuple.mapFirst BuildModel
                |> handleNotFound notFound route

        JobModel model ->
            Job.update msg ( model, [] )
                |> Tuple.mapFirst JobModel
                |> handleNotFound notFound route

        PipelineModel model ->
            Pipeline.update msg ( model, [] )
                |> Tuple.mapFirst PipelineModel
                |> handleNotFound notFound route

        ResourceModel model ->
            Resource.update msg ( model, [] )
                |> Tuple.mapFirst ResourceModel
                |> handleNotFound notFound route

        DashboardModel model ->
            Dashboard.update msg ( model, [] )
                |> Tuple.mapFirst DashboardModel

        FlySuccessModel model ->
            FlySuccess.update msg ( model, [] )
                |> Tuple.mapFirst FlySuccessModel

        NotFoundModel model ->
            NotFound.update msg ( model, [] )
                |> Tuple.mapFirst NotFoundModel


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
                    }
                |> Tuple.mapFirst ResourceModel

        ( Routes.Job { id, page }, JobModel mdl ) ->
            mdl
                |> Job.changeToJob { jobId = id, paging = page }
                |> Tuple.mapFirst JobModel

        ( Routes.Build { id, highlight }, BuildModel buildModel ) ->
            ( { buildModel | highlight = highlight }, [] )
                |> Build.changeToBuild (Build.Models.JobBuildPage id)
                |> Tuple.mapFirst BuildModel

        _ ->
            ( model, [] )


view : UserState -> Model -> Html Message
view userState mdl =
    case mdl of
        BuildModel model ->
            Build.view userState model

        JobModel model ->
            Job.view userState model

        PipelineModel model ->
            Pipeline.view userState model

        ResourceModel model ->
            Resource.view userState model

        DashboardModel model ->
            Dashboard.view userState model

        NotFoundModel model ->
            NotFound.view userState model

        FlySuccessModel model ->
            FlySuccess.view userState model


subscriptions : Model -> List Subscription
subscriptions mdl =
    case mdl of
        BuildModel model ->
            Build.subscriptions model

        JobModel model ->
            Job.subscriptions model

        PipelineModel model ->
            Pipeline.subscriptions model

        ResourceModel model ->
            Resource.subscriptions model

        DashboardModel model ->
            Dashboard.subscriptions model

        NotFoundModel _ ->
            []

        FlySuccessModel _ ->
            []
