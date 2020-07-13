module Dashboard.Pipeline exposing
    ( favoritedView
    , hdPipelineView
    , pipelineNotSetView
    , pipelineStatus
    , pipelineView
    )

import Assets
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.PipelineStatus as PipelineStatus
import Dashboard.DashboardPreview as DashboardPreview
import Dashboard.Group.Models exposing (Pipeline)
import Dashboard.Styles as Styles
import Duration
import HoverState
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , classList
        , draggable
        , href
        , id
        , style
        )
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Message.Effects as Effects
import Message.Message exposing (DomID(..), Message(..))
import Routes
import Time
import UserState exposing (UserState)
import Views.Icon as Icon
import Views.PauseToggle as PauseToggle
import Views.Spinner as Spinner
import Views.Styles


previewPlaceholder : Html Message
previewPlaceholder =
    Html.div
        (class "card-body" :: Styles.cardBody)
        [ Html.div Styles.previewPlaceholder [] ]


pipelineNotSetView : Html Message
pipelineNotSetView =
    Html.div (class "card" :: Styles.noPipelineCard)
        [ Html.div
            (class "card-header" :: Styles.noPipelineCardHeader)
            [ Html.text "no pipeline set"
            ]
        , previewPlaceholder
        , Html.div
            (class "card-footer" :: Styles.cardFooter)
            []
        ]


hdPipelineView :
    { pipeline : Pipeline
    , pipelineRunningKeyframes : String
    , resourceError : Bool
    , existingJobs : List Concourse.Job
    }
    -> Html Message
hdPipelineView { pipeline, pipelineRunningKeyframes, resourceError, existingJobs } =
    Html.a
        ([ class "card"
         , attribute "data-pipeline-name" pipeline.name
         , attribute "data-team-name" pipeline.teamName
         , onMouseEnter <| TooltipHd pipeline.name pipeline.teamName
         , href <| Routes.toString <| Routes.pipelineRoute pipeline
         ]
            ++ Styles.pipelineCardHd (pipelineStatus existingJobs pipeline)
        )
    <|
        [ Html.div
            (if pipeline.stale then
                Styles.pipelineCardBannerStaleHd

             else if pipeline.archived then
                Styles.pipelineCardBannerArchivedHd

             else
                Styles.pipelineCardBannerHd
                    { status = pipelineStatus existingJobs pipeline
                    , pipelineRunningKeyframes = pipelineRunningKeyframes
                    }
            )
            []
        , Html.div
            (class "dashboardhd-pipeline-name" :: Styles.pipelineCardBodyHd)
            [ Html.text pipeline.name ]
        ]
            ++ (if resourceError then
                    [ Html.div Styles.resourceErrorTriangle [] ]

                else
                    []
               )


pipelineView :
    { now : Maybe Time.Posix
    , pipeline : Pipeline
    , hovered : HoverState.HoverState
    , pipelineRunningKeyframes : String
    , userState : UserState
    , resourceError : Bool
    , existingJobs : List Concourse.Job
    , layers : List (List Concourse.Job)
    }
    -> Html Message
pipelineView { now, pipeline, hovered, pipelineRunningKeyframes, userState, resourceError, existingJobs, layers } =
    let
        bannerStyle =
            if pipeline.stale then
                Styles.pipelineCardBannerStale

            else if pipeline.archived then
                Styles.pipelineCardBannerArchived

            else
                Styles.pipelineCardBanner
                    { status = pipelineStatus existingJobs pipeline
                    , pipelineRunningKeyframes = pipelineRunningKeyframes
                    }
    in
    Html.div
        (Styles.pipelineCard
            ++ (if not pipeline.stale then
                    [ style "cursor" "move" ]

                else
                    []
               )
            ++ (if pipeline.stale then
                    [ style "opacity" "0.45" ]

                else
                    []
               )
        )
        [ Html.div
            (class "banner" :: bannerStyle)
            []
        , headerView pipeline resourceError
        , if pipeline.jobsDisabled || pipeline.archived then
            previewPlaceholder

          else
            bodyView hovered layers
        , footerView userState pipeline now hovered existingJobs
        ]


pipelineStatus : List Concourse.Job -> Pipeline -> PipelineStatus.PipelineStatus
pipelineStatus jobs pipeline =
    if pipeline.paused then
        PipelineStatus.PipelineStatusPaused

    else
        let
            isRunning =
                List.any (\job -> job.nextBuild /= Nothing) jobs

            mostImportantJobStatus =
                jobs
                    |> List.map jobStatus
                    |> List.sortWith Concourse.BuildStatus.ordering
                    |> List.head

            firstNonSuccess =
                jobs
                    |> List.filter (jobStatus >> (/=) BuildStatusSucceeded)
                    |> List.filterMap transition
                    |> List.sortBy Time.posixToMillis
                    |> List.head

            lastTransition =
                jobs
                    |> List.filterMap transition
                    |> List.sortBy Time.posixToMillis
                    |> List.reverse
                    |> List.head

            transitionTime =
                case firstNonSuccess of
                    Just t ->
                        Just t

                    Nothing ->
                        lastTransition
        in
        case ( mostImportantJobStatus, transitionTime ) of
            ( _, Nothing ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Nothing, _ ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Just BuildStatusPending, _ ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Just BuildStatusStarted, _ ) ->
                PipelineStatus.PipelineStatusPending isRunning

            ( Just BuildStatusSucceeded, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusSucceeded PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusSucceeded (PipelineStatus.Since since)

            ( Just BuildStatusFailed, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusFailed PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusFailed (PipelineStatus.Since since)

            ( Just BuildStatusErrored, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusErrored PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusErrored (PipelineStatus.Since since)

            ( Just BuildStatusAborted, Just since ) ->
                if isRunning then
                    PipelineStatus.PipelineStatusAborted PipelineStatus.Running

                else
                    PipelineStatus.PipelineStatusAborted (PipelineStatus.Since since)


jobStatus : Concourse.Job -> BuildStatus
jobStatus job =
    case job.finishedBuild of
        Just build ->
            build.status

        Nothing ->
            BuildStatusPending


transition : Concourse.Job -> Maybe Time.Posix
transition =
    .transitionBuild >> Maybe.andThen (.duration >> .finishedAt)


headerView : Pipeline -> Bool -> Html Message
headerView pipeline resourceError =
    Html.a
        [ href <| Routes.toString <| Routes.pipelineRoute pipeline, draggable "false" ]
        [ Html.div
            ([ class "card-header"
             , onMouseEnter <| Tooltip pipeline.name pipeline.teamName
             ]
                ++ Styles.pipelineCardHeader
            )
            [ Html.div
                (class "dashboard-pipeline-name" :: Styles.pipelineName)
                [ Html.text pipeline.name ]
            , Html.div
                [ classList
                    [ ( "dashboard-resource-error", resourceError )
                    ]
                ]
                []
            ]
        ]


bodyView : HoverState.HoverState -> List (List Concourse.Job) -> Html Message
bodyView hovered layers =
    Html.div
        (class "card-body" :: Styles.pipelineCardBody)
        [ DashboardPreview.view hovered layers ]


footerView :
    UserState
    -> Pipeline
    -> Maybe Time.Posix
    -> HoverState.HoverState
    -> List Concourse.Job
    -> Html Message
footerView userState pipeline now hovered existingJobs =
    let
        spacer =
            Html.div [ style "width" "13.5px" ] []

        pipelineId =
            { pipelineName = pipeline.name
            , teamName = pipeline.teamName
            }

        status =
            pipelineStatus existingJobs pipeline

        pauseToggle =
            PauseToggle.view
                { isPaused =
                    status == PipelineStatus.PipelineStatusPaused
                , pipeline = pipelineId
                , isToggleHovered =
                    HoverState.isHovered (PipelineButton pipelineId) hovered
                , isToggleLoading = pipeline.isToggleLoading
                , tooltipPosition = Views.Styles.Above
                , margin = "0"
                , userState = userState
                }

        visibilityButton =
            visibilityView
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

        favoritedIcon =
            favoritedView
                { isFavorited = pipeline.isFavorited
                , isClickable =
                    UserState.isAnonymous userState
                        || UserState.isMember
                            { teamName = pipeline.teamName
                            , userState = userState
                            }
                , isHovered = HoverState.isHovered (PipelineCardFavoritedIcon pipeline.id) hovered
                , pipelineId = pipeline.id
                }
    in
    Html.div
        (class "card-footer" :: Styles.pipelineCardFooter)
        [ pipelineStatusView pipeline status now
        , Html.div
            [ style "display" "flex" ]
          <|
            List.intersperse spacer
                (if pipeline.archived then
                    [ visibilityButton, favoritedIcon ]

                 else
                    [ pauseToggle, visibilityButton, favoritedIcon ]
                )
        ]


pipelineStatusView : Pipeline -> PipelineStatus.PipelineStatus -> Maybe Time.Posix -> Html Message
pipelineStatusView pipeline status now =
    let
        pipelineId =
            { pipelineName = pipeline.name
            , teamName = pipeline.teamName
            }
    in
    Html.div
        [ style "display" "flex"
        , class "pipeline-status"
        ]
        (if pipeline.archived then
            []

         else
            [ if pipeline.jobsDisabled then
                Icon.icon
                    { sizePx = 20, image = Assets.PipelineStatusIconJobsDisabled }
                    ([ style "opacity" "0.5"
                     , id <| Effects.toHtmlID <| PipelineStatusIcon pipelineId
                     , onMouseEnter <| Hover <| Just <| PipelineStatusIcon pipelineId
                     ]
                        ++ Styles.pipelineStatusIcon
                    )

              else if pipeline.stale then
                Icon.icon
                    { sizePx = 20, image = Assets.PipelineStatusIconStale }
                    Styles.pipelineStatusIcon

              else
                Icon.icon
                    { sizePx = 20, image = Assets.PipelineStatusIcon status }
                    Styles.pipelineStatusIcon
            , if pipeline.jobsDisabled then
                Html.div
                    (class "build-duration"
                        :: Styles.pipelineCardTransitionAgeStale
                    )
                    [ Html.text "no data" ]

              else if pipeline.stale then
                Html.div
                    (class "build-duration"
                        :: Styles.pipelineCardTransitionAgeStale
                    )
                    [ Html.text "loading..." ]

              else
                transitionView now status
            ]
        )


favoritedView :
    { isFavorited : Bool
    , isClickable : Bool
    , isHovered : Bool
    , pipelineId : Concourse.DatabaseID
    }
    -> Html Message
favoritedView { isFavorited, isClickable, isHovered, pipelineId } =
    Html.div
        (Styles.favoritedToggle
            { isFavorited = isFavorited
            , isClickable = isClickable
            , isHovered = isHovered
            }
            ++ [ onMouseEnter <| Hover <| Just <| PipelineCardFavoritedIcon pipelineId
               , onMouseLeave <| Hover Nothing
               , id <| Effects.toHtmlID <| PipelineCardFavoritedIcon pipelineId
               ]
            ++ (if isClickable then
                    [ onClick <| Click <| PipelineCardFavoritedIcon pipelineId ]

                else
                    []
               )
        )
        []


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
                   , id <| Effects.toHtmlID <| VisibilityButton pipelineId
                   ]
                ++ (if isClickable then
                        [ onClick <| Click <| VisibilityButton pipelineId ]

                    else
                        []
                   )
            )
            []


sinceTransitionText : PipelineStatus.StatusDetails -> Time.Posix -> String
sinceTransitionText details now =
    case details of
        PipelineStatus.Running ->
            "running"

        PipelineStatus.Since time ->
            Duration.format <| Duration.between time now


transitionView : Maybe Time.Posix -> PipelineStatus.PipelineStatus -> Html Message
transitionView t status =
    case ( status, t ) of
        ( PipelineStatus.PipelineStatusPaused, _ ) ->
            Html.div
                (class "build-duration"
                    :: Styles.pipelineCardTransitionAge status
                )
                [ Html.text "paused" ]

        ( PipelineStatus.PipelineStatusPending False, _ ) ->
            Html.div
                (class "build-duration"
                    :: Styles.pipelineCardTransitionAge status
                )
                [ Html.text "pending" ]

        ( PipelineStatus.PipelineStatusPending True, _ ) ->
            Html.div
                (class "build-duration"
                    :: Styles.pipelineCardTransitionAge status
                )
                [ Html.text "running" ]

        ( PipelineStatus.PipelineStatusAborted details, Just now ) ->
            Html.div
                (class "build-duration"
                    :: Styles.pipelineCardTransitionAge status
                )
                [ Html.text <| sinceTransitionText details now ]

        ( PipelineStatus.PipelineStatusErrored details, Just now ) ->
            Html.div
                (class "build-duration"
                    :: Styles.pipelineCardTransitionAge status
                )
                [ Html.text <| sinceTransitionText details now ]

        ( PipelineStatus.PipelineStatusFailed details, Just now ) ->
            Html.div
                (class "build-duration"
                    :: Styles.pipelineCardTransitionAge status
                )
                [ Html.text <| sinceTransitionText details now ]

        ( PipelineStatus.PipelineStatusSucceeded details, Just now ) ->
            Html.div
                (class "build-duration"
                    :: Styles.pipelineCardTransitionAge status
                )
                [ Html.text <| sinceTransitionText details now ]

        _ ->
            Html.text ""
