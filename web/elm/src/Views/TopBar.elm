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
import Routes
import SideBar.SideBar exposing (lookupPipeline)
import Url
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


pipelineBreadcumb : Concourse.PipelineIdentifier -> Html Message
pipelineBreadcumb pipelineId =
    Html.a
        ([ id "breadcrumb-pipeline"
         , href <|
            Routes.toString <|
                Routes.Pipeline { id = pipelineId, groups = [] }
         ]
            ++ Styles.breadcrumbItem True
        )
        (breadcrumbComponent
            { icon =
                { component = Assets.PipelineComponent
                , widthPx = 28
                , heightPx = 16
                }
            , name = pipelineId.pipelineName
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
