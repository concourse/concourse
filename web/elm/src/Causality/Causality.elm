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
import Dict
import EffectTransformer exposing (ET)
import Graph exposing (Edge, Graph, Node, NodeId)
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
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects as Effects exposing (Effect(..), toHtmlID)
import Message.Message as Message exposing (DomID(..), Message(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Routes
import SideBar.SideBar as SideBar exposing (byPipelineId, lookupPipeline)
import Tooltip
import Views.Styles
import Views.TopBar as TopBar


type alias Model =
    Login.Model
        { versionId : Concourse.VersionedResourceIdentifier
        , direction : Concourse.CausalityDirection
        , fetchedCausality : Maybe Concourse.CausalityResourceVersion
        , graph : Graph NodeMetadata ()
        , renderedJobs : Maybe (List Concourse.Job)
        , renderedBuilds : Maybe (List Concourse.Build)
        , renderedResources : Maybe (List Concourse.Resource)
        , renderedResourceVersions : Maybe (List Concourse.VersionedResource)
        }


type alias Flags =
    { versionId : Concourse.VersionedResourceIdentifier
    , direction : Concourse.CausalityDirection
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
        CausalityFetched (Ok ( _, crv )) ->
            let
                graph =
                    case crv of
                        Just rv ->
                            constructGraph rv

                        _ ->
                            model.graph
            in
            ( { model
                | fetchedCausality = crv
                , graph = graph
              }
            , effects
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
            , Html.div
                []
                [ renderGraph model.graph ]
            ]
        ]


type alias NodeId =
    Int


type alias NodeMetadata =
    { nodeType : NodeType
    }


type NodeType
    = BuildNode String String
    | ResourceVersionNode String Concourse.Version


type alias GraphConstructor =
    { nodes : List (Node NodeMetadata)
    , edges : List (Edge ())
    }


constructResourceVersion : NodeId -> Bool -> Concourse.CausalityResourceVersion -> GraphConstructor
constructResourceVersion parentId downstream rv =
    let
        nodeId =
            rv.versionId

        edge =
            if downstream then
                Edge parentId nodeId ()

            else
                Edge nodeId parentId ()

        recurseChild : Bool -> Concourse.CausalityBuild -> GraphConstructor -> GraphConstructor
        recurseChild d child acc =
            let
                result =
                    constructBuild nodeId d child
            in
            { nodes = acc.nodes ++ result.nodes
            , edges = acc.edges ++ result.edges
            }

        children =
            List.foldl (recurseChild True) { nodes = [], edges = [] } rv.builds
    in
    { nodes =
        Node nodeId { nodeType = ResourceVersionNode rv.resourceName rv.version }
            :: children.nodes
    , edges =
        edge
            :: children.edges
    }


constructBuild : NodeId -> Bool -> Concourse.CausalityBuild -> GraphConstructor
constructBuild parentId downstream (Concourse.CausalityBuildVariant b) =
    let
        nodeId =
            -b.id

        edge =
            if downstream then
                Edge parentId nodeId ()

            else
                Edge nodeId parentId ()

        recurseChild : Bool -> Concourse.CausalityResourceVersion -> GraphConstructor -> GraphConstructor
        recurseChild d child acc =
            let
                result =
                    constructResourceVersion nodeId d child
            in
            { nodes = acc.nodes ++ result.nodes
            , edges = acc.edges ++ result.edges
            }

        children =
            List.foldl (recurseChild True) { nodes = [], edges = [] } b.resourceVersions
    in
    { nodes =
        Node nodeId { nodeType = BuildNode b.jobName b.name }
            :: children.nodes
    , edges =
        edge
            :: children.edges
    }


constructGraph : Concourse.CausalityResourceVersion -> Graph NodeMetadata ()
constructGraph rv =
    let
        { nodes, edges } =
            constructResourceVersion 0 True rv

        -- because the first node is constructed with fictious parentId 0, the first edge needs to be removed
        trimmedEdges =
            case edges of
                x :: xs ->
                    xs

                [] ->
                    []
    in
    Graph.fromNodesAndEdges nodes trimmedEdges


renderGraph : Graph NodeMetadata () -> Html.Html msg
renderGraph graph =
    let
        styles : DOT.Styles
        styles =
            { rankdir = DOT.TB
            , graph = ""
            , node = "shape=box, style=\"filled\""
            , edge = ""
            }

        versionString =
            Dict.foldl (\k v acc -> k ++ ":" ++ v ++ "\n" ++ acc) ""

        nodeAttrs { nodeType } =
            Dict.fromList <|
                case nodeType of
                    BuildNode jobName buildName ->
                        [ ( "label", jobName ++ "\n" ++ "#" ++ buildName )
                        , ( "fillcolor", "chartreuse" )
                        ]

                    ResourceVersionNode resourceName version ->
                        [ ( "label", resourceName ++ "\n" ++ versionString version )
                        , ( "fillcolor", "deepskyblue" )
                        ]

        edgeAttrs _ =
            Dict.empty
    in
    Html.div
        []
        [ Html.text <| DOT.outputWithStylesAndAttributes styles nodeAttrs edgeAttrs graph ]
