module Dashboard.Pipeline exposing
    ( hdPipelineView
    , pipelineNotSetView
    , pipelineView
    )

import Concourse
import Concourse.PipelineStatus as PipelineStatus
import Dashboard.DashboardPreview as DashboardPreview
import Dashboard.Group.Models exposing (Pipeline)
import Dashboard.Styles as Styles
import Duration
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, draggable, href, style)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Message exposing (DomID(..), Message(..))
import Routes
import Time
import UserState exposing (UserState)
import Views.PauseToggle as PauseToggle
import Views.Spinner as Spinner
import Views.Styles


pipelineNotSetView : Html Message
pipelineNotSetView =
    Html.div [ class "card" ]
        [ Html.div
            (class "card-header" :: Styles.noPipelineCardHeader)
            [ Html.text "no pipeline set"
            ]
        , Html.div
            (class "card-body" :: Styles.cardBody)
            [ Html.div Styles.previewPlaceholder [] ]
        , Html.div
            (class "card-footer" :: Styles.cardFooter)
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
            (class "dashboardhd-pipeline-name" :: Styles.pipelineCardBodyHd)
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
    , hovered : HoverState.HoverState
    , pipelineRunningKeyframes : String
    , userState : UserState
    }
    -> Html Message
pipelineView { now, pipeline, hovered, pipelineRunningKeyframes, userState } =
    Html.div
        Styles.pipelineCard
        [ Html.div
            (class "banner"
                :: Styles.pipelineCardBanner
                    { status = pipeline.status
                    , pipelineRunningKeyframes = pipelineRunningKeyframes
                    }
            )
            []
        , headerView pipeline
        , bodyView hovered pipeline
        , footerView userState pipeline now hovered
        ]


headerView : { a | hovered : HoverState.HoverState } -> Pipeline -> Html Message
headerView { hovered } pipeline =
    Html.a
        [ href <| Routes.toString <| Routes.pipelineRoute pipeline, draggable "false" ]
        [ Html.div
            ([ class "card-header"
             , onMouseEnter <| Tooltip pipeline.name pipeline.teamName
             , if HoverState.isHovered Message.Pipeline hovered then
                style "background-color" "1f1f1f"
                else
                style "background-color" ""
             ]
                ++ Styles.pipelineCardHeader
            )
            [ Html.div
                (class "dashboard-pipeline-name" :: Styles.pipelineName)
                [ Html.text pipeline.name ]
            , Html.div
                [ classList
                    [ ( "dashboard-resource-error", pipeline.resourceError )
                    ]
                ]
                []
            ]
        ]


bodyView : HoverState.HoverState -> Pipeline -> Html Message
bodyView hovered pipeline =
    Html.div
        (class "card-body" :: Styles.pipelineCardBody)
        [ DashboardPreview.view hovered pipeline.jobs ]


footerView :
    UserState
    -> Pipeline
    -> Time.Posix
    -> HoverState.HoverState
    -> Html Message
footerView userState pipeline now hovered =
    let
        spacer =
            Html.div [ style "width" "13.5px" ] []

        pipelineId =
            { pipelineName = pipeline.name
            , teamName = pipeline.teamName
            }
    in
    Html.div
        (class "card-footer" :: Styles.pipelineCardFooter)
        [ Html.div
            [ style "display" "flex" ]
            [ PipelineStatus.icon pipeline.status
            , transitionView now pipeline
            ]
        , Html.div
            [ style "display" "flex" ]
          <|
            List.intersperse spacer
                [ PauseToggle.view
                    { isPaused =
                        pipeline.status == PipelineStatus.PipelineStatusPaused
                    , pipeline = pipelineId
                    , isToggleHovered =
                        HoverState.isHovered (PipelineButton pipelineId) hovered
                    , isToggleLoading = pipeline.isToggleLoading
                    , tooltipPosition = Views.Styles.Above
                    , margin = "0"
                    , userState = userState
                    }
                , visibilityView
                    { public = pipeline.public
                    , pipelineId = pipelineId
                    , isClickable =
                        UserState.isAnonymous userState
                            || UserState.isMember
                                { teamName = pipeline.teamName
                                , userState = userState
                                }
                    , isHovered =
                        HoverState.isHovered (VisibilityButton pipelineId) hovered
                    , isVisibilityLoading = pipeline.isVisibilityLoading
                    }
                ]
        ]


visibilityView :
    { public : Bool
    , pipelineId : Concourse.PipelineIdentifier
    , isClickable : Bool
    , isHovered : Bool
    , isVisibilityLoading : Bool
    }
    -> Html Message
visibilityView { public, pipelineId, isClickable, isHovered, isVisibilityLoading } =
    if isVisibilityLoading then
        Spinner.hoverableSpinner
            { sizePx = 20
            , margin = "0"
            , hoverable = Just <| VisibilityButton pipelineId
            }

    else
        Html.div
            (Styles.visibilityToggle
                { public = public
                , isClickable = isClickable
                , isHovered = isHovered
                }
                ++ [ onMouseEnter <| Hover <| Just <| VisibilityButton pipelineId
                   , onMouseLeave <| Hover Nothing
                   ]
                ++ (if isClickable then
                        [ onClick <| Click <| VisibilityButton pipelineId ]

                    else
                        []
                   )
            )
            (if isClickable && isHovered then
                [ Html.div
                    Styles.visibilityTooltip
                    [ Html.text <|
                        if public then
                            "hide pipeline"

                        else
                            "expose pipeline"
                    ]
                ]

             else
                []
            )


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
        (class "build-duration"
            :: Styles.pipelineCardTransitionAge pipeline.status
        )
        [ Html.text <| statusAgeText pipeline time ]
