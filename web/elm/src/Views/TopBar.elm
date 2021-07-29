module Views.TopBar exposing
    ( breadcrumbs
    , concourseLogo
    , paused
    )

import Application.Models exposing (Session)
import Assets
import ColorValues
import Concourse exposing (hyphenNotation)
import Dashboard.FilterBuilder exposing (instanceGroupFilter)
import Dict
import DateFormat
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
import Time
import Url
import Views.InstanceGroupBadge as InstanceGroupBadge
import Views.Styles as Styles


concourseLogo : Html Message
concourseLogo =
    Html.a (href "/" :: Styles.concourseLogo) []


paused :
    { paused : Bool
    , pausedBy : Maybe String
    , pausedAt : Maybe Time.Posix
    , timeZone : Time.Zone
    }
    -> Html Message
paused p =
    let
        text =
            pausedText p
    in
    Html.span (class "pause-details" :: Styles.pauseDetails) [ Html.text text ]


pausedText :
    { paused : Bool
    , pausedBy : Maybe String
    , pausedAt : Maybe Time.Posix
    , timeZone : Time.Zone
    }
    -> String
pausedText p =
    if p.paused then
        case ( p.pausedBy, p.pausedAt ) of
            ( Just by, Just at ) ->
                "paused by " ++ by ++ " on " ++ formatDate p.timeZone at

            ( Just by, Nothing ) ->
                "paused by " ++ by

            ( Nothing, Just at ) ->
                "paused on " ++ formatDate p.timeZone at

            ( Nothing, Nothing ) ->
                ""

    else
        ""


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
                            pipelineBreadcrumbs session pipeline []

                Routes.Build { id, groups } ->
                    case lookupPipeline (byPipelineId id) session of
                        Nothing ->
                            []

                        Just pipeline ->
                            pipelineBreadcrumbs session pipeline groups
                                ++ [ breadcrumbSeparator
                                   , jobBreadcrumb id.jobName
                                   ]

                Routes.Resource { id, groups } ->
                    case lookupPipeline (byPipelineId id) session of
                        Nothing ->
                            []

                        Just pipeline ->
                            pipelineBreadcrumbs session pipeline groups
                                ++ [ breadcrumbSeparator
                                   , resourceBreadcrumb id
                                   ]

                Routes.Job { id, groups } ->
                    case lookupPipeline (byPipelineId id) session of
                        Nothing ->
                            []

                        Just pipeline ->
                            pipelineBreadcrumbs session pipeline groups
                                ++ [ breadcrumbSeparator
                                   , jobBreadcrumb id.jobName
                                   ]

                Routes.Dashboard _ ->
                    [ clusterNameBreadcrumb session ]

                Routes.Causality { id, direction, version, groups } ->
                    case lookupPipeline (byPipelineId id) session of
                        Nothing ->
                            []

                        Just pipeline ->
                            pipelineBreadcrumbs session pipeline groups
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


pipelineBreadcrumbs : Session -> Concourse.Pipeline -> List String -> List (Bool -> Html Message)
pipelineBreadcrumbs session pipeline groups =
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
        ++ [ pipelineBreadcrumb inInstanceGroup pipeline groups ]


pipelineBreadcrumb : Bool -> Concourse.Pipeline -> List String -> Bool -> Html Message
pipelineBreadcrumb inInstanceGroup pipeline groups isLastBreadcrumb =
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
                Routes.pipelineRoute pipeline groups
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


formatDate : Time.Zone -> Time.Posix -> String
formatDate =
    DateFormat.format
        [ DateFormat.monthNameAbbreviated
        , DateFormat.text " "
        , DateFormat.dayOfMonthNumber
        , DateFormat.text " "
        , DateFormat.yearNumber
        , DateFormat.text " "
        , DateFormat.hourFixed
        , DateFormat.text ":"
        , DateFormat.minuteFixed
        , DateFormat.text ":"
        , DateFormat.secondFixed
        , DateFormat.text " "
        , DateFormat.amPmUppercase
        ]
