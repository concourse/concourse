module TopologicalSort exposing (Digraph, flattenToLayers, tsort)

import List.Extra exposing (find)



-- we use Tarjan's Algorithm
-- https://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm


type alias Digraph a =
    List ( a, List a )


type alias InternalState a =
    { index : Int
    , stack : List a
    , indices : a -> Maybe Int
    , lowlinks : a -> Int
    , sccs : List (List a)
    }


tsort : Digraph a -> List (List a)
tsort graph =
    let
        strongconnect : InternalState a -> ( a, List a ) -> InternalState a
        strongconnect { index, stack, indices, lowlinks, sccs } ( v, children ) =
            let
                newState : InternalState a
                newState =
                    { index = index + 1
                    , stack = v :: stack
                    , indices = extendFun indices v (Just index)
                    , lowlinks = extendFun lowlinks v index
                    , sccs = sccs
                    }

                foldConnect : a -> InternalState a -> InternalState a
                foldConnect w state =
                    case state.indices w of
                        Nothing ->
                            let
                                newState2 =
                                    strongconnect state ( w, getChildren graph w )

                                newVLowlink : Int
                                newVLowlink =
                                    min (newState2.lowlinks v) (newState2.lowlinks w)
                            in
                            { newState2 | lowlinks = extendFun newState2.lowlinks v newVLowlink }

                        Just wIndex ->
                            if List.member w state.stack then
                                let
                                    newVLowlink : Int
                                    newVLowlink =
                                        min (state.lowlinks v) wIndex
                                in
                                { state | lowlinks = extendFun state.lowlinks v newVLowlink }

                            else
                                state

                newerState =
                    List.foldr foldConnect newState children
            in
            if Just (newerState.lowlinks v) == newerState.indices v then
                let
                    ( component, newStack ) =
                        takeUpTo v newerState.stack
                in
                { newerState | stack = newStack, sccs = newerState.sccs ++ [ component ] }

            else
                newerState

        foldGraph : ( a, List a ) -> InternalState a -> InternalState a
        foldGraph ( v, children ) state =
            if state.indices v == Nothing then
                strongconnect state ( v, children )

            else
                state

        initialState : InternalState a
        initialState =
            { index = 0
            , stack = []
            , indices = always Nothing
            , lowlinks = always 1000000
            , sccs = []
            }
    in
    (List.foldr foldGraph initialState graph).sccs



-- we now need to flatten the strongly-connected components into "layers", which should be much easier now that there are no loops


flattenToLayers : Digraph a -> List (List a)
flattenToLayers graph =
    let
        depths : a -> Maybe Int
        depths =
            flattenToLayers_ graph (tsort graph) (always Nothing)
    in
    flattenMap (List.map Tuple.first graph) depths 0


flattenToLayers_ : Digraph a -> List (List a) -> (a -> Maybe Int) -> (a -> Maybe Int)
flattenToLayers_ graph stronglyConnectedComponents depths =
    case stronglyConnectedComponents of
        [] ->
            depths

        scc :: sccs ->
            let
                children : List a
                children =
                    scc
                        |> List.concatMap (getChildren graph)
                        |> List.filter (\x -> not (List.member x scc))

                childDepths : Maybe (List Int)
                childDepths =
                    List.map depths children
                        |> allDefined
            in
            case childDepths of
                Nothing ->
                    -- "same size" recursion is safe here, because the tsort ensures we should never hit this case
                    -- (even if they weren't in order, we should always have at least one scc that depends only on previously covered sccs)
                    flattenToLayers_ graph (sccs ++ [ scc ]) depths

                Just cds ->
                    let
                        depth : Maybe Int
                        depth =
                            cds
                                |> List.maximum
                                |> Maybe.map ((+) 1)
                                |> Maybe.withDefault 0
                                |> Just
                    in
                    flattenToLayers_ graph sccs (extendFunMany depths scc depth)



-- helper functions


extendFun : (a -> b) -> a -> b -> (a -> b)
extendFun f a b =
    \x ->
        if x == a then
            b

        else
            f x


extendFunMany : (a -> b) -> List a -> b -> (a -> b)
extendFunMany f xs b =
    \x ->
        if List.member x xs then
            b

        else
            f x


getChildren : Digraph a -> a -> List a
getChildren graph v =
    case find (\( n, _ ) -> n == v) graph of
        Just ( _, children ) ->
            children

        Nothing ->
            -- impossible - each node should have an entry in the children list
            []


takeUpTo : a -> List a -> ( List a, List a )
takeUpTo t ts =
    case ts of
        [] ->
            ( [], [] )

        x :: xs ->
            if t == x then
                ( [ x ], xs )

            else
                let
                    ( init, end ) =
                        takeUpTo t xs
                in
                ( x :: init, end )


allDefined : List (Maybe a) -> Maybe (List a)
allDefined xs =
    case xs of
        [] ->
            Just []

        Nothing :: _ ->
            Nothing

        (Just a) :: ys ->
            case allDefined ys of
                Nothing ->
                    Nothing

                Just zs ->
                    Just (a :: zs)



-- this assumes that every member of xs is in the domain of f
-- For our usecase (flattenToLayers of a tsorted graph) this will be true


flattenMap : List a -> (a -> Maybe Int) -> Int -> List (List a)
flattenMap xs f n =
    if xs == [] then
        []

    else
        List.filter (\x -> f x == Just n) xs :: flattenMap (List.filter (\x -> f x /= Just n) xs) f (n + 1)
