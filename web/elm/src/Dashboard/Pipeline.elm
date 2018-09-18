module Dashboard.Pipeline
    exposing
        ( Msg(..)
        , DragState(..)
        , DropState(..)
        , PipelineWithJobs
        , pipelineNotSetView
        , pipelineDropAreaView
        , pipelineView
        , pipelineStatus
        , pipelineStatusFromJobs
        )

import Concourse
import Concourse.PipelineStatus
import Duration
import DashboardPreview
import Date
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (on, onMouseEnter)
import List.Extra
import Maybe.Extra
import Json.Decode
import Routes
import StrictEvents exposing (onLeftClick)
import Time exposing (Time)


type alias PipelineWithJobs =
    { pipeline : Concourse.Pipeline
    , jobs : List Concourse.Job
    , resourceError : Bool
    }


type alias PipelineIndex =
    Int


type DragState
    = NotDragging
    | Dragging Concourse.TeamName PipelineIndex


type DropState
    = NotDropping
    | Dropping PipelineIndex


type Msg
    = DragStart String Int
    | DragOver String Int
    | DragEnd
    | Tooltip String String
    | TogglePipelinePaused Concourse.Pipeline


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


pipelineDropAreaView : DragState -> DropState -> String -> Int -> Html Msg
pipelineDropAreaView dragState dropState teamName index =
    let
        ( active, over ) =
            case ( dragState, dropState ) of
                ( Dragging team dragIndex, NotDropping ) ->
                    ( team == teamName, index == dragIndex )

                ( Dragging team dragIndex, Dropping dropIndex ) ->
                    ( team == teamName, index == dropIndex )

                _ ->
                    ( False, False )
    in
        Html.div
            [ classList [ ( "drop-area", True ), ( "active", active ), ( "over", over ), ( "animation", dropState /= NotDropping ) ]
            , on "dragenter" (Json.Decode.succeed (DragOver teamName index))
            ]
            [ Html.text "" ]


pipelineView : DragState -> Time -> PipelineWithJobs -> Int -> Html Msg
pipelineView dragState now ({ pipeline, jobs, resourceError } as pipelineWithJobs) index =
    Html.div
        [ classList
            [ ( "dashboard-pipeline", True )
            , ( "dashboard-paused", pipeline.paused )
            , ( "dashboard-running", not <| List.isEmpty <| List.filterMap .nextBuild jobs )
            , ( "dashboard-status-" ++ Concourse.PipelineStatus.show (pipelineStatusFromJobs jobs False), not pipeline.paused )
            , ( "dragging", dragState == Dragging pipeline.teamName index )
            ]
        , attribute "data-pipeline-name" pipeline.name
        , attribute "ondragstart" "event.dataTransfer.setData('text/plain', '');"
        , draggable "true"
        , on "dragstart" (Json.Decode.succeed (DragStart pipeline.teamName index))
        , on "dragend" (Json.Decode.succeed DragEnd)
        ]
        [ Html.div [ class "dashboard-pipeline-banner" ] []
        , Html.div
            [ class "dashboard-pipeline-content" ]
            [ headerView pipelineWithJobs
            , DashboardPreview.view jobs
            , footerView pipelineWithJobs now
            ]
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


footerView : PipelineWithJobs -> Time -> Html Msg
footerView pipelineWithJobs now =
    Html.div [ class "dashboard-pipeline-footer" ]
        [ Html.div [ class "dashboard-pipeline-icon" ] []
        , transitionView now pipelineWithJobs
        , pauseToggleView pipelineWithJobs.pipeline
        ]


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


pauseToggleView : Concourse.Pipeline -> Html Msg
pauseToggleView pipeline =
    Html.a
        [ classList
            [ ( "pause-toggle", True )
            , ( "icon-play", pipeline.paused )
            , ( "icon-pause", not pipeline.paused )
            ]
        , onLeftClick <| TogglePipelinePaused pipeline
        ]
        []
