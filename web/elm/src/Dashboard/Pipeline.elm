module Dashboard.Pipeline exposing
    ( hdPipelineView
    , pipelineNotSetView
    , pipelineStatus
    , pipelineView
    )

import Application.Models exposing (Session)
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
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Routes
import Set
import SideBar.SideBar as SideBar
import Time
import UserState
import Views.FavoritedIcon
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
    Session
    ->
        { now : Maybe Time.Posix
        , pipeline : Pipeline
        , hovered : HoverState.HoverState
        , pipelineRunningKeyframes : String
        , resourceError : Bool
        , existingJobs : List Concourse.Job
        , layers : List (List Concourse.Job)
        , section : PipelinesSection
        }
    -> Html Message
pipelineView session { now, pipeline, hovered, pipelineRunningKeyframes, resourceError, existingJobs, layers, section } =
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
            ++ (if section == AllPipelinesSection && not pipeline.stale then
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
            bodyView section hovered layers
        , footerView session pipeline section now hovered existingJobs
        ]


pipelineStatus : List Concourse.Job -> Pipeline -> PipelineStatus.PipelineStatus
pipelineStatus jobs pipeline =
    if pipeline.archived then
        PipelineStatus.PipelineStatusArchived

    else if pipeline.paused then
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


bodyView : PipelinesSection -> HoverState.HoverState -> List (List Concourse.Job) -> Html Message
bodyView section hovered layers =
    Html.div
        (class "card-body" :: Styles.pipelineCardBody)
        [ DashboardPreview.view section hovered layers ]


footerView :
    Session
    -> Pipeline
    -> PipelinesSection
    -> Maybe Time.Posix
    -> HoverState.HoverState
    -> List Concourse.Job
    -> Html Message
footerView session pipeline section now hovered existingJobs =
    let
        spacer =
            Html.div [ style "width" "12px" ] []

        status =
            pipelineStatus existingJobs pipeline

        pauseToggle =
            PauseToggle.view
                { isClickable =
                    UserState.isAnonymous session.userState
                        || UserState.isMember
                            { teamName = pipeline.teamName
                            , userState = session.userState
                            }
                , isPaused =
                    status == PipelineStatus.PipelineStatusPaused
                , pipeline = SideBar.lookupPipeline pipeline.id session
                , isToggleHovered =
                    HoverState.isHovered (PipelineCardPauseToggle section pipeline.id) hovered
                , isToggleLoading = pipeline.isToggleLoading
                , tooltipPosition = Views.Styles.Above
                , margin = "0"
                , userState = session.userState
                , domID = PipelineCardPauseToggle section pipeline.id
                }

        visibilityButton =
            visibilityView
                { public = pipeline.public
                , pipelineId = pipeline.id
                , isClickable =
                    UserState.isAnonymous session.userState
                        || UserState.isMember
                            { teamName = pipeline.teamName
                            , userState = session.userState
                            }
                , isHovered =
                    HoverState.isHovered (VisibilityButton section pipeline.id) hovered
                , isVisibilityLoading = pipeline.isVisibilityLoading
                , section = section
                }

        favoritedIcon =
            Views.FavoritedIcon.view
                { isFavorited = Set.member pipeline.id session.favoritedPipelines
                , isHovered = HoverState.isHovered (PipelineCardFavoritedIcon section pipeline.id) hovered
                , domID = PipelineCardFavoritedIcon section pipeline.id
                }
                [ id <| Effects.toHtmlID <| PipelineCardFavoritedIcon section pipeline.id ]
    in
    Html.div
        (class "card-footer" :: Styles.pipelineCardFooter)
        [ pipelineStatusView section pipeline status now
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


pipelineStatusView : PipelinesSection -> Pipeline -> PipelineStatus.PipelineStatus -> Maybe Time.Posix -> Html Message
pipelineStatusView section pipeline status now =
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
                     , id <| Effects.toHtmlID <| PipelineStatusIcon section pipeline.id
                     , onMouseEnter <| Hover <| Just <| PipelineStatusIcon section pipeline.id
                     ]
                        ++ Styles.pipelineStatusIcon
                    )

              else if pipeline.stale then
                Icon.icon
                    { sizePx = 20, image = Assets.PipelineStatusIconStale }
                    Styles.pipelineStatusIcon

              else
                case Assets.pipelineStatusIcon status of
                    Just asset ->
                        Icon.icon
                            { sizePx = 20, image = asset }
                            Styles.pipelineStatusIcon

                    Nothing ->
                        Html.text ""
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


visibilityView :
    { public : Bool
    , pipelineId : Concourse.PipelineIdentifier
    , isClickable : Bool
    , isHovered : Bool
    , isVisibilityLoading : Bool
    , section : PipelinesSection
    }
    -> Html Message
visibilityView { public, pipelineId, isClickable, isHovered, isVisibilityLoading, section } =
    if isVisibilityLoading then
        Spinner.hoverableSpinner
            { sizePx = 20
            , margin = "0"
            , hoverable = Just <| VisibilityButton section pipelineId
            }

    else
        Html.div
            (Styles.visibilityToggle
                { public = public
                , isClickable = isClickable
                , isHovered = isHovered
                }
                ++ [ onMouseEnter <| Hover <| Just <| VisibilityButton section pipelineId
                   , onMouseLeave <| Hover Nothing
                   , id <| Effects.toHtmlID <| VisibilityButton section pipelineId
                   ]
                ++ (if isClickable then
                        [ onClick <| Click <| VisibilityButton section pipelineId ]

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
