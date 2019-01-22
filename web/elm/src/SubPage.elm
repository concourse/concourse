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

import Autoscroll
import Build
import Build.Msgs
import Concourse
import Dashboard
import Effects exposing (Callback, Effect)
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
    | BuildModel (Autoscroll.Model Build.Model)
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
                |> Tuple.mapFirst (Autoscroll.Model Build.getScrollBehavior >> BuildModel)

        Routes.OneOffBuild buildId ->
            Build.BuildPage (Result.withDefault 0 (String.toInt buildId))
                |> Build.init { csrfToken = flags.csrfToken, hash = route.hash }
                |> Tuple.mapFirst (Autoscroll.Model Build.getScrollBehavior >> BuildModel)

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
                }
                |> Tuple.mapFirst DashboardModel

        Routes.DashboardHd ->
            Dashboard.init
                { turbulencePath = flags.turbulencePath
                , csrfToken = flags.csrfToken
                , search = querySearchForRoute route
                , highDensity = True
                , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
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
            Build.getUpdateMessage mdl.subModel

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
        BuildModel scrollModel ->
            let
                subModel =
                    scrollModel.subModel
            in
            Build.handleCallback callback { subModel | csrfToken = csrfToken }
                |> Tuple.mapFirst (\m -> { scrollModel | subModel = m })
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
        ( NewCSRFToken c, BuildModel scrollModel ) ->
            Build.update (Build.Msgs.NewCSRFToken c) scrollModel.subModel
                |> Tuple.mapFirst (\newBuildModel -> BuildModel { scrollModel | subModel = newBuildModel })

        ( BuildMsg message, BuildModel scrollModel ) ->
            let
                subModel =
                    scrollModel.subModel

                model =
                    { scrollModel | subModel = { subModel | csrfToken = csrfToken } }
            in
            Autoscroll.update Build.update message model
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

        ( Routes.Build teamName pipelineName jobName buildName, BuildModel scrollModel ) ->
            let
                ( submodel, cmd ) =
                    Build.changeToBuild
                        (Build.JobBuildPage
                            { teamName = teamName
                            , pipelineName = pipelineName
                            , jobName = jobName
                            , buildName = buildName
                            }
                        )
                        scrollModel.subModel
            in
            ( BuildModel { scrollModel | subModel = submodel }, cmd )

        _ ->
            ( model, [] )


view : Model -> Html Msg
view mdl =
    case mdl of
        BuildModel model ->
            Html.map BuildMsg <| Autoscroll.view Build.view model

        JobModel model ->
            Html.map JobMsg <| Job.view model

        PipelineModel model ->
            Html.map PipelineMsg <| Pipeline.view model

        ResourceModel model ->
            Html.map ResourceMsg <| (HS.toUnstyled << Resource.view) model

        DashboardModel model ->
            (Html.map DashboardMsg << HS.toUnstyled) <| Dashboard.view model

        WaitingModel _ ->
            Html.div [] []

        NotFoundModel model ->
            NotFound.view model

        FlySuccessModel model ->
            Html.map FlySuccessMsg <| FlySuccess.view model


subscriptions : Model -> Sub Msg
subscriptions mdl =
    case mdl of
        BuildModel model ->
            Sub.map BuildMsg <| Autoscroll.subscriptions Build.subscriptions model

        JobModel model ->
            Sub.map JobMsg <| Job.subscriptions model

        PipelineModel model ->
            Sub.map PipelineMsg <| Pipeline.subscriptions model

        ResourceModel model ->
            Sub.map ResourceMsg <| Resource.subscriptions model

        DashboardModel model ->
            Sub.map DashboardMsg <| Dashboard.subscriptions model

        WaitingModel _ ->
            Sub.none

        NotFoundModel _ ->
            Sub.none

        FlySuccessModel _ ->
            Sub.none
