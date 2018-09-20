port module SubPage exposing (Model(..), Msg(..), init, subscriptions, update, urlUpdate, view)

import Autoscroll
import Build
import Concourse
import Html exposing (Html)
import Http
import Job
import Json.Encode
import Resource
import Build
import NotFound
import Pipeline
import QueryString
import Resource
import Routes
import String
import UpdateMsg exposing (UpdateMsg)
import Dashboard
import DashboardHd


-- TODO: move ports somewhere else


port renderPipeline : ( Json.Encode.Value, Json.Encode.Value ) -> Cmd msg


port setTitle : String -> Cmd msg


type Model
    = WaitingModel Routes.ConcourseRoute
    | BuildModel (Autoscroll.Model Build.Model)
    | JobModel Job.Model
    | ResourceModel Resource.Model
    | PipelineModel Pipeline.Model
    | NotFoundModel NotFound.Model
    | DashboardModel Dashboard.Model
    | DashboardHdModel DashboardHd.Model


type Msg
    = BuildMsg (Autoscroll.Msg Build.Msg)
    | JobMsg Job.Msg
    | ResourceMsg Resource.Msg
    | PipelineMsg Pipeline.Msg
    | NewCSRFToken String
    | DashboardPipelinesFetched (Result Http.Error (List Concourse.Pipeline))
    | DashboardMsg Dashboard.Msg
    | DashboardHdMsg DashboardHd.Msg


type alias Flags =
    { csrfToken : String
    , turbulencePath : String
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
                    << Build.init
                        { title = setTitle }
                        { csrfToken = flags.csrfToken, hash = route.hash }
                <|
                    Build.BuildPage <|
                        Result.withDefault 0 (String.toInt buildId)

        Routes.Resource teamName pipelineName resourceName ->
            superDupleWrap ( ResourceModel, ResourceMsg ) <|
                Resource.init
                    { title = setTitle }
                    { resourceName = resourceName
                    , teamName = teamName
                    , pipelineName = pipelineName
                    , paging = route.page
                    , csrfToken = flags.csrfToken
                    }

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
                    }

        Routes.DashboardHd ->
            superDupleWrap ( DashboardModel, DashboardMsg ) <|
                Dashboard.init
                    { title = setTitle }
                    { turbulencePath = flags.turbulencePath
                    , csrfToken = flags.csrfToken
                    , search = querySearchForRoute route
                    , highDensity = True
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

                ( newBuildModel, buildCmd ) =
                    Build.update (Build.NewCSRFToken c) buildModel
            in
                ( BuildModel { scrollModel | subModel = newBuildModel }, buildCmd |> Cmd.map (\buildMsg -> BuildMsg (Autoscroll.SubMsg buildMsg)) )

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
            superDupleWrap ( DashboardModel, DashboardMsg ) <| Dashboard.update message model

        ( NewCSRFToken _, _ ) ->
            ( mdl, Cmd.none )

        ( DashboardHdMsg message, DashboardHdModel model ) ->
            superDupleWrap ( DashboardHdModel, DashboardHdMsg ) <| DashboardHd.update message model

        unknown ->
            flip always (Debug.log ("impossible combination") unknown) <|
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
            superDupleWrap ( ResourceModel, ResourceMsg ) <|
                Resource.changeToResource
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , resourceName = resourceName
                    , paging = route.page
                    , csrfToken = mdl.csrfToken
                    }
                    mdl

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
            Html.map ResourceMsg <| Resource.view model

        DashboardModel model ->
            Html.map DashboardMsg <| Dashboard.view model

        DashboardHdModel model ->
            Html.map DashboardHdMsg <| DashboardHd.view model

        WaitingModel _ ->
            Html.div [] []

        NotFoundModel model ->
            NotFound.view model


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

        DashboardHdModel model ->
            Sub.map DashboardHdMsg <| DashboardHd.subscriptions model

        WaitingModel _ ->
            Sub.none

        NotFoundModel _ ->
            Sub.none
