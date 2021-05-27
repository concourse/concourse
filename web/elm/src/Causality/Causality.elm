module Causality.Causality exposing
    ( Model
    , changeToVersionedResource
    , documentTitle
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Application.Models exposing (Session)
import Concourse
    exposing
        ( CausalityBuild(..)
        , CausalityDirection(..)
        , CausalityResourceVersion
        )
import Dict
import EffectTransformer exposing (ET)
import Graph exposing (Graph, NodeContext, NodeId)
import Graph.DOT as DOT
import Html exposing (Html)
import Html.Attributes
    exposing
        ( class
        , href
        , id
        , src
        , style
        )
import IntDict
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Routes
import SideBar.SideBar as SideBar exposing (byPipelineId, lookupPipeline)
import Svg
import Svg.Attributes as SvgAttributes
import Tooltip
import Views.Styles
import Views.TopBar as TopBar


type alias Model =
    Login.Model
        { versionId : Concourse.VersionedResourceIdentifier
        , direction : CausalityDirection
        , fetchedCausality : Maybe CausalityResourceVersion
        , graph : Graph NodeMetadata ()
        , renderedJobs : Maybe (List Concourse.Job)
        , renderedBuilds : Maybe (List Concourse.Build)
        , renderedResources : Maybe (List Concourse.Resource)
        , renderedResourceVersions : Maybe (List Concourse.VersionedResource)
        }


type alias Flags =
    { versionId : Concourse.VersionedResourceIdentifier
    , direction : CausalityDirection
    }


documentTitle : Model -> String
documentTitle model =
    model.versionId.resourceName


init : Flags -> ( Model, List Effect )
init flags =
    let
        fetchCausality =
            case flags.direction of
                Concourse.Downstream ->
                    FetchDownstreamCausality flags.versionId

                Concourse.Upstream ->
                    FetchUpstreamCausality flags.versionId
    in
    ( { isUserMenuExpanded = False
      , versionId = flags.versionId
      , direction = flags.direction
      , fetchedCausality = Nothing
      , graph = Graph.empty
      , renderedJobs = Nothing
      , renderedBuilds = Nothing
      , renderedResources = Nothing
      , renderedResourceVersions = Nothing
      }
    , [ fetchCausality ]
    )


changeToVersionedResource : Flags -> ET Model
changeToVersionedResource flags ( _, effects ) =
    let
        ( newModel, newEffects ) =
            init flags
    in
    ( newModel, effects ++ newEffects )


subscriptions : List Subscription
subscriptions =
    []


tooltip : Model -> Session -> Maybe Tooltip.Tooltip
tooltip _ _ =
    Nothing


handleCallback : Callback -> ET Model
handleCallback callback ( model, effects ) =
    case callback of
        CausalityFetched (Ok ( direction, crv )) ->
            let
                graph =
                    case crv of
                        Just rv ->
                            constructGraph direction rv

                        _ ->
                            model.graph
            in
            ( { model
                | fetchedCausality = crv
                , graph = graph
              }
            , effects
                ++ [ RenderCausality <| graphvizDotNotation graph ]
            )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        _ ->
            ( model, effects )


update : Message -> ET Model
update msg ( model, effects ) =
    case msg of
        _ ->
            ( model, effects )


view : Session -> Model -> Html Message
view session model =
    let
        route =
            Routes.Causality
                { id = model.versionId
                , direction = model.direction
                }
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            [ SideBar.sideBarIcon session
            , TopBar.concourseLogo
            , TopBar.breadcrumbs session route
            , Login.view session.userState model
            ]
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
            [ SideBar.view session
                (Just
                    { pipelineName = model.versionId.pipelineName
                    , pipelineInstanceVars = model.versionId.pipelineInstanceVars
                    , teamName = model.versionId.teamName
                    }
                )

            -- , Html.text <| graphvizDotNotation model.graph
            , Html.div
                [ class "causality-view"
                , id "causality-container"
                , style "display" "flex"
                , style "flex-direction" "column"
                , style "flex-grow" "1"
                ]
                [ Html.div
                    [ class "causality-content" ]
                    [ Svg.svg
                        [ SvgAttributes.class "causality-graph" ]
                        []
                    ]
                ]
            ]
        ]


type alias NodeId =
    Int


type alias NodeMetadata =
    { typ : NodeType
    , name : String
    , labels : List String
    }


type NodeType
    = Job
    | Resource


insert : List String -> String -> List String
insert lst str =
    if List.member str lst then
        lst

    else
        lst ++ [ str ]


constructResourceVersion : NodeId -> CausalityDirection -> CausalityResourceVersion -> Graph NodeMetadata () -> Graph NodeMetadata ()
constructResourceVersion parentId dir rv graph =
    let
        -- NodeId is the (positive) versionId for resourceVersions and the (negative) buildId for builds
        nodeId =
            rv.resourceId

        childEdges =
            IntDict.fromList <| List.map (\(CausalityBuildVariant b) -> ( -b.jobId, () )) rv.builds

        addEdge : NodeContext NodeMetadata () -> NodeContext NodeMetadata ()
        addEdge ctx =
            case dir of
                Downstream ->
                    { ctx
                        | incoming = IntDict.insert parentId () ctx.incoming
                        , outgoing = childEdges
                    }

                Upstream ->
                    { ctx
                        | incoming = childEdges
                        , outgoing = IntDict.insert parentId () ctx.outgoing
                    }

        versionStr =
            String.join "," <| List.map (\( k, v ) -> k ++ ":" ++ v) <| Dict.toList rv.version

        updateNode : Maybe (NodeContext NodeMetadata ()) -> Maybe (NodeContext NodeMetadata ())
        updateNode nodeContext =
            Just <|
                case nodeContext of
                    Just { node, incoming, outgoing } ->
                        let
                            metadata : NodeMetadata
                            metadata =
                                node.label
                        in
                        addEdge
                            { node =
                                { node | label = { metadata | labels = insert metadata.labels versionStr } }
                            , incoming = incoming
                            , outgoing = outgoing
                            }

                    Nothing ->
                        addEdge
                            { node =
                                { id = nodeId
                                , label =
                                    { typ = Resource
                                    , name = rv.resourceName
                                    , labels = [ versionStr ]
                                    }
                                }
                            , incoming = IntDict.empty
                            , outgoing = IntDict.empty
                            }

        updatedGraph =
            Graph.update nodeId updateNode graph
    in
    List.foldl (\build acc -> constructBuild nodeId dir build acc) updatedGraph rv.builds


constructBuild : NodeId -> CausalityDirection -> CausalityBuild -> Graph NodeMetadata () -> Graph NodeMetadata ()
constructBuild parentId dir (CausalityBuildVariant b) graph =
    let
        -- NodeId is the (positive) resourceId for resourceVersions and the (negative) jobId for builds
        nodeId =
            -b.jobId

        childEdges =
            IntDict.fromList <| List.map (\rv -> ( rv.resourceId, () )) b.resourceVersions

        addEdge : NodeContext NodeMetadata () -> NodeContext NodeMetadata ()
        addEdge ctx =
            case dir of
                Downstream ->
                    { ctx
                        | incoming = IntDict.insert parentId () ctx.incoming
                        , outgoing = childEdges
                    }

                Upstream ->
                    { ctx
                        | incoming = childEdges
                        , outgoing = IntDict.insert parentId () ctx.outgoing
                    }

        buildName =
            "#" ++ b.name

        updateNode : Maybe (NodeContext NodeMetadata ()) -> Maybe (NodeContext NodeMetadata ())
        updateNode nodeContext =
            Just <|
                case nodeContext of
                    Just { node, incoming, outgoing } ->
                        let
                            metadata =
                                node.label
                        in
                        addEdge
                            { node =
                                { node | label = { metadata | labels = insert metadata.labels buildName } }
                            , incoming = incoming
                            , outgoing = outgoing
                            }

                    Nothing ->
                        addEdge
                            { node =
                                { id = nodeId
                                , label =
                                    { typ = Job
                                    , name = b.jobName
                                    , labels = [ buildName ]
                                    }
                                }
                            , incoming = IntDict.empty
                            , outgoing = IntDict.empty
                            }

        updatedGraph =
            Graph.update nodeId updateNode graph
    in
    List.foldl (\build acc -> constructResourceVersion nodeId dir build acc) updatedGraph b.resourceVersions


constructGraph : CausalityDirection -> CausalityResourceVersion -> Graph NodeMetadata ()
constructGraph direction rv =
    let
        graph =
            constructResourceVersion 0 direction rv Graph.empty
    in
    -- because the first node is constructed with fictious parentId 0, the first edge needs to be removed
    Graph.update rv.versionId
        (Maybe.map
            (\ctx ->
                case direction of
                    Downstream ->
                        { ctx | incoming = IntDict.remove 0 ctx.incoming }

                    Upstream ->
                        { ctx | outgoing = IntDict.remove 0 ctx.outgoing }
            )
        )
        graph


graphvizDotNotation : Graph NodeMetadata () -> String
graphvizDotNotation =
    let
        styles : DOT.Styles
        styles =
            { rankdir = DOT.LR
            , graph = ""
            , node = "style=\"filled\""
            , edge = ""
            }

        nodeAttrs { typ, name, labels } =
            Dict.fromList <|
                ( "label"
                , "\n" ++ name ++ "\n" ++ String.join "\n" labels
                )
                    :: (case typ of
                            Job ->
                                [ ( "class", "job" )
                                , ( "shape", "rect" )
                                , ( "fillcolor", "chartreuse" )
                                ]

                            Resource ->
                                [ ( "class", "resource" )
                                , ( "shape", "ellipse" )
                                , ( "fillcolor", "deepskyblue" )
                                ]
                       )

        edgeAttrs _ =
            Dict.empty
    in
    DOT.outputWithStylesAndAttributes styles nodeAttrs edgeAttrs
