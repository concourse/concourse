port module Dashboard exposing (Model, Msg, init, update, subscriptions, view)

import BuildDuration
import Concourse
import Concourse.BuildStatus
import Concourse.Cli
import Concourse.Info
import Concourse.Job
import Concourse.Pipeline
import DashboardPreview
import Date exposing (Date)
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class, classList, id, href, src)
import Html.Attributes.Aria exposing (ariaLabel)
import Http
import Keyboard
import Mouse
import NewTopBar
import RemoteData
import Task exposing (Task)
import Time exposing (Time)


type alias Model =
    { topBar : NewTopBar.Model
    , pipelines : RemoteData.WebData (List Concourse.Pipeline)
    , jobs : Dict Int (RemoteData.WebData (List Concourse.Job))
    , concourseVersion : String
    , turbulenceImgSrc : String
    , now : Maybe Time
    , hideFooter : Bool
    , hideFooterCounter : Time
    }


type Msg
    = PipelinesResponse (RemoteData.WebData (List Concourse.Pipeline))
    | JobsResponse Int (RemoteData.WebData (List Concourse.Job))
    | ClockTick Time.Time
    | VersionFetched (Result Http.Error String)
    | AutoRefresh Time
    | ShowFooter
    | TopBarMsg NewTopBar.Msg


type alias PipelineState =
    { pipeline : Concourse.Pipeline
    , jobs : RemoteData.WebData (List Concourse.Job)
    }


init : String -> ( Model, Cmd Msg )
init turbulencePath =
    let
        ( topBar, topBarMsg ) =
            NewTopBar.init
    in
        ( { topBar = topBar
          , pipelines = RemoteData.NotAsked
          , jobs = Dict.empty
          , now = Nothing
          , turbulenceImgSrc = turbulencePath
          , concourseVersion = ""
          , hideFooter = False
          , hideFooterCounter = 0
          }
        , Cmd.batch
            [ fetchPipelines
            , fetchVersion
            , getCurrentTime
            , Cmd.map TopBarMsg topBarMsg
            ]
        )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        PipelinesResponse response ->
            ( { model | pipelines = response }
            , case response of
                RemoteData.Success pipelines ->
                    Cmd.batch (List.map fetchJobs pipelines)

                _ ->
                    Cmd.none
            )

        JobsResponse pipelineId response ->
            ( { model | jobs = Dict.insert pipelineId response model.jobs }, Cmd.none )

        VersionFetched (Ok version) ->
            ( { model | concourseVersion = version }, Cmd.none )

        VersionFetched (Err err) ->
            flip always (Debug.log ("failed to fetch version") (err)) <|
                ( { model | concourseVersion = "" }, Cmd.none )

        ClockTick now ->
            if model.hideFooterCounter + Time.second > 5 * Time.second then
                ( { model | now = Just now, hideFooter = True }, Cmd.none )
            else
                ( { model | now = Just now, hideFooterCounter = model.hideFooterCounter + Time.second }, Cmd.none )

        AutoRefresh _ ->
            ( model, Cmd.batch [ fetchPipelines, fetchVersion ] )

        ShowFooter ->
            ( { model | hideFooter = False, hideFooterCounter = 0 }, Cmd.none )

        TopBarMsg msg ->
            let
                ( newTopBar, newTopBarMsg ) =
                    NewTopBar.update msg model.topBar
            in
                ( { model | topBar = newTopBar }, Cmd.map TopBarMsg newTopBarMsg )


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ Time.every Time.second ClockTick
        , Time.every (5 * Time.second) AutoRefresh
        , Mouse.moves (\_ -> ShowFooter)
        , Mouse.clicks (\_ -> ShowFooter)
        , Keyboard.presses (\_ -> ShowFooter)
        ]


view : Model -> Html Msg
view model =
    Html.div [ class "page" ]
        [ Html.map TopBarMsg (NewTopBar.view model.topBar)
        , viewDashboard model
        ]


viewDashboard : Model -> Html Msg
viewDashboard model =
    case model.pipelines of
        RemoteData.Success pipelines ->
            let
                pipelineStates =
                    List.filter ((/=) RemoteData.NotAsked << .jobs) <|
                        List.map
                            (\pipeline ->
                                { pipeline = pipeline
                                , jobs =
                                    Maybe.withDefault RemoteData.NotAsked <|
                                        Dict.get pipeline.id model.jobs
                                }
                            )
                            pipelines

                pipelinesByTeam =
                    List.foldl
                        (\pipelineState byTeam ->
                            addPipelineState byTeam ( pipelineState.pipeline.teamName, pipelineState )
                        )
                        []
                        pipelineStates
            in
                Html.div [ class "dashboard" ] <|
                    [ Html.div [ class "dashboard-content" ] <|
                        List.map (\( teamName, pipelineStates ) -> viewGroup model.now teamName (List.reverse pipelineStates)) pipelinesByTeam
                    , Html.div
                        [ if model.hideFooter then
                            class "dashboard-footer hidden"
                          else
                            class "dashboard-footer"
                        ]
                        [ Html.div [ class "dashboard-legend" ]
                            [ Html.div [ class "dashboard-status-failed" ]
                                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "failing" ]
                            , Html.div [ class "dashboard-status-succeeded" ]
                                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "succeeded" ]
                            , Html.div [ class "dashboard-paused" ]
                                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "paused" ]
                            , Html.div [ class "dashboard-status-errored" ]
                                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "errored" ]
                            , Html.div [ class "dashboard-status-aborted" ]
                                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "aborted" ]
                            , Html.div [ class "dashboard-status-pending" ]
                                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "pending" ]
                            , Html.div [ class "dashboard-running" ]
                                [ Html.div [ class "dashboard-pipeline-icon" ] [], Html.text "running" ]
                            ]
                        , Html.div [ class "concourse-version" ]
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

        RemoteData.Failure _ ->
            Html.div
                [ class "error-message" ]
                [ Html.div [ class "message" ]
                    [ Html.img [ src model.turbulenceImgSrc, class "seatbelt" ] []
                    , Html.p [] [ Html.text "experiencing turbulence" ]
                    , Html.p [ class "explanation" ] []
                    ]
                ]

        _ ->
            Html.text ""


addPipelineState : List ( String, List PipelineState ) -> ( String, PipelineState ) -> List ( String, List PipelineState )
addPipelineState pipelineStates ( teamName, pipelineState ) =
    case pipelineStates of
        [] ->
            [ ( teamName, [ pipelineState ] ) ]

        s :: ss ->
            if Tuple.first s == teamName then
                ( teamName, pipelineState :: (Tuple.second s) ) :: ss
            else
                s :: (addPipelineState ss ( teamName, pipelineState ))


viewGroup : Maybe Time -> String -> List PipelineState -> Html msg
viewGroup now teamName pipelines =
    Html.div [ id teamName, class "dashboard-team-group" ]
        [ Html.div [ class "dashboard-team-name" ]
            [ Html.text teamName ]
        , Html.div [ class "dashboard-team-pipelines" ]
            (List.map (viewPipeline now) pipelines)
        ]


viewPipeline : Maybe Time -> PipelineState -> Html msg
viewPipeline now state =
    let
        status =
            pipelineStatus state

        mJobs =
            case state.jobs of
                RemoteData.Success js ->
                    Just js

                _ ->
                    Nothing

        mpreview =
            Maybe.map DashboardPreview.view mJobs
    in
        Html.div
            [ classList
                [ ( "dashboard-pipeline", True )
                , ( "dashboard-paused", state.pipeline.paused )
                , ( "dashboard-running", isPipelineRunning state )
                , ( "dashboard-status-" ++ Concourse.BuildStatus.show status, not state.pipeline.paused )
                ]
            ]
            [ Html.div [ class "dashboard-pipeline-banner" ] []
            , Html.div
                [ class "dashboard-pipeline-content" ]
                [ Html.a [ href state.pipeline.url ]
                    [ Html.div
                        [ class "dashboard-pipeline-header" ]
                        [ Html.div [ class "dashboard-pipeline-name" ]
                            [ Html.text state.pipeline.name ]
                        ]
                    ]
                , case mpreview of
                    Just preview ->
                        preview

                    Nothing ->
                        Html.text ""
                , Html.div [ class "dashboard-pipeline-footer" ]
                    [ Html.div [ class "dashboard-pipeline-icon" ]
                        []
                    , timeSincePipelineTransitioned status now state
                    ]
                ]
            ]


timeSincePipelineTransitioned : Concourse.BuildStatus -> Maybe Time -> PipelineState -> Html a
timeSincePipelineTransitioned status time state =
    case state.jobs of
        RemoteData.Success js ->
            let
                transitionedJobs =
                    List.filter ((==) status << jobStatus) <| js

                transitionedDurations =
                    List.map
                        (\job ->
                            Maybe.withDefault { startedAt = Nothing, finishedAt = Nothing } <|
                                Maybe.map .duration job.transitionBuild
                        )
                        transitionedJobs

                sortedTransitionedDurations =
                    List.sortBy
                        (\duration ->
                            case duration.startedAt of
                                Just date ->
                                    Time.inSeconds <| Date.toTime date

                                Nothing ->
                                    0
                        )
                        transitionedDurations

                transitionedDuration =
                    if status == Concourse.BuildStatusSucceeded then
                        List.head << List.reverse <| sortedTransitionedDurations
                    else
                        List.head <| sortedTransitionedDurations
            in
                if state.pipeline.paused then
                    Html.div [ class "build-duration" ] [ Html.text "paused" ]
                else if status == Concourse.BuildStatusPending then
                    Html.div [ class "build-duration" ] [ Html.text "pending" ]
                else
                    case ( time, transitionedDuration ) of
                        ( Just now, Just duration ) ->
                            BuildDuration.show duration now

                        _ ->
                            Html.text ""

        _ ->
            Html.text ""


isPipelineRunning : PipelineState -> Bool
isPipelineRunning { jobs } =
    case jobs of
        RemoteData.Success js ->
            List.any (\job -> job.nextBuild /= Nothing) js

        _ ->
            False


pipelineStatus : PipelineState -> Concourse.BuildStatus
pipelineStatus { jobs } =
    case jobs of
        RemoteData.Success js ->
            jobsStatus js

        _ ->
            Concourse.BuildStatusPending


jobStatus : Concourse.Job -> Concourse.BuildStatus
jobStatus job =
    Maybe.withDefault Concourse.BuildStatusPending <| Maybe.map .status job.finishedBuild


jobsStatus : List Concourse.Job -> Concourse.BuildStatus
jobsStatus jobs =
    let
        isHanging =
            List.any (\job -> (Just Concourse.BuildStatusPending) == (Maybe.map .status job.nextBuild)) jobs

        statuses =
            List.map (\job -> Maybe.withDefault Concourse.BuildStatusPending <| Maybe.map .status job.finishedBuild) jobs
    in
        if isHanging then
            Concourse.BuildStatusPending
        else if List.member Concourse.BuildStatusFailed statuses then
            Concourse.BuildStatusFailed
        else if List.member Concourse.BuildStatusErrored statuses then
            Concourse.BuildStatusErrored
        else if List.member Concourse.BuildStatusAborted statuses then
            Concourse.BuildStatusAborted
        else if List.member Concourse.BuildStatusSucceeded statuses then
            Concourse.BuildStatusSucceeded
        else
            Concourse.BuildStatusPending


fetchPipelines : Cmd Msg
fetchPipelines =
    Cmd.map PipelinesResponse <|
        RemoteData.asCmd Concourse.Pipeline.fetchPipelines


fetchJobs : Concourse.Pipeline -> Cmd Msg
fetchJobs pipeline =
    Cmd.map (JobsResponse pipeline.id) <|
        RemoteData.asCmd <|
            Concourse.Job.fetchJobsWithTransitionBuilds
                { teamName = pipeline.teamName
                , pipelineName = pipeline.name
                }


fetchVersion : Cmd Msg
fetchVersion =
    Concourse.Info.fetch
        |> Task.map (.version)
        |> Task.attempt VersionFetched


getCurrentTime : Cmd Msg
getCurrentTime =
    Task.perform ClockTick Time.now
