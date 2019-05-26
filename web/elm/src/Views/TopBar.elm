module Views.TopBar exposing
    ( breadcrumbs
    , concourseLogo
    )

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
import Url
import Views.Styles as Styles


concourseLogo : Html Message
concourseLogo =
    Html.a (href "/" :: Styles.concourseLogo) []


breadcrumbs : Routes.Route -> Html Message
breadcrumbs route =
    Html.div
        (id "breadcrumbs" :: Styles.breadcrumbContainer)
    <|
        case route of
            Routes.Pipeline { id } ->
                [ pipelineBreadcumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                ]

            Routes.Build { id } ->
                [ pipelineBreadcumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , jobBreadcrumb id.jobName
                ]

            Routes.Resource { id } ->
                [ pipelineBreadcumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , resourceBreadcrumb id
                ]

            Routes.ResourceVersion id ->
                [ pipelineBreadcumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , resourceBreadcrumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    , resourceName = id.resourceName
                    }
                ]

            Routes.Job { id } ->
                [ pipelineBreadcumb
                    { teamName = id.teamName
                    , pipelineName = id.pipelineName
                    }
                , breadcrumbSeparator
                , jobBreadcrumb id.jobName
                ]

            _ ->
                []


breadcrumbComponent : String -> String -> List (Html Message)
breadcrumbComponent componentType name =
    [ Html.div
        (Styles.breadcrumbComponent componentType)
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
        (breadcrumbComponent "pipeline" pipelineId.pipelineName)


jobBreadcrumb : String -> Html Message
jobBreadcrumb jobName =
    Html.li
        (id "breadcrumb-job" :: Styles.breadcrumbItem False)
        (breadcrumbComponent "job" jobName)


resourceBreadcrumb : Concourse.ResourceIdentifier -> Html Message
resourceBreadcrumb resourceId =
    Html.a
        ([ id "breadcrumb-resource"
         , href <|
            Routes.toString <|
                Routes.Resource { id = resourceId, page = Nothing }
         ]
            ++ Styles.breadcrumbItem True
        )
        (breadcrumbComponent "resource" resourceId.resourceName)


decodeName : String -> String
decodeName name =
    Maybe.withDefault name (Url.percentDecode name)
