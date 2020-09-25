module Views.TopBar exposing
    ( breadcrumbs
    , concourseLogo
    )

import Application.Models exposing (Session)
import Assets
import Concourse
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
import SideBar.SideBar exposing (lookupPipeline)
import Url
import Views.InstanceGroupBadge as InstanceGroupBadge
import Views.Styles as Styles


concourseLogo : Html Message
concourseLogo =
    Html.a (href "/" :: Styles.concourseLogo) []


breadcrumbs : Session -> Routes.Route -> Html Message
breadcrumbs session route =
    Html.div
        (id "breadcrumbs" :: Styles.breadcrumbContainer)
    <|
        case route of
            Routes.Pipeline { id } ->
                case lookupPipeline id session of
                    Nothing ->
                        []

                    Just pipeline ->
                        [ pipelineBreadcrumb pipeline ]

            Routes.Build { id } ->
                case lookupPipeline id.pipelineId session of
                    Nothing ->
                        []

                    Just pipeline ->
                        [ pipelineBreadcrumb pipeline
                        , breadcrumbSeparator
                        , jobBreadcrumb id.jobName
                        ]

            Routes.Resource { id } ->
                case lookupPipeline id.pipelineId session of
                    Nothing ->
                        []

                    Just pipeline ->
                        [ pipelineBreadcrumb pipeline
                        , breadcrumbSeparator
                        , resourceBreadcrumb id.resourceName
                        ]

            Routes.Job { id } ->
                case lookupPipeline id.pipelineId session of
                    Nothing ->
                        []

                    Just pipeline ->
                        [ pipelineBreadcrumb pipeline
                        , breadcrumbSeparator
                        , jobBreadcrumb id.jobName
                        ]

            Routes.Dashboard { searchType } ->
                case searchType of
                    Routes.Normal _ (Just ig) ->
                        [ instanceGroupBreadcrumb session ig ]

                    _ ->
                        [ clusterNameBreadcrumb session ]

            _ ->
                []


breadcrumbComponent :
    { icon :
        { component : Assets.ComponentType
        , widthPx : Float
        , heightPx : Float
        }
    , name : String
    }
    -> List (Html Message)
breadcrumbComponent { name, icon } =
    [ Html.div
        (Styles.breadcrumbComponent icon)
        []
    , Html.text <| decodeName name
    ]


breadcrumbSeparator : Html Message
breadcrumbSeparator =
    Html.li
        (class "breadcrumb-separator" :: Styles.breadcrumbItem False)
        [ Html.text "/" ]


instanceGroupBreadcrumb : Session -> Concourse.InstanceGroupIdentifier -> Html Message
instanceGroupBreadcrumb session ig =
    let
        numPipelines =
            session.pipelines
                |> RemoteData.withDefault []
                |> List.filter (\p -> p.name == ig.name && p.teamName == ig.teamName)
                |> List.length
    in
    Html.a
        (id "breadcrumb-instance-group"
            :: (href <|
                    Routes.toString <|
                        -- TODO: don't lose prev state
                        Routes.Dashboard
                            { searchType = Routes.Normal "" <| Just ig
                            , dashboardView = Routes.ViewNonArchivedPipelines
                            }
               )
            :: Styles.breadcrumbItem True
        )
        [ InstanceGroupBadge.view numPipelines
        , Html.text ig.name
        ]


clusterNameBreadcrumb : Session -> Html Message
clusterNameBreadcrumb session =
    Html.div
        Styles.clusterName
        [ Html.text session.clusterName ]


pipelineBreadcrumb : Concourse.Pipeline -> Html Message
pipelineBreadcrumb pipeline =
    Html.a
        ([ id "breadcrumb-pipeline"
         , href <|
            Routes.toString <|
                Routes.Pipeline { id = pipeline.id, groups = [] }
         ]
            ++ Styles.breadcrumbItem True
        )
        (breadcrumbComponent
            { icon =
                { component = Assets.PipelineComponent
                , widthPx = 28
                , heightPx = 16
                }
            , name = pipeline.name
            }
        )


jobBreadcrumb : String -> Html Message
jobBreadcrumb jobName =
    Html.li
        (id "breadcrumb-job" :: Styles.breadcrumbItem False)
        (breadcrumbComponent
            { icon =
                { component = Assets.JobComponent
                , widthPx = 32
                , heightPx = 17
                }
            , name = jobName
            }
        )


resourceBreadcrumb : String -> Html Message
resourceBreadcrumb resourceName =
    Html.li
        (id "breadcrumb-resource" :: Styles.breadcrumbItem False)
        (breadcrumbComponent
            { icon =
                { component = Assets.ResourceComponent
                , widthPx = 32
                , heightPx = 17
                }
            , name = resourceName
            }
        )


decodeName : String -> String
decodeName name =
    Maybe.withDefault name (Url.percentDecode name)
