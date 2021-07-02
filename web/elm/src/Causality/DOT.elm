{-
   Vendered from https://github.com/elm-community/graph/blob/6.0.0/src/Graph/DOT.elm
-}


module Causality.DOT exposing
    ( Styles, Rankdir(..), outputWithStylesAndAttributes
    , Attr(..)
    )

{-| This module provides a means of converting the `Graph` data type into a
valid [DOT](https://en.wikipedia.org/wiki/DOT_(graph_description_language))
string for visualizing your graph structure.
You can easily preview your graph by inserting the generated string into an
online GraphViz tool like <https://dreampuf.github.io/GraphvizOnline/>.
You can also dynamically draw your graph in your application by sending the
string over a port to the javascript version of the GraphViz library,
<https://github.com/mdaines/viz.js/> (see the examples there fore more
specifics on how to embed the generated visualization).

@docs output


# Attrs

GraphViz allows for customizing the graph's look via "Attrs."

@docs Styles, Rankdir, defaultStyles, outputWithStyles, outputWithStylesAndAttributes

-}

import Dict exposing (Dict)
import Graph exposing (Graph)
import Json.Encode


{-| A type representing the attrs to apply at the graph, node, and edge
entities (subgraphs and cluster subgraphs are not supported).
Note that `Styles` is made up of strings, which loses type safety, but
allows you to use any GraphViz attrs without having to model them out in
entirety in this module. It is up to you to make sure you provide valid
attr strings. See <http://www.graphviz.org/content/attrs> for available
options.
-}
type alias Styles =
    { rankdir : Rankdir
    , graph : String
    , node : String
    , edge : String
    }


{-| Values to control the direction of the graph
-}
type Rankdir
    = TB
    | LR
    | BT
    | RL


type Attr
    = EscString String
    | HtmlLabel String


{-| Same as `outputWithStyles`, but allows each node and edge to include its
own attrs. Note that you must supply a conversion function for node and edge
labels that return a `Dict String String` of the attribute mappings.
Note that you have to take care of setting the appropriate node and edge labels
yourself.
-}
outputWithStylesAndAttributes :
    Styles
    -> (n -> Dict String Attr)
    -> (e -> Dict String Attr)
    -> Graph n e
    -> String
outputWithStylesAndAttributes styles nodeAttrs edgeAttrs graph =
    let
        encode : Attr -> String
        encode attr =
            case attr of
                EscString s ->
                    Json.Encode.string s
                        |> Json.Encode.encode 0

                HtmlLabel h ->
                    "<" ++ h ++ ">"

        attrAssocs : Dict String Attr -> String
        attrAssocs =
            Dict.toList
                >> List.map (\( k, v ) -> k ++ "=" ++ encode v)
                >> String.join ", "

        makeAttrs : Dict String Attr -> String
        makeAttrs d =
            if Dict.isEmpty d then
                ""

            else
                " [" ++ attrAssocs d ++ "]"

        edges =
            let
                compareEdge a b =
                    case compare a.from b.from of
                        LT ->
                            LT

                        GT ->
                            GT

                        EQ ->
                            compare a.to b.to
            in
            Graph.edges graph
                |> List.sortWith compareEdge

        nodes =
            Graph.nodes graph

        edgesString =
            List.map edge edges
                |> String.join "\n"

        edge e =
            "  "
                ++ String.fromInt e.from
                ++ " -> "
                ++ String.fromInt e.to
                ++ makeAttrs (edgeAttrs e.label)

        nodesString =
            List.map node nodes
                |> String.join "\n"

        node n =
            "  "
                ++ String.fromInt n.id
                ++ makeAttrs (nodeAttrs n.label)

        rankDirToString r =
            case r of
                TB ->
                    "TB"

                LR ->
                    "LR"

                BT ->
                    "BT"

                RL ->
                    "RL"
    in
    String.join "\n"
        [ "digraph G {"
        , "  rankdir=" ++ rankDirToString styles.rankdir
        , "  graph [" ++ styles.graph ++ "]"
        , "  node [" ++ styles.node ++ "]"
        , "  edge [" ++ styles.edge ++ "]"
        , ""
        , edgesString
        , ""
        , nodesString
        , "}"
        ]
