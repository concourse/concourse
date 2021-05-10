module Resource.Causality exposing
    ( Entity
    , NodeId
    , NodeMetadata
    , NodeType(..)
    , buildGraph
    , viewCausality
    )

import Color
import Concourse
import Force
import Graph exposing (Edge, Graph, Node, NodeContext, NodeId)
import Html
import TypedSvg exposing (circle, g, line, rect, svg, text_)
import TypedSvg.Attributes as Attrs exposing (fill, fontSize, pointerEvents, stroke, viewBox)
import TypedSvg.Attributes.InPx exposing (cx, cy, dx, dy, height, r, strokeWidth, width, x, x1, x2, y, y1, y2)
import TypedSvg.Core exposing (Svg, text)
import TypedSvg.Types as Types exposing (Paint(..))


type alias NodeId =
    Int


type alias NodeMetadata =
    { nodeType : NodeType
    , name : String
    , version : String
    }


type NodeType
    = BuildNode
    | ResourceVersionNode


type alias Entity =
    Force.Entity NodeId { value : NodeMetadata }


viewCausality : Graph Entity () -> Html.Html msg
viewCausality graph =
    let
        ( w, h ) =
            graphDimensions <| Graph.nodes graph
    in
    svg [ viewBox 0 0 w h ]
        [ Graph.edges graph
            |> List.map (linkElement graph)
            |> g [ Attrs.class [ "causality-links" ] ]
        , Graph.nodes graph
            |> List.map nodeElement
            |> g [ Attrs.class [ "causality-nodes" ] ]
        ]



-- linkELement and nodeElement converts graph data structures into html elements


linkElement : Graph Entity () -> Edge () -> Svg msg
linkElement graph edge =
    let
        emptyMetadata =
            { nodeType = ResourceVersionNode
            , name = ""
            , version = ""
            }

        source =
            Maybe.withDefault (Force.entity 0 emptyMetadata) <| Maybe.map (.node >> .label) <| Graph.get edge.from graph

        target =
            Maybe.withDefault (Force.entity 0 emptyMetadata) <| Maybe.map (.node >> .label) <| Graph.get edge.to graph
    in
    line
        [ strokeWidth 1
        , stroke <| Paint <| Color.rgb255 170 170 170
        , x1 source.x
        , y1 source.y
        , x2 target.x
        , y2 target.y
        ]
        []


nodeElement : { a | id : NodeId, label : { b | x : Float, y : Float, value : NodeMetadata } } -> Svg msg
nodeElement node =
    g [ Attrs.class [ "causality-node" ] ]
        [ rect
            [ case node.label.value.nodeType of
                BuildNode ->
                    fill <| Paint Color.lightGreen

                ResourceVersionNode ->
                    fill <| Paint Color.lightBlue
            , stroke <| Paint <| Color.rgba 0 0 0 0
            , strokeWidth 7

            -- , r 30
            -- , cx node.label.x
            -- , cy node.label.y
            , x <| node.label.x - 30
            , y <| node.label.y - 12
            , width 60
            , height 25
            ]
            [ TypedSvg.title [] [ TypedSvg.Core.text node.label.value.name ] ]
        , -- apparently svg doesn't have a nice way of rendering newline in text
          text_
            [ dx <| node.label.x
            , dy <| node.label.y - 5
            , Attrs.alignmentBaseline Types.AlignmentMiddle
            , Attrs.textAnchor Types.AnchorMiddle
            , fontSize <| Types.Px 9
            , fill (Paint Color.black)
            , pointerEvents "none"
            ]
            [ text node.label.value.name ]
        , text_
            [ dx <| node.label.x
            , dy <| node.label.y + 5
            , Attrs.alignmentBaseline Types.AlignmentMiddle
            , Attrs.textAnchor Types.AnchorMiddle
            , fontSize <| Types.Px 9
            , fill (Paint Color.black)
            , pointerEvents "none"
            ]
            [ text node.label.value.version ]
        ]


graphDimensions : List (Node Entity) -> ( Float, Float )
graphDimensions nodes =
    let
        x =
            toFloat <| List.length nodes

        width =
            round <| (x * 10.0) + 300
    in
    ( toFloat width, toFloat <| round <| toFloat width * 2 / 3 )



-- set of mutually recursive functions to construct the graph from the tree
-- using DFS. this constructor record is used mainly to keep track of the
-- incrementing nodeId. It seems to really dislike it if the nodeId isn't
-- sequential and/or is negative


type alias GraphConstructor =
    { nodes : List (Node NodeMetadata)
    , edges : List (Edge ())
    , maxId : Int
    }


constructResourceVersion : NodeId -> NodeId -> Concourse.CausalityResourceVersion -> GraphConstructor
constructResourceVersion parentId maxId rv =
    let
        nodeId =
            maxId + 1

        updater : Concourse.CausalityBuild -> GraphConstructor -> GraphConstructor
        updater child acc =
            let
                result =
                    constructBuild nodeId acc.maxId child
            in
            { nodes = acc.nodes ++ result.nodes
            , edges = acc.edges ++ result.edges
            , maxId = result.maxId
            }
    in
    List.foldl
        updater
        { nodes = [ Node nodeId { nodeType = ResourceVersionNode, name = rv.resourceName, version = rv.version } ]
        , edges =
            if parentId > 0 then
                [ Edge parentId nodeId () ]

            else
                []
        , maxId = nodeId
        }
        rv.inputTo


constructBuild : NodeId -> NodeId -> Concourse.CausalityBuild -> GraphConstructor
constructBuild parentId maxId (Concourse.CausalityBuildVariant b) =
    let
        nodeId =
            maxId + 1

        updater : Concourse.CausalityResourceVersion -> GraphConstructor -> GraphConstructor
        updater child acc =
            let
                result =
                    constructResourceVersion nodeId acc.maxId child
            in
            { nodes = acc.nodes ++ result.nodes
            , edges = acc.edges ++ result.edges
            , maxId = result.maxId
            }
    in
    List.foldl
        updater
        { nodes = [ Node nodeId { nodeType = BuildNode, name = b.jobName, version = "#" ++ b.name } ]
        , edges = [ Edge parentId nodeId () ]
        , maxId = nodeId
        }
        b.outputs


initializeNode : NodeContext NodeMetadata () -> NodeContext Entity ()
initializeNode ctx =
    { node =
        { label = Force.entity ctx.node.id ctx.node.label
        , id = ctx.node.id
        }
    , incoming = ctx.incoming
    , outgoing = ctx.outgoing
    }


buildGraph : Concourse.CausalityResourceVersion -> Graph Entity ()
buildGraph rv =
    let
        { nodes, edges } =
            constructResourceVersion 0 0 rv

        graphData =
            Graph.fromNodesAndEdges nodes edges

        initialGraph =
            Graph.mapContexts initializeNode graphData

        ( w, h ) =
            graphDimensions <| Graph.nodes initialGraph

        link { from, to } =
            ( from, to )

        forces =
            [ Force.links <| List.map link <| Graph.edges initialGraph
            , Force.manyBody <| List.map .id <| Graph.nodes initialGraph
            , Force.center (w / 2) (h / 2)
            ]

        simulation : List Entity
        simulation =
            Force.computeSimulation (Force.simulation forces) <| List.map .label <| Graph.nodes initialGraph

        updateNode : Float -> Float -> NodeContext Entity () -> NodeContext Entity ()
        updateNode x y nodeEntity =
            let
                node =
                    nodeEntity.node

                entity =
                    node.label
            in
            { nodeEntity | node = { node | label = { entity | x = x, y = y } } }

        updateEntity : Entity -> Graph Entity () -> Graph Entity ()
        updateEntity { id, x, y } graph =
            Graph.update id (Maybe.map <| updateNode x y) graph
    in
    List.foldl updateEntity initialGraph simulation
