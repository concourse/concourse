module Dashboard.Pipeline
    exposing
        ( PipelineWithJobs
        , SummaryPipeline
        , PreviewPipeline
        , pipelineNotSetView
        , pipelineView
        , hdPipelineView
        , pipelineStatus
        , pipelineStatusFromJobs
        )

import Colors
import Concourse
import Concourse.PipelineStatus
import Duration
import Dashboard.Msgs exposing (Msg(..))
import DashboardPreview
import Date
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (on, onMouseEnter, onMouseLeave)
import List.Extra
import Maybe.Extra
import Routes
import StrictEvents exposing (onLeftClick)
import Time exposing (Time)


type SummaryPipeline
    = SummaryPipeline PipelineWithJobs


type PreviewPipeline
    = PreviewPipeline PipelineWithJobs


type alias PipelineWithJobs =
    { pipeline : Concourse.Pipeline
    , jobs : List Concourse.Job
    , resourceError : Bool
    }


pipelineNotSetView : Html msg
pipelineNotSetView =
    Html.div [ class "pipeline-wrapper" ]
        [ Html.div
            [ class "dashboard-pipeline no-set"
            ]
            [ Html.div
                [ class "dashboard-pipeline-content" ]
                [ Html.div [ class "no-set-wrapper" ]
                    [ Html.text "no pipelines set" ]
                ]
            ]
        ]


viewSummary : SummaryPipeline -> Html Msg
viewSummary (SummaryPipeline pwj) =
    hdPipelineView pwj


hdPipelineView : PipelineWithJobs -> Html Msg
hdPipelineView { pipeline, jobs, resourceError } =
    Html.div
        [ classList
            [ ( "dashboard-pipeline", True )
            , ( "dashboard-paused", pipeline.paused )
            , ( "dashboard-running", List.any (\job -> job.nextBuild /= Nothing) jobs )
            , ( "dashboard-status-" ++ Concourse.PipelineStatus.show (pipelineStatusFromJobs jobs False), not pipeline.paused )
            ]
        , attribute "data-pipeline-name" pipeline.name
        , attribute "data-team-name" pipeline.teamName
        ]
        [ Html.div [ class "dashboard-pipeline-banner" ] []
        , Html.div
            [ class "dashboard-pipeline-content"
            , onMouseEnter <| TooltipHd pipeline.name pipeline.teamName
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


pipelineView : { now : Time, pipelineWithJobs : PipelineWithJobs, hovered : Bool } -> Html Msg
pipelineView { now, pipelineWithJobs, hovered } =
    Html.div [ class "dashboard-pipeline-content" ]
        [ headerView pipelineWithJobs
        , DashboardPreview.view pipelineWithJobs.jobs
        , footerView pipelineWithJobs now hovered
        ]


headerView : PipelineWithJobs -> Html Msg
headerView ({ pipeline, resourceError } as pipelineWithJobs) =
    Html.a [ href <| Routes.pipelineRoute pipeline, draggable "false" ]
        [ Html.div
            [ class "dashboard-pipeline-header"
            , onMouseEnter <| Tooltip pipeline.name pipeline.teamName
            ]
            [ Html.div [ class "dashboard-pipeline-name" ]
                [ Html.text pipeline.name ]
            , Html.div [ classList [ ( "dashboard-resource-error", resourceError ) ] ] []
            ]
        ]


footerView : PipelineWithJobs -> Time -> Bool -> Html Msg
footerView pipelineWithJobs now hovered =
    let
        spacer =
            Html.div [ style [ ( "width", "13.5px" ) ] ] []
    in
        Html.div
            [ class "dashboard-pipeline-footer"
            , style
                [ ( "border-top", "2px solid " ++ Colors.dashboardBackground )
                , ( "padding", "13.5px" )
                , ( "display", "flex" )
                , ( "justify-content", "space-between" )
                ]
            ]
            [ Html.div
                [ style [ ( "display", "flex" ) ]
                ]
                [ Html.div [ class "dashboard-pipeline-icon" ] []
                , transitionView now pipelineWithJobs
                ]
            , Html.div
                [ style [ ( "display", "flex" ) ]
                ]
              <|
                List.intersperse spacer
                    [ pauseToggleView pipelineWithJobs.pipeline hovered
                    , visibilityView pipelineWithJobs.pipeline.public
                    ]
            ]


visibilityView : Bool -> Html Msg
visibilityView public =
    Html.div
        [ style
            [ ( "background-image"
              , if public then
                    "url(public/images/baseline-visibility-24px.svg)"
                else
                    "url(public/images/baseline-visibility_off-24px.svg)"
              )
            , ( "background-position", "50% 50%" )
            , ( "background-repeat", "no-repeat" )
            , ( "background-size", "contain" )
            , ( "width", "20px" )
            , ( "height", "20px" )
            ]
        ]
        []


type alias Event =
    { succeeded : Bool
    , time : Time
    }


transitionTime : PipelineWithJobs -> Maybe Time
transitionTime pipeline =
    let
        events =
            pipeline.jobs |> List.filterMap jobEvent |> List.sortBy .time
    in
        events
            |> List.Extra.dropWhile .succeeded
            |> List.head
            |> Maybe.map Just
            |> Maybe.withDefault (List.Extra.last events)
            |> Maybe.map .time


jobEvent : Concourse.Job -> Maybe Event
jobEvent job =
    Maybe.map
        (Event <| jobSucceeded job)
        (transitionStart job)


equalBy : (a -> b) -> a -> a -> Bool
equalBy f x y =
    f x == f y


jobSucceeded : Concourse.Job -> Bool
jobSucceeded =
    .finishedBuild
        >> Maybe.map (.status >> (==) Concourse.BuildStatusSucceeded)
        >> Maybe.withDefault False


transitionStart : Concourse.Job -> Maybe Time
transitionStart =
    .transitionBuild
        >> Maybe.map (.duration >> .startedAt)
        >> Maybe.Extra.join
        >> Maybe.map Date.toTime


sinceTransitionText : PipelineWithJobs -> Time -> String
sinceTransitionText pipeline now =
    Maybe.map (flip Duration.between now) (transitionTime pipeline)
        |> Maybe.map Duration.format
        |> Maybe.withDefault ""


statusAgeText : PipelineWithJobs -> Time -> String
statusAgeText pipeline =
    case pipelineStatus pipeline of
        Concourse.PipelineStatusPaused ->
            always "paused"

        Concourse.PipelineStatusPending ->
            always "pending"

        Concourse.PipelineStatusRunning ->
            always "running"

        _ ->
            sinceTransitionText pipeline


transitionView : Time -> PipelineWithJobs -> Html a
transitionView time pipeline =
    Html.div [ class "build-duration" ]
        [ Html.text <| statusAgeText pipeline time ]


pipelineStatus : PipelineWithJobs -> Concourse.PipelineStatus
pipelineStatus { pipeline, jobs } =
    if pipeline.paused then
        Concourse.PipelineStatusPaused
    else
        pipelineStatusFromJobs jobs True


pipelineStatusFromJobs : List Concourse.Job -> Bool -> Concourse.PipelineStatus
pipelineStatusFromJobs jobs includeNextBuilds =
    let
        statuses =
            jobStatuses jobs
    in
        if containsStatus Concourse.BuildStatusPending statuses then
            Concourse.PipelineStatusPending
        else if includeNextBuilds && List.any (\job -> job.nextBuild /= Nothing) jobs then
            Concourse.PipelineStatusRunning
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
containsStatus =
    List.member << Just


pauseToggleView : Concourse.Pipeline -> Bool -> Html Msg
pauseToggleView pipeline hovered =
    Html.a
        [ style
            [ ( "background-image"
              , if pipeline.paused then
                    "url(public/images/ic_play_white.svg)"
                else
                    "url(public/images/ic_pause_white.svg)"
              )
            , ( "background-position", "50% 50%" )
            , ( "background-repeat", "no-repeat" )
            , ( "width", "20px" )
            , ( "height", "20px" )
            , ( "cursor", "pointer" )
            , ( "opacity"
              , if hovered then
                    "1"
                else
                    "0.5"
              )
            ]
        , onLeftClick <| TogglePipelinePaused pipeline
        , onMouseEnter <| PipelineButtonHover <| Just pipeline
        , onMouseLeave <| PipelineButtonHover Nothing
        ]
        []
