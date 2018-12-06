module Dashboard.Pipeline
    exposing
        ( PipelineWithJobs
        , pipelineNotSetView
        , pipelineView
        , hdPipelineView
        )

import Concourse
import Concourse.PipelineStatus as PipelineStatus
import Duration
import Dashboard.Msgs exposing (Msg(..))
import Dashboard.Styles as Styles
import DashboardPreview
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (on, onMouseEnter, onMouseLeave)
import Routes
import StrictEvents exposing (onLeftClick)
import Time exposing (Time)


type alias PipelineWithJobs =
    { pipeline : Concourse.Pipeline
    , jobs : List Concourse.Job
    , resourceError : Bool
    , status : PipelineStatus.PipelineStatus
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


hdPipelineView :
    { pipeline : Concourse.Pipeline
    , jobs : List Concourse.Job
    , resourceError : Bool
    , status : PipelineStatus.PipelineStatus
    , pipelineRunningKeyframes : String
    }
    -> Html Msg
hdPipelineView { pipeline, jobs, resourceError, status, pipelineRunningKeyframes } =
    Html.div
        [ class "dashboard-pipeline"
        , attribute "data-pipeline-name" pipeline.name
        , attribute "data-team-name" pipeline.teamName
        , style Styles.pipelineCardHd
        ]
        [ Html.div
            [ class "dashboard-pipeline-banner"
            , style <|
                Styles.pipelineCardBannerHd
                    { status = status
                    , pipelineRunningKeyframes = pipelineRunningKeyframes
                    }
            ]
            []
        , Html.div
            [ class "dashboard-pipeline-content"
            , style <| Styles.pipelineCardBodyHd status
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
            , style Styles.pipelineCardFooter
            ]
            [ Html.div
                [ style [ ( "display", "flex" ) ]
                ]
                [ Html.div
                    [ style <|
                        Styles.pipelineStatusIcon pipelineWithJobs.status
                    ]
                    []
                , transitionView now pipelineWithJobs
                ]
            , Html.div
                [ style [ ( "display", "flex" ) ]
                ]
              <|
                List.intersperse spacer
                    [ pauseToggleView pipelineWithJobs hovered
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


sinceTransitionText : PipelineStatus.StatusDetails -> Time -> String
sinceTransitionText details now =
    case details of
        PipelineStatus.Running ->
            "running"

        PipelineStatus.Since time ->
            Duration.format <| Duration.between time now


statusAgeText : PipelineWithJobs -> Time -> String
statusAgeText pipeline now =
    case pipeline.status of
        PipelineStatus.PipelineStatusPaused ->
            "paused"

        PipelineStatus.PipelineStatusPending False ->
            "pending"

        PipelineStatus.PipelineStatusPending True ->
            "running"

        PipelineStatus.PipelineStatusAborted details ->
            sinceTransitionText details now

        PipelineStatus.PipelineStatusErrored details ->
            sinceTransitionText details now

        PipelineStatus.PipelineStatusFailed details ->
            sinceTransitionText details now

        PipelineStatus.PipelineStatusSucceeded details ->
            sinceTransitionText details now


transitionView : Time -> PipelineWithJobs -> Html a
transitionView time pipeline =
    Html.div
        [ class "build-duration"
        , style <| Styles.pipelineCardTransitionAge pipeline.status
        ]
        [ Html.text <| statusAgeText pipeline time ]


pauseToggleView : PipelineWithJobs -> Bool -> Html Msg
pauseToggleView pipeline hovered =
    Html.a
        [ style
            [ ( "background-image"
              , case pipeline.status of
                    PipelineStatus.PipelineStatusPaused ->
                        "url(public/images/ic_play_white.svg)"

                    _ ->
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
        , onLeftClick <| TogglePipelinePaused pipeline.pipeline
        , onMouseEnter <| PipelineButtonHover <| Just pipeline.pipeline
        , onMouseLeave <| PipelineButtonHover Nothing
        ]
        []
