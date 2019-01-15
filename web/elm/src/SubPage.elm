port module SubPage exposing (Model(..), Msg(..), init, subscriptions, update, urlUpdate, view)

import Autoscroll
import Build
import Build.Effects as Effects
import Build.Msgs
import Concourse
import Dashboard
import Dashboard.Msgs
import Effects exposing (setTitle)
import FlySuccess
import Html exposing (Html)
import Html.Styled as HS
import Http
import Job
import Json.Encode
import NotFound
import Pipeline
import QueryString
import Resource
import Resource.Effects
import Resource.Models
import Resource.Msgs
import Routes
import String
import UpdateMsg exposing (UpdateMsg)



-- TODO: move ports somewhere else


port renderPipeline : ( Json.Encode.Value, Json.Encode.Value ) -> Cmd msg


type Model
    = WaitingModel Routes.ConcourseRoute
    | BuildModel (Autoscroll.Model Build.Model)
    | JobModel Job.Model
    | ResourceModel Resource.Models.Model
    | PipelineModel Pipeline.Model
    | NotFoundModel NotFound.Model
    | DashboardModel Dashboard.Model
    | FlySuccessModel FlySuccess.Model


type Msg
    = BuildMsg (Autoscroll.Msg Build.Msgs.Msg)
    | JobMsg Job.Msg
    | ResourceMsg Resource.Msgs.Msg
    | PipelineMsg Pipeline.Msg
    | NewCSRFToken String
    | DashboardPipelinesFetched (Result Http.Error (List Concourse.Pipeline))
    | DashboardMsg Dashboard.Msgs.Msg
    | FlySuccessMsg FlySuccess.Msg


type alias Flags =
    { csrfToken : String
    , authToken : String
    , turbulencePath : String
    , pipelineRunningKeyframes : String
    }


superDupleWrap : ( a -> b, c -> d ) -> ( a, Cmd c ) -> ( b, Cmd d )
superDupleWrap ( modelFunc, msgFunc ) ( model, msg ) =
    ( modelFunc model, Cmd.map msgFunc msg )


queryGroupsForRoute : Routes.ConcourseRoute -> List String
queryGroupsForRoute route =
    QueryString.all "groups" route.queries


querySearchForRoute : Routes.ConcourseRoute -> String
querySearchForRoute route =
    QueryString.one QueryString.string "search" route.queries
        |> Maybe.withDefault ""


init : Flags -> Routes.ConcourseRoute -> ( Model, Cmd Msg )
init flags route =
    case route.logical of
        Routes.Build teamName pipelineName jobName buildName ->
            superDupleWrap ( BuildModel, BuildMsg ) <|
                Autoscroll.init
                    Build.getScrollBehavior
                    << Tuple.mapSecond (Cmd.batch << List.map Effects.toCmd)
                    << Build.init
                        { title = setTitle }
                        { csrfToken = flags.csrfToken, hash = route.hash }
                <|
                    Build.JobBuildPage
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , jobName = jobName
                        , buildName = buildName
                        }

        Routes.OneOffBuild buildId ->
            superDupleWrap ( BuildModel, BuildMsg ) <|
                Autoscroll.init
                    Build.getScrollBehavior
                    << Tuple.mapSecond (Cmd.batch << List.map Effects.toCmd)
                    << Build.init
                        { title = setTitle }
                        { csrfToken = flags.csrfToken, hash = route.hash }
                <|
                    Build.BuildPage <|
                        Result.withDefault 0 (String.toInt buildId)

        Routes.Resource teamName pipelineName resourceName ->
            superDupleWrap ( ResourceModel, ResourceMsg )
                (Resource.init
                    { resourceName = resourceName
                    , teamName = teamName
                    , pipelineName = pipelineName
                    , paging = route.page
                    , csrfToken = flags.csrfToken
                    }
                    |> Tuple.mapSecond
                        (List.map Resource.Effects.runEffect >> Cmd.batch)
                )

        Routes.Job teamName pipelineName jobName ->
            superDupleWrap ( JobModel, JobMsg ) <|
                Job.init
                    { title = setTitle }
                    { jobName = jobName
                    , teamName = teamName
                    , pipelineName = pipelineName
                    , paging = route.page
                    , csrfToken = flags.csrfToken
                    }

        Routes.Pipeline teamName pipelineName ->
            superDupleWrap ( PipelineModel, PipelineMsg ) <|
                Pipeline.init
                    { render = renderPipeline
                    , title = setTitle
                    }
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , turbulenceImgSrc = flags.turbulencePath
                    , route = route
                    }

        Routes.Dashboard ->
            superDupleWrap ( DashboardModel, DashboardMsg ) <|
                Dashboard.init
                    { title = setTitle }
                    { turbulencePath = flags.turbulencePath
                    , csrfToken = flags.csrfToken
                    , search = querySearchForRoute route
                    , highDensity = False
                    , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                    }

        Routes.DashboardHd ->
            superDupleWrap ( DashboardModel, DashboardMsg ) <|
                Dashboard.init
                    { title = setTitle }
                    { turbulencePath = flags.turbulencePath
                    , csrfToken = flags.csrfToken
                    , search = querySearchForRoute route
                    , highDensity = True
                    , pipelineRunningKeyframes = flags.pipelineRunningKeyframes
                    }

        Routes.FlySuccess ->
            superDupleWrap ( FlySuccessModel, FlySuccessMsg ) <|
                FlySuccess.init
                    { authToken = flags.authToken
                    , flyPort = QueryString.one QueryString.int "fly_port" route.queries
                    }


handleNotFound : String -> ( a -> Model, c -> Msg ) -> ( a, Cmd c, Maybe UpdateMsg ) -> ( Model, Cmd Msg )
handleNotFound notFound ( mdlFunc, msgFunc ) ( mdl, msg, outMessage ) =
    case outMessage of
        Just UpdateMsg.NotFound ->
            ( NotFoundModel { notFoundImgSrc = notFound }, setTitle "Not Found " )

        Nothing ->
            superDupleWrap ( mdlFunc, msgFunc ) <| ( mdl, msg )


update : String -> String -> Concourse.CSRFToken -> Msg -> Model -> ( Model, Cmd Msg )
update turbulence notFound csrfToken msg mdl =
    case ( msg, mdl ) of
        ( NewCSRFToken c, BuildModel scrollModel ) ->
            let
                buildModel =
                    scrollModel.subModel

                ( newBuildModel, buildEffects ) =
                    Build.update (Build.Msgs.NewCSRFToken c) buildModel
            in
            ( BuildModel { scrollModel | subModel = newBuildModel }
            , buildEffects
                |> List.map Effects.toCmd
                |> Cmd.batch
                |> Cmd.map (\buildMsg -> BuildMsg (Autoscroll.SubMsg buildMsg))
            )

        ( BuildMsg message, BuildModel scrollModel ) ->
            let
                subModel =
                    scrollModel.subModel

                model =
                    { scrollModel | subModel = { subModel | csrfToken = csrfToken } }
            in
            handleNotFound notFound ( BuildModel, BuildMsg ) (Autoscroll.update Build.updateWithMessage message model)

        ( NewCSRFToken c, JobModel model ) ->
            ( JobModel { model | csrfToken = c }, Cmd.none )

        ( JobMsg message, JobModel model ) ->
            handleNotFound notFound ( JobModel, JobMsg ) (Job.updateWithMessage message { model | csrfToken = csrfToken })

        ( PipelineMsg message, PipelineModel model ) ->
            handleNotFound notFound ( PipelineModel, PipelineMsg ) (Pipeline.updateWithMessage message model)

        ( NewCSRFToken c, ResourceModel model ) ->
            ( ResourceModel { model | csrfToken = c }, Cmd.none )

        ( ResourceMsg message, ResourceModel model ) ->
            handleNotFound notFound ( ResourceModel, ResourceMsg ) (Resource.updateWithMessage message { model | csrfToken = csrfToken })

        ( NewCSRFToken c, DashboardModel model ) ->
            ( DashboardModel { model | csrfToken = c }, Cmd.none )

        ( DashboardMsg message, DashboardModel model ) ->
            Dashboard.update message model
                |> Tuple.mapSecond (List.map Dashboard.toCmd >> Cmd.batch)
                |> superDupleWrap ( DashboardModel, DashboardMsg )

        ( FlySuccessMsg message, FlySuccessModel model ) ->
            superDupleWrap ( FlySuccessModel, FlySuccessMsg ) <| FlySuccess.update message model

        ( NewCSRFToken _, _ ) ->
            ( mdl, Cmd.none )

        unknown ->
            flip always (Debug.log "impossible combination" unknown) <|
                ( mdl, Cmd.none )


urlUpdate : Routes.ConcourseRoute -> Model -> ( Model, Cmd Msg )
urlUpdate route model =
    case ( route.logical, model ) of
        ( Routes.Pipeline team pipeline, PipelineModel mdl ) ->
            superDupleWrap ( PipelineModel, PipelineMsg ) <|
                Pipeline.changeToPipelineAndGroups
                    { teamName = team
                    , pipelineName = pipeline
                    , turbulenceImgSrc = mdl.turbulenceImgSrc
                    , route = route
                    }
                    mdl

        ( Routes.Resource teamName pipelineName resourceName, ResourceModel mdl ) ->
            superDupleWrap ( ResourceModel, ResourceMsg )
                (Resource.changeToResource
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , resourceName = resourceName
                    , paging = route.page
                    , csrfToken = mdl.csrfToken
                    }
                    mdl
                    |> Tuple.mapSecond Resource.Effects.runEffect
                )

        ( Routes.Job teamName pipelineName jobName, JobModel mdl ) ->
            superDupleWrap ( JobModel, JobMsg ) <|
                Job.changeToJob
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , jobName = jobName
                    , paging = route.page
                    , csrfToken = mdl.csrfToken
                    }
                    mdl

        ( Routes.Build teamName pipelineName jobName buildName, BuildModel scrollModel ) ->
            let
                ( submodel, subcmd ) =
                    Build.changeToBuild
                        (Build.JobBuildPage
                            { teamName = teamName
                            , pipelineName = pipelineName
                            , jobName = jobName
                            , buildName = buildName
                            }
                        )
                        scrollModel.subModel
                        |> Tuple.mapSecond (List.map Effects.toCmd)
                        |> Tuple.mapSecond Cmd.batch
            in
            ( BuildModel { scrollModel | subModel = submodel }
            , Cmd.map BuildMsg (Cmd.map Autoscroll.SubMsg subcmd)
            )

        _ ->
            ( model, Cmd.none )


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
