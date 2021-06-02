module Causality.Causality exposing
    ( Model
    , NodeType(..)
    , changeToVersionedResource
    , constructGraph
    , documentTitle
    , getUpdateMessage
    , graphvizDotNotation
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
        , id
        , style
        )
import Http
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
import SideBar.SideBar as SideBar
import Svg
import Svg.Attributes as SvgAttributes
import Tooltip
import UpdateMsg exposing (UpdateMsg)
import Views.Styles
import Views.TopBar as TopBar


type alias Model =
    Login.Model
        { versionId : Concourse.VersionedResourceIdentifier
        , direction : CausalityDirection
        , fetchedCausality : Maybe CausalityResourceVersion
        , graph : Graph NodeType ()
        , renderedJobs : Maybe (List Concourse.Job)
        , renderedBuilds : Maybe (List Concourse.Build)
        , renderedResources : Maybe (List Concourse.Resource)
        , renderedResourceVersions : Maybe (List Concourse.VersionedResource)
        , pageStatus : Result () ()
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
      , pageStatus = Ok ()
      }
    , [ FetchAllPipelines
      , fetchCausality
      ]
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
        CausalityFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, effects ++ [ RedirectToLogin ] )

                    else if status.code == 404 then
                        ( { model | pageStatus = Err () }, effects )

                    else
                        ( model, effects )

                _ ->
                    ( model, effects )

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


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    case model.pageStatus of
        Err () ->
            UpdateMsg.NotFound

        Ok () ->
            UpdateMsg.AOK


view : Session -> Model -> Html Message
view session model =
    let
        route =
            Routes.Causality
                { id = model.versionId
                , direction = model.direction
                , version = Maybe.map .version model.fetchedCausality
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



-- type alias NodeType =
--     { typ : NodeType
--     , name : String
--     , labels : List String
--     }


type NodeType
    = Job String (List String)
    | Resource String (List Concourse.Version)


insert : List a -> a -> List a
insert lst e =
    if List.member e lst then
        lst

    else
        lst ++ [ e ]


constructResourceVersion : NodeId -> CausalityDirection -> CausalityResourceVersion -> Graph NodeType () -> Graph NodeType ()
constructResourceVersion parentId dir rv graph =
    let
        -- NodeId is the (positive) versionId for resourceVersions and the (negative) buildId for builds
        nodeId =
            rv.resourceId

        childEdges =
            IntDict.fromList <| List.map (\(CausalityBuildVariant b) -> ( -b.jobId, () )) rv.builds

        addEdge : NodeContext NodeType () -> NodeContext NodeType ()
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

        updateNode : Maybe (NodeContext NodeType ()) -> Maybe (NodeContext NodeType ())
        updateNode nodeContext =
            Just <|
                case nodeContext of
                    Just { node, incoming, outgoing } ->
                        let
                            oldNode =
                                node.label

                            newNodeType : NodeType
                            newNodeType =
                                case oldNode of
                                    Resource name versions ->
                                        Resource name <| insert versions rv.version

                                    _ ->
                                        oldNode
                        in
                        addEdge
                            { node =
                                { node | label = newNodeType }
                            , incoming = incoming
                            , outgoing = outgoing
                            }

                    Nothing ->
                        addEdge
                            { node =
                                { id = nodeId
                                , label = Resource rv.resourceName [ rv.version ]
                                }
                            , incoming = IntDict.empty
                            , outgoing = IntDict.empty
                            }

        updatedGraph =
            Graph.update nodeId updateNode graph
    in
    List.foldl (\build acc -> constructBuild nodeId dir build acc) updatedGraph rv.builds


constructBuild : NodeId -> CausalityDirection -> CausalityBuild -> Graph NodeType () -> Graph NodeType ()
constructBuild parentId dir (CausalityBuildVariant b) graph =
    let
        -- NodeId is the (positive) resourceId for resourceVersions and the (negative) jobId for builds
        nodeId =
            -b.jobId

        childEdges =
            IntDict.fromList <| List.map (\rv -> ( rv.resourceId, () )) b.resourceVersions

        addEdge : NodeContext NodeType () -> NodeContext NodeType ()
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

        updateNode : Maybe (NodeContext NodeType ()) -> Maybe (NodeContext NodeType ())
        updateNode nodeContext =
            Just <|
                case nodeContext of
                    Just { node, incoming, outgoing } ->
                        let
                            oldNode =
                                node.label

                            newNodeType : NodeType
                            newNodeType =
                                case oldNode of
                                    Job name builds ->
                                        Job name <| insert builds buildName

                                    _ ->
                                        oldNode
                        in
                        addEdge
                            { node =
                                { node | label = newNodeType }
                            , incoming = incoming
                            , outgoing = outgoing
                            }

                    Nothing ->
                        addEdge
                            { node =
                                { id = nodeId
                                , label = Job b.jobName [ buildName ]
                                }
                            , incoming = IntDict.empty
                            , outgoing = IntDict.empty
                            }

        updatedGraph =
            Graph.update nodeId updateNode graph
    in
    List.foldl (\build acc -> constructResourceVersion nodeId dir build acc) updatedGraph b.resourceVersions


constructGraph : CausalityDirection -> CausalityResourceVersion -> Graph NodeType ()
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


graphvizDotNotation : Graph NodeType () -> String
graphvizDotNotation =
    let
        styles : DOT.Styles
        styles =
            { rankdir = DOT.LR
            , graph = ""
            , node = "style=\"filled\""
            , edge = ""
            }

        nodeAttrs typ =
            Dict.fromList <|
                case typ of
                    Job name builds ->
                        [ ( "class", "job" )
                        , ( "shape", "rect" )
                        , ( "label", "<B>" ++ name ++ String.join " " builds ++ "</B>" )
                        ]

                    Resource name versions ->
                        [ ( "class", "resource" )
                        , ( "shape", "ellipse" )
                        , ( "label", name ++ "\n" ++ (String.join "\n" <| List.map (\v -> String.join "," <| Concourse.versionQuery v) versions) )
                        ]

        edgeAttrs _ =
            Dict.empty
    in
    DOT.outputWithStylesAndAttributes styles nodeAttrs edgeAttrs
