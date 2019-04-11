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

import Browser
import Build.Build as Build
import Build.Models
import Dashboard.Dashboard as Dashboard
import Dashboard.Models
import EffectTransformer exposing (ET)
import FlySuccess.FlySuccess as FlySuccess
import FlySuccess.Models
import Html
import Job.Job as Job
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..), Subscription)
import Message.TopLevelMessage exposing (TopLevelMessage(..))
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
    , instanceName : String
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
                , instanceName = flags.instanceName
                }
                |> Tuple.mapFirst DashboardModel

        Routes.FlySuccess flyPort ->
            FlySuccess.init
                { authToken = flags.authToken
                , flyPort = flyPort
                }
                |> Tuple.mapFirst FlySuccessModel


handleNotFound : String -> Routes.Route -> ET Model
handleNotFound notFound route ( model, effects ) =
    case getUpdateMessage model of
        UpdateMsg.NotFound ->
            let
                ( newModel, newEffects ) =
                    NotFound.init { notFoundImgSrc = notFound, route = route }
            in
            ( NotFoundModel newModel, effects ++ newEffects )

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


genericUpdate :
    ET Build.Models.Model
    -> ET Job.Model
    -> ET Resource.Models.Model
    -> ET Pipeline.Model
    -> ET Dashboard.Models.Model
    -> ET NotFound.Model.Model
    -> ET FlySuccess.Models.Model
    -> ET Model
genericUpdate fBuild fJob fRes fPipe fDash fNF fFS ( model, effects ) =
    case model of
        BuildModel buildModel ->
            fBuild ( buildModel, effects )
                |> Tuple.mapFirst BuildModel

        JobModel jobModel ->
            fJob ( jobModel, effects )
                |> Tuple.mapFirst JobModel

        PipelineModel pipelineModel ->
            fPipe ( pipelineModel, effects )
                |> Tuple.mapFirst PipelineModel

        ResourceModel resourceModel ->
            fRes ( resourceModel, effects )
                |> Tuple.mapFirst ResourceModel

        DashboardModel dashboardModel ->
            fDash ( dashboardModel, effects )
                |> Tuple.mapFirst DashboardModel

        FlySuccessModel flySuccessModel ->
            fFS ( flySuccessModel, effects )
                |> Tuple.mapFirst FlySuccessModel

        NotFoundModel notFoundModel ->
            fNF ( notFoundModel, effects )
                |> Tuple.mapFirst NotFoundModel


handleCallback : Callback -> ET Model
handleCallback callback =
    genericUpdate
        (Build.handleCallback callback)
        (Job.handleCallback callback)
        (Resource.handleCallback callback)
        (Pipeline.handleCallback callback)
        (Dashboard.handleCallback callback)
        identity
        (FlySuccess.handleCallback callback)
        >> (case callback of
                LoggedOut (Ok ()) ->
                    genericUpdate
                        handleLoggedOut
                        handleLoggedOut
                        handleLoggedOut
                        handleLoggedOut
                        handleLoggedOut
                        handleLoggedOut
                        handleLoggedOut

                _ ->
                    identity
           )


handleLoggedOut : ET { a | isUserMenuExpanded : Bool }
handleLoggedOut ( m, effs ) =
    ( { m | isUserMenuExpanded = False }
    , effs ++ [ NavigateTo <| Routes.toString <| Routes.dashboardRoute False ]
    )


handleDelivery : Delivery -> ET Model
handleDelivery delivery =
    genericUpdate
        (Build.handleDelivery delivery)
        (Job.handleDelivery delivery)
        (Resource.handleDelivery delivery)
        (Pipeline.handleDelivery delivery)
        (Dashboard.handleDelivery delivery)
        identity
        identity


update : Message -> ET Model
update msg =
    genericUpdate
        (Login.update msg >> Build.update msg)
        (Login.update msg >> Job.update msg)
        (Login.update msg >> Resource.update msg)
        (Login.update msg >> Pipeline.update msg)
        (Login.update msg >> Dashboard.update msg)
        (Login.update msg)
        (Login.update msg >> FlySuccess.update msg)
        >> (case msg of
                GoToRoute route ->
                    handleGoToRoute route

                _ ->
                    identity
           )


handleGoToRoute : Routes.Route -> ET a
handleGoToRoute route ( a, effs ) =
    ( a, effs ++ [ NavigateTo <| Routes.toString route ] )


urlUpdate : Routes.Route -> ET Model
urlUpdate route =
    genericUpdate
        (case route of
            Routes.Build { id, highlight } ->
                Build.changeToBuild
                    { pageType = Build.Models.JobBuildPage id
                    , highlight = highlight
                    }

            _ ->
                identity
        )
        (case route of
            Routes.Job { id, page } ->
                Job.changeToJob { jobId = id, paging = page }

            _ ->
                identity
        )
        (case route of
            Routes.Resource { id, page } ->
                Resource.changeToResource { resourceId = id, paging = page }

            _ ->
                identity
        )
        (case route of
            Routes.Pipeline { id, groups } ->
                Pipeline.changeToPipelineAndGroups
                    { pipelineLocator = id
                    , selectedGroups = groups
                    }

            _ ->
                identity
        )
        identity
        identity
        identity


view : UserState -> Model -> Browser.Document TopLevelMessage
view userState mdl =
    let
        ( title, body ) =
            case mdl of
                BuildModel model ->
                    ( Build.documentTitle model
                    , Build.view userState model
                    )

                JobModel model ->
                    ( Job.documentTitle model
                    , Job.view userState model
                    )

                PipelineModel model ->
                    ( Pipeline.documentTitle model
                    , Pipeline.view userState model
                    )

                ResourceModel model ->
                    ( Resource.documentTitle model
                    , Resource.view userState model
                    )

                DashboardModel model ->
                    ( Dashboard.documentTitle
                    , Dashboard.view userState model
                    )

                NotFoundModel model ->
                    ( NotFound.documentTitle
                    , NotFound.view userState model
                    )

                FlySuccessModel model ->
                    ( FlySuccess.documentTitle
                    , FlySuccess.view userState model
                    )
    in
    { title = title ++ " - Concourse", body = [ Html.map Update body ] }


subscriptions : Model -> List Subscription
subscriptions mdl =
    case mdl of
        BuildModel model ->
            Build.subscriptions model

        JobModel _ ->
            Job.subscriptions

        PipelineModel _ ->
            Pipeline.subscriptions

        ResourceModel _ ->
            Resource.subscriptions

        DashboardModel _ ->
            Dashboard.subscriptions

        NotFoundModel _ ->
            []

        FlySuccessModel _ ->
            []
