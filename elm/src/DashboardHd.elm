port module DashboardHd exposing (Model, Msg, init, update, subscriptions, view)

import Concourse
import Concourse.Cli
import Concourse.Info
import Concourse.Job
import Concourse.Pipeline
import Concourse.PipelineStatus
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class, classList, id, href, src, attribute)
import Html.Attributes.Aria exposing (ariaLabel)
import Http
import Mouse
import NewTopBar
import RemoteData
import Routes
import Task exposing (Task)
import Time exposing (Time)


type alias Model =
    { topBar : NewTopBar.Model
    , mPipelines : RemoteData.WebData (List Concourse.Pipeline)
    , pipelines : List Concourse.Pipeline
    , mJobs : RemoteData.WebData (List Concourse.Job)
    , pipelineJobs : Dict Int (List Concourse.Job)
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
    | ClockTick Time.Time
    | VersionFetched (Result Http.Error String)
    | AutoRefresh Time
    | ShowFooter
    | TopBarMsg NewTopBar.Msg


type alias PipelineId =
    Int


type alias PipelineWithJobs =
    { pipeline : Concourse.Pipeline
    , jobs : List Concourse.Job
    }


init : String -> ( Model, Cmd Msg )
init turbulencePath =
    let
        ( topBar, topBarMsg ) =
            NewTopBar.init False
    in
        ( { topBar = topBar
          , mPipelines = RemoteData.NotAsked
          , pipelines = []
          , mJobs = RemoteData.NotAsked
          , pipelineJobs = Dict.empty
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
            ]
        )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Noop ->
            ( model, Cmd.none )

        PipelinesResponse response ->
            case response of
                RemoteData.Success pipelines ->
                    ( { model | mPipelines = response, pipelines = pipelines }, Cmd.batch [ fetchAllJobs ] )

                _ ->
                    ( model, Cmd.none )

        JobsResponse response ->
            case ( response, model.mPipelines ) of
                ( RemoteData.Success jobs, RemoteData.Success pipelines ) ->
                    ( { model | mJobs = response, pipelineJobs = jobsByPipelineId pipelines jobs }, Cmd.none )

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
            ( model, Cmd.batch [ fetchPipelines, fetchVersion, Cmd.map TopBarMsg NewTopBar.fetchUser ] )

        ShowFooter ->
            ( { model | hideFooter = False, hideFooterCounter = 0 }, Cmd.none )

        TopBarMsg msg ->
            let
                ( newTopBar, newTopBarMsg ) =
                    NewTopBar.update msg model.topBar
            in
                ( { model | topBar = newTopBar }, Cmd.map TopBarMsg newTopBarMsg )


classifyJob : Concourse.Job -> Dict ( String, String ) Concourse.Pipeline -> Dict PipelineId (List Concourse.Job) -> Dict PipelineId (List Concourse.Job)
classifyJob job pipelines pipelineJobs =
    let
        pipelineIdentifier =
            ( job.teamName, job.pipelineName )

        mPipeline =
            Dict.get pipelineIdentifier pipelines
    in
        case mPipeline of
            Nothing ->
                pipelineJobs

            Just pipeline ->
                let
                    jobs =
                        Maybe.withDefault [] <| Dict.get pipeline.id pipelineJobs
                in
                    Dict.insert pipeline.id (job :: jobs) pipelineJobs


jobsByPipelineId : List Concourse.Pipeline -> List Concourse.Job -> Dict PipelineId (List Concourse.Job)
jobsByPipelineId pipelines jobs =
    let
        pipelinesByIdentifier =
            List.foldl
                (\pipeline byIdentifier -> Dict.insert ( pipeline.teamName, pipeline.name ) pipeline byIdentifier)
                Dict.empty
                pipelines
    in
        List.foldl
            (\job byPipelineId -> classifyJob job pipelinesByIdentifier byPipelineId)
            Dict.empty
            jobs


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
                (pipelinesWithJobs model.pipelineJobs pipelines)

        sortedPipelinesByTeam =
            case model.topBar.user of
                RemoteData.Success user ->
                    case pipelinesByTeam of
                        [] ->
                            []

                        p :: ps ->
                            p :: (List.reverse <| List.sortBy (List.length << Tuple.second) ps)

                _ ->
                    List.reverse <| List.sortBy (List.length << Tuple.second) pipelinesByTeam

        pipelinesByTeamView =
            List.concatMap (\( teamName, pipelines ) -> groupView model.now teamName (List.reverse pipelines)) sortedPipelinesByTeam
    in
        Html.div
            [ class "dashboard dashboard-hd" ]
        <|
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


groupView : Maybe Time -> String -> List PipelineWithJobs -> List (Html msg)
groupView now teamName pipelines =
    let
        teamPiplines =
            List.map (pipelineView now) pipelines
    in
        case teamPiplines of
            [] ->
                [ Html.div [ class "dashboard-team-name" ] [ Html.text teamName ] ]

            p :: ps ->
                -- Wrap the team name and the first pipeline together so the team name is not the last element in a column
                List.append [ Html.div [ class "dashboard-team-name-wrapper" ] [ Html.div [ class "dashboard-team-name" ] [ Html.text teamName ], p ] ] ps


pipelineView : Maybe Time -> PipelineWithJobs -> Html msg
pipelineView now { pipeline, jobs } =
    Html.div
        [ classList
            [ ( "dashboard-pipeline", True )
            , ( "dashboard-paused", pipeline.paused )
            , ( "dashboard-running", List.any (\job -> job.nextBuild /= Nothing) jobs )
            , ( "dashboard-status-" ++ Concourse.PipelineStatus.show (pipelineStatusFromJobs jobs), not pipeline.paused )
            ]
        , attribute "data-pipeline-name" pipeline.name
        ]
        [ Html.div [ class "dashboard-pipeline-banner" ] []
        , Html.div
            [ class "dashboard-pipeline-content" ]
            [ Html.a [ href <| Routes.pipelineRoute pipeline ]
                [ Html.text pipeline.name ]
            ]
        ]


pipelineStatusFromJobs : List Concourse.Job -> Concourse.PipelineStatus
pipelineStatusFromJobs jobs =
    let
        statuses =
            jobStatuses jobs
    in
        if containsStatus Concourse.BuildStatusPending statuses then
            Concourse.PipelineStatusPending
        else if containsStatus Concourse.BuildStatusFailed statuses then
            Concourse.PipelineStatusFailed
        else if containsStatus Concourse.BuildStatusErrored statuses then
            Concourse.PipelineStatusErrored
        else if containsStatus Concourse.BuildStatusAborted statuses then
            Concourse.PipelineStatusAborted
        else if containsStatus Concourse.BuildStatusSucceeded statuses then
            Concourse.PipelineStatusSucceeded
        else
            Concourse.PipelineStatusPending


jobStatuses : List Concourse.Job -> List (Maybe Concourse.BuildStatus)
jobStatuses jobs =
    List.concatMap
        (\job ->
            [ Maybe.map .status job.finishedBuild
            , Maybe.map .status job.nextBuild
            ]
        )
        jobs


containsStatus : Concourse.BuildStatus -> List (Maybe Concourse.BuildStatus) -> Bool
containsStatus status statuses =
    List.any
        (\s ->
            case s of
                Just s ->
                    status == s

                Nothing ->
                    False
        )
        statuses


fetchPipelines : Cmd Msg
fetchPipelines =
    Cmd.map PipelinesResponse <|
        RemoteData.asCmd Concourse.Pipeline.fetchPipelines


fetchAllJobs : Cmd Msg
fetchAllJobs =
    Cmd.map JobsResponse <|
        RemoteData.asCmd <|
            Concourse.Job.fetchAllJobs


fetchVersion : Cmd Msg
fetchVersion =
    Concourse.Info.fetch
        |> Task.map (.version)
        |> Task.attempt VersionFetched


pipelinesWithJobs : Dict PipelineId (List Concourse.Job) -> List Concourse.Pipeline -> List PipelineWithJobs
pipelinesWithJobs pipelineJobs pipelines =
    List.map
        (\pipeline ->
            { pipeline = pipeline
            , jobs =
                Maybe.withDefault [] <| Dict.get pipeline.id pipelineJobs
            }
        )
        pipelines


groupPipelines : List ( String, List PipelineWithJobs ) -> ( String, PipelineWithJobs ) -> List ( String, List PipelineWithJobs )
groupPipelines pipelines ( teamName, pipeline ) =
    case pipelines of
        [] ->
            [ ( teamName, [ pipeline ] ) ]

        s :: ss ->
            if Tuple.first s == teamName then
                ( teamName, pipeline :: (Tuple.second s) ) :: ss
            else
                s :: (groupPipelines ss ( teamName, pipeline ))
