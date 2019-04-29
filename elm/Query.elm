module Query exposing (Result, matchWords)

import Dict exposing (Dict)
import Maybe.Extra as ME


type alias Result =
    List Match


type alias Match =
    ( Int, Int )


matchWords : String -> String -> Maybe Result
matchWords needle haystack =
    let
        lns =
            String.words (String.toLower needle)

        lh =
            String.toLower haystack

        matches =
            List.map (wordMatches lh) lns
    in
    if List.any List.isEmpty matches then
        Nothing

    else
        matches
            |> List.concat
            |> List.sortWith largestMatchFirst
            |> List.foldl simplifyResult ( [], 0 )
            |> Tuple.first
            |> Just


wordMatches : String -> String -> Result
wordMatches lowerHaystack lowerNeedle =
    let
        l =
            String.length lowerNeedle
    in
    String.indexes lowerNeedle lowerHaystack
        |> List.map (\i -> ( i, l ))


largestMatchFirst : Match -> Match -> Order
largestMatchFirst ( xi, xl ) ( yi, yl ) =
    if xi == yi then
        compare yl xl

    else
        compare xi yi


simplifyResult : Match -> ( Result, Int ) -> ( Result, Int )
simplifyResult ( i, l ) ( ms, o ) =
    if i + l <= o then
        ( ms, o )

    else if i < o then
        ( ms ++ [ ( o, l - (o - i) ) ], o + (l - (o - i)) )

    else
        ( ms ++ [ ( i, l ) ], i + l )
