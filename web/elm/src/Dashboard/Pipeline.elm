module Dashboard.Pipeline
    exposing
        ( pipelineNotSetView
        , pipelineView
        , hdPipelineView
        )

import Concourse.PipelineStatus as PipelineStatus
import Duration
import Dashboard.Models exposing (Pipeline)
import Dashboard.Msgs exposing (Msg(..))
import Dashboard.Styles as Styles
import DashboardPreview
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (on, onMouseEnter, onMouseLeave)
import Routes
import StrictEvents exposing (onLeftClick)
import Time exposing (Time)


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
    { pipeline : Pipeline
    , pipelineRunningKeyframes : String
    }
    -> Html Msg
hdPipelineView { pipeline, pipelineRunningKeyframes } =
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
                    { status = pipeline.status
                    , pipelineRunningKeyframes = pipelineRunningKeyframes
                    }
            ]
            []
        , Html.div
            [ class "dashboard-pipeline-content"
            , style <| Styles.pipelineCardBodyHd pipeline.status
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
        , Html.div [ classList [ ( "dashboard-resource-error", pipeline.resourceError ) ] ] []
        ]


pipelineView : { now : Time, pipeline : Pipeline, hovered : Bool } -> Html Msg
pipelineView { now, pipeline, hovered } =
    Html.div [ class "dashboard-pipeline-content" ]
        [ headerView pipeline
        , DashboardPreview.view pipeline.jobs
        , footerView pipeline now hovered
        ]


headerView : Pipeline -> Html Msg
headerView pipeline =
    Html.a [ href <| Routes.pipelineRoute pipeline, draggable "false" ]
        [ Html.div
            [ class "dashboard-pipeline-header"
            , onMouseEnter <| Tooltip pipeline.name pipeline.teamName
            ]
            [ Html.div [ class "dashboard-pipeline-name" ]
                [ Html.text pipeline.name ]
            , Html.div [ classList [ ( "dashboard-resource-error", pipeline.resourceError ) ] ] []
            ]
        ]


footerView : Pipeline -> Time -> Bool -> Html Msg
footerView pipeline now hovered =
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
                        Styles.pipelineStatusIcon pipeline.status
                    ]
                    []
                , transitionView now pipeline
                ]
            , Html.div
                [ style [ ( "display", "flex" ) ]
                ]
              <|
                List.intersperse spacer
                    [ pauseToggleView pipeline hovered
                    , visibilityView pipeline.public
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


statusAgeText : Pipeline -> Time -> String
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


transitionView : Time -> Pipeline -> Html a
transitionView time pipeline =
    Html.div
        [ class "build-duration"
        , style <| Styles.pipelineCardTransitionAge pipeline.status
        ]
        [ Html.text <| statusAgeText pipeline time ]


pauseToggleView : Pipeline -> Bool -> Html Msg
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
        , onLeftClick <| TogglePipelinePaused pipeline
        , onMouseEnter <| PipelineButtonHover <| Just pipeline
        , onMouseLeave <| PipelineButtonHover Nothing
        ]
        []
