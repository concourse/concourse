module Dashboard.InstanceGroup exposing (cardView, hdCardView)

import Application.Models exposing (Session)
import ColorValues
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Dashboard.FilterBuilder exposing (instanceGroupFilter)
import Dashboard.Group.Models exposing (Pipeline)
import Dashboard.Pipeline exposing (pipelineStatus)
import Dashboard.Styles as Styles
import Dict exposing (Dict)
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
        , style
        )
import List.Extra
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Routes
import Tooltip
import Views.FavoritedIcon
import Views.InstanceGroupBadge as InstanceGroupBadge


instanceGroupRoute :
    { pipeline : Pipeline
    , dashboardView : Routes.DashboardView
    , query : String
    }
    -> Routes.Route
instanceGroupRoute { pipeline, dashboardView, query } =
    let
        instanceGroupQuery =
            instanceGroupFilter pipeline

        newQuery =
            if query /= "" then
                query ++ " " ++ instanceGroupQuery

            else
                instanceGroupQuery
    in
    Routes.Dashboard
        { searchType = Routes.Normal newQuery
        , dashboardView = dashboardView
        }


hdCardView :
    { pipeline : Pipeline
    , pipelines : List Pipeline
    , resourceError : Bool
    , dashboardView : Routes.DashboardView
    , query : String
    }
    -> Html Message
hdCardView { pipeline, pipelines, resourceError, dashboardView, query } =
    Html.a
        ([ class "card"
         , attribute "data-pipeline-name" pipeline.name
         , attribute "data-team-name" pipeline.teamName
         , href <|
            Routes.toString <|
                instanceGroupRoute
                    { pipeline = pipeline
                    , dashboardView = dashboardView
                    , query = query
                    }
         ]
            ++ Styles.instanceGroupCardHd
        )
    <|
        [ Html.div
            Styles.instanceGroupCardBodyHd
            [ InstanceGroupBadge.view ColorValues.grey20 <| List.length (pipeline :: pipelines)
            , Html.div
                (class "dashboardhd-group-name"
                    :: Styles.instanceGroupCardNameHd
                    ++ Tooltip.hoverAttrs (InstanceGroupCardNameHD pipeline.teamName pipeline.name)
                )
                [ Html.text pipeline.name ]
            ]
        ]
            ++ (if resourceError then
                    [ Html.div Styles.resourceErrorTriangle [] ]

                else
                    []
               )


cardView :
    Session
    ->
        { pipeline : Pipeline
        , pipelines : List Pipeline
        , resourceError : Bool
        , pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobName)
        , jobs : Dict ( Concourse.DatabaseID, String ) Concourse.Job
        , section : PipelinesSection
        , dashboardView : Routes.DashboardView
        , query : String
        , headerHeight : Float
        }
    -> Html Message
cardView session { pipeline, pipelines, resourceError, pipelineJobs, jobs, section, dashboardView, query, headerHeight } =
    Html.div
        (Styles.instanceGroupCard
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
        [ Html.div (class "banner" :: Styles.instanceGroupCardBanner) []
        , headerView section dashboardView query pipeline pipelines resourceError headerHeight
        , bodyView session section (pipeline :: pipelines) pipelineJobs jobs
        , footerView session pipeline section
        ]


headerView : PipelinesSection -> Routes.DashboardView -> String -> Pipeline -> List Pipeline -> Bool -> Float -> Html Message
headerView section dashboardView query pipeline pipelines resourceError headerHeight =
    Html.a
        [ href <|
            Routes.toString <|
                instanceGroupRoute
                    { pipeline = pipeline
                    , dashboardView = dashboardView
                    , query = query
                    }
        , draggable "false"
        ]
        [ Html.div
            (class "card-header" :: Styles.instanceGroupCardHeader headerHeight)
            [ Html.div
                (class "dashboard-group-name"
                    :: Styles.instanceGroupName
                    ++ Tooltip.hoverAttrs (InstanceGroupCardName section pipeline.teamName pipeline.name)
                )
                [ InstanceGroupBadge.view ColorValues.grey20 <| List.length (pipeline :: pipelines)
                , Html.text pipeline.name
                ]
            , Html.div
                [ classList [ ( "dashboard-resource-error", resourceError ) ] ]
                []
            ]
        ]


bodyView :
    Session
    -> PipelinesSection
    -> List Pipeline
    -> Dict Concourse.DatabaseID (List Concourse.JobName)
    -> Dict ( Concourse.DatabaseID, Concourse.JobName ) Concourse.Job
    -> Html Message
bodyView session section pipelines pipelineJobs jobs =
    let
        cols =
            ceiling <| sqrt <| toFloat <| List.length pipelines

        padRow row =
            let
                padding =
                    List.range 1 (cols - List.length row)
                        |> List.map (always Nothing)
            in
            List.map Just row ++ padding

        pipelineBox p =
            let
                pipelinePage =
                    Routes.toString <|
                        Routes.pipelineRoute p []

                curPipelineJobs =
                    Dict.get p.id pipelineJobs
                        |> Maybe.withDefault []
                        |> List.filterMap
                            (\jobName ->
                                Dict.get
                                    ( p.id, jobName )
                                    jobs
                            )
            in
            Html.div
                (Styles.instanceGroupCardPipelineBox
                    session.pipelineRunningKeyframes
                    (HoverState.isHovered
                        (PipelinePreview section p.id)
                        session.hovered
                    )
                    (pipelineStatus curPipelineJobs p)
                    ++ Tooltip.hoverAttrs (PipelinePreview section p.id)
                )
                [ Html.a
                    [ href pipelinePage
                    , style "flex-grow" "1"
                    ]
                    []
                ]

        emptyBox =
            Html.div [ style "margin" "2px", style "flex-grow" "1" ] []

        viewRow row =
            List.map
                (\mp ->
                    case mp of
                        Nothing ->
                            emptyBox

                        Just p ->
                            pipelineBox p
                )
                row
    in
    Html.div
        (class "card-body" :: Styles.instanceGroupCardBody)
        (List.Extra.greedyGroupsOf cols pipelines
            |> List.map padRow
            |> List.map
                (\paddedRow ->
                    Html.div
                        [ style "display" "flex"
                        , style "flex-grow" "1"
                        ]
                        (viewRow paddedRow)
                )
        )


footerView :
    Session
    -> Pipeline
    -> PipelinesSection
    -> Html Message
footerView session pipeline section =
    let
        groupID =
            Concourse.toInstanceGroupId pipeline

        domID =
            InstanceGroupCardFavoritedIcon section groupID

        favoritedIcon =
            Views.FavoritedIcon.view
                { isFavorited = Favorites.isInstanceGroupFavorited session groupID
                , isHovered = HoverState.isHovered domID session.hovered
                , isSideBar = False
                , domID = domID
                }
                []
    in
    Html.div (class "card-footer" :: Styles.instanceGroupCardFooter) [ favoritedIcon ]
