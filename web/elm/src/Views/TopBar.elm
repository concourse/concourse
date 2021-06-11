module Views.TopBar exposing
    ( breadcrumbs
    , concourseLogo
    )

import Application.Models exposing (Session)
import Assets
import ColorValues
import Concourse exposing (hyphenNotation)
import Dashboard.FilterBuilder exposing (instanceGroupFilter)
import Dict
import Html exposing (Html)
import Html.Attributes
    exposing
        ( class
        , href
        , id
        )
import Message.Message exposing (DomID(..), Message(..))
import RemoteData
import Routes
import SideBar.SideBar exposing (byPipelineId, isPipelineVisible, lookupPipeline)
import Url
import Views.InstanceGroupBadge as InstanceGroupBadge
import Views.Styles as Styles


concourseLogo : Html Message
concourseLogo =
    Html.a (href "/" :: Styles.concourseLogo) []


breadcrumbs : Session -> Routes.Route -> Html Message
breadcrumbs session route =
    let
        buildBreadcrumbs components =
            case List.reverse components of
                x :: xs ->
                    (List.map (\fn -> fn False) xs
                        |> List.reverse
                    )
                        ++ [ x True ]

                [] ->
                    []
    in
    Html.div
        (id "breadcrumbs" :: Styles.breadcrumbContainer)
    <|
        buildBreadcrumbs <|
            case route of
                Routes.Pipeline { id } ->
                    case lookupPipeline (byPipelineId id) session of
                        Nothing ->
                            []

                        Just pipeline ->
                            pipelineBreadcrumbs session pipeline

                Routes.Build { id } ->
                    case lookupPipeline (byPipelineId id) session of
                        Nothing ->
                            []

                        Just pipeline ->
                            pipelineBreadcrumbs session pipeline
                                ++ [ breadcrumbSeparator
                                   , jobBreadcrumb id.jobName
                                   ]

                Routes.Resource { id } ->
                    case lookupPipeline (byPipelineId id) session of
                        Nothing ->
                            []

                        Just pipeline ->
                            pipelineBreadcrumbs session pipeline
                                ++ [ breadcrumbSeparator
                                   , resourceBreadcrumb id
                                   ]

                Routes.Job { id } ->
                    case lookupPipeline (byPipelineId id) session of
                        Nothing ->
                            []

                        Just pipeline ->
                            pipelineBreadcrumbs session pipeline
                                ++ [ breadcrumbSeparator
                                   , jobBreadcrumb id.jobName
                                   ]

                Routes.Dashboard _ ->
                    [ clusterNameBreadcrumb session ]

                Routes.Causality { id, direction, version } ->
                    case lookupPipeline (byPipelineId id) session of
                        Nothing ->
                            []

                        Just pipeline ->
                            pipelineBreadcrumbs session pipeline
                                ++ [ breadcrumbSeparator
                                   , resourceBreadcrumb <| Concourse.resourceIdFromVersionedResourceId id
                                   , breadcrumbSeparator
                                   , causalityBreadCrumb id direction (Maybe.withDefault Dict.empty version)
                                   ]

                _ ->
                    []


breadcrumbComponent :
    Bool
    ->
        { icon :
            { component : Assets.ComponentType
            , widthPx : Float
            , heightPx : Float
            }
        , name : String
        }
    -> List (Html Message)
breadcrumbComponent isLastBreadcrumb { name, icon } =
    [ Html.div
        (Styles.breadcrumbComponent icon)
        []
    , if isLastBreadcrumb then
        Html.div
            Styles.ellipsedText
            [ Html.text <| decodeName name ]

      else
        Html.text <| decodeName name
    ]


breadcrumbSeparator : Bool -> Html Message
breadcrumbSeparator _ =
    Html.li
        (class "breadcrumb-separator" :: Styles.breadcrumbItem False False)
        [ Html.text "/" ]


clusterNameBreadcrumb : Session -> Bool -> Html Message
clusterNameBreadcrumb session _ =
    Html.div
        Styles.clusterName
        [ Html.text session.clusterName ]


pipelineBreadcrumbs : Session -> Concourse.Pipeline -> List (Bool -> Html Message)
pipelineBreadcrumbs session pipeline =
    let
        pipelineGroup =
            session.pipelines
                |> RemoteData.withDefault []
                |> List.filter (\p -> p.name == pipeline.name && p.teamName == pipeline.teamName)
                |> List.filter (isPipelineVisible session)

        inInstanceGroup =
            Concourse.isInstanceGroup pipelineGroup

        instanceGroupBreadcrumb isLastBreadcrumb =
            Html.a
                (id "breadcrumb-instance-group"
                    :: (href <|
                            Routes.toString <|
                                Routes.Dashboard
                                    { searchType = Routes.Normal <| instanceGroupFilter pipeline
                                    , dashboardView = Routes.ViewNonArchivedPipelines
                                    }
                       )
                    :: Styles.breadcrumbItem True isLastBreadcrumb
                )
                [ InstanceGroupBadge.view ColorValues.white (List.length pipelineGroup)
                , Html.text pipeline.name
                ]
    in
    (if inInstanceGroup then
        [ instanceGroupBreadcrumb
        , breadcrumbSeparator
        ]

     else
        []
    )
        ++ [ pipelineBreadcrumb inInstanceGroup pipeline ]


pipelineBreadcrumb : Bool -> Concourse.Pipeline -> Bool -> Html Message
pipelineBreadcrumb inInstanceGroup pipeline isLastBreadcrumb =
    let
        text =
            if inInstanceGroup then
                hyphenNotation pipeline.instanceVars

            else
                pipeline.name
    in
    Html.a
        ([ id "breadcrumb-pipeline"
         , href <|
            Routes.toString <|
                Routes.pipelineRoute pipeline
         ]
            ++ Styles.breadcrumbItem True isLastBreadcrumb
        )
        (breadcrumbComponent
            isLastBreadcrumb
            { icon =
                { component = Assets.PipelineComponent
                , widthPx = 28
                , heightPx = 16
                }
            , name = text
            }
        )


jobBreadcrumb : String -> Bool -> Html Message
jobBreadcrumb jobName isLastBreadcrumb =
    Html.li
        (id "breadcrumb-job" :: Styles.breadcrumbItem False isLastBreadcrumb)
        (breadcrumbComponent
            isLastBreadcrumb
            { icon =
                { component = Assets.JobComponent
                , widthPx = 32
                , heightPx = 17
                }
            , name = jobName
            }
        )


resourceBreadcrumb : Concourse.ResourceIdentifier -> Bool -> Html Message
resourceBreadcrumb resource isLastBreadcrumb =
    Html.a
        ([ id "breadcrumb-resource"
         , href <|
            Routes.toString <|
                Routes.resourceRoute resource Nothing
         ]
            ++ Styles.breadcrumbItem True isLastBreadcrumb
        )
        (breadcrumbComponent
            isLastBreadcrumb
            { icon =
                { component = Assets.ResourceComponent
                , widthPx = 32
                , heightPx = 17
                }
            , name = resource.resourceName
            }
        )


causalityBreadCrumb : Concourse.VersionedResourceIdentifier -> Concourse.CausalityDirection -> Concourse.Version -> Bool -> Html Message
causalityBreadCrumb rv direction version isLastBreadcrumb =
    let
        component =
            case direction of
                Concourse.Downstream ->
                    Assets.DownstreamCausalityComponent

                Concourse.Upstream ->
                    Assets.UpstreamCausalityComponent

        name =
            String.join "," <| Concourse.versionQuery version
    in
    Html.a
        ([ id "breadcrumb-causality"
         , href <|
            Routes.toString <|
                Routes.resourceRoute (Concourse.resourceIdFromVersionedResourceId rv) (Just version)
         ]
            ++ Styles.breadcrumbItem True isLastBreadcrumb
        )
        (breadcrumbComponent
            isLastBreadcrumb
            { icon =
                { component = component
                , widthPx = 32
                , heightPx = 17
                }
            , name = name
            }
        )


decodeName : String -> String
decodeName name =
    Maybe.withDefault name (Url.percentDecode name)
