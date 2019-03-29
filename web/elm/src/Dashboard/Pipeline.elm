module Dashboard.Pipeline exposing
    ( hdPipelineView
    , pipelineNotSetView
    , pipelineView
    )

import Concourse.PipelineStatus as PipelineStatus
import Dashboard.DashboardPreview as DashboardPreview
import Dashboard.Group.Models exposing (Pipeline)
import Dashboard.Styles as Styles
import Duration
import Html exposing (..)
import Html.Attributes exposing (..)
import Html.Events exposing (on, onMouseEnter, onMouseLeave)
import Message.Message exposing (Hoverable(..), Message(..))
import Routes
import Time
import UserState exposing (UserState)
import Views.Icon as Icon
import Views.PauseToggle as PauseToggle


pipelineNotSetView : Html Message
pipelineNotSetView =
    Html.div [ class "card" ]
        [ Html.div
            ([ class "card-header" ] ++ Styles.noPipelineCardHeader)
            [ Html.text "no pipeline set"
            ]
        , Html.div
            ([ class "card-body" ] ++ Styles.cardBody)
            [ Html.div Styles.previewPlaceholder [] ]
        , Html.div
            ([ class "card-footer" ] ++ Styles.cardFooter)
            []
        ]


hdPipelineView :
    { pipeline : Pipeline
    , pipelineRunningKeyframes : String
    }
    -> Html Message
hdPipelineView { pipeline, pipelineRunningKeyframes } =
    Html.a
        ([ class "card"
         , attribute "data-pipeline-name" pipeline.name
         , attribute "data-team-name" pipeline.teamName
         , onMouseEnter <| TooltipHd pipeline.name pipeline.teamName
         , href <| Routes.toString <| Routes.pipelineRoute pipeline
         ]
            ++ Styles.pipelineCardHd pipeline.status
        )
    <|
        [ Html.div
            (Styles.pipelineCardBannerHd
                { status = pipeline.status
                , pipelineRunningKeyframes = pipelineRunningKeyframes
                }
            )
            []
        , Html.div
            ([ class "dashboardhd-pipeline-name" ] ++ Styles.pipelineCardBodyHd)
            [ Html.text pipeline.name ]
        ]
            ++ (if pipeline.resourceError then
                    [ Html.div Styles.resourceErrorTriangle [] ]

                else
                    []
               )


pipelineView :
    { now : Time.Posix
    , pipeline : Pipeline
    , hovered : Bool
    , pipelineRunningKeyframes : String
    , userState : UserState
    }
    -> Html Message
pipelineView { now, pipeline, hovered, pipelineRunningKeyframes, userState } =
    Html.div
        Styles.pipelineCard
        [ Html.div
            ([ class "banner" ]
                ++ Styles.pipelineCardBanner
                    { status = pipeline.status
                    , pipelineRunningKeyframes = pipelineRunningKeyframes
                    }
            )
            []
        , headerView pipeline
        , bodyView pipeline
        , footerView userState pipeline now hovered
        ]


headerView : Pipeline -> Html Message
headerView pipeline =
    Html.a
        [ href <| Routes.toString <| Routes.pipelineRoute pipeline, draggable "false" ]
        [ Html.div
            ([ class "card-header"
             , onMouseEnter <| Tooltip pipeline.name pipeline.teamName
             ]
                ++ Styles.pipelineCardHeader
            )
            [ Html.div
                ([ class "dashboard-pipeline-name" ]
                    ++ Styles.pipelineName
                )
                [ Html.text pipeline.name ]
            , Html.div
                [ classList
                    [ ( "dashboard-resource-error", pipeline.resourceError )
                    ]
                ]
                []
            ]
        ]


bodyView : Pipeline -> Html Message
bodyView pipeline =
    Html.div
        ([ class "card-body" ] ++ Styles.pipelineCardBody)
        [ DashboardPreview.view pipeline.jobs ]


footerView : UserState -> Pipeline -> Time.Posix -> Bool -> Html Message
footerView userState pipeline now hovered =
    let
        spacer =
            Html.div [ style "width" "13.5px" ] []
    in
    Html.div
        ([ class "card-footer" ]
            ++ Styles.pipelineCardFooter
        )
        [ Html.div
            [ style "display" "flex" ]
            [ PipelineStatus.icon pipeline.status
            , transitionView now pipeline
            ]
        , Html.div
            [ style "display" "flex" ]
          <|
            List.intersperse spacer
                [ PauseToggle.view "0"
                    userState
                    { isPaused =
                        pipeline.status == PipelineStatus.PipelineStatusPaused
                    , pipeline =
                        { pipelineName = pipeline.name
                        , teamName = pipeline.teamName
                        }
                    , isToggleHovered = hovered
                    , isToggleLoading = pipeline.isToggleLoading
                    }
                , visibilityView pipeline.public
                ]
        ]


visibilityView : Bool -> Html Message
visibilityView public =
    Icon.icon
        { sizePx = 20
        , image =
            if public then
                "baseline-visibility-24px.svg"

            else
                "baseline-visibility-off-24px.svg"
        }
        [ style "background-size" "contain" ]


sinceTransitionText : PipelineStatus.StatusDetails -> Time.Posix -> String
sinceTransitionText details now =
    case details of
        PipelineStatus.Running ->
            "running"

        PipelineStatus.Since time ->
            Duration.format <| Duration.between time now


statusAgeText : Pipeline -> Time.Posix -> String
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


transitionView : Time.Posix -> Pipeline -> Html Message
transitionView time pipeline =
    Html.div
        ([ class "build-duration" ]
            ++ Styles.pipelineCardTransitionAge pipeline.status
        )
        [ Html.text <| statusAgeText pipeline time ]
