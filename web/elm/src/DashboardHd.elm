port module DashboardHd exposing (Model, Msg, init, update, subscriptions, view, groupView, tooltipHd)

import Concourse
import Concourse.Cli
import Concourse.Info
import Concourse.Job
import Concourse.Pipeline
import Concourse.PipelineStatus
import Concourse.Resource
import Dashboard.Pipeline as Pipeline
import DashboardHelpers exposing (..)
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class, classList, id, href, src, attribute)
import Html.Attributes.Aria exposing (ariaLabel)
import Html.Events exposing (onMouseEnter)
import Http
import Mouse
import NewTopBar
import NoPipeline exposing (view, Msg)
import RemoteData
import Routes
import Set
import Task exposing (Task)
import Time exposing (Time)
import UserState


type alias Ports =
    { title : String -> Cmd Msg
    }


port tooltipHd : ( String, String ) -> Cmd msg


type alias Model =
    { topBar : NewTopBar.Model
    , mPipelines : RemoteData.WebData (List Concourse.Pipeline)
    , pipelines : List Concourse.Pipeline
    , mJobs : RemoteData.WebData (List Concourse.Job)
    , pipelineJobs : Dict Int (List Concourse.Job)
    , pipelineResourceErrors : Dict ( String, String ) Bool
    , concourseVersion : String
    , turbulenceImgSrc : String
    , now : Maybe Time
    , showHelp : Bool
    , hideFooter : Bool
    , hideFooterCounter : Time
    }


type Msg
    = Noop
    | PipelinesResponse (RemoteData.WebData (List Concourse.Pipeline))
    | JobsResponse (RemoteData.WebData (List Concourse.Job))
    | ResourcesResponse (RemoteData.WebData (List Concourse.Resource))
    | ClockTick Time.Time
    | VersionFetched (Result Http.Error String)
    | AutoRefresh Time
    | ShowFooter
    | TopBarMsg NewTopBar.Msg
    | Tooltip String String


init : Ports -> String -> String -> ( Model, Cmd Msg )
init ports turbulencePath search =
    let
        ( topBar, topBarMsg ) =
            NewTopBar.init False search
    in
        ( { topBar = topBar
          , mPipelines = RemoteData.NotAsked
          , pipelines = []
          , mJobs = RemoteData.NotAsked
          , pipelineJobs = Dict.empty
          , pipelineResourceErrors = Dict.empty
          , now = Nothing
          , turbulenceImgSrc = turbulencePath
          , concourseVersion = ""
          , showHelp = False
          , hideFooter = False
          , hideFooterCounter = 0
          }
        , Cmd.batch
            [ fetchPipelines
            , fetchVersion
            , Cmd.map TopBarMsg topBarMsg
            , ports.title <| "Dashboard HD" ++ " - "
            ]
        )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    let
        reload =
            Cmd.batch <|
                (if model.mPipelines == RemoteData.Loading then
                    []
                 else
                    [ fetchPipelines ]
                )
                    ++ [ fetchVersion, Cmd.map TopBarMsg NewTopBar.fetchUser ]
    in
        case msg of
            Noop ->
                ( model, Cmd.none )

            PipelinesResponse response ->
                case response of
                    RemoteData.Success pipelines ->
                        ( { model | mPipelines = response, pipelines = pipelines }, Cmd.batch [ fetchAllJobs, fetchAllResources ] )

                    _ ->
                        ( model, Cmd.none )

            JobsResponse response ->
                case ( response, model.mPipelines ) of
                    ( RemoteData.Success jobs, RemoteData.Success pipelines ) ->
                        ( { model | mJobs = response, pipelineJobs = jobsByPipelineId pipelines jobs }, Cmd.none )

                    _ ->
                        ( model, Cmd.none )

            ResourcesResponse response ->
                case ( response, model.mPipelines ) of
                    ( RemoteData.Success resources, RemoteData.Success pipelines ) ->
                        ( { model | pipelineResourceErrors = resourceErrorsByPipelineIdentifier resources }, Cmd.none )

                    _ ->
                        ( model, Cmd.none )

            VersionFetched (Ok version) ->
                ( { model | concourseVersion = version }, Cmd.none )

            VersionFetched (Err err) ->
                ( { model | concourseVersion = "" }, Cmd.none )

            ClockTick now ->
                if model.hideFooterCounter + Time.second > 5 * Time.second then
                    ( { model | now = Just now, hideFooter = True }, Cmd.none )
                else
                    ( { model | now = Just now, hideFooterCounter = model.hideFooterCounter + Time.second }, Cmd.none )

            AutoRefresh _ ->
                ( model
                , reload
                )

            ShowFooter ->
                ( { model | hideFooter = False, hideFooterCounter = 0 }, Cmd.none )

            TopBarMsg msg ->
                let
                    ( newTopBar, newTopBarMsg ) =
                        NewTopBar.update msg model.topBar

                    newMsg =
                        case msg of
                            NewTopBar.LoggedOut (Ok _) ->
                                reload

                            _ ->
                                Cmd.map TopBarMsg newTopBarMsg
                in
                    ( { model | topBar = newTopBar }, newMsg )

            Tooltip pipelineName teamName ->
                ( model, tooltipHd ( pipelineName, teamName ) )


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ Time.every Time.second ClockTick
        , Time.every (5 * Time.second) AutoRefresh
        , Mouse.moves (\_ -> ShowFooter)
        , Mouse.clicks (\_ -> ShowFooter)
        ]


view : Model -> Html Msg
view model =
    Html.div [ class "page" ]
        [ Html.map TopBarMsg (NewTopBar.view model.topBar)
        , dashboardView model
        ]


dashboardView : Model -> Html Msg
dashboardView model =
    case model.mPipelines of
        RemoteData.Success [] ->
            Html.map (\_ -> Noop) NoPipeline.view

        RemoteData.Success _ ->
            pipelinesView model model.pipelines

        RemoteData.Failure _ ->
            turbulenceView model

        _ ->
            Html.text ""


pipelinesView : Model -> List Concourse.Pipeline -> Html Msg
pipelinesView model pipelines =
    let
        pipelinesByTeam =
            List.foldl
                (\pipelineWithJobs byTeam ->
                    groupPipelines byTeam ( pipelineWithJobs.pipeline.teamName, pipelineWithJobs )
                )
                []
                (pipelinesWithJobs model.pipelineJobs model.pipelineResourceErrors pipelines)

        sortedPipelinesByTeam =
            case model.topBar.userState of
                UserState.UserStateLoggedIn _ ->
                    case pipelinesByTeam of
                        [] ->
                            []

                        p :: ps ->
                            p :: (List.reverse <| List.sortBy (List.length << Tuple.second) ps)

                _ ->
                    List.reverse <| List.sortBy (List.length << Tuple.second) pipelinesByTeam

        teamsWithPipelines =
            List.map (Tuple.first) pipelinesByTeam

        emptyTeams =
            case model.topBar.teams of
                RemoteData.Success teams ->
                    Set.toList <| Set.diff (Set.fromList (List.map .name teams)) (Set.fromList teamsWithPipelines)

                _ ->
                    []

        pipelinesByTeamView =
            List.append
                (List.concatMap (\( teamName, pipelines ) -> groupView model.now teamName (List.reverse pipelines))
                    sortedPipelinesByTeam
                )
                (List.concatMap (\team -> groupView model.now team [])
                    emptyTeams
                )
    in
        Html.div
            [ class "dashboard dashboard-hd" ]
            [ Html.div [ class "dashboard-content" ] pipelinesByTeamView
            , footerView model
            ]


footerView : Model -> Html Msg
footerView model =
    Html.div
        [ if model.hideFooter || model.showHelp then
            class "dashboard-footer hidden"
          else
            class "dashboard-footer"
        ]
        [ Html.div [ class "dashboard-legend" ]
            [ Html.div [ class "dashboard-status-pending" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "pending" ]
            , Html.div [ class "dashboard-paused" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "paused" ]
            , Html.div [ class "dashboard-running" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "running" ]
            , Html.div [ class "dashboard-status-failed" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "failing" ]
            , Html.div [ class "dashboard-status-errored" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "errored" ]
            , Html.div [ class "dashboard-status-aborted" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "aborted" ]
            , Html.div [ class "dashboard-status-succeeded" ]
                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "succeeded" ]
            , Html.div [ class "dashboard-status-separator" ] [ Html.text "|" ]
            , Html.div [ class "dashboard-high-density" ]
                [ Html.a [ class "toggle-high-density", href Routes.dashboardRoute, ariaLabel "Toggle high-density view" ]
                    [ Html.div [ class "dashboard-pipeline-icon hd-on" ] [], Html.text "high-density" ]
                ]
            ]
        , Html.div [ class "concourse-info" ]
            [ Html.div [ class "concourse-version" ]
                [ Html.text "version: v", Html.text model.concourseVersion ]
            , Html.div [ class "concourse-cli" ]
                [ Html.text "cli: "
                , Html.a [ href (Concourse.Cli.downloadUrl "amd64" "darwin"), ariaLabel "Download OS X CLI" ]
                    [ Html.i [ class "fa fa-apple" ] [] ]
                , Html.a [ href (Concourse.Cli.downloadUrl "amd64" "windows"), ariaLabel "Download Windows CLI" ]
                    [ Html.i [ class "fa fa-windows" ] [] ]
                , Html.a [ href (Concourse.Cli.downloadUrl "amd64" "linux"), ariaLabel "Download Linux CLI" ]
                    [ Html.i [ class "fa fa-linux" ] [] ]
                ]
            ]
        ]


turbulenceView : Model -> Html Msg
turbulenceView model =
    Html.div
        [ class "error-message" ]
        [ Html.div [ class "message" ]
            [ Html.img [ src model.turbulenceImgSrc, class "seatbelt" ] []
            , Html.p [] [ Html.text "experiencing turbulence" ]
            , Html.p [ class "explanation" ] []
            ]
        ]


groupView : Maybe Time -> String -> List Pipeline.PipelineWithJobs -> List (Html Msg)
groupView now teamName pipelines =
    let
        teamPipelines =
            if List.isEmpty pipelines then
                [ pipelineNotSetView ]
            else
                List.map (pipelineView now) pipelines
    in
        case teamPipelines of
            [] ->
                [ Html.div [ class "dashboard-team-name" ] [ Html.text teamName ] ]

            p :: ps ->
                -- Wrap the team name and the first pipeline together so the team name is not the last element in a column
                List.append [ Html.div [ class "dashboard-team-name-wrapper" ] [ Html.div [ class "dashboard-team-name" ] [ Html.text teamName ], p ] ] ps


pipelineView : Maybe Time -> Pipeline.PipelineWithJobs -> Html Msg
pipelineView now { pipeline, jobs, resourceError } =
    Html.div
        [ classList
            [ ( "dashboard-pipeline", True )
            , ( "dashboard-paused", pipeline.paused )
            , ( "dashboard-running", List.any (\job -> job.nextBuild /= Nothing) jobs )
            , ( "dashboard-status-" ++ Concourse.PipelineStatus.show (Pipeline.pipelineStatusFromJobs jobs False), not pipeline.paused )
            ]
        , attribute "data-pipeline-name" pipeline.name
        , attribute "data-team-name" pipeline.teamName
        ]
        [ Html.div [ class "dashboard-pipeline-banner" ] []
        , Html.div
            [ class "dashboard-pipeline-content"
            , onMouseEnter <| Tooltip pipeline.name pipeline.teamName
            ]
            [ Html.a [ href <| Routes.pipelineRoute pipeline ]
                [ Html.div
                    [ class "dashboardhd-pipeline-name"
                    , attribute "data-team-name" pipeline.teamName
                    ]
                    [ Html.text pipeline.name ]
                ]
            ]
        , Html.div [ classList [ ( "dashboard-resource-error", resourceError ) ] ] []
        ]


pipelineNotSetView : Html msg
pipelineNotSetView =
    Html.div
        [ class "dashboard-pipeline" ]
        [ Html.div
            [ classList
                [ ( "dashboard-pipeline-content", True )
                , ( "no-set", True )
                ]
            ]
            [ Html.a [] [ Html.text "no pipelines set" ]
            ]
        ]


fetchPipelines : Cmd Msg
fetchPipelines =
    Cmd.map PipelinesResponse <|
        RemoteData.asCmd Concourse.Pipeline.fetchPipelines


fetchAllJobs : Cmd Msg
fetchAllJobs =
    Cmd.map JobsResponse <|
        RemoteData.asCmd <|
            (Concourse.Job.fetchAllJobs |> Task.map (Maybe.withDefault []))


fetchAllResources : Cmd Msg
fetchAllResources =
    Cmd.map ResourcesResponse <|
        RemoteData.asCmd (Concourse.Resource.fetchAllResources |> Task.map (Maybe.withDefault []))


fetchVersion : Cmd Msg
fetchVersion =
    Concourse.Info.fetch
        |> Task.map (.version)
        |> Task.attempt VersionFetched
