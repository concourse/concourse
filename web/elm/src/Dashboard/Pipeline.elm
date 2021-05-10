module Dashboard.Pipeline exposing
    ( hdPipelineView
    , headerRows
    , pipelineNotSetView
    , pipelineStatus
    , pipelineView
    )

import Application.Models exposing (Session)
import Assets
import Colors
import Concourse exposing (flattenJson)
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.PipelineStatus as PipelineStatus
import Dashboard.DashboardPreview as DashboardPreview
import Dashboard.Grid.Constants as GridConstants
import Dashboard.Group.Models exposing (Pipeline)
import Dashboard.Styles as Styles
import Dict
import Duration
import Favorites
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
import Time
import Tooltip
import UserState
import Views.FavoritedIcon
import Views.Icon as Icon
import Views.PauseToggle as PauseToggle
import Views.Spinner as Spinner
import Views.Styles


previewPlaceholder : Html Message
previewPlaceholder =
    Html.div
        (class "card-body" :: Styles.emptyCardBody)
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
    { u | pipelineRunningKeyframes : String }
    ->
        { pipeline : Pipeline
        , resourceError : Bool
        , existingJobs : List Concourse.Job
        }
    -> Html Message
hdPipelineView { pipelineRunningKeyframes } { pipeline, resourceError, existingJobs } =
    let
        bannerStyle =
            if pipeline.stale then
                Styles.pipelineCardBannerStaleHd

            else if pipeline.archived then
                Styles.pipelineCardBannerArchivedHd

            else
                Styles.pipelineCardBannerHd
                    { status = pipelineStatus existingJobs pipeline
                    , pipelineRunningKeyframes = pipelineRunningKeyframes
                    }
    in
    Html.a
        ([ class "card"
         , attribute "data-pipeline-name" pipeline.name
         , attribute "data-team-name" pipeline.teamName
         , href <| Routes.toString <| Routes.pipelineRoute pipeline
         ]
            ++ Styles.pipelineCardHd (pipelineStatus existingJobs pipeline)
        )
    <|
        [ Html.div
            (class "banner" :: bannerStyle)
            []
        , Html.div
            (class "dashboardhd-pipeline-name"
                :: Styles.pipelineCardBodyHd
                ++ Tooltip.hoverAttrs (PipelineCardNameHD pipeline.id)
            )
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
        , resourceError : Bool
        , existingJobs : List Concourse.Job
        , layers : List (List Concourse.Job)
        , section : PipelinesSection
        , headerHeight : Float
        , viewingInstanceGroups : Bool
        , inInstanceGroup : Bool
        }
    -> Html Message
pipelineView session { now, pipeline, hovered, resourceError, existingJobs, layers, section, headerHeight, viewingInstanceGroups, inInstanceGroup } =
    let
        bannerStyle =
            if pipeline.stale then
                Styles.pipelineCardBannerStale

            else if pipeline.archived then
                Styles.pipelineCardBannerArchived

            else
                Styles.pipelineCardBanner
                    { status = pipelineStatus existingJobs pipeline
                    , pipelineRunningKeyframes = session.pipelineRunningKeyframes
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
        , headerView section pipeline resourceError headerHeight viewingInstanceGroups inInstanceGroup
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

            unpausedJobs =
                jobs |> List.filter (\job -> not job.paused)

            mostImportantJobStatus =
                unpausedJobs
                    |> List.map jobStatus
                    |> List.sortWith Concourse.BuildStatus.ordering
                    |> List.head

            firstNonSuccess =
                unpausedJobs
                    |> List.filter (jobStatus >> (/=) BuildStatusSucceeded)
                    |> List.filterMap transition
                    |> List.sortBy Time.posixToMillis
                    |> List.head

            lastTransition =
                unpausedJobs
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


headerView : PipelinesSection -> Pipeline -> Bool -> Float -> Bool -> Bool -> Html Message
headerView section pipeline resourceError headerHeight viewingInstanceGroups inInstanceGroup =
    let
        verticalSpacer =
            Html.div [ style "height" <| String.fromInt GridConstants.cardHeaderRowGap ++ "px" ] []

        rows =
            List.intersperse verticalSpacer
                (headerRows section viewingInstanceGroups pipeline inInstanceGroup)

        resourceErrorElem =
            Html.div
                [ classList [ ( "dashboard-resource-error", resourceError ) ] ]
                []
    in
    Html.a
        [ href <| Routes.toString <| Routes.pipelineRoute pipeline, draggable "false" ]
        [ Html.div
            (class "card-header" :: Styles.pipelineCardHeader headerHeight)
            (rows ++ [ resourceErrorElem ])
        ]


headerRows : PipelinesSection -> Bool -> Pipeline -> Bool -> List (Html Message)
headerRows section viewingInstanceGroups pipeline inInstanceGroup =
    let
        nameRow =
            if viewingInstanceGroups then
                []

            else
                [ Html.div
                    (class "dashboard-pipeline-name"
                        :: Styles.pipelineName
                        ++ Tooltip.hoverAttrs (PipelineCardName section pipeline.id)
                    )
                    [ Html.text pipeline.name ]
                ]

        instanceVarRows =
            if not inInstanceGroup then
                []

            else if Dict.isEmpty pipeline.instanceVars then
                [ Html.div Styles.noInstanceVars [ Html.text "no instance vars" ] ]

            else if viewingInstanceGroups then
                -- one row per key/value pair
                pipeline.instanceVars
                    |> Dict.toList
                    |> List.concatMap (\( k, v ) -> flattenJson k v)
                    |> List.map
                        (\( k, v ) ->
                            Html.div
                                (class "instance-var"
                                    :: Styles.instanceVar
                                    ++ Tooltip.hoverAttrs (PipelineCardInstanceVar section pipeline.id k v)
                                )
                                [ Html.span [ style "color" Colors.pending ]
                                    [ Html.text <| k ++ ":" ]
                                , Html.text v
                                ]
                        )

            else
                -- single row consisting of inline key/value pairs
                [ Html.div
                    (class "instance-vars"
                        :: Styles.instanceVar
                        ++ Tooltip.hoverAttrs (PipelineCardInstanceVars section pipeline.id pipeline.instanceVars)
                    )
                    (pipeline.instanceVars
                        |> Dict.toList
                        |> List.concatMap (\( k, v ) -> flattenJson k v)
                        |> List.map
                            (\( k, v ) ->
                                Html.span Styles.inlineInstanceVar
                                    [ Html.span [ style "color" Colors.pending ]
                                        [ Html.text <| k ++ ":" ]
                                    , Html.text v
                                    ]
                            )
                    )
                ]
    in
    nameRow ++ instanceVarRows


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
            Html.div [ style "width" "16px" ] []

        pipelineId =
            Concourse.toPipelineId pipeline

        status =
            pipelineStatus existingJobs pipeline

        pauseToggle =
            PauseToggle.view
                { isPaused =
                    status == PipelineStatus.PipelineStatusPaused
                , pipeline = pipelineId
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
                { isFavorited = Favorites.isPipelineFavorited session pipeline
                , isHovered = HoverState.isHovered (PipelineCardFavoritedIcon section pipeline.id) hovered
                , isSideBar = False
                , domID = PipelineCardFavoritedIcon section pipeline.id
                }
                []
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
    , pipelineId : Concourse.DatabaseID
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
