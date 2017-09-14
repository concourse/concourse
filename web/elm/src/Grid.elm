module Grid exposing (Grid(..), insert, fromGraph, MatrixCell(..), toMatrix, width, height)

import Graph
import IntDict
import Matrix exposing (Matrix)
import Set exposing (Set)
import String


type Grid n e
    = Cell (Graph.NodeContext n e)
    | Serial (Grid n e) (Grid n e)
    | Parallel (List (Grid n e))
    | End


fromGraph : Graph.Graph n e -> Grid n e
fromGraph graph =
    List.foldl insert End <|
        List.concat (Graph.heightLevels graph)


type alias HeightFunc n e =
    Graph.NodeContext n e -> Int


type MatrixCell n e
    = MatrixNode (Graph.NodeContext n e)
    | MatrixSpacer
    | MatrixFilled


toMatrix : HeightFunc n e -> Grid n e -> Matrix (MatrixCell n e)
toMatrix nh grid =
    toMatrix_ nh 0 0 (Matrix.matrix (height nh grid) (width grid) (always MatrixSpacer)) grid


toMatrix_ : HeightFunc n e -> Int -> Int -> Matrix (MatrixCell n e) -> Grid n e -> Matrix (MatrixCell n e)
toMatrix_ nh row col matrix grid =
    case grid of
        End ->
            matrix

        Serial a b ->
            toMatrix_ nh row (col + width a) (toMatrix_ nh row col matrix a) b

        Parallel grids ->
            Tuple.first <| List.foldl (\g ( m, row_ ) -> ( toMatrix_ nh row_ col m g, row_ + height nh g )) ( matrix, row ) grids

        Cell nc ->
            Matrix.set ( row, col ) (MatrixNode nc) (clearHeight row col (nh nc - 1) matrix)


showMatrix : Matrix (MatrixCell n e) -> String
showMatrix m =
    let
        showCell c =
            case c of
                MatrixSpacer ->
                    "  "

                MatrixFilled ->
                    "--"

                MatrixNode nc ->
                    if nc.node.id < 10 then
                        " " ++ toString nc.node.id
                    else
                        toString nc.node.id
    in
        Matrix.toList m
            |> List.map (\r -> String.join "|" (List.map showCell r))
            |> String.join "\n"


clearHeight : Int -> Int -> Int -> Matrix (MatrixCell n e) -> Matrix (MatrixCell n e)
clearHeight row col height matrix =
    if height == 0 then
        matrix
    else
        clearHeight row col (height - 1) (Matrix.set ( row + height, col ) MatrixFilled matrix)


width : Grid n e -> Int
width grid =
    case grid of
        End ->
            0

        Serial a b ->
            width a + width b

        Parallel grids ->
            Maybe.withDefault 0 (List.maximum (List.map width grids))

        Cell _ ->
            1


height : HeightFunc n e -> Grid n e -> Int
height nh grid =
    case grid of
        End ->
            0

        Serial a b ->
            max (height nh a) (height nh b)

        Parallel grids ->
            List.sum (List.map (height nh) grids)

        Cell nc ->
            nh nc


insert : Graph.NodeContext n e -> Grid n e -> Grid n e
insert nc grid =
    case IntDict.size nc.incoming of
        0 ->
            addToStart (Cell nc) grid

        _ ->
            addAfterUpstreams nc grid


addToStart : Grid n e -> Grid n e -> Grid n e
addToStart a b =
    case b of
        End ->
            a

        Parallel bs ->
            case a of
                Parallel as_ ->
                    Parallel (bs ++ as_)

                _ ->
                    Parallel (bs ++ [ a ])

        _ ->
            case a of
                Parallel as_ ->
                    Parallel (b :: as_)

                _ ->
                    Parallel [ b, a ]


addAfterUpstreams : Graph.NodeContext n e -> Grid n e -> Grid n e
addAfterUpstreams nc grid =
    case grid of
        End ->
            End

        Parallel grids ->
            let
                ( dependent, rest ) =
                    List.partition (leadsTo nc) grids
            in
                case dependent of
                    [] ->
                        grid

                    [ singlePath ] ->
                        Parallel (addAfterUpstreams nc singlePath :: rest)

                    _ ->
                        addToStart
                            (Parallel rest)
                            (addAfterMixedUpstreamsAndReinsertExclusiveOnes nc dependent)

        Serial a b ->
            if leadsTo nc a then
                Serial a (addToStart (Cell nc) b)
            else
                Serial a (addAfterUpstreams nc b)

        Cell upstreamOrUnrelated ->
            if IntDict.member nc.node.id upstreamOrUnrelated.outgoing then
                Serial grid (Cell nc)
            else
                grid


addAfterMixedUpstreamsAndReinsertExclusiveOnes : Graph.NodeContext n e -> List (Grid n e) -> Grid n e
addAfterMixedUpstreamsAndReinsertExclusiveOnes nc dependent =
    let
        ( remainder, exclusives ) =
            extractExclusiveUpstreams nc (Parallel dependent)
    in
        case ( remainder, exclusives ) of
            ( Nothing, [] ) ->
                Debug.crash "impossible"

            ( Nothing, _ ) ->
                Serial (Parallel exclusives) (Cell nc)

            ( Just rem, [] ) ->
                Serial (Parallel dependent) (Cell nc)

            ( Just rem, _ ) ->
                List.foldr
                    checkAndAddBeforeDownstream
                    (addAfterUpstreams nc rem)
                    exclusives


checkAndAddBeforeDownstream : Grid n e -> Grid n e -> Grid n e
checkAndAddBeforeDownstream up grid =
    let
        after =
            addBeforeDownstream up grid
    in
        if after == grid then
            Debug.crash ("failed to add: " ++ toString up)
        else
            after


addBeforeDownstream : Grid n e -> Grid n e -> Grid n e
addBeforeDownstream up grid =
    case grid of
        End ->
            End

        Parallel grids ->
            if comesDirectlyFrom up grid then
                Debug.crash "too late to add in front of Parallel"
            else
                Parallel (List.map (addBeforeDownstream up) grids)

        Serial a b ->
            if comesDirectlyFrom up a then
                Debug.crash "too late to add in front of Serial"
            else if comesDirectlyFrom up b then
                Serial (addToStart up a) b
            else
                Serial a (addBeforeDownstream up b)

        Cell upstreamOrUnrelated ->
            if comesDirectlyFrom up grid then
                Debug.crash "too late to add in front of Cell"
            else
                grid


leadsTo : Graph.NodeContext n e -> Grid n e -> Bool
leadsTo nc grid =
    case grid of
        End ->
            False

        Parallel grids ->
            List.any (leadsTo nc) grids

        Serial a b ->
            leadsTo nc a || leadsTo nc b

        Cell upstreamOrUnrelated ->
            IntDict.member nc.node.id upstreamOrUnrelated.outgoing


comesDirectlyFrom : Grid n e -> Grid n e -> Bool
comesDirectlyFrom up grid =
    case grid of
        End ->
            False

        Parallel grids ->
            List.any (comesDirectlyFrom up) grids

        Serial a _ ->
            comesDirectlyFrom up a

        Cell nc ->
            Set.member nc.node.id (terminals up)


terminals : Grid n e -> Set Int
terminals grid =
    case grid of
        End ->
            Set.empty

        Parallel grids ->
            if List.any (Set.isEmpty << terminals) grids then
                -- this is kind of a hack, but without it, if inserting the last node
                -- of a matrix of pipelines, the entire matrix is considered to be
                -- "exclusive" to that node, and so the last node ends up being placed
                -- after the entire matrix if it has any other "truly" exclusive nodes
                -- (i.e. an unconstrained input)
                Set.empty
            else
                List.foldl (\g s -> Set.union s (terminals g)) Set.empty grids

        Serial a b ->
            let
                aTerms =
                    terminals a

                bTerms =
                    terminals b

                joined =
                    Set.union aTerms bTerms

                bNodes =
                    nodes b
            in
                Set.diff joined bNodes

        Cell nc ->
            Set.fromList (IntDict.keys nc.outgoing)


nodes : Grid n e -> Set Int
nodes grid =
    case grid of
        End ->
            Set.empty

        Parallel grids ->
            List.foldl (\g s -> Set.union s (nodes g)) Set.empty grids

        Serial a b ->
            Set.union (nodes a) (nodes b)

        Cell nc ->
            Set.singleton nc.node.id


extractExclusiveUpstreams : Graph.NodeContext n e -> Grid n e -> ( Maybe (Grid n e), List (Grid n e) )
extractExclusiveUpstreams target grid =
    case grid of
        End ->
            ( Just grid, [] )

        Parallel grids ->
            let
                recurse =
                    List.map (extractExclusiveUpstreams target) grids

                remainders =
                    List.map Tuple.first recurse

                exclusives =
                    List.concatMap Tuple.second recurse
            in
                if List.all ((==) Nothing) remainders then
                    ( Nothing, exclusives )
                else
                    ( Just (Parallel <| List.filterMap identity remainders), exclusives )

        Serial a b ->
            let
                terms =
                    terminals grid
            in
                if Set.size terms == 1 && Set.member target.node.id terms then
                    ( Nothing, [ grid ] )
                else
                    ( Just grid, [] )

        Cell source ->
            if IntDict.size source.outgoing == 1 && IntDict.member target.node.id source.outgoing then
                ( Nothing, [ grid ] )
            else
                ( Just grid, [] )
